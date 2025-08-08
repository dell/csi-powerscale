/*
Copyright (c) 2019-2025 Dell Inc, or its subsidiaries.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	_ "net/http/pprof" // #nosec G108
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/akutz/gournal"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/cucumber/godog"
	"github.com/dell/csi-isilon/v2/common/constants"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/spf13/viper"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestMain(m *testing.M) {
	// Set required environment variables for Kubernetes client
	os.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	os.Setenv("KUBERNETES_SERVICE_PORT", "6443")
	defer func() {
		os.Unsetenv("KUBERNETES_SERVICE_HOST")
		os.Unsetenv("KUBERNETES_SERVICE_PORT")
	}()

	status := 0

	go http.ListenAndServe("localhost:6060", nil) // #nosec G114
	fmt.Printf("starting godog...\n")

	configFile := "mock/secret/secret.yaml"
	os.Setenv(constants.EnvIsilonConfigFile, configFile)

	opts := godog.Options{
		Format: "pretty",
		Paths:  []string{"features"},
		Tags:   "~todo",
	}

	status = godog.TestSuite{
		Name:                "godogs",
		ScenarioInitializer: FeatureContext,
		Options:             &opts,
	}.Run()

	fmt.Printf("godog finished\n")

	if st := m.Run(); st > status {
		status = st
	}

	fmt.Printf("status %d\n", status)

	os.Exit(status)
}

func TestGetLoggerfunc(t *testing.T) {
	ctx, _ := GetLogger(nil)
	assert.Equal(t, nil, ctx)
}

func TestGetRunIDLogfunc(t *testing.T) {
	ctx, _, _ := GetRunIDLog(nil)
	assert.Equal(t, nil, ctx)
}

func TestGetIsiPathForVolumeFromClusterConfig(t *testing.T) {
	o := Opts{
		Path: "path",
	}
	s := service{
		opts: o,
	}

	isilonConfig := IsilonClusterConfig{
		IsiPath: "",
	}
	path := s.getIsiPathForVolumeFromClusterConfig(&isilonConfig)
	assert.Equal(t, "path", path)

	isilonConfig = IsilonClusterConfig{
		IsiPath: "path/path",
	}
	path = s.getIsiPathForVolumeFromClusterConfig(&isilonConfig)
	assert.Equal(t, "path/path", path)
}

func TestGetCSINodeIP(t *testing.T) {
	s := service{
		nodeIP: "",
	}
	ctx := context.Background()
	_, err := s.GetCSINodeIP(ctx)
	assert.Equal(t, errors.New("cannot get node IP"), err)
}

func TestGetCSINodeID(t *testing.T) {
	s := service{
		nodeID: "",
	}
	ctx := context.Background()
	_, err := s.GetCSINodeID(ctx)
	assert.Equal(t, errors.New("cannot get node id"), err)
}

func TestGetIsilonClusterLength(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*sync.Map)
		expected  int
	}{
		{
			name: "Empty Map",
			setupFunc: func(_ *sync.Map) {
				// No setup, map remains empty
			},
			expected: 0,
		},
		{
			name: "Single Entry",
			setupFunc: func(m *sync.Map) {
				m.Store("cluster1", struct{}{})
			},
			expected: 1,
		},
		{
			name: "Multiple Entries",
			setupFunc: func(m *sync.Map) {
				m.Store("cluster1", struct{}{})
				m.Store("cluster2", struct{}{})
				m.Store("cluster3", struct{}{})
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serviceInstance := &service{
				isiClusters: &sync.Map{},
			}

			tt.setupFunc(serviceInstance.isiClusters)
			actual := serviceInstance.getIsilonClusterLength()

			if actual != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, actual)
			}
		})
	}
}

func TestGetNodeLabels(t *testing.T) {
	s := service{
		nodeID: "",
	}
	_, err := s.GetNodeLabels()
	assert.NotEqual(t, nil, err)
}

func TestGetNodeLabelsWithName(t *testing.T) {
	originalGetKubeClientSet := getKubeClientSet
	originalGetK8sNodeByName := getK8sNodeByName

	after := func() {
		getKubeClientSet = originalGetKubeClientSet
		getK8sNodeByName = originalGetK8sNodeByName
	}
	defer after()

	tests := []struct {
		name                string
		kubeConfigPath      string
		nodeName            string
		expectedLabels      map[string]string
		expectedErr         bool
		getKubeClientSetErr error
		getK8sNodeByNameErr error
	}{
		{
			name:                "Successful retrieval of node labels",
			kubeConfigPath:      "path/to/kube/config",
			nodeName:            "node-name",
			expectedLabels:      map[string]string{"label1": "value1", "label2": "value2"},
			expectedErr:         false,
			getKubeClientSetErr: nil,
			getK8sNodeByNameErr: nil,
		},
		{
			name:                "Error creating Kubernetes client set",
			kubeConfigPath:      "path/to/kube/config",
			nodeName:            "node-name",
			expectedLabels:      nil,
			expectedErr:         true,
			getKubeClientSetErr: fmt.Errorf("error creating client set"),
			getK8sNodeByNameErr: nil,
		},
		{
			name:                "Error getting node by name",
			kubeConfigPath:      "path/to/kube/config",
			nodeName:            "node-name",
			expectedLabels:      nil,
			expectedErr:         true,
			getKubeClientSetErr: nil,
			getK8sNodeByNameErr: fmt.Errorf("error getting node"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getKubeClientSet = func(_ string) (*kubernetes.Clientset, error) {
				return &kubernetes.Clientset{}, tt.getKubeClientSetErr
			}

			getK8sNodeByName = func(_ *kubernetes.Clientset, nodeName string) (*v1.Node, error) {
				node := &v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name:   nodeName,
						Labels: tt.expectedLabels,
					},
				}
				return node, tt.getK8sNodeByNameErr
			}

			s := &service{
				opts: Opts{KubeConfigPath: tt.kubeConfigPath},
			}

			labels, err := s.GetNodeLabelsWithName(tt.nodeName)
			if (err != nil) != tt.expectedErr {
				t.Errorf("GetNodeLabelsWithName() error = %v, wantErr %v", err, tt.expectedErr)
				return
			}

			if !reflect.DeepEqual(labels, tt.expectedLabels) {
				t.Errorf("GetNodeLabelsWithName() labels = %v, want %v", labels, tt.expectedLabels)
			}
		})
	}
}

func TestServiceInitializeServiceOpts(t *testing.T) {
	wantOps := Opts{
		Port:                     "8080",
		Path:                     "/ifs",
		IsiVolumePathPermissions: "0777",
		AccessZone:               "System",
		KubeConfigPath:           "/home/kubeconfig",
		replicationContextPrefix: "prefix/",
		replicationPrefix:        "prefix",
		IgnoreUnresolvableHosts:  false,
		csiVolPrefix:             "vol",
	}

	wantEnvNodeName := "node"
	wantEnvNodeIP := "10.0.0.1"
	wantEnvIsilonConfigFile := "X_CSI_ISI_CONFIG_PATH"

	os.Setenv(constants.EnvPort, wantOps.Port)
	os.Setenv(constants.EnvPath, "")
	os.Setenv(constants.EnvIsiVolumePathPermissions, "")
	os.Setenv(constants.EnvAccessZone, "")
	os.Setenv(constants.EnvNodeName, wantEnvNodeName)
	os.Setenv(constants.EnvNodeIP, wantEnvNodeIP)
	os.Setenv(constants.EnvKubeConfigPath, wantOps.KubeConfigPath)
	os.Setenv(constants.EnvIsilonConfigFile, wantEnvIsilonConfigFile)
	os.Setenv(constants.EnvReplicationContextPrefix, "prefix")
	os.Setenv(constants.EnvReplicationPrefix, wantOps.replicationPrefix)
	os.Setenv(constants.EnvCsiVolPrefix, "vol")

	defer func() {
		os.Unsetenv(constants.EnvPort)
		os.Unsetenv(constants.EnvPath)
		os.Unsetenv(constants.EnvIsiVolumePathPermissions)
		os.Unsetenv(constants.EnvAccessZone)
		os.Unsetenv(constants.EnvNodeName)
		os.Unsetenv(constants.EnvNodeIP)
		os.Unsetenv(constants.EnvKubeConfigPath)
		os.Unsetenv(constants.EnvIsilonConfigFile)
		os.Unsetenv(constants.EnvReplicationContextPrefix)
		os.Unsetenv(constants.EnvReplicationPrefix)
		os.Unsetenv(constants.EnvCsiVolPrefix)
	}()

	serviceInstance := &service{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = context.WithValue(ctx, constants.EnvMaxVolumesPerNode, "test")

	serviceInstance.initializeServiceOpts(ctx)

	assert.Equal(t, wantOps, serviceInstance.opts)
	assert.Equal(t, wantEnvNodeName, serviceInstance.nodeID)
	assert.Equal(t, wantEnvNodeIP, serviceInstance.nodeIP)

	os.Unsetenv(constants.EnvIsilonConfigFile)
	serviceInstance.initializeServiceOpts(ctx)
	assert.Equal(t, wantEnvNodeIP, serviceInstance.nodeIP)

	os.Unsetenv(constants.EnvPort)
	serviceInstance.initializeServiceOpts(ctx)
	assert.Equal(t, constants.DefaultPortNumber, serviceInstance.opts.Port)

	os.Unsetenv(constants.EnvPath)
	serviceInstance.initializeServiceOpts(ctx)
	assert.Equal(t, constants.DefaultIsiPath, serviceInstance.opts.Path)

	os.Unsetenv(constants.EnvIsiVolumePathPermissions)
	serviceInstance.initializeServiceOpts(ctx)
	assert.Equal(t, constants.DefaultIsiVolumePathPermissions, serviceInstance.opts.IsiVolumePathPermissions)

	os.Unsetenv(constants.EnvAccessZone)
	serviceInstance.initializeServiceOpts(ctx)
	assert.Equal(t, constants.DefaultAccessZone, serviceInstance.opts.AccessZone)

	os.Unsetenv(constants.EnvCsiVolPrefix)
	serviceInstance.initializeServiceOpts(ctx)
	assert.Equal(t, constants.DefaultCsiVolumePrefix, serviceInstance.opts.csiVolPrefix)

	// parsing error
	os.Setenv(constants.EnvMaxVolumesPerNode, "test!@#$")
	serviceInstance.initializeServiceOpts(ctx)
	assert.EqualValues(t, 0, serviceInstance.opts.MaxVolumesPerNode)

	os.Setenv(constants.EnvAllowedNetworks, "!@#")
	err := serviceInstance.initializeServiceOpts(ctx)
	assert.NotNil(t, err)
}

func TestSyncIsilonConfigs(t *testing.T) {
	isilonConfigFile = ""
	s := &service{}
	ctx := context.Background()
	err := s.syncIsilonConfigs(ctx)
	assert.NotEqual(t, nil, err)
}

func TestGetNewIsilonConfigs(t *testing.T) {
	ctx := context.Background()
	s := &service{}

	isilonConfigFile = "config.yaml"
	tmpDir := t.TempDir()
	isilonConfigFile := filepath.Join(tmpDir, isilonConfigFile)

	// scenario 1: invalid yaml
	content := `isilonClusters:
  - clusterName=`
	configBytes, err := writeToFileandRead(isilonConfigFile, content)
	_, _, err = s.getNewIsilonConfigs(ctx, configBytes)
	assert.NotEqual(t, nil, err)

	// scenario 2 : invalid cluster details
	content = `isilon:
  - clusterName: "abc"`
	configBytes, err = writeToFileandRead(isilonConfigFile, content)
	_, _, err = s.getNewIsilonConfigs(ctx, configBytes)
	assert.NotEqual(t, nil, err)

	// scenario 3
	opt := Opts{
		CustomTopologyEnabled: true,
	}
	s = &service{
		opts: opt,
	}
	content = `isilonClusters:
  - clusterName: "cluster1"
    username: "user"
    password: "password"
    endpoint: "1.2.3.4"
    isDefault: true
  - clusterName: "cluster2"
    username: "user"
    password: "password"
    endpoint: "1.2.3.4"
    isDefault: true`
	configBytes, err = writeToFileandRead(isilonConfigFile, content)
	_, _, err = s.getNewIsilonConfigs(ctx, configBytes)
	assert.NotEqual(t, nil, err)

	// scenario 4
	opt = Opts{
		CustomTopologyEnabled: false,
	}
	s = &service{
		opts: opt,
	}

	content = `isilonClusters:
  - clusterName: ""
    username: "user"
    password: "password"
    endpoint: "1.2.3.4"
    isDefault: true`
	configBytes, err = writeToFileandRead(isilonConfigFile, content)
	_, _, err = s.getNewIsilonConfigs(ctx, configBytes)
	assert.NotEqual(t, nil, err)

	// scenario 5: empty username
	content = `isilonClusters:
  - clusterName: "cluster1"
    username: ""
    password: "password"
    endpoint: "1.2.3.4"
    isDefault: true`
	configBytes, err = writeToFileandRead(isilonConfigFile, content)
	_, _, err = s.getNewIsilonConfigs(ctx, configBytes)
	assert.NotEqual(t, nil, err)

	// scenario 6 : empty password
	content = `isilonClusters:
  - clusterName: "cluster1"
    username: "user"
    password: ""
    endpoint: "1.2.3.4"
    isDefault: true`
	configBytes, err = writeToFileandRead(isilonConfigFile, content)
	_, _, err = s.getNewIsilonConfigs(ctx, configBytes)
	assert.NotEqual(t, nil, err)

	// scenario 7 : empty endpoint
	content = `isilonClusters:
  - clusterName: "cluster1"
    username: "user"
    password: "password"
    endpoint: ""
    isDefault: true`
	configBytes, err = writeToFileandRead(isilonConfigFile, content)
	_, _, err = s.getNewIsilonConfigs(ctx, configBytes)
	assert.NotEqual(t, nil, err)

	// scenario 8 : endpoint with https
	opt = Opts{
		CustomTopologyEnabled:     false,
		Port:                      "1234",
		SkipCertificateValidation: true,
		Path:                      "path",
		IsiVolumePathPermissions:  "true",
		IgnoreUnresolvableHosts:   false,
		Verbose:                   uint(1),
		isiAuthType:               uint8(1),
	}
	s = &service{
		opts: opt,
	}
	content = `isilonClusters:
  - clusterName: "cluster1"
    username: "user"
    password: "password"
    endpoint: "https://1.2.3.4"
    isDefault: true`
	copyNoProbeOnStart := noProbeOnStart
	noProbeOnStart = true
	configBytes, err = writeToFileandRead(isilonConfigFile, content)
	_, _, err = s.getNewIsilonConfigs(ctx, configBytes)
	assert.Equal(t, nil, err)

	// scenario 9 : without default value
	content = `isilonClusters:
  - clusterName: "cluster1"
    username: "user"
    password: "password"
    endpoint: "https://1.2.3.4"`
	configBytes, err = writeToFileandRead(isilonConfigFile, content)
	_, _, err = s.getNewIsilonConfigs(ctx, configBytes)
	assert.Equal(t, nil, err)

	// scenario 10 : 2 default values
	content = `isilonClusters:
  - clusterName: "cluster1"
    username: "user"
    password: "password"
    endpoint: "1.2.3.4"
    isDefault: true
  - clusterName: "cluster2"
    username: "user"
    password: "password"
    endpoint: "1.2.3.4"
    isDefault: true`
	configBytes, err = writeToFileandRead(isilonConfigFile, content)
	_, _, err = s.getNewIsilonConfigs(ctx, configBytes)
	assert.NotEqual(t, nil, err)

	// scenario 11 : same cluster name
	content = `isilonClusters:
  - clusterName: "cluster1"
    username: "user"
    password: "password"
    endpoint: "1.2.3.4"
    isDefault: true
  - clusterName: "cluster1"
    username: "user"
    password: "password"
    endpoint: "1.2.3.4"
    isDefault: false`
	configBytes, err = writeToFileandRead(isilonConfigFile, content)
	_, _, err = s.getNewIsilonConfigs(ctx, configBytes)
	assert.NotEqual(t, nil, err)

	noProbeOnStart = copyNoProbeOnStart
}

func writeToFileandRead(filePath, content string) ([]byte, error) {
	err := os.WriteFile(filePath, []byte(content), 0o600)
	if err != nil {
		return nil, fmt.Errorf("error writing to file: %w", err)
	}
	fmt.Println("File written successfully:", filePath)
	configBytes, err := os.ReadFile(filepath.Clean(filePath))
	return configBytes, nil
}

// Mocking the logger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Info(args ...interface{}) {
	m.Called(args...)
}

func (m *MockLogger) Debug(args ...interface{}) {
	m.Called(args...)
}

func (m *MockLogger) Error(args ...interface{}) {
	m.Called(args...)
}

func TestLoadIsilonConfigs(t *testing.T) {
	// Create a temporary directory to simulate the config file path
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Create a dummy config file
	err := os.WriteFile(configFile, []byte("dummy-content"), 0o600)
	require.NoError(t, err)

	// Create the service instance
	svc := &service{
		isiClusters: new(sync.Map),
	}

	ctx := context.Background()

	// Run loadIsilonConfigs in a separate goroutine
	go func() {
		err := svc.loadIsilonConfigs(ctx, configFile)
		require.NoError(t, err)
	}()

	// Simulate a config file update event
	parentFolder := filepath.Dir(configFile)
	eventPath := filepath.Join(parentFolder, "..data")

	time.Sleep(500 * time.Millisecond) // Give some time for the watcher to start

	// Create the simulated event folder
	err = os.Mkdir(eventPath, 0o755)
	require.NoError(t, err)

	// Wait to let the watcher pick up the event
	time.Sleep(1 * time.Second)
}

func TestSetRunIDContext(t *testing.T) {
	ctx := context.Background()
	runID := "test-run-123"
	newCtx, _ := setRunIDContext(ctx, runID)
	require.NotNil(t, newCtx)
}

func TestGetIsiClient(t *testing.T) {
	o := Opts{
		CustomTopologyEnabled: true,
	}
	s := service{
		opts: o,
	}
	ctx := context.Background()
	isiConfig := IsilonClusterConfig{}
	logLevel := logrus.InfoLevel
	_, err := s.GetIsiClient(ctx, &isiConfig, logLevel)
	assert.NotEqual(t, nil, err)
}

func TestGetIsilonConfig(t *testing.T) {
	// utils.GetLogger()
	ctx := context.Background()
	clusterName := ""
	s := service{
		defaultIsiClusterName: "",
	}
	_, err := s.getIsilonConfig(ctx, &clusterName)
	assert.NotEqual(t, "nil", err)
}

func TestIsVolumeTypeBlock(t *testing.T) {
	// Test case: empty VolumeCapabilities
	vcs := []*csi.VolumeCapability{}
	isBlock := isVolumeTypeBlock(vcs)
	if isBlock {
		t.Errorf("isVolumeTypeBlock returned true, expected false")
	}

	// Test case: VolumeCapability with Block access type
	block := &csi.VolumeCapability_BlockVolume{}
	accessType := &csi.VolumeCapability_Block{Block: block}
	vc := &csi.VolumeCapability{AccessType: accessType}
	vcs = []*csi.VolumeCapability{vc}
	isBlock = isVolumeTypeBlock(vcs)
	if !isBlock {
		t.Errorf("isVolumeTypeBlock returned false, expected true")
	}
}

func TestString(t *testing.T) {
	trueVar := true
	clusterConfig := IsilonClusterConfig{
		ClusterName:               "cluster1",
		Endpoint:                  "1.2.3.4",
		EndpointPort:              "8080",
		MountEndpoint:             "https://1.2.3.4:8080",
		EndpointURL:               "https://1.2.3.4:8080",
		accessZone:                "System",
		User:                      "user1",
		Password:                  "password1",
		IsiPath:                   "/ifs/data/csi-isilon",
		IsDefault:                 &trueVar,
		SkipCertificateValidation: &trueVar,
		IgnoreUnresolvableHosts:   &trueVar,
		IsiVolumePathPermissions:  "0777",
		isiSvc:                    &isiService{},
		ReplicationCertificateID:  "replicationCertificateID",
	}
	expectedOutput := "ClusterName: cluster1, Endpoint: 1.2.3.4, EndpointPort: 8080, EndpointURL: https://1.2.3.4:8080, User: user1, SkipCertificateValidation: true, IsiPath: /ifs/data/csi-isilon, IsiVolumePathPermissions: 0777, IsDefault: true, IgnoreUnresolvableHosts: true, AccessZone: System, isiSvc: &{ <nil>}"

	// Call the function that prints to stdout
	capturedOutput := clusterConfig.String()

	// Compare the captured output to the expected output
	if capturedOutput != expectedOutput {
		t.Errorf("Captured output '%s' does not match expected output '%s'", capturedOutput, expectedOutput)
	}
}

func TestValidateCreateVolumeRequest(t *testing.T) {
	o := Opts{
		Path: "path",
	}
	s := service{
		opts: o,
	}
	// Test case: empty CreateVolumeRequest
	req := &csi.CreateVolumeRequest{}
	size, err := s.ValidateCreateVolumeRequest(req)
	if err == nil {
		t.Errorf("ValidateCreateVolumeRequest returned nil error, expected error")
	}

	// Test case: valid CreateVolumeRequest
	req = &csi.CreateVolumeRequest{
		Name: "volume1",
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: 10 * 1024 * 1024 * 1024,
		},
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{
						FsType: "nfs",
					},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
		},
		Parameters: map[string]string{
			AccessZoneParam: "System",
			IsiPathParam:    "/ifs/data/csi-isilon",
		},
	}
	expectedSize := int64(10 * 1024 * 1024 * 1024)
	size, err = s.ValidateCreateVolumeRequest(req)
	if err != nil {
		t.Errorf("ValidateCreateVolumeRequest returned error '%s', expected nil", err.Error())
	}
	if size != expectedSize {
		t.Errorf("ValidateCreateVolumeRequest returned size '%d', expected '%d'", size, expectedSize)
	}

	// Test case: invalid CreateVolumeRequest
	req = &csi.CreateVolumeRequest{
		Name: "volume1",
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: -1,
		},
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{
						FsType: "nfs",
					},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
				},
			},
		},
		Parameters: map[string]string{
			AccessZoneParam: "System",
			IsiPathParam:    "/ifs/data/csi-isilon",
		},
	}
	_, err = s.ValidateCreateVolumeRequest(req)
	if err == nil {
		t.Errorf("ValidateCreateVolumeRequest returned nil error, expected error")
	}
}

// Mocking the service struct
// We will mock this method to control its behavior in tests
type mockService struct {
	service
	mock.Mock
}

func (m *mockService) syncIsilonConfigs(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockService) probe(ctx context.Context, isiConfig *IsilonClusterConfig) error {
	args := m.Called(ctx, isiConfig)
	return args.Error(0)
}

func TestUpdateDriverConfigParams(t *testing.T) {
	// Create a mock service
	mockSvc := new(mockService)
	// Create a new Viper instance and set test configuration
	v := viper.New()
	v.Set(constants.ParamCSILogLevel, "debug") // Example log level
	mockSvc.On("syncIsilonConfigs", mock.Anything).Return(nil)
	err := mockSvc.updateDriverConfigParams(context.Background(), v)
	assert.NotEqual(t, nil, err)

	v = viper.New()
	v.Set(constants.ParamCSILogLevel, "invalid-log-level")
	err = mockSvc.updateDriverConfigParams(context.Background(), v)
	// Assertions
	assert.Error(t, err, "Expected error due to invalid log level")
	assert.Contains(t, err.Error(), "not valid", "Error message should indicate invalid log level")

	v = viper.New()
	v.Set(constants.ParamCSILogLevel, "info")

	// Mock syncIsilonConfigs to return an error
	syncErr := errors.New("sync failure")
	mockSvc.On("syncIsilonConfigs", mock.Anything).Return(syncErr)
	err = mockSvc.updateDriverConfigParams(context.Background(), v)
	assert.NotEqual(t, nil, err)
}

func TestLogServiceStats(_ *testing.T) {
	s := &service{
		opts: Opts{
			Path:                      "/test/path",
			SkipCertificateValidation: true,
			AutoProbe:                 true,
			AccessZone:                "test-access-zone",
			QuotaEnabled:              true,
		},
		mode: "test-mode",
	}

	s.logServiceStats()
}

func TestGetIsiService(t *testing.T) {
	s := service{}
	clusterConfig := IsilonClusterConfig{
		ClusterName: "cluster1",
		Password:    "",
	}
	err := s.validateOptsParameters(&clusterConfig)
	assert.NotEqual(t, nil, err)
}

func TestAutoProbe(t *testing.T) {
	mockSvc := new(mockService)
	mockSvc.opts.AutoProbe = false
	ctx := context.Background()
	isiConfig := &IsilonClusterConfig{}

	err := mockSvc.autoProbe(ctx, isiConfig)

	assert.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))

	mockSvc = new(mockService)
	mockSvc.opts.AutoProbe = true
	mockSvc.On("probe", ctx, isiConfig).Return(nil).Twice()

	err = mockSvc.autoProbe(ctx, isiConfig)

	assert.Error(t, err)
}

func TestValidateDeleteVolumeRequest(t *testing.T) {
	mockSvc := new(mockService)
	ctx := context.Background()

	req := &csi.DeleteVolumeRequest{VolumeId: "1234"}
	mockSvc.On("ValidateDeleteVolumeRequest", ctx, req).Return(nil).Twice()
	err := mockSvc.ValidateDeleteVolumeRequest(ctx, req)
	assert.Error(t, err)

	// empty volume id
	mockSvc = new(mockService)

	req = &csi.DeleteVolumeRequest{VolumeId: ""}
	err = mockSvc.ValidateDeleteVolumeRequest(ctx, req)
	assert.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "no volume id is provided")
}

func TestLogStatistics(_ *testing.T) {
	s := service{
		statisticsCounter: -1,
	}
	s.logStatistics()
}

func TestProbe(t *testing.T) {
	s := service{
		mode: constants.ModeController,
	}
	ctx := context.Background()
	clusterConfig := IsilonClusterConfig{
		ClusterName: "c1",
	}
	err := s.probe(ctx, &clusterConfig)
	assert.NotEqual(t, nil, err)

	s = service{
		mode: constants.ModeNode,
	}
	err = s.probe(ctx, &clusterConfig)
	assert.NotEqual(t, nil, err)

	s = service{
		mode: "",
	}
	err = s.probe(ctx, &clusterConfig)
	assert.NotEqual(t, nil, err)

	s = service{
		mode: "error",
	}
	err = s.probe(ctx, &clusterConfig)
	assert.NotEqual(t, nil, err)
}

func TestSetNoProbeOnStart(_ *testing.T) {
	s := service{}
	ctx := context.Background()
	s.setNoProbeOnStart(ctx)
}

func TestGetGournalLevel(t *testing.T) {
	logLevel := logrus.InfoLevel
	level := getGournalLevelFromLogrusLevel(logLevel)
	assert.Equal(t, gournal.ParseLevel(logLevel.String()), level)
}

func TestValidateIsiPath(t *testing.T) {
	// Create a new instance of the service struct
	client := fake.NewSimpleClientset()
	s := &service{
		k8sclient: client,
	}
	s.k8sclient = client

	// Create a context.Context
	ctx := context.Background()

	// Create a fake PersistentVolume and StorageClass
	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pv",
		},
		Spec: v1.PersistentVolumeSpec{
			StorageClassName: "test-sc",
		},
	}
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sc",
		},
		Parameters: map[string]string{
			IsiPathParam: "/ifs/data",
		},
	}

	// Add the PersistentVolume and StorageClass to the fake clientset
	_, err := s.k8sclient.CoreV1().PersistentVolumes().Create(ctx, pv, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create PersistentVolume: %v", err)
	}
	_, err = s.k8sclient.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create StorageClass: %v", err)
	}

	// Test case: valid isiPath
	volName := "test-pv"
	expectedIsiPath := "/ifs/data"
	isiPath, err := s.validateIsiPath(ctx, volName)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if isiPath != expectedIsiPath {
		t.Errorf("expected isiPath %q, got %q", expectedIsiPath, isiPath)
	}

	// Test case: no isiPath
	sc.Parameters = map[string]string{}
	_, err = s.k8sclient.StorageV1().StorageClasses().Update(ctx, sc, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("failed to update StorageClass: %v", err)
	}
	isiPath, err = s.validateIsiPath(ctx, volName)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if isiPath != "" {
		t.Errorf("expected empty isiPath, got %q", isiPath)
	}

	// Test case: storage class does not exist
	pv.Spec.StorageClassName = "nonexistent-sc"
	_, err = s.k8sclient.CoreV1().PersistentVolumes().Update(ctx, pv, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("failed to update PersistentVolume: %v", err)
	}
	isiPath, err = s.validateIsiPath(ctx, volName)
	if err == nil {
		t.Errorf("expected error, got: %v", err)
	}
	if isiPath != "" {
		t.Errorf("expected empty isiPath, got %q", isiPath)
	}

	// Test case: empty storage class
	pv.Spec.StorageClassName = ""
	_, err = s.k8sclient.CoreV1().PersistentVolumes().Update(ctx, pv, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("failed to update PersistentVolume: %v", err)
	}
	isiPath, err = s.validateIsiPath(ctx, volName)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if isiPath != "" {
		t.Errorf("expected empty isiPath, got %q", isiPath)
	}

	// Test case: error getting PersistentVolume
	s.k8sclient.CoreV1().PersistentVolumes().Delete(ctx, "test-pv", metav1.DeleteOptions{})
	_, err = s.validateIsiPath(ctx, volName)
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}

func TestPatchNodeLabels(t *testing.T) {
	type checkFn func(t *testing.T, err error, node *v1.Node)
	type args struct {
		add    map[string]string
		remove []string
	}
	tests := []struct {
		name    string
		args    args
		node    *v1.Node
		wantErr bool
		check   checkFn
	}{
		{
			name: "success",
			args: args{
				add:    map[string]string{"label1": "value1"},
				remove: []string{"label2"},
			},
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						"label2": "value2",
					},
				},
			},
			wantErr: false,
			check: func(t *testing.T, err error, node *v1.Node) {
				assert.NoError(t, err)
				assert.Contains(t, node.Labels, "label1")
				assert.Equal(t, "value1", node.Labels["label1"])
				_, exists := node.Labels["label2"]
				assert.False(t, exists, "label2 should be removed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &service{
				mode:      "test",
				nodeID:    "test-node",
				k8sclient: fake.NewSimpleClientset(tt.node),
			}

			err := s.PatchNodeLabels(tt.args.add, tt.args.remove)
			if (err != nil) != tt.wantErr {
				t.Errorf("PatchNodeLabels() error = %v, wantErr %v", err, tt.wantErr)
			}

			node, err := s.k8sclient.CoreV1().Nodes().Get(context.TODO(), s.nodeID, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			tt.check(t, err, node)
		})
	}
}

func TestSetAZReconcileInterval(t *testing.T) {
	tests := []struct {
		name             string
		intervalStr      string
		expectedInterval time.Duration
	}{
		{
			name:             "valid interval",
			intervalStr:      "10s",
			expectedInterval: 10 * time.Second,
		},
		{
			name:             "invalid interval",
			intervalStr:      "invalid",
			expectedInterval: constants.DefaultAZReconcileInterval,
		},
		{
			name:             "empty interval",
			intervalStr:      "",
			expectedInterval: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &service{}
			v := viper.New()
			if tt.intervalStr != "" {
				v.Set(constants.ParamAZReconcileInterval, tt.intervalStr)
			}

			log := logrus.New()

			s.setAZReconcileInterval(log, v)
			assert.Equal(t, tt.expectedInterval, s.azReconcileInterval)
		})
	}
}

func TestService_reconcileNodeAzLabels(t *testing.T) {
	nodeName := "test-node"
	nodeLabels := map[string]string{
		"csi-isilon.dellemc.com/aznetwork-10.0.0.0_24": "10.0.0.1",
	}

	k8sclient := fake.NewSimpleClientset(&v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   nodeName,
			Labels: nodeLabels,
		},
	})

	s := &service{
		azReconcileInterval: 10 * time.Second,
		nodeID:              nodeName,
		k8sclient:           k8sclient,
	}

	err := s.reconcileNodeAzLabels(s.azReconcileInterval)
	assert.NoError(t, err)
}
