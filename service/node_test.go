/*
 Copyright (c) 2025 Dell Inc, or its subsidiaries.

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
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	isi "github.com/dell/goisilon"
	isimocks "github.com/dell/goisilon/mocks"
	v1 "github.com/dell/gopowerscale/api/v1"
	v2 "github.com/dell/gopowerscale/api/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const testTargetPath = "/tmp/csi-powerscale-test"

func Test_node_readFileFunc(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(tmpfile, []byte("dummy-content"), 0o600)
	result, err := readFileFunc(tmpfile)
	assert.NoError(t, err)
	assert.Equal(t, []byte("dummy-content"), result)
}

func setK8sClient(s *service) {
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "volume-id",
		},
		Spec: corev1.PersistentVolumeSpec{
			StorageClassName: "test-sc",
		},
	}
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sc",
		},
		Parameters: map[string]string{
			IsiPathParam: "/new/isi/path",
		},
	}
	s.k8sclient = fake.NewClientset(pv, sc)
}

func setNewIsiClientWithArgsFunc(mockClient *isimocks.Client) {
	newIsiClientWithArgsFunc = func(
		_ context.Context,
		_ string,
		_ bool,
		_ uint,
		_ string,
		_ string,
		_ string,
		_ string,
		_ string,
		_ bool,
		_ uint8,
	) (*isi.Client, error) {
		return &isi.Client{
			API: mockClient,
		}, nil
	}
}

func TestNodeGetVolumeStats(t *testing.T) {
	// Original function references
	originalGetIsVolumeExistentFunc := getIsVolumeExistentFunc
	originalGetIsVolumeMounted := getIsVolumeMounted
	originalGetOsReadDir := getOsReadDir
	originalGetK8sutilsGetStats := getK8sutilsGetStats
	originalNewIsiClientWithArgsFunc := newIsiClientWithArgsFunc

	// Reset function to reset mocks after tests
	resetMocks := func() {
		getIsVolumeExistentFunc = originalGetIsVolumeExistentFunc
		getIsVolumeMounted = originalGetIsVolumeMounted
		getOsReadDir = originalGetOsReadDir
		getK8sutilsGetStats = originalGetK8sutilsGetStats
		newIsiClientWithArgsFunc = originalNewIsiClientWithArgsFunc
	}

	mockClient := &isimocks.Client{}

	// Setup mock IsiCluster and service
	IsiClusters := new(sync.Map)
	testBool := false
	testIsilonClusterConfig := IsilonClusterConfig{
		ClusterName:               "TestCluster",
		Endpoint:                  "http://testendpoint",
		EndpointPort:              "8080",
		MountEndpoint:             "http://mountendpoint",
		EndpointURL:               "http://endpointurl",
		accessZone:                "TestAccessZone",
		User:                      "testuser",
		Password:                  "testpassword",
		SkipCertificateValidation: &testBool,
		IsiPath:                   "/ifs/data",
		IsiVolumePathPermissions:  "0777",
		IsDefault:                 &testBool,
		ReplicationCertificateID:  "certID",
		IgnoreUnresolvableHosts:   &testBool,
		isiSvc: &isiService{
			endpoint: "http://testendpoint:8080",
			client: &isi.Client{
				API: mockClient,
			},
		},
	}

	IsiClusters.Store(testIsilonClusterConfig.ClusterName, &testIsilonClusterConfig)
	s := &service{
		defaultIsiClusterName: "TestCluster",
		isiClusters:           IsiClusters,
	}
	mockClient.On("VolumesPath").Return("/path/to/volumes")
	mockClient.On(
		"Get",
		mock.AnythingOfType("*context.valueCtx"),
		"platform/2/protocols/nfs/exports",
		mock.AnythingOfType("string"),
		mock.AnythingOfType("api.OrderedValues"),
		mock.AnythingOfType("map[string]string"),
		mock.MatchedBy(func(arg interface{}) bool {
			_, ok := arg.(*v2.ExportList)
			return ok
		}),
	).Return(errors.New("mocked export lookup failure"))

	mockClient.On(
		"Get",
		mock.AnythingOfType("*context.valueCtx"),
		"namespace/path/to/volumes",
		"volume-id",
		mock.AnythingOfType("api.OrderedValues"),
		mock.AnythingOfType("map[string]string"),
		mock.MatchedBy(func(arg interface{}) bool {
			_, ok := arg.(**v1.GetIsiVolumeAttributesResp)
			return ok
		}),
	).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**v1.GetIsiVolumeAttributesResp)
		*resp = &v1.GetIsiVolumeAttributesResp{}
	})

	tests := []struct {
		name         string
		ctx          context.Context
		req          *csi.NodeGetVolumeStatsRequest
		setup        func()
		wantResponse *csi.NodeGetVolumeStatsResponse
		wantErr      bool
	}{
		{
			name: "Failed to get volume stats metrics",
			ctx:  context.Background(),
			req: &csi.NodeGetVolumeStatsRequest{
				VolumeId:   "volume-id",
				VolumePath: "/path/to/volume",
			},
			setup: func() {
				setK8sClient(s)
				getIsVolumeExistentFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, isiPath, volID, name string) bool {
					return func(_ context.Context, _, _, _ string) bool {
						return true
					}
				}

				getIsVolumeMounted = func(_ context.Context, _ string, _ string) (bool, error) {
					return true, nil
				}

				getOsReadDir = func(_ string) ([]os.DirEntry, error) {
					return []os.DirEntry{}, nil
				}

				getK8sutilsGetStats = func(_ context.Context, _ string) (int64, int64, int64, int64, int64, int64, error) {
					return 0, 0, 0, 0, 0, 0, errors.New("failed to get volume stats metrics")
				}
				setNewIsiClientWithArgsFunc(mockClient)
			},
			wantResponse: &csi.NodeGetVolumeStatsResponse{
				Usage: []*csi.VolumeUsage{
					{
						Unit:      csi.VolumeUsage_UNKNOWN,
						Available: 0,
						Total:     0,
						Used:      0,
					},
				},
				VolumeCondition: &csi.VolumeCondition{
					Abnormal: true,
					Message:  "failed to get volume stats metrics : failed to get volume stats metrics",
				},
			},
			wantErr: false,
		},
		{
			name: "No volume is mounted at path",
			ctx:  context.Background(),
			req: &csi.NodeGetVolumeStatsRequest{
				VolumeId:   "volume-id",
				VolumePath: "/path/to/volume",
			},
			setup: func() {
				getIsVolumeExistentFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, isiPath, volID, name string) bool {
					return func(_ context.Context, _, _, _ string) bool {
						return true
					}
				}

				getIsVolumeMounted = func(_ context.Context, _ string, _ string) (bool, error) {
					return false, errors.New("test error msg")
				}
			},
			wantResponse: nil,
			wantErr:      true,
		},
		{
			name: "Volume Path is not accessible",
			ctx:  context.Background(),
			req: &csi.NodeGetVolumeStatsRequest{
				VolumeId:   "volume-id",
				VolumePath: "/path/to/volume",
			},
			setup: func() {
				getIsVolumeExistentFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, isiPath, volID, name string) bool {
					return func(_ context.Context, _, _, _ string) bool {
						return true
					}
				}

				getIsVolumeMounted = func(_ context.Context, _ string, _ string) (bool, error) {
					return true, nil
				}

				getOsReadDir = func(_ string) ([]os.DirEntry, error) {
					return []os.DirEntry{}, errors.New("volume Path is not accessible")
				}
			},
			wantResponse: nil,
			wantErr:      true,
		},
		{
			name: "Success in NodeGetVolumeStats",
			ctx:  context.Background(),
			req: &csi.NodeGetVolumeStatsRequest{
				VolumeId:   "volume-id",
				VolumePath: "/path/to/volume",
			},
			setup: func() {
				setK8sClient(s)
				getIsVolumeExistentFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, isiPath, volID, name string) bool {
					return func(_ context.Context, _, _, _ string) bool {
						return true
					}
				}

				getIsVolumeMounted = func(_ context.Context, _ string, _ string) (bool, error) {
					return true, nil
				}

				getOsReadDir = func(_ string) ([]os.DirEntry, error) {
					return []os.DirEntry{}, nil
				}

				getK8sutilsGetStats = func(_ context.Context, _ string) (int64, int64, int64, int64, int64, int64, error) {
					return 1000, 2000, 1000, 4, 2, 2, nil
				}
				setNewIsiClientWithArgsFunc(mockClient)
			},
			wantResponse: &csi.NodeGetVolumeStatsResponse{
				Usage: []*csi.VolumeUsage{
					{
						Unit:      csi.VolumeUsage_BYTES,
						Available: 1000,
						Total:     2000,
						Used:      1000,
					},
					{
						Unit:      csi.VolumeUsage_INODES,
						Available: 2,
						Total:     4,
						Used:      2,
					},
				},
				VolumeCondition: &csi.VolumeCondition{
					Abnormal: false,
					Message:  "",
				},
			},
			wantErr: false,
		},
		{
			name: "Success in NodeGetVolumeStats- isiPathFromParams is used",
			ctx:  context.Background(),
			req: &csi.NodeGetVolumeStatsRequest{
				VolumeId:   "volume-id",
				VolumePath: "/path/to/volume",
			},
			setup: func() {
				pv := &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "volume-id",
					},
					Spec: corev1.PersistentVolumeSpec{
						StorageClassName: "test-sc",
					},
				}
				sc := &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-sc",
					},
					Parameters: map[string]string{
						IsiPathParam: "/new/isi/path",
					},
				}
				s.k8sclient = fake.NewClientset(pv, sc)

				getIsVolumeExistentFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, isiPath, volID, name string) bool {
					return func(_ context.Context, _, _, _ string) bool {
						return true
					}
				}

				getIsVolumeMounted = func(_ context.Context, _ string, _ string) (bool, error) {
					return true, nil
				}

				getOsReadDir = func(_ string) ([]os.DirEntry, error) {
					return []os.DirEntry{}, nil
				}

				getK8sutilsGetStats = func(_ context.Context, _ string) (int64, int64, int64, int64, int64, int64, error) {
					return 1000, 2000, 1000, 4, 2, 2, nil
				}
				setNewIsiClientWithArgsFunc(mockClient)
			},
			wantResponse: &csi.NodeGetVolumeStatsResponse{
				Usage: []*csi.VolumeUsage{
					{
						Unit:      csi.VolumeUsage_BYTES,
						Available: 1000,
						Total:     2000,
						Used:      1000,
					},
					{
						Unit:      csi.VolumeUsage_INODES,
						Available: 2,
						Total:     4,
						Used:      2,
					},
				},
				VolumeCondition: &csi.VolumeCondition{
					Abnormal: false,
					Message:  "",
				},
			},
			wantErr: false,
		},
		{
			name: "Success in NodeGetVolumeStats- isiPath from pv is used",
			ctx:  context.Background(),
			req: &csi.NodeGetVolumeStatsRequest{
				VolumeId:   "volume-id",
				VolumePath: "/path/to/volume",
			},
			setup: func() {
				pv := &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "volume-id",
					},
					Spec: corev1.PersistentVolumeSpec{
						StorageClassName: "test-sc",
						PersistentVolumeSource: corev1.PersistentVolumeSource{
							CSI: &corev1.CSIPersistentVolumeSource{
								VolumeAttributes: map[string]string{
									"Path": "/new/isi/path",
								},
							},
						},
					},
				}
				sc := &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-sc",
					},
					Parameters: map[string]string{
						IsiPathParam: "/new/isi/path",
					},
				}
				s.k8sclient = fake.NewClientset(pv, sc)

				getIsVolumeExistentFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, isiPath, volID, name string) bool {
					return func(_ context.Context, _, _, _ string) bool {
						return true
					}
				}

				getIsVolumeMounted = func(_ context.Context, _ string, _ string) (bool, error) {
					return true, nil
				}

				getOsReadDir = func(_ string) ([]os.DirEntry, error) {
					return []os.DirEntry{}, nil
				}

				getK8sutilsGetStats = func(_ context.Context, _ string) (int64, int64, int64, int64, int64, int64, error) {
					return 1000, 2000, 1000, 4, 2, 2, nil
				}

				setNewIsiClientWithArgsFunc(mockClient)
			},
			wantResponse: &csi.NodeGetVolumeStatsResponse{
				Usage: []*csi.VolumeUsage{
					{
						Unit:      csi.VolumeUsage_BYTES,
						Available: 1000,
						Total:     2000,
						Used:      1000,
					},
					{
						Unit:      csi.VolumeUsage_INODES,
						Available: 2,
						Total:     4,
						Used:      2,
					},
				},
				VolumeCondition: &csi.VolumeCondition{
					Abnormal: false,
					Message:  "",
				},
			},
			wantErr: false,
		},
		{
			name: "Failure in NodeGetVolumeStats- isiPathFromParams is used but cannot get isi Service",
			ctx:  context.Background(),
			req: &csi.NodeGetVolumeStatsRequest{
				VolumeId:   "volume-id",
				VolumePath: "/path/to/volume",
			},
			setup: func() {
				pv := &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "volume-id",
					},
					Spec: corev1.PersistentVolumeSpec{
						StorageClassName: "test-sc",
					},
				}
				sc := &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-sc",
					},
					Parameters: map[string]string{
						IsiPathParam: "/new/isi/path",
					},
				}

				s.k8sclient = fake.NewClientset(pv, sc)

				newIsiClientWithArgsFunc = func(
					_ context.Context,
					_ string,
					_ bool,
					_ uint,
					_ string,
					_ string,
					_ string,
					_ string,
					_ string,
					_ bool,
					_ uint8,
				) (*isi.Client, error) {
					return nil, errors.New("cannot get isi Service")
				}
			},
			wantResponse: nil,
			wantErr:      true,
		},
	}

	// Run the test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer resetMocks() // Ensures any mocks or overrides are reset after each test

			// Setup test case specific mocks and overrides
			if tt.setup != nil {
				tt.setup()
			}

			// Call the function under test
			got, err := s.NodeGetVolumeStats(tt.ctx, tt.req)

			// reset the k8sclient
			s.k8sclient = nil

			// Check if the error status matches
			if (err != nil) != tt.wantErr {
				t.Errorf("NodeGetVolumeStats() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Validate the response
			if !assert.Equal(t, tt.wantResponse, got) {
				t.Errorf("NodeGetVolumeStats() = %v, want %v", got, tt.wantResponse)
			}
		})
	}

	t.Run("Volume does not exist", func(t *testing.T) {
		defer resetMocks()
		setK8sClient(s)
		setNewIsiClientWithArgsFunc(mockClient)
		mockClient.ExpectedCalls = nil
		mockClient.On("VolumesPath").Return("/path/to/volumes")
		mockClient.On("Get", anyArgs[0:6]...).Return(fmt.Errorf("not found"))
		req := &csi.NodeGetVolumeStatsRequest{
			VolumeId:   "volume-id",
			VolumePath: "/path/to/volume",
		}
		resp, err := s.NodeGetVolumeStats(context.Background(), req)
		assert.ErrorContains(t, err, "volume volume-id does not exist at path /path/to/volume")
		assert.Nil(t, resp)
	})
}

func TestEphemeralNodePublish(t *testing.T) {
	ctx := context.Background()
	IsiClusters := new(sync.Map)
	testBool := false
	testIsilonClusterConfig := IsilonClusterConfig{
		ClusterName:               "TestCluster",
		Endpoint:                  "http://testendpoint",
		EndpointPort:              "8080",
		MountEndpoint:             "http://mountendpoint",
		EndpointURL:               "http://endpointurl",
		accessZone:                "TestAccessZone",
		User:                      "testuser",
		Password:                  "testpassword",
		SkipCertificateValidation: &testBool,
		IsiPath:                   "/ifs/data",
		IsiVolumePathPermissions:  "0777",
		IsDefault:                 &testBool,
		ReplicationCertificateID:  "certID",
		IgnoreUnresolvableHosts:   &testBool,
		isiSvc: &isiService{
			endpoint: "http://testendpoint:8080",
			client:   &isi.Client{},
		},
	}
	IsiClusters.Store(testIsilonClusterConfig.ClusterName, &testIsilonClusterConfig)

	defaultService := &service{
		defaultIsiClusterName: "TestCluster",
		isiClusters:           IsiClusters,
		nodeIP:                "1.2.3.4",
		nodeID:                "TestNodeID",
		opts: Opts{
			AccessZone:            "TestAccessZone",
			CustomTopologyEnabled: true,
		},
	}
	s := defaultService

	// functions that may be overridden for injection
	defaultEphemeralNodeUnpublishFunc := ephemeralNodeUnpublishFunc
	defaultGetControllerPublishVolume := getControllerPublishVolume
	defaultGetUtilsGetFQDNByIP := getUtilsGetFQDNByIP
	defaultGetCreateVolumeFunc := getCreateVolumeFunc
	defaultCloseFileFunc := closeFileFunc
	defaultMakeDirAllFunc := mkDirAllFunc
	defaultCreateFileFunc := createFileFunc
	defaultWriteStringFunc := writeStringFunc
	defaultGetVolByNameFunc := getVolByNameFunc
	defaultPublishVolFunc := publishVolumeFunc
	defaultStatFileFunc := statFileFunc

	after := func() {
		ephemeralNodeUnpublishFunc = defaultEphemeralNodeUnpublishFunc
		getControllerPublishVolume = defaultGetControllerPublishVolume
		getUtilsGetFQDNByIP = defaultGetUtilsGetFQDNByIP
		getCreateVolumeFunc = defaultGetCreateVolumeFunc
		closeFileFunc = defaultCloseFileFunc
		mkDirAllFunc = defaultMakeDirAllFunc
		createFileFunc = defaultCreateFileFunc
		writeStringFunc = defaultWriteStringFunc
		getVolByNameFunc = defaultGetVolByNameFunc
		publishVolumeFunc = defaultPublishVolFunc
		statFileFunc = defaultStatFileFunc

		// reset service/context
		s = defaultService
	}

	type testCase struct {
		name     string
		req      *csi.NodePublishVolumeRequest
		expected *csi.NodePublishVolumeResponse
		wantErr  bool
		setup    func()
	}

	testCases := []testCase{
		{
			name: "Failed create volume check",
			req: &csi.NodePublishVolumeRequest{
				VolumeId: "123",
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					"csi.storage.k8s.io/ephemeral": "true",
				},
				TargetPath: testTargetPath,
			},
			setup: func() {
				getCreateVolumeFunc = func(_ *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
						return nil, errors.New("failed create vol check")
					}
				}
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "Failed to get node ID",
			req: &csi.NodePublishVolumeRequest{
				VolumeId: "123",
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					"csi.storage.k8s.io/ephemeral": "true",
				},
				TargetPath: testTargetPath,
			},
			setup: func() {
				// make s a service with no nodeID
				s = &service{
					defaultIsiClusterName: "TestCluster",
					isiClusters:           IsiClusters,
					opts: Opts{
						AccessZone:            "TestAccessZone",
						CustomTopologyEnabled: true,
					},
				}
				getCreateVolumeFunc = func(_ *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
						return &csi.CreateVolumeResponse{
							Volume: &csi.Volume{
								VolumeId:      "volume-id",
								VolumeContext: map[string]string{"Name": "volname", "Path": "/path/volname", "AccessZone": "volaccesszone"},
							},
						}, nil
					}
				}
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "Failed in ControllerPublishVolume",
			req: &csi.NodePublishVolumeRequest{
				VolumeId: "123",
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					"csi.storage.k8s.io/ephemeral": "true",
				},
				TargetPath: testTargetPath,
			},
			setup: func() {
				getCreateVolumeFunc = func(_ *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
						return &csi.CreateVolumeResponse{
							Volume: &csi.Volume{
								VolumeId: "volume-id",
							},
						}, nil
					}
				}
				getUtilsGetFQDNByIP = func(_ context.Context, _ string) (string, error) {
					return "testFQDN", nil
				}
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "Failed in ControllerPublishVolume but succeed rollback",
			req: &csi.NodePublishVolumeRequest{
				VolumeId: "123",
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					"csi.storage.k8s.io/ephemeral": "true",
				},
				TargetPath: testTargetPath,
			},
			setup: func() {
				getCreateVolumeFunc = func(_ *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
						return &csi.CreateVolumeResponse{
							Volume: &csi.Volume{
								VolumeId: "volume-id",
							},
						}, nil
					}
				}
				getUtilsGetFQDNByIP = func(_ context.Context, _ string) (string, error) {
					return "testFQDN", nil
				}
				ephemeralNodeUnpublishFunc = func(_ *service, _ context.Context, _ *csi.NodeUnpublishVolumeRequest) error {
					return nil
				}
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "Failed in NodePublishVolume",
			req: &csi.NodePublishVolumeRequest{
				VolumeId: "123",
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					"csi.storage.k8s.io/ephemeral": "true",
				},
				TargetPath: testTargetPath,
			},
			setup: func() {
				getControllerPublishVolume = func(_ *service) func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
					return func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
						return &csi.ControllerPublishVolumeResponse{}, nil
					}
				}
				getCreateVolumeFunc = func(_ *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
						return &csi.CreateVolumeResponse{
							Volume: &csi.Volume{
								VolumeId: "volume-id",
							},
						}, nil
					}
				}

				getUtilsGetFQDNByIP = func(_ context.Context, _ string) (string, error) {
					return "testFQDN", nil
				}
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "Failed in NodePublishVolume but succeed rollback",
			req: &csi.NodePublishVolumeRequest{
				VolumeId: "123",
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					"csi.storage.k8s.io/ephemeral": "true",
				},
				TargetPath: testTargetPath,
			},
			setup: func() {
				getControllerPublishVolume = func(_ *service) func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
					return func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
						return &csi.ControllerPublishVolumeResponse{}, nil
					}
				}
				getCreateVolumeFunc = func(_ *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
						return &csi.CreateVolumeResponse{
							Volume: &csi.Volume{
								VolumeId: "volume-id",
							},
						}, nil
					}
				}

				getUtilsGetFQDNByIP = func(_ context.Context, _ string) (string, error) {
					return "testFQDN", nil
				}
				ephemeralNodeUnpublishFunc = func(_ *service, _ context.Context, _ *csi.NodeUnpublishVolumeRequest) error {
					return nil
				}
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "success run",
			req: &csi.NodePublishVolumeRequest{
				VolumeId: "123",
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					"csi.storage.k8s.io/ephemeral": "true",
				},
				TargetPath: testTargetPath,
			},
			setup: func() {
				publishVolumeFunc = func(_ context.Context, _ *csi.NodePublishVolumeRequest, _ string) error {
					return nil
				}
				getVolByNameFunc = func(_ *service, _ context.Context, _ string, _ string, _ *IsilonClusterConfig) (isi.Volume, error) {
					return nil, nil
				}

				getControllerPublishVolume = func(_ *service) func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
					return func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
						return &csi.ControllerPublishVolumeResponse{}, nil
					}
				}
				getCreateVolumeFunc = func(_ *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
						return &csi.CreateVolumeResponse{
							Volume: &csi.Volume{
								VolumeId:      "volume-id",
								VolumeContext: map[string]string{"Name": "volname", "Path": "/path/volname", "AccessZone": "volaccesszone"},
							},
						}, nil
					}
				}

				getUtilsGetFQDNByIP = func(_ context.Context, _ string) (string, error) {
					return "testFQDN", nil
				}
			},
			expected: &csi.NodePublishVolumeResponse{XXX_NoUnkeyedLiteral: struct{}{}, XXX_unrecognized: nil, XXX_sizecache: 0},
			wantErr:  false,
		},
		{
			name: "fail to make directory",
			req: &csi.NodePublishVolumeRequest{
				VolumeId: "123",
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					"csi.storage.k8s.io/ephemeral": "true",
				},
				TargetPath: testTargetPath,
			},
			setup: func() {
				statFileFunc = func(_ string) (fs.FileInfo, error) {
					newErr := fs.ErrNotExist
					return nil, newErr
				}
				publishVolumeFunc = func(_ context.Context, _ *csi.NodePublishVolumeRequest, _ string) error {
					return nil
				}
				getVolByNameFunc = func(_ *service, _ context.Context, _ string, _ string, _ *IsilonClusterConfig) (isi.Volume, error) {
					return nil, nil
				}
				getControllerPublishVolume = func(_ *service) func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
					return func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
						return &csi.ControllerPublishVolumeResponse{}, nil
					}
				}
				getCreateVolumeFunc = func(_ *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
						return &csi.CreateVolumeResponse{
							Volume: &csi.Volume{
								VolumeId:      "volume-id",
								VolumeContext: map[string]string{"Name": "volname", "Path": "/path/volname", "AccessZone": "volaccesszone"},
							},
						}, nil
					}
				}

				getUtilsGetFQDNByIP = func(_ context.Context, _ string) (string, error) {
					return "testFQDN", nil
				}
				mkDirAllFunc = func(_ string, _ os.FileMode) error {
					return errors.New("fail to make directory")
				}

				ephemeralNodeUnpublishFunc = func(_ *service, _ context.Context, _ *csi.NodeUnpublishVolumeRequest) error {
					return errors.New("failed to unpublish")
				}
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "fail to make directory but succeed rollback",
			req: &csi.NodePublishVolumeRequest{
				VolumeId: "123",
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					"csi.storage.k8s.io/ephemeral": "true",
				},
				TargetPath: testTargetPath,
			},
			setup: func() {
				statFileFunc = func(_ string) (fs.FileInfo, error) {
					newErr := fs.ErrNotExist
					return nil, newErr
				}
				publishVolumeFunc = func(_ context.Context, _ *csi.NodePublishVolumeRequest, _ string) error {
					return nil
				}
				getVolByNameFunc = func(_ *service, _ context.Context, _ string, _ string, _ *IsilonClusterConfig) (isi.Volume, error) {
					return nil, nil
				}
				getControllerPublishVolume = func(_ *service) func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
					return func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
						return &csi.ControllerPublishVolumeResponse{}, nil
					}
				}
				getCreateVolumeFunc = func(_ *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
						return &csi.CreateVolumeResponse{
							Volume: &csi.Volume{
								VolumeId:      "volume-id",
								VolumeContext: map[string]string{"Name": "volname", "Path": "/path/volname", "AccessZone": "volaccesszone"},
							},
						}, nil
					}
				}

				getUtilsGetFQDNByIP = func(_ context.Context, _ string) (string, error) {
					return "testFQDN", nil
				}
				mkDirAllFunc = func(_ string, _ os.FileMode) error {
					return errors.New("fail to make directory")
				}

				ephemeralNodeUnpublishFunc = func(_ *service, _ context.Context, _ *csi.NodeUnpublishVolumeRequest) error {
					return nil
				}
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "fail to make file",
			req: &csi.NodePublishVolumeRequest{
				VolumeId: "123",
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					"csi.storage.k8s.io/ephemeral": "true",
				},
				TargetPath: testTargetPath,
			},
			setup: func() {
				publishVolumeFunc = func(_ context.Context, _ *csi.NodePublishVolumeRequest, _ string) error {
					return nil
				}
				getVolByNameFunc = func(_ *service, _ context.Context, _ string, _ string, _ *IsilonClusterConfig) (isi.Volume, error) {
					return nil, nil
				}

				getControllerPublishVolume = func(_ *service) func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
					return func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
						return &csi.ControllerPublishVolumeResponse{}, nil
					}
				}
				getCreateVolumeFunc = func(_ *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
						return &csi.CreateVolumeResponse{
							Volume: &csi.Volume{
								VolumeId:      "volume-id",
								VolumeContext: map[string]string{"Name": "volname", "Path": "/path/volname", "AccessZone": "volaccesszone"},
							},
						}, nil
					}
				}

				getUtilsGetFQDNByIP = func(_ context.Context, _ string) (string, error) {
					return "testFQDN", nil
				}
				createFileFunc = func(_ string) (*os.File, error) {
					return nil, errors.New("fail to make file")
				}

				ephemeralNodeUnpublishFunc = func(_ *service, _ context.Context, _ *csi.NodeUnpublishVolumeRequest) error {
					return errors.New("failed to unpublish")
				}
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "fail to make file but succeed rollback",
			req: &csi.NodePublishVolumeRequest{
				VolumeId: "123",
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					"csi.storage.k8s.io/ephemeral": "true",
				},
				TargetPath: testTargetPath,
			},
			setup: func() {
				publishVolumeFunc = func(_ context.Context, _ *csi.NodePublishVolumeRequest, _ string) error {
					return nil
				}
				getVolByNameFunc = func(_ *service, _ context.Context, _ string, _ string, _ *IsilonClusterConfig) (isi.Volume, error) {
					return nil, nil
				}

				getControllerPublishVolume = func(_ *service) func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
					return func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
						return &csi.ControllerPublishVolumeResponse{}, nil
					}
				}
				getCreateVolumeFunc = func(_ *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
						return &csi.CreateVolumeResponse{
							Volume: &csi.Volume{
								VolumeId:      "volume-id",
								VolumeContext: map[string]string{"Name": "volname", "Path": "/path/volname", "AccessZone": "volaccesszone"},
							},
						}, nil
					}
				}

				getUtilsGetFQDNByIP = func(_ context.Context, _ string) (string, error) {
					return "testFQDN", nil
				}
				createFileFunc = func(_ string) (*os.File, error) {
					return nil, errors.New("fail to make file")
				}

				ephemeralNodeUnpublishFunc = func(_ *service, _ context.Context, _ *csi.NodeUnpublishVolumeRequest) error {
					return nil
				}
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "fail to write to + close file",
			req: &csi.NodePublishVolumeRequest{
				VolumeId: "123",
				VolumeCapability: &csi.VolumeCapability{
					AccessMode: &csi.VolumeCapability_AccessMode{
						Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
					},
				},
				VolumeContext: map[string]string{
					"csi.storage.k8s.io/ephemeral": "true",
				},
				TargetPath: testTargetPath,
			},
			setup: func() {
				closeFileFunc = func(_ *os.File) error {
					return errors.New("fail to close file")
				}
				writeStringFunc = func(_ *os.File, _ string) (int, error) {
					return 0, errors.New("fail to write to file")
				}
				publishVolumeFunc = func(_ context.Context, _ *csi.NodePublishVolumeRequest, _ string) error {
					return nil
				}
				getVolByNameFunc = func(_ *service, _ context.Context, _ string, _ string, _ *IsilonClusterConfig) (isi.Volume, error) {
					return nil, nil
				}

				getControllerPublishVolume = func(_ *service) func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
					return func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
						return &csi.ControllerPublishVolumeResponse{}, nil
					}
				}
				getCreateVolumeFunc = func(_ *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
						return &csi.CreateVolumeResponse{
							Volume: &csi.Volume{
								VolumeId:      "volume-id",
								VolumeContext: map[string]string{"Name": "volname", "Path": "/path/volname", "AccessZone": "volaccesszone"},
							},
						}, nil
					}
				}

				getUtilsGetFQDNByIP = func(_ context.Context, _ string) (string, error) {
					return "testFQDN", nil
				}
			},
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer after()
			if tc.setup != nil {
				tc.setup()
			}

			// Calling the function
			got, err := s.ephemeralNodePublish(ctx, tc.req)
			if (err != nil) != tc.wantErr {
				t.Errorf("ephemeralNodePublish() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("ephemeralNodePublish() = %v, want %v", got, tc.expected)
			}
		})
	}
}

///////

func TestNodeUnpublishVolume(t *testing.T) {
	ctx := context.Background()

	// functions that may be overridden for injection
	defaultReadFileFunc := readFileFunc

	after := func() {
		readFileFunc = defaultReadFileFunc
	}

	// clean service for each run
	// allows custom isilonConfig injections
	initService := func() *service {
		IsiClusters := new(sync.Map)
		testBool := false
		defaultIsilonClusterConfig := IsilonClusterConfig{
			ClusterName:               "TestCluster",
			Endpoint:                  "http://testendpoint",
			EndpointPort:              "8080",
			MountEndpoint:             "http://mountendpoint",
			EndpointURL:               "http://endpointurl",
			accessZone:                "TestAccessZone",
			User:                      "testuser",
			Password:                  "testpassword",
			SkipCertificateValidation: &testBool,
			IsiPath:                   "/ifs/data",
			IsiVolumePathPermissions:  "0777",
			IsDefault:                 &testBool,
			ReplicationCertificateID:  "certID",
			IgnoreUnresolvableHosts:   &testBool,
			isiSvc: &isiService{
				endpoint: "http://testendpoint:8080",
				client:   &isi.Client{},
			},
		}
		IsiClusters.Store(defaultIsilonClusterConfig.ClusterName, &defaultIsilonClusterConfig)

		return &service{
			defaultIsiClusterName: "TestCluster",
			isiClusters:           IsiClusters,
			nodeIP:                "1.2.3.4",
			nodeID:                "TestNodeID",
			opts: Opts{
				AccessZone:            "TestAccessZone",
				CustomTopologyEnabled: true,
			},
		}
	}

	type testCase struct {
		name          string
		req           *csi.NodeUnpublishVolumeRequest
		expected      *csi.NodeUnpublishVolumeResponse
		wantErr       bool
		setup         func()
		customContext func() *service // if the context needs to be overwritten, use this
	}

	testCases := []testCase{
		{
			name: "Fail to get isilon config",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId: "123",
			},
			customContext: func() *service {
				// no clusters to find will cause an error
				IsiClustersTemp := new(sync.Map)
				newService := &service{
					defaultIsiClusterName: "TestCluster",
					isiClusters:           IsiClustersTemp,
					nodeIP:                "1.2.3.4",
					nodeID:                "TestNodeID",
					opts: Opts{
						AccessZone:            "TestAccessZone",
						CustomTopologyEnabled: true,
					},
				}
				return newService
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "Fail to read file",
			req: &csi.NodeUnpublishVolumeRequest{
				VolumeId: "123",
			},
			setup: func() {
				readFileFunc = func(_ string) ([]byte, error) {
					return nil, errors.New("fail to read file")
				}
			},
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer after()
			var s *service
			if tc.customContext != nil {
				s = tc.customContext()
			} else {
				s = initService()
			}
			if tc.setup != nil {
				tc.setup()
			}

			// Calling the function
			got, err := s.NodeUnpublishVolume(ctx, tc.req)
			if (err != nil) != tc.wantErr {
				t.Errorf("nodeUnpublishVolume() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("nodeUnpublishVolume() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestNodeLabelsNeedPatching(t *testing.T) {
	type args struct {
		labels         map[string]string
		labelsToAdd    map[string]string
		labelsToRemove []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "all nil parameters",
			args: args{
				labels:         nil,
				labelsToAdd:    nil,
				labelsToRemove: nil,
			},
			want: false,
		},
		{
			name: "nil node labels but need to add",
			args: args{
				labels:         nil,
				labelsToAdd:    map[string]string{"key1": "value1", "key2": "value2"},
				labelsToRemove: []string{"key3", "key4"},
			},
			want: true,
		},
		{
			name: "nil labels to add",
			args: args{
				labels:         map[string]string{"key1": "value1", "key2": "value2"},
				labelsToAdd:    nil,
				labelsToRemove: []string{},
			},
			want: false,
		},
		{
			name: "nil labels to remove with labels to add",
			args: args{
				labels:         map[string]string{"key1": "value1", "key2": "value2"},
				labelsToAdd:    map[string]string{"key3": "value3"},
				labelsToRemove: nil,
			},
			want: true,
		},
		{
			name: "nil labels to remove with no labels to add",
			args: args{
				labels:         map[string]string{"key1": "value1", "key2": "value2"},
				labelsToAdd:    map[string]string{},
				labelsToRemove: nil,
			},
			want: false,
		},
		{
			name: "no labels to add or remove",
			args: args{
				labels:         map[string]string{"key1": "value1", "key2": "value2"},
				labelsToAdd:    map[string]string{},
				labelsToRemove: []string{},
			},
			want: false,
		},
		{
			name: "labels to add",
			args: args{
				labels:         map[string]string{"key1": "value1", "key2": "value2"},
				labelsToAdd:    map[string]string{"key3": "value3", "key4": "value4"},
				labelsToRemove: []string{},
			},
			want: true,
		},
		{
			name: "labels to remove",
			args: args{
				labels:         map[string]string{"key1": "value1", "key2": "value2"},
				labelsToAdd:    map[string]string{},
				labelsToRemove: []string{"key1", "key2"},
			},
			want: true,
		},
		{
			name: "labels to add and remove",
			args: args{
				labels:         map[string]string{"key1": "value1", "key2": "value2"},
				labelsToAdd:    map[string]string{"key3": "value3", "key4": "value4"},
				labelsToRemove: []string{"key1", "key2"},
			},
			want: true,
		},
		{
			name: "labels to add already exist",
			args: args{
				labels:         map[string]string{"key1": "value1", "key2": "value2"},
				labelsToAdd:    map[string]string{"key1": "value1", "key2": "value2"},
				labelsToRemove: []string{},
			},
			want: false,
		},
		{
			name: "labels to remove do not exist",
			args: args{
				labels:         map[string]string{"key1": "value1", "key2": "value2"},
				labelsToAdd:    map[string]string{},
				labelsToRemove: []string{"key3", "key4"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nodeLabelsNeedPatching(tt.args.labels, tt.args.labelsToAdd, tt.args.labelsToRemove); got != tt.want {
				t.Errorf("nodeLabelsNeedPatching() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReconcileNodeAzLabels(t *testing.T) {
	defaultGetInterfaceAddressesFunc := getInterfaceAddrsFunc
	defaultGetNodeLabelsFunc := getNodeLabelsFunc
	defaultPatchNodeLabelsFunc := getPatchNodeLabelsFunc

	after := func() {
		getInterfaceAddrsFunc = defaultGetInterfaceAddressesFunc
		getNodeLabelsFunc = defaultGetNodeLabelsFunc
		getPatchNodeLabelsFunc = defaultPatchNodeLabelsFunc
	}

	tests := []struct {
		name                   string
		addrs                  []net.Addr
		addrErr                error
		nodeLabels             map[string]string
		expectedLabelsToAdd    map[string]string
		expectedLabelsToRemove []string
		getNodeLabelsErr       error
		patchNodeLabelsErr     error
		wantErr                bool
	}{
		{
			name: "add new labels",
			addrs: []net.Addr{
				&net.IPNet{
					IP:   net.ParseIP("192.168.1.1").To4(),
					Mask: net.CIDRMask(24, 32),
				},
			},
			nodeLabels: map[string]string{},
			expectedLabelsToAdd: map[string]string{
				"csi-isilon.dellemc.com/az-192.168.1.0-24-192.168.1.1": "true",
			},
			expectedLabelsToRemove: []string{},
			wantErr:                false,
		},
		{
			name:  "remove labels",
			addrs: []net.Addr{},
			nodeLabels: map[string]string{
				"csi-isilon.dellemc.com/az-192.168.1.0-24-192.168.1.1": "true",
			},
			expectedLabelsToAdd: map[string]string{},
			expectedLabelsToRemove: []string{
				"csi-isilon.dellemc.com/az-192.168.1.0-24-192.168.1.1",
			},
			wantErr: false,
		},
		{
			name: "multiple addresses in same network",
			addrs: []net.Addr{
				&net.IPNet{
					IP:   net.ParseIP("192.168.100.100").To4(),
					Mask: net.CIDRMask(24, 32),
				},
				&net.IPNet{
					IP:   net.ParseIP("192.168.100.101").To4(),
					Mask: net.CIDRMask(24, 32),
				},
				&net.IPNet{
					IP:   net.ParseIP("192.168.100.102").To4(),
					Mask: net.CIDRMask(24, 32),
				},
				&net.IPNet{
					IP:   net.ParseIP("192.168.100.103").To4(),
					Mask: net.CIDRMask(24, 32),
				},
				&net.IPNet{
					IP:   net.ParseIP("192.168.100.104").To4(),
					Mask: net.CIDRMask(24, 32),
				},
			},
			nodeLabels: map[string]string{},
			expectedLabelsToAdd: map[string]string{
				"csi-isilon.dellemc.com/az-192.168.100.0-24-192.168.100.100": "true",
				"csi-isilon.dellemc.com/az-192.168.100.0-24-192.168.100.101": "true",
				"csi-isilon.dellemc.com/az-192.168.100.0-24-192.168.100.102": "true",
				"csi-isilon.dellemc.com/az-192.168.100.0-24-192.168.100.103": "true",
				"csi-isilon.dellemc.com/az-192.168.100.0-24-192.168.100.104": "true",
			},
			expectedLabelsToRemove: []string{},
			wantErr:                false,
		},
		{
			name:                   "failed to get interface addresses",
			addrs:                  nil,
			addrErr:                errors.New("permission denied"),
			nodeLabels:             map[string]string{},
			expectedLabelsToAdd:    map[string]string{},
			expectedLabelsToRemove: []string{},
			wantErr:                true,
		},
		{
			name: "handle invalid CIDR mask",
			addrs: []net.Addr{
				&net.IPNet{
					IP:   net.ParseIP("192.168.100.100").To4(),
					Mask: net.CIDRMask(24, 32),
				},
				&net.IPNet{
					IP:   net.ParseIP("192.168.100.101").To4(),
					Mask: net.CIDRMask(24, 32),
				},
				&net.IPNet{
					IP:   net.ParseIP("192.168.100.102").To4(),
					Mask: net.CIDRMask(24, 32),
				},
				&net.IPNet{
					IP:   net.ParseIP("192.169.100.103").To4(),
					Mask: net.CIDRMask(25, 31), // <- invalid
				},
				&net.IPNet{
					IP:   net.ParseIP("192.168.100.104").To4(),
					Mask: net.CIDRMask(24, 32),
				},
			},
			nodeLabels: map[string]string{},
			expectedLabelsToAdd: map[string]string{
				"csi-isilon.dellemc.com/az-192.168.100.0-24-192.168.100.100": "true",
				"csi-isilon.dellemc.com/az-192.168.100.0-24-192.168.100.101": "true",
				"csi-isilon.dellemc.com/az-192.168.100.0-24-192.168.100.102": "true",
				"csi-isilon.dellemc.com/az-192.168.100.0-24-192.168.100.104": "true",
			},
			expectedLabelsToRemove: []string{},
			wantErr:                false,
		},
		{
			name: "failure to get node labels",
			addrs: []net.Addr{
				&net.IPNet{
					IP:   net.ParseIP("192.168.1.1").To4(),
					Mask: net.CIDRMask(24, 32),
				},
			},
			expectedLabelsToAdd: map[string]string{
				"csi-isilon.dellemc.com/az-192.168.1.0-24-192.168.1.1": "true",
			},
			expectedLabelsToRemove: []string{},
			getNodeLabelsErr:       errors.New("permission denied"),
			wantErr:                false,
		},
		{
			name: "failure to patch node labels",
			addrs: []net.Addr{
				&net.IPNet{
					IP:   net.ParseIP("192.168.1.1").To4(),
					Mask: net.CIDRMask(24, 32),
				},
			},
			expectedLabelsToAdd: map[string]string{
				"csi-isilon.dellemc.com/az-192.168.1.0-24-192.168.1.1": "true",
			},
			expectedLabelsToRemove: []string{},
			patchNodeLabelsErr:     errors.New("injected failed"),
			wantErr:                true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer after()

			s := &service{
				nodeID: "test-node",
			}
			getInterfaceAddrsFunc = func() func() ([]net.Addr, error) {
				return func() ([]net.Addr, error) {
					return tt.addrs, tt.addrErr
				}
			}
			getNodeLabelsFunc = func(_ *service) func() (map[string]string, error) {
				return func() (map[string]string, error) {
					return tt.nodeLabels, tt.getNodeLabelsErr
				}
			}
			getPatchNodeLabelsFunc = func(_ *service) func(map[string]string, []string) error {
				return func(labelsToAdd map[string]string, labelsToRemove []string) error {
					if !reflect.DeepEqual(labelsToAdd, tt.expectedLabelsToAdd) {
						t.Errorf("labelsToAdd = %v, want %v", labelsToAdd, tt.expectedLabelsToAdd)
					}
					if !reflect.DeepEqual(labelsToRemove, tt.expectedLabelsToRemove) {
						t.Errorf("labelsToRemove = %v, want %v", labelsToRemove, tt.expectedLabelsToRemove)
					}
					return tt.patchNodeLabelsErr
				}
			}

			if err := s.ReconcileNodeAzLabels(context.Background()); (err != nil) != tt.wantErr {
				t.Errorf("ReconcileNodeAzLabels() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
