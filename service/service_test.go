package service

/*
 Copyright (c) 2019-2021 Dell Inc, or its subsidiaries.

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

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	_ "net/http/pprof" // #nosec G108
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/dell/csi-isilon/v2/common/constants"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
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
			setupFunc: func(m *sync.Map) {
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
	copy_noProbeOnStart := noProbeOnStart
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

	noProbeOnStart = copy_noProbeOnStart
}

func writeToFileandRead(filePath, content string) ([]byte, error) {
	err := os.WriteFile(filePath, []byte(content), 0o644)
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
	err := os.WriteFile(configFile, []byte("dummy-content"), 0o644)
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
