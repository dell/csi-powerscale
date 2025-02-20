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
	"io/fs"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	isi "github.com/dell/goisilon"
	"golang.org/x/net/context"
)

// TODO: Fix this test and uncomment it.
/*
func TestNodeGetVolumeStats(t *testing.T) {
	originalGetIsVolumeExistentFunc := getIsVolumeExistentFunc
	getIsVolumeExistentFunc = func(isiConfig *IsilonClusterConfig) func(ctx context.Context, isiPath, volID, name string) bool {
		return func(ctx context.Context, isiPath, volID, name string) bool {
			return true
		}
	}

	defer func() { getIsVolumeExistentFunc = originalGetIsVolumeExistentFunc }()

	originalGetIsVolumeMounted := getIsVolumeMounted
	getIsVolumeMounted = func(ctx context.Context, filterStr string, target string) (bool, error) {
		return true, nil
	}
	defer func() { getIsVolumeMounted = originalGetIsVolumeMounted }()

	originalGetOsReadDir := getOsReadDir
	getOsReadDir = func(path string) ([]os.DirEntry, error) {
		return []os.DirEntry{}, nil
	}
	defer func() { getOsReadDir = originalGetOsReadDir }()

	tests := []struct {
		name         string
		ctx          context.Context
		req          *csi.NodeGetVolumeStatsRequest
		wantResponse *csi.NodeGetVolumeStatsResponse
		wantErr      bool
	}{
		{
			name: "Valid volume ID",
			ctx:  context.Background(),
			req: &csi.NodeGetVolumeStatsRequest{
				VolumeId:   "volume-id",
				VolumePath: "/path/to/volume",
			},
			wantResponse: &csi.NodeGetVolumeStatsResponse{
				Usage: []*csi.VolumeUsage{
					{},
				},
				VolumeCondition: &csi.VolumeCondition{
					Abnormal: false,
					Message:  "failed to get volume stats metrics : rpc error: code = Internal desc = failed to get volume stats: no such file or directory",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			s := &service{
				defaultIsiClusterName: "TestCluster",
				isiClusters:           IsiClusters,
			}
			got, err := s.NodeGetVolumeStats(tt.ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("NodeGetVolumeStats() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.wantResponse) {
				t.Errorf("NodeGetVolumeStats() = %v, want %v", got, tt.wantResponse)
			}
		})
	}
}*/

func TestEphemeralNodePublish(t *testing.T) {
	ctx := context.Background()
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
	}

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
	s := &service{
		defaultIsiClusterName: "TestCluster",
		isiClusters:           IsiClusters,
		nodeIP:                "1.2.3.4",
		nodeID:                "TestNodeID",
		opts: Opts{
			AccessZone:            "TestAccessZone",
			CustomTopologyEnabled: true,
		},
	}

	type testCase struct {
		name string
		req  *csi.NodePublishVolumeRequest
		//mockedFuncs mockedFuncsStruct
		expected *csi.NodePublishVolumeResponse
		wantErr  bool
		setup    func()
	}

	testCases := []testCase{
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
			},
			setup: func() {
				getCreateVolumeFunc = func(s *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
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
			},
			setup: func() {

				getControllerPublishVolume = func(s *service) func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
					return func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
						return &csi.ControllerPublishVolumeResponse{}, nil
					}
				}
				getCreateVolumeFunc = func(s *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
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

		// new
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
			},
			setup: func() {
				publishVolumeFunc = func(_ context.Context, _ *csi.NodePublishVolumeRequest, _ string) error {
					return nil
				}
				getVolByNameFunc = func(_ *service, _ context.Context, _ string, _ string, _ *IsilonClusterConfig) (isi.Volume, error) {
					return nil, nil
				}

				getControllerPublishVolume = func(s *service) func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
					return func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
						return &csi.ControllerPublishVolumeResponse{}, nil
					}
				}
				getCreateVolumeFunc = func(s *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
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
				getControllerPublishVolume = func(s *service) func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
					return func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
						return &csi.ControllerPublishVolumeResponse{}, nil
					}
				}
				getCreateVolumeFunc = func(s *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
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

				ephemeralNodeUnpublishFunc = func(s *service, ctx context.Context, req *csi.NodeUnpublishVolumeRequest) error {
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
				getControllerPublishVolume = func(s *service) func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
					return func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
						return &csi.ControllerPublishVolumeResponse{}, nil
					}
				}
				getCreateVolumeFunc = func(s *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
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

				ephemeralNodeUnpublishFunc = func(s *service, ctx context.Context, req *csi.NodeUnpublishVolumeRequest) error {
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
			},
			setup: func() {
				publishVolumeFunc = func(_ context.Context, _ *csi.NodePublishVolumeRequest, _ string) error {
					return nil
				}
				getVolByNameFunc = func(_ *service, _ context.Context, _ string, _ string, _ *IsilonClusterConfig) (isi.Volume, error) {
					return nil, nil
				}

				getControllerPublishVolume = func(s *service) func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
					return func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
						return &csi.ControllerPublishVolumeResponse{}, nil
					}
				}
				getCreateVolumeFunc = func(s *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
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

				ephemeralNodeUnpublishFunc = func(s *service, ctx context.Context, req *csi.NodeUnpublishVolumeRequest) error {
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
			},
			setup: func() {
				publishVolumeFunc = func(_ context.Context, _ *csi.NodePublishVolumeRequest, _ string) error {
					return nil
				}
				getVolByNameFunc = func(_ *service, _ context.Context, _ string, _ string, _ *IsilonClusterConfig) (isi.Volume, error) {
					return nil, nil
				}

				getControllerPublishVolume = func(s *service) func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
					return func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
						return &csi.ControllerPublishVolumeResponse{}, nil
					}
				}
				getCreateVolumeFunc = func(s *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
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

				ephemeralNodeUnpublishFunc = func(s *service, ctx context.Context, req *csi.NodeUnpublishVolumeRequest) error {
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
			},
			setup: func() {
				closeFileFunc = func(_ *os.File) error {
					return errors.New("fail to close file")
				}
				writeStringFunc = func(f *os.File, _ string) (int, error) {
					return 0, errors.New("fail to write to file")
				}
				publishVolumeFunc = func(_ context.Context, _ *csi.NodePublishVolumeRequest, _ string) error {
					return nil
				}
				getVolByNameFunc = func(_ *service, _ context.Context, _ string, _ string, _ *IsilonClusterConfig) (isi.Volume, error) {
					return nil, nil
				}

				getControllerPublishVolume = func(s *service) func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
					return func(_ context.Context, _ *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
						return &csi.ControllerPublishVolumeResponse{}, nil
					}
				}
				getCreateVolumeFunc = func(s *service) func(_ context.Context, _ *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
					return func(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
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
				t.Errorf("NodeGetVolumeStats() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("NodeGetVolumeStats() = %v, want %v", got, tc.expected)
			}
		})
	}
}
