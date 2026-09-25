// Copyright © 2019-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	isi "github.com/Ecosystems/container-storage-modules/src/gopowerscale"
	"github.com/Ecosystems/container-storage-modules/src/gopowerscale/api"
	apiv1 "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api/v1"
	apiv14 "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api/v14"
	apiv17 "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api/v17"
	apiv2 "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api/v2"
	apiv5 "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api/v5"
	isimocks "github.com/Ecosystems/container-storage-modules/src/gopowerscale/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCopySnapshot(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: mockClient,
		},
	}

	// Define the test cases
	testCases := []struct {
		name                        string
		isiPath                     string
		snapshotSourceVolumeIsiPath string
		srcSnapshotID               int64
		dstVolumeName               string
		accessZone                  string
		expected                    isi.Volume
		err                         error
	}{
		{
			name:                        "error case",
			isiPath:                     "/ifs/data",
			snapshotSourceVolumeIsiPath: "/ifs/data/snapshots",
			srcSnapshotID:               456,
			dstVolumeName:               "new_volume",
			accessZone:                  "System",
			expected:                    nil,
			err:                         errors.New("mock error"),
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			mockClient.On("Get", anyArgs...).Return(errors.New("mock error")).Once()
			volumeNew, err := svc.CopySnapshot(ctx, tc.isiPath, tc.snapshotSourceVolumeIsiPath, tc.srcSnapshotID, tc.dstVolumeName, tc.accessZone)
			if err != nil {
				if tc.err == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.err.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.err, err)
				}
			} else {
				if tc.err != nil {
					t.Errorf("Expected error '%v', but got nil", tc.err)
				} else {
					// Check if the returned volume matches the expected volume
					if !reflect.DeepEqual(volumeNew, tc.expected) {
						t.Errorf("Expected volume '%v', but got '%v'", tc.expected, volumeNew)
					}
				}
			}
		})
	}
}

func TestCopyVolume(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: mockClient,
		},
	}

	// Define the test cases
	testCases := []struct {
		name          string
		isiPath       string
		srcVolumeName string
		dstVolumeName string
		expected      isi.Volume
		err           error
	}{
		{
			name:          "error case",
			isiPath:       "/ifs/data",
			srcVolumeName: "src_volume",
			dstVolumeName: "dst_volume",
			expected:      nil,
			err:           errors.New("mock error"),
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc.client.API.(*isimocks.Client).On("Put", anyArgs...).Return(errors.New("mock error")).Once()
			volumeNew, err := svc.CopyVolume(ctx, tc.isiPath, tc.srcVolumeName, tc.dstVolumeName)
			if err != nil {
				if tc.err == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.err.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.err, err)
				}
			} else {
				if tc.err != nil {
					t.Errorf("Expected error '%v', but got nil", tc.err)
				} else {
					// Check if the returned volume matches the expected volume
					if !reflect.DeepEqual(volumeNew, tc.expected) {
						t.Errorf("Expected volume '%v', but got '%v'", tc.expected, volumeNew)
					}
				}
			}
		})
	}
}

func TestCreateVolume(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: mockClient,
		},
	}

	// Define the test cases
	testCases := []struct {
		name                     string
		isiPath                  string
		volName                  string
		isiVolumePathPermissions string
		expected                 isi.Volume
		err                      error
	}{
		{
			name:                     "error case",
			isiPath:                  "/ifs/data",
			volName:                  "test_volume",
			isiVolumePathPermissions: "755",
			expected:                 nil,
			err:                      errors.New("mock error"),
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc.client.API.(*isimocks.Client).On("Put", anyArgs...).Return(errors.New("mock error")).Once()
			err := svc.CreateVolume(ctx, tc.isiPath, tc.volName, tc.isiVolumePathPermissions)
			if err != nil {
				if tc.err == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.err.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.err, err)
				}
			} else {
				if tc.err != nil {
					t.Errorf("Expected error '%v', but got nil", tc.err)
				}
			}
		})
	}
}

func TestCreateVolumeWithMetaData(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: mockClient,
		},
	}

	// Define the test cases
	testCases := []struct {
		name                     string
		isiPath                  string
		volName                  string
		isiVolumePathPermissions string
		metadata                 map[string]string
		expected                 isi.Volume
		err                      error
	}{
		{
			name:                     "error case",
			isiPath:                  "/ifs/data",
			volName:                  "test_volume",
			isiVolumePathPermissions: "755",
			metadata: map[string]string{
				"key3": "value3",
			},
			expected: nil,
			err:      errors.New("mock error"),
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc.client.API.(*isimocks.Client).On("Put", anyArgs...).Return(errors.New("mock error")).Once()
			err := svc.CreateVolumeWithMetaData(ctx, tc.isiPath, tc.volName, tc.isiVolumePathPermissions, tc.metadata)
			if err != nil {
				if tc.err == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.err.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.err, err)
				}
			} else {
				if tc.err != nil {
					t.Errorf("Expected error '%v', but got nil", tc.err)
				}
			}
		})
	}
}

func TestCreateWritableSnapshotLookupFailureReturnsError(t *testing.T) {
	mockClient := &isimocks.Client{}
	svc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: mockClient,
		},
	}

	ctx := context.Background()
	mockClient.On("Post", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(6).(**apiv14.IsiWritableSnapshotResponse)
		*resp = &apiv14.IsiWritableSnapshotResponse{
			DstPath: "/ifs/data/dst",
			State:   "available",
		}
	}).Once()
	mockClient.On("Get", anyArgs...).Return(errors.New("lookup failed")).Once()

	vol, err := svc.CreateWritableSnapshot(ctx, "/ifs/data", "/ifs/data/dst", "snap-123", "new-vol", "System")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lookup failed")
	assert.Nil(t, vol)
}

func TestGetLicenseByID(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error from gopowerscale client", func(t *testing.T) {
		mockClient := &isimocks.Client{}
		mockClient.On("Get", mock.Anything, "platform/17/license/licenses", "SNAPSHOTIQ", mock.Anything, mock.Anything, mock.Anything).
			Return(errors.New("mock error")).Once()

		svc := &isiService{
			endpoint: "http://localhost:8080",
			client: &isi.Client{
				API: mockClient,
			},
		}

		license, err := svc.GetLicenseByID(ctx, "SNAPSHOTIQ")
		assert.Error(t, err)
		assert.Nil(t, license)
		assert.Contains(t, err.Error(), "mock error")
		mockClient.AssertExpectations(t)
	})

	t.Run("returns license when lookup succeeds", func(t *testing.T) {
		mockClient := &isimocks.Client{}
		mockClient.On("Get", mock.Anything, "platform/17/license/licenses", "SNAPSHOTIQ", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				resp := args.Get(5).(*apiv17.LicensesResponse)
				resp.Licenses = []apiv17.License{{ID: "SNAPSHOTIQ", Status: "Licensed"}}
			}).
			Return(nil).Once()

		svc := &isiService{
			endpoint: "http://localhost:8080",
			client: &isi.Client{
				API: mockClient,
			},
		}

		license, err := svc.GetLicenseByID(ctx, "SNAPSHOTIQ")
		assert.NoError(t, err)
		if assert.NotNil(t, license) {
			assert.Equal(t, "SNAPSHOTIQ", license.ID)
			assert.Equal(t, "Licensed", license.Status)
		}
		mockClient.AssertExpectations(t)
	})
}

func TestGetVolumeQuota(t *testing.T) {
	testCases := []struct {
		name         string
		setup        func(svc *isiService)
		volName      string
		exportID     int
		accessZone   string
		expectedQuot isi.Quota
		expectedErr  error
	}{
		{
			name: "failed to get export",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
			},
			volName:      "test_volume",
			exportID:     456,
			accessZone:   "System",
			expectedQuot: nil,
			expectedErr:  errors.New("failed to get export 'test_volume':'456' with access zone 'System', error: 'mock error'"),
		},
		{
			name: "nil export",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(nil).Once()
			},
			volName:      "test_volume",
			exportID:     456,
			accessZone:   "System",
			expectedQuot: nil,
			expectedErr:  errors.New("failed to get quota for volume 'test_volume'"),
		},
		{
			name: "no quota id for export",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(*apiv2.ExportList)
					*resp = apiv2.ExportList{
						&apiv2.Export{},
					}
				})
			},
			volName:      "test_volume",
			exportID:     456,
			accessZone:   "System",
			expectedQuot: nil,
			expectedErr:  errors.New("failed to get quota: No quota set on the volume 'test_volume'"),
		},
		{
			name: "success case",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(*apiv2.ExportList)
					*resp = apiv2.ExportList{
						&apiv2.Export{
							ID:          456,
							Description: "CSI_QUOTA_ID:123",
						},
					}
				}).Once().On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(*apiv1.IsiQuotaListResp)
					*resp = apiv1.IsiQuotaListResp{
						Quotas: []apiv1.IsiQuota{
							{
								ID: "123",
							},
						},
					}
				})
			},
			volName:    "test_volume",
			exportID:   456,
			accessZone: "System",
			expectedQuot: &apiv1.IsiQuota{
				ID: "123",
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc := &isiService{
				endpoint: "http://localhost:8080",
				client: &isi.Client{
					API: &isimocks.Client{},
				},
			}

			if tc.setup != nil {
				tc.setup(svc)
			}

			quota, err := svc.GetVolumeQuota(ctx, tc.volName, tc.exportID, tc.accessZone)
			if err != nil {
				if tc.expectedErr == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.expectedErr.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.expectedErr, err)
				}
			} else {
				if tc.expectedErr != nil {
					t.Errorf("Expected error '%v', but got nil", tc.expectedErr)
				} else {
					// Check if the returned quota matches the expected quota
					if !reflect.DeepEqual(quota, tc.expectedQuot) {
						t.Errorf("Expected quota '%v', but got '%v'", tc.expectedQuot, quota)
					}
				}
			}
		})
	}
}

func TestCreateQuota(t *testing.T) {
	testCases := []struct {
		name            string
		setup           func(svc *isiService)
		isiPath         string
		volName         string
		softLimit       string
		advisoryLimit   string
		softGracePrd    string
		sizeInBytes     int64
		quotaEnabled    bool
		expectedQuotaID string
		expectedError   error
	}{
		{
			name: "quota not enabled skip creating quotas",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).RunFn = func(_ mock.Arguments) {
					panic("should not be called")
				}
			},
			quotaEnabled: false,
		},
		{
			name: "invalid smart quota value skip create",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(*apiv5.QuotaLicense)
					*resp = apiv5.QuotaLicense{
						STATUS: "invalid",
					}
				}).Once()
			},
			sizeInBytes:  100,
			quotaEnabled: true,
		},
		{
			name: "failed to create quota",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(*apiv5.QuotaLicense)
					*resp = apiv5.QuotaLicense{
						STATUS: "Licensed",
					}
				}).Once()

				svc.client.API.(*isimocks.Client).On("Post", anyArgs...).Return(errors.New("mock error"))
			},
			sizeInBytes:   100,
			quotaEnabled:  true,
			expectedError: errors.New("SmartQuotas is activated, but creating quota failed with error: 'mock error'"),
		},
		{
			name: "invalid soft grace period use default",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(*apiv5.QuotaLicense)
					*resp = apiv5.QuotaLicense{
						STATUS: "Licensed",
					}
				}).Once()

				svc.client.API.(*isimocks.Client).On("Post", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(6).(*apiv1.IsiQuota)
					*resp = apiv1.IsiQuota{
						ID: "mock-id",
					}
				}).Once()
			},
			isiPath:         "/ifs/data/csi-isilon",
			volName:         "volume3",
			softLimit:       "70",
			advisoryLimit:   "invalid",
			softGracePrd:    "invalid",
			sizeInBytes:     100,
			quotaEnabled:    true,
			expectedQuotaID: "mock-id",
		},
		{
			name: "invalid soft limit use default",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(nil).Once()
			},
			isiPath:       "/ifs/data/csi-isilon",
			volName:       "volume3",
			softLimit:     "invalid",
			advisoryLimit: "invalid",
			softGracePrd:  "invalid",
			sizeInBytes:   100,
			quotaEnabled:  true,
		},
		{
			name: "invalid advisory limit use default", // TODO need to validate
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(nil).Once()
			},
			isiPath:       "/ifs/data/csi-isilon",
			volName:       "volume3",
			softLimit:     "100",
			advisoryLimit: "invalid",
			softGracePrd:  "30",
			sizeInBytes:   100,
			quotaEnabled:  true,
		},
		{
			name: "size zero skip creating quotas",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).RunFn = func(_ mock.Arguments) {
					panic("should not be called")
				}
			},
			sizeInBytes:  0,
			quotaEnabled: true,
		},
		{
			name: "size negative skip creating quotas",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).RunFn = func(_ mock.Arguments) {
					panic("should not be called")
				}
			},
			sizeInBytes:  -1,
			quotaEnabled: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc := &isiService{
				endpoint: "http://localhost:8080",
				client: &isi.Client{
					API: &isimocks.Client{},
				},
			}

			if tc.setup != nil {
				tc.setup(svc)
			}

			quotaID, err := svc.CreateQuota(ctx, tc.isiPath, tc.volName, tc.softLimit, tc.advisoryLimit, tc.softGracePrd, tc.sizeInBytes, tc.quotaEnabled)
			if err != nil {
				if tc.expectedError == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.expectedError.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.expectedError, err)
				}
			} else {
				if tc.expectedError != nil {
					t.Errorf("Expected error '%v', but got nil", tc.expectedError)
				} else {
					if quotaID != tc.expectedQuotaID {
						t.Errorf("Expected quota ID '%s', but got '%s'", tc.expectedQuotaID, quotaID)
					}
				}
			}
		})
	}
}

func TestGetExportsWithParams(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: mockClient,
		},
	}

	// Define the test cases
	testCases := []struct {
		name     string
		params   api.OrderedValues
		expected isi.Exports
		err      error
	}{
		{
			name: "error case",
			params: api.OrderedValues{
				{[]byte("zone"), []byte("")},
			},
			expected: nil,
			err:      errors.New("failed to get exports with params"),
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
			exports, err := svc.GetExportsWithParams(ctx, tc.params)
			if err != nil {
				if tc.err == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.err.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.err, err)
				}
			} else {
				if tc.err != nil {
					t.Errorf("Expected error '%v', but got nil", tc.err)
				} else {
					// Check if the returned exports match the expected exports
					if !reflect.DeepEqual(exports, tc.expected) {
						t.Errorf("Expected exports '%v', but got '%v'", tc.expected, exports)
					}
				}
			}
		})
	}
}

func TestGetExportsWithResume(t *testing.T) {
	ctx := context.Background()

	t.Run("error case", func(t *testing.T) {
		mockClient := &isimocks.Client{}
		svc := &isiService{
			endpoint: "http://localhost:8080",
			client:   &isi.Client{API: mockClient},
		}
		mockClient.On("Get", anyArgs...).Return(errors.New("failed to get exports")).Once()
		exports, resume, err := svc.GetExportsWithResume(ctx, "")
		assert.Error(t, err)
		assert.Nil(t, exports)
		assert.Empty(t, resume)
	})

	t.Run("success case", func(t *testing.T) {
		mockClient := &isimocks.Client{}
		svc := &isiService{
			endpoint: "http://localhost:8080",
			client:   &isi.Client{API: mockClient},
		}
		mockClient.On("Get", anyArgs...).Run(func(args mock.Arguments) {
			resp := args.Get(5).(*apiv2.Exports)
			resp.Resume = "next-token"
		}).Return(nil).Once()
		exports, resume, err := svc.GetExportsWithResume(ctx, "")
		assert.NoError(t, err)
		assert.Equal(t, "next-token", resume)
		assert.Nil(t, exports)
	})
}

func TestGetVolumeSize(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: mockClient,
		},
	}

	// Define the test cases
	testCases := []struct {
		name         string
		isiPath      string
		volName      string
		expectedSize int64
		expectedErr  error
	}{
		{
			name:         "error case",
			isiPath:      "/ifs/data",
			volName:      "test_volume",
			expectedSize: 0,
			expectedErr:  errors.New("mock error"),
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
			size := svc.GetVolumeSize(ctx, tc.isiPath, tc.volName)
			assert.Equal(t, tc.expectedSize, size)
		})
	}
}

func TestIsIOInProgress(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: mockClient,
		},
	}

	// Define the test cases
	testCases := []struct {
		name            string
		expectedClients isi.Clients
		expectedErr     error
	}{
		{
			name:            "error case",
			expectedClients: nil,
			expectedErr:     errors.New("mock error"),
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
			clients, err := svc.IsIOInProgress(ctx)
			if err != nil {
				if tc.expectedErr == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.expectedErr.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.expectedErr, err)
				}
			} else {
				if tc.expectedErr != nil {
					t.Errorf("Expected error '%v', but got nil", tc.expectedErr)
				} else {
					// Check if the returned clients match the expected clients
					if !reflect.DeepEqual(clients, tc.expectedClients) {
						t.Errorf("Expected clients '%v', but got '%v'", tc.expectedClients, clients)
					}
				}
			}
		})
	}
}

func TestOtherClientsAlreadyAdded(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: mockClient,
		},
	}

	ctx := context.Background()

	t.Run("export not found returns true", func(t *testing.T) {
		mockClient.On("Get", anyArgs...).Return(errors.New("mock error")).Once()
		result := svc.OtherClientsAlreadyAdded(ctx, 456, "System", "node2")
		assert.True(t, result)
	})

	t.Run("invalid node ID returns true", func(t *testing.T) {
		mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(*apiv2.ExportList)
			clients := []string{"client1"}
			roClients := []string{}
			rwClients := []string{}
			rootClients := []string{}
			*resp = apiv2.ExportList{
				&apiv2.Export{
					ID:               1,
					Clients:          &clients,
					ReadOnlyClients:  &roClients,
					ReadWriteClients: &rwClients,
					RootClients:      &rootClients,
				},
			}
		}).Once()
		result := svc.OtherClientsAlreadyAdded(ctx, 1, "System", "")
		assert.True(t, result)
	})

	t.Run("other clients exist returns true", func(t *testing.T) {
		mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(*apiv2.ExportList)
			clients := []string{"other-client", "another-client"}
			roClients := []string{}
			rwClients := []string{}
			rootClients := []string{}
			*resp = apiv2.ExportList{
				&apiv2.Export{
					ID:               1,
					Clients:          &clients,
					ReadOnlyClients:  &roClients,
					ReadWriteClients: &rwClients,
					RootClients:      &rootClients,
				},
			}
		}).Once()
		result := svc.OtherClientsAlreadyAdded(ctx, 1, "System", "node1=#=#=node1.example.com=#=#=10.0.0.1")
		assert.True(t, result)
	})

	t.Run("only this node exists returns false", func(t *testing.T) {
		mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(*apiv2.ExportList)
			clients := []string{"node1"}
			roClients := []string{}
			rwClients := []string{}
			rootClients := []string{}
			*resp = apiv2.ExportList{
				&apiv2.Export{
					ID:               1,
					Clients:          &clients,
					ReadOnlyClients:  &roClients,
					ReadWriteClients: &rwClients,
					RootClients:      &rootClients,
				},
			}
		}).Once()
		result := svc.OtherClientsAlreadyAdded(ctx, 1, "System", "node1=#=#=node1.example.com=#=#=10.0.0.1")
		assert.False(t, result)
	})

	t.Run("only localhost exists returns false", func(t *testing.T) {
		mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(*apiv2.ExportList)
			clients := []string{"localhost"}
			roClients := []string{}
			rwClients := []string{}
			rootClients := []string{}
			*resp = apiv2.ExportList{
				&apiv2.Export{
					ID:               1,
					Clients:          &clients,
					ReadOnlyClients:  &roClients,
					ReadWriteClients: &rwClients,
					RootClients:      &rootClients,
				},
			}
		}).Once()
		result := svc.OtherClientsAlreadyAdded(ctx, 1, "System", "node1=#=#=node1.example.com=#=#=10.0.0.1")
		assert.False(t, result)
	})

	t.Run("node IP in client fields returns false", func(t *testing.T) {
		mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(*apiv2.ExportList)
			clients := []string{"10.0.0.1"}
			roClients := []string{}
			rwClients := []string{}
			rootClients := []string{}
			*resp = apiv2.ExportList{
				&apiv2.Export{
					ID:               1,
					Clients:          &clients,
					ReadOnlyClients:  &roClients,
					ReadWriteClients: &rwClients,
					RootClients:      &rootClients,
				},
			}
		}).Once()
		result := svc.OtherClientsAlreadyAdded(ctx, 1, "System", "node1=#=#=node1.example.com=#=#=10.0.0.1")
		assert.False(t, result)
	})
}

func TestAddExportClientNetworkIdentifierByIDWithZone(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: mockClient,
		},
	}

	// Define the test cases
	testCases := []struct {
		name                    string
		clusterName             string
		exportID                int
		accessZone              string
		nodeID                  string
		ignoreUnresolvableHosts bool
		addClientFunc           func(ctx context.Context, exportID int, accessZone, clientIP string, ignoreUnresolvableHosts bool) error
		expectedErr             error
	}{
		{
			name:                    "error case - invalid node ID",
			clusterName:             "cluster2",
			exportID:                456,
			accessZone:              "System",
			nodeID:                  "!@$%~^",
			ignoreUnresolvableHosts: true,
			addClientFunc: func(_ context.Context, _ int, _, _ string, _ bool) error {
				return nil
			},
			expectedErr: errors.New("node ID '!@$%~^' cannot match the expected '^(.+)=#=#=(.+)=#=#=(.+)$' pattern"),
		},
		{
			name:                    "success case with ignoreUnresolvableHosts",
			clusterName:             "cluster1",
			exportID:                123,
			accessZone:              "System",
			nodeID:                  "node1=#=#=node1.example.com=#=#=192.168.1.1",
			ignoreUnresolvableHosts: true,
			addClientFunc: func(_ context.Context, _ int, _, _ string, _ bool) error {
				return nil
			},
			expectedErr: nil,
		},
		{
			name:                    "error case with ignoreUnresolvableHosts - addClientFunc fails",
			clusterName:             "cluster1",
			exportID:                123,
			accessZone:              "System",
			nodeID:                  "node1=#=#=node1.example.com=#=#=192.168.1.1",
			ignoreUnresolvableHosts: true,
			addClientFunc: func(_ context.Context, _ int, _, _ string, _ bool) error {
				return errors.New("add client failed")
			},
			expectedErr: errors.New("failed to add client '192.168.1.1' to the export id '123'"),
		},
		{
			name:                    "success case without ignoreUnresolvableHosts - FQDN works",
			clusterName:             "cluster1",
			exportID:                123,
			accessZone:              "System",
			nodeID:                  "node1=#=#=node1.example.com=#=#=192.168.1.1",
			ignoreUnresolvableHosts: false,
			addClientFunc: func(_ context.Context, _ int, _, _ string, _ bool) error {
				// First call with FQDN succeeds
				return nil
			},
			expectedErr: nil,
		},
		{
			name:                    "success case without ignoreUnresolvableHosts - FQDN fails, IP works",
			clusterName:             "cluster1",
			exportID:                124,
			accessZone:              "System",
			nodeID:                  "node2=#=#=node2.example.com=#=#=192.168.1.2",
			ignoreUnresolvableHosts: false,
			addClientFunc: func() func(ctx context.Context, exportID int, accessZone, clientIP string, ignoreUnresolvableHosts bool) error {
				callCount := 0
				return func(_ context.Context, _ int, _, _ string, _ bool) error {
					callCount++
					if callCount == 1 {
						// First call with FQDN fails
						return errors.New("FQDN resolution failed")
					}
					// Second call with IP succeeds
					return nil
				}
			}(),
			expectedErr: nil,
		},
		{
			name:                    "error case without ignoreUnresolvableHosts - both FQDN and IP fail",
			clusterName:             "cluster1",
			exportID:                125,
			accessZone:              "System",
			nodeID:                  "node3=#=#=node3.example.com=#=#=192.168.1.3",
			ignoreUnresolvableHosts: false,
			addClientFunc: func(_ context.Context, _ int, _, _ string, _ bool) error {
				return errors.New("add client failed")
			},
			expectedErr: errors.New("failed to add clients 'node3.example.com' or '192.168.1.3' to export id '125'"),
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.AddExportClientNetworkIdentifierByIDWithZone(context.Background(), tc.clusterName, tc.exportID, tc.accessZone, tc.nodeID, tc.ignoreUnresolvableHosts, tc.addClientFunc)

			if err != nil {
				if tc.expectedErr == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.expectedErr.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.expectedErr, err)
				}
			} else {
				if tc.expectedErr != nil {
					t.Errorf("Expected error '%v', but got nil", tc.expectedErr)
				}
			}
		})
	}
}

func TestAddExportClientByIDWithZone(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	// Define the test cases
	testCases := []struct {
		name                    string
		exportID                int
		accessZone              string
		clientIP                string
		ignoreUnresolvableHosts bool
		expectedErr             error
	}{
		{
			name:                    "error case",
			exportID:                456,
			accessZone:              "System",
			clientIP:                "5.6.7.8",
			ignoreUnresolvableHosts: true,
			expectedErr:             errors.New("failed to add client to export id '456' with access zone 'System' : 'mock error'"),
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
			err := svc.AddExportClientByIDWithZone(ctx, tc.exportID, tc.accessZone, tc.clientIP, tc.ignoreUnresolvableHosts)

			if err != nil {
				if tc.expectedErr == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.expectedErr.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.expectedErr, err)
				}
			} else {
				if tc.expectedErr != nil {
					t.Errorf("Expected error '%v', but got nil", tc.expectedErr)
				}
			}
		})
	}
}

func TestAddExportRootClientByIDWithZone(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	// Define the test cases
	testCases := []struct {
		name        string
		exportID    int
		accessZone  string
		clientIP    string
		expectedErr error
	}{
		{
			name:        "error case",
			exportID:    456,
			accessZone:  "System",
			clientIP:    "5.6.7.8",
			expectedErr: errors.New("failed to add client to export id '456' with access zone 'System' : 'mock error'"),
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
			err := svc.AddExportRootClientByIDWithZone(ctx, tc.exportID, tc.accessZone, tc.clientIP, false)

			if err != nil {
				if tc.expectedErr == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.expectedErr.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.expectedErr, err)
				}
			} else {
				if tc.expectedErr != nil {
					t.Errorf("Expected error '%v', but got nil", tc.expectedErr)
				}
			}
		})
	}
}

func TestAddExportReadOnlyClientByIDWithZone(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	// Define the test cases
	testCases := []struct {
		name                    string
		exportID                int
		accessZone              string
		clientIP                string
		ignoreUnresolvableHosts bool
		expectedErr             error
	}{
		{
			name:                    "error case",
			exportID:                456,
			accessZone:              "System",
			clientIP:                "5.6.7.8",
			ignoreUnresolvableHosts: true,
			expectedErr:             errors.New("failed to add read only client to export id '456' with access zone 'System' : 'mock error'"),
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
			err := svc.AddExportReadOnlyClientByIDWithZone(ctx, tc.exportID, tc.accessZone, tc.clientIP, tc.ignoreUnresolvableHosts)

			if err != nil {
				if tc.expectedErr == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.expectedErr.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.expectedErr, err)
				}
			} else {
				if tc.expectedErr != nil {
					t.Errorf("Expected error '%v', but got nil", tc.expectedErr)
				}
			}
		})
	}
}

func TestAddExportClientByIPWithZone(t *testing.T) {
	// Define the test cases
	testCases := []struct {
		name          string
		clusterName   string
		exportID      int
		accessZone    string
		nodeID        string
		clientIPs     []string
		addClientFunc func(ctx context.Context, exportID int, accessZone string, clientIP string, ignoreUnresolvableHosts bool) error
		expectedErr   error
	}{
		{
			name:        "Success",
			clusterName: "test",
			exportID:    456,
			accessZone:  "System",
			nodeID:      "node1",
			clientIPs:   []string{"5.6.7.8"},
			addClientFunc: func(_ context.Context, _ int, _ string, _ string, _ bool) error {
				return nil
			},
			expectedErr: nil,
		},
		{
			name:        "Error adding clients",
			clusterName: "test",
			exportID:    456,
			accessZone:  "System",
			nodeID:      "node1",
			clientIPs:   []string{},
			addClientFunc: func(_ context.Context, _ int, _ string, _ string, _ bool) error {
				return errors.New("error")
			},
			expectedErr: fmt.Errorf("failed to add clients '%v' to export id '%d'", []string{}, 456),
		},
		{
			name:        "Error no client IPs",
			clusterName: "test",
			exportID:    456,
			accessZone:  "System",
			nodeID:      "node1",
			clientIPs:   []string{},
			addClientFunc: func(_ context.Context, _ int, _ string, _ string, _ bool) error {
				return nil
			},
			expectedErr: fmt.Errorf("failed to add clients '%v' to export id '%d'", []string{}, 456),
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			svc := &isiService{
				client: &isi.Client{
					API: &isimocks.Client{},
				},
			}

			err := svc.AddExportClientByIPWithZone(ctx, tc.clusterName, tc.exportID, tc.accessZone, tc.nodeID, tc.clientIPs, tc.addClientFunc, nil, constants.AllowedNetworksModeDefault)

			if err != nil {
				if tc.expectedErr == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.expectedErr.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.expectedErr, err)
				}
			} else {
				if tc.expectedErr != nil {
					t.Errorf("Expected error '%v', but got nil", tc.expectedErr)
				}
			}
		})
	}
}

func TestAddExportClientByIPWithZoneMultiMode(t *testing.T) {
	ctx := context.Background()

	t.Run("Multi mode adds all IPs", func(t *testing.T) {
		svc := &isiService{client: &isi.Client{API: &isimocks.Client{}}}
		var addedIPs []string
		addFunc := func(_ context.Context, _ int, _ string, ip string, _ bool) error {
			addedIPs = append(addedIPs, ip)
			return nil
		}
		err := svc.AddExportClientByIPWithZone(ctx, "cluster1", 1, "System", "node1",
			[]string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}, addFunc, nil, constants.AllowedNetworksModeMulti)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(addedIPs))
	})

	t.Run("Multi mode partial failure succeeds", func(t *testing.T) {
		svc := &isiService{client: &isi.Client{API: &isimocks.Client{}}}
		callCount := 0
		addFunc := func(_ context.Context, _ int, _ string, _ string, _ bool) error {
			callCount++
			if callCount == 2 {
				return errors.New("transient error")
			}
			return nil
		}
		err := svc.AddExportClientByIPWithZone(ctx, "cluster1", 1, "System", "node1",
			[]string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}, addFunc, nil, constants.AllowedNetworksModeMulti)
		assert.NoError(t, err)
	})

	t.Run("Multi mode all fail returns error", func(t *testing.T) {
		svc := &isiService{client: &isi.Client{API: &isimocks.Client{}}}
		addFunc := func(_ context.Context, _ int, _ string, _ string, _ bool) error {
			return errors.New("all fail")
		}
		err := svc.AddExportClientByIPWithZone(ctx, "cluster1", 1, "System", "node1",
			[]string{"10.0.0.1", "10.0.0.2"}, addFunc, nil, constants.AllowedNetworksModeMulti)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to add any of clients")
	})

	t.Run("Multi mode batches all IPs with addClientsFunc", func(t *testing.T) {
		svc := &isiService{client: &isi.Client{API: &isimocks.Client{}}}
		var batchIPs []string
		addClientsFunc := func(_ context.Context, _ int, _ string, clientIPs []string, _ bool) error {
			batchIPs = append(batchIPs, clientIPs...)
			return nil
		}
		addFunc := func(_ context.Context, _ int, _ string, _ string, _ bool) error {
			t.Fatal("expected batch addClientsFunc, not per-IP addClientFunc")
			return nil
		}
		err := svc.AddExportClientByIPWithZone(ctx, "cluster1", 1, "System", "node1",
			[]string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}, addFunc, addClientsFunc, constants.AllowedNetworksModeMulti)
		assert.NoError(t, err)
		assert.Equal(t, []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}, batchIPs)
	})

	t.Run("Single mode stops at first success", func(t *testing.T) {
		svc := &isiService{client: &isi.Client{API: &isimocks.Client{}}}
		var addedIPs []string
		addFunc := func(_ context.Context, _ int, _ string, ip string, _ bool) error {
			addedIPs = append(addedIPs, ip)
			return nil
		}
		err := svc.AddExportClientByIPWithZone(ctx, "cluster1", 1, "System", "node1",
			[]string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}, addFunc, nil, constants.AllowedNetworksModeDefault)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(addedIPs))
	})
}

func TestRemoveExportClientByIDWithZone(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	// Define the test cases
	testCases := []struct {
		name                    string
		exportID                int
		accessZone              string
		clientIP                string
		ignoreUnresolvableHosts bool
		expectedErr             error
	}{
		{
			name:                    "Node id doesn't match pattern",
			exportID:                456,
			accessZone:              "System",
			clientIP:                "5.6.7.8",
			ignoreUnresolvableHosts: true,
			expectedErr:             errors.New("node ID '5.6.7.8' cannot match the expected '^(.+)=#=#=(.+)=#=#=(.+)$' pattern"),
		},
		{
			name:                    "error case",
			exportID:                456,
			accessZone:              "System",
			clientIP:                "abc=#=#=def=#=#=xyz",
			ignoreUnresolvableHosts: true,
			expectedErr:             errors.New("failed to remove clients from export '456' with access zone 'System' : 'mock error'"),
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
			err := svc.RemoveExportClientByIDWithZone(ctx, tc.exportID, tc.accessZone, tc.clientIP, tc.ignoreUnresolvableHosts)

			if err != nil {
				if tc.expectedErr == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.expectedErr.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.expectedErr, err)
				}
			} else {
				if tc.expectedErr != nil {
					t.Errorf("Expected error '%v', but got nil", tc.expectedErr)
				}
			}
		})
	}
}

func TestCreateSnapshot(t *testing.T) {
	tests := []struct {
		name             string
		path             string
		snapshotName     string
		setup            func(svc *isiService)
		expectedSnapshot isi.Snapshot
		wantErr          error
	}{
		{
			name:         "success case",
			path:         "/ifs/data/csi-isilon/volume2",
			snapshotName: "ut-snapshot",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Post", anyArgs...).Return(nil)
			},
		},
		{
			name:         "failure case",
			path:         "/ifs/data/csi-isilon/volume2",
			snapshotName: "ut-snapshot",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Post", anyArgs...).Return(errors.New("mock error"))
			},
			wantErr: errors.New("mock error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &isiService{
				endpoint: "http://localhost:8080",
				client: &isi.Client{
					API: &isimocks.Client{},
				},
			}

			if tc.setup != nil {
				tc.setup(svc)
			}

			ctx := context.Background()
			snapshot, err := svc.CreateSnapshot(ctx, tc.path, tc.snapshotName)
			if err != nil {
				if tc.wantErr == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.wantErr.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.wantErr, err)
				}
			} else {
				if tc.wantErr != nil {
					t.Errorf("Expected error '%v', but got nil", tc.wantErr)
				} else {
					if !reflect.DeepEqual(snapshot, tc.expectedSnapshot) {
						t.Errorf("Expected snapshot '%v', but got '%v'", tc.expectedSnapshot, snapshot)
					}
				}
			}
		})
	}
}

func TestDeleteSnapshot(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	tests := []struct {
		name         string
		snapshotID   int64
		snapshotName string
		expectedErr  error
	}{
		{
			name:         "Snapshot not found",
			snapshotID:   2,
			snapshotName: "snapshot2",
			expectedErr:  errors.New("mock error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(errors.New("mock error")).Twice()
			err := svc.DeleteSnapshot(ctx, tc.snapshotID, tc.snapshotName)
			if err != nil {
				if tc.expectedErr == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.expectedErr.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.expectedErr, err)
				}
			} else {
				if tc.expectedErr != nil {
					t.Errorf("Expected error '%v', but got nil", tc.expectedErr)
				}
			}
		})
	}
}

func TestGetSnapshotIsiPathComponents(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	tests := []struct {
		name                 string
		snapshotIsiPath      string
		zonePath             string
		expectedIsiPath      string
		expectedSnapshotName string
		expectedSrcVolName   string
	}{
		{
			name:                 "Invalid snapshot isi path",
			snapshotIsiPath:      "/ifs/path/to/volume",
			zonePath:             "/ifs",
			expectedIsiPath:      "/ifs",
			expectedSnapshotName: "to",
			expectedSrcVolName:   "volume",
		},
		{
			name:                 "Length of directories slice is less than 3",
			snapshotIsiPath:      "/ifs/path/test",
			zonePath:             "/ifs",
			expectedIsiPath:      "/ifs",
			expectedSnapshotName: "test",
			expectedSrcVolName:   "test",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isiPath, snapshotName, srcVolName := svc.GetSnapshotIsiPathComponents(test.snapshotIsiPath, test.zonePath)

			if isiPath != test.expectedIsiPath {
				t.Errorf("Expected isiPath '%s', got '%s'", test.expectedIsiPath, isiPath)
			}

			if snapshotName != test.expectedSnapshotName {
				t.Errorf("Expected snapshotName '%s', got '%s'", test.expectedSnapshotName, snapshotName)
			}

			if srcVolName != test.expectedSrcVolName {
				t.Errorf("Expected srcVolName '%s', got '%s'", test.expectedSrcVolName, srcVolName)
			}
		})
	}
}

func TestIsHostAlreadyAdded(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	ctx := context.Background()

	t.Run("export not found returns true", func(t *testing.T) {
		mockClient.On("Get", anyArgs...).Return(errors.New("mock error")).Once()
		result := svc.IsHostAlreadyAdded(ctx, 789, "System", "node2")
		assert.True(t, result)
	})

	t.Run("invalid node ID returns true", func(t *testing.T) {
		mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(*apiv2.ExportList)
			clients := []string{"client1"}
			roClients := []string{}
			rwClients := []string{}
			rootClients := []string{}
			*resp = apiv2.ExportList{
				&apiv2.Export{
					ID:               1,
					Clients:          &clients,
					ReadOnlyClients:  &roClients,
					ReadWriteClients: &rwClients,
					RootClients:      &rootClients,
				},
			}
		}).Once()
		// Invalid node ID format
		result := svc.IsHostAlreadyAdded(ctx, 1, "System", "")
		assert.True(t, result)
	})

	t.Run("node in client fields returns true", func(t *testing.T) {
		mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(*apiv2.ExportList)
			clients := []string{"node1"}
			roClients := []string{}
			rwClients := []string{}
			rootClients := []string{}
			*resp = apiv2.ExportList{
				&apiv2.Export{
					ID:               1,
					Clients:          &clients,
					ReadOnlyClients:  &roClients,
					ReadWriteClients: &rwClients,
					RootClients:      &rootClients,
				},
			}
		}).Once()
		result := svc.IsHostAlreadyAdded(ctx, 1, "System", "node1=#=#=node1.example.com=#=#=10.0.0.1")
		assert.True(t, result)
	})

	t.Run("node not in client fields returns false", func(t *testing.T) {
		mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(*apiv2.ExportList)
			clients := []string{"other-node"}
			roClients := []string{}
			rwClients := []string{}
			rootClients := []string{}
			*resp = apiv2.ExportList{
				&apiv2.Export{
					ID:               1,
					Clients:          &clients,
					ReadOnlyClients:  &roClients,
					ReadWriteClients: &rwClients,
					RootClients:      &rootClients,
				},
			}
		}).Once()
		result := svc.IsHostAlreadyAdded(ctx, 1, "System", "node1=#=#=node1.example.com=#=#=10.0.0.1")
		assert.False(t, result)
	})

	t.Run("node IP in client fields returns true", func(t *testing.T) {
		mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(*apiv2.ExportList)
			clients := []string{"10.0.0.1"}
			roClients := []string{}
			rwClients := []string{}
			rootClients := []string{}
			*resp = apiv2.ExportList{
				&apiv2.Export{
					ID:               1,
					Clients:          &clients,
					ReadOnlyClients:  &roClients,
					ReadWriteClients: &rwClients,
					RootClients:      &rootClients,
				},
			}
		}).Once()
		result := svc.IsHostAlreadyAdded(ctx, 1, "System", "node1=#=#=node1.example.com=#=#=10.0.0.1")
		assert.True(t, result)
	})
}

func TestGetExportsCountAttachedToNode(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	// Define the test cases
	tests := []struct {
		name      string
		nodeip    string
		wantCount int64
		wantErr   bool
	}{
		{
			name:      "Failed to get exports count",
			nodeip:    "1.1.1.1",
			wantCount: 0,
			wantErr:   true,
		},
		{
			name: "Context cancelled",
			nodeip: func() string {
				_, cancel := context.WithCancel(context.Background())
				cancel()
				return "1.1.1.1"
			}(),
			wantCount: 0,
			wantErr:   true,
		},
		{
			name:      "Get exports count successfully",
			nodeip:    "10.0.0.1",
			wantCount: 1,
			wantErr:   false,
		},
	}

	// Run the test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.name == "Context cancelled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			// Adjust the mock setup based on the test case
			if tt.wantErr {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
			} else {
				svc.client.API.(*isimocks.Client).ExpectedCalls = nil
				svc.client.API.(*isimocks.Client).On("Get", anyArgs[0:6]...).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(**apiv1.GetIsiExportsResp)
					*resp = &apiv1.GetIsiExportsResp{
						ExportList: []*apiv1.IsiExport{
							{Clients: []string{"10.0.0.1"}},
						},
					}
				}).Once()
			}

			got, err := svc.GetExportsCountAttachedToNode(ctx, tt.nodeip)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetExportsCountAttachedToNode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantCount {
				t.Errorf("GetExportsCountAttachedToNode() = %v, want %v", got, tt.wantCount)
			}
		})
	}
}

func TestGetExportsCountAttachedToNodeIPs(t *testing.T) {
	ctx := context.Background()

	t.Run("Empty IPs returns zero", func(t *testing.T) {
		svc := &isiService{
			client: &isi.Client{API: &isimocks.Client{}},
		}
		count, err := svc.GetExportsCountAttachedToNodeIPs(ctx, []string{}, "System")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("API error returns error", func(t *testing.T) {
		mockAPI := &isimocks.Client{}
		svc := &isiService{
			client: &isi.Client{API: mockAPI},
		}
		mockAPI.On("Get", anyArgs...).Return(errors.New("mock error")).Once()
		_, err := svc.GetExportsCountAttachedToNodeIPs(ctx, []string{"10.0.0.1"}, "System")
		assert.Error(t, err)
	})
}

func TestGetExports(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(svc *isiService)
		want    isi.ExportList
		wantErr bool
	}{
		{
			name: "Success case",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "error case",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(errors.New("mock error"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &isiService{
				endpoint: "http://localhost:8080",
				client: &isi.Client{
					API: &isimocks.Client{},
				},
			}

			if tc.setup != nil {
				tc.setup(svc)
			}

			ctx := context.Background()
			got, err := svc.GetExports(ctx)
			if (err != nil) != tc.wantErr {
				t.Errorf("isiService.GetExports() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("isiService.GetExports() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExportVolumeWithZone(t *testing.T) {
	type args struct {
		isiPath     string
		volName     string
		accessZone  string
		description string
	}
	tests := []struct {
		name    string
		setup   func(svc *isiService)
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "Test ExportVolumeWithZone Success",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Post", anyArgs...).Return(nil)
			},
			args: args{
				isiPath:     "/ifs/data",
				volName:     "test_volume",
				accessZone:  "System",
				description: "Test volume",
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "Test ExportVolumeWithZone Failure",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Post", anyArgs...).Return(errors.New("mock error"))
			},
			args: args{
				isiPath:     "/ifs/data",
				volName:     "test_volume",
				accessZone:  "System",
				description: "Test volume",
			},
			want:    -1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &isiService{
				endpoint: "http://localhost:8080",
				client: &isi.Client{
					API: &isimocks.Client{},
				},
			}

			if tt.setup != nil {
				tt.setup(svc)
			}

			ctx := context.Background()
			got, err := svc.ExportVolumeWithZone(ctx, tt.args.isiPath, tt.args.volName, tt.args.accessZone, tt.args.description)
			if (err != nil) != tt.wantErr {
				t.Errorf("isiService.ExportVolumeWithZone() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("isiService.ExportVolumeWithZone() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteQuotaByExportIDWithZone(t *testing.T) {
	type args struct {
		volName    string
		exportID   int
		accessZone string
	}
	tests := []struct {
		name    string
		setup   func(svc *isiService)
		args    args
		wantErr bool
	}{
		{
			name: "failure to get export",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(errors.New("mock error"))
			},
			args: args{
				volName:    "test-volume",
				exportID:   123,
				accessZone: "System",
			},
			wantErr: true,
		},
		{
			name: "no quota set on the volume, skip deleting quota",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(*apiv2.ExportList)
					*resp = apiv2.ExportList{
						&apiv2.Export{
							ID: 123,
						},
					}
				})
			},
			args: args{
				volName:    "test-volume",
				exportID:   123,
				accessZone: "System",
			},
			wantErr: false,
		},
		{
			name: "successful quota delete",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(*apiv2.ExportList)
					*resp = apiv2.ExportList{
						&apiv2.Export{
							ID:          123,
							Description: fmt.Sprintf("CSI_QUOTA_ID:%d", 123),
						},
					}
					svc.client.API.(*isimocks.Client).On("Delete", anyArgs...).Return(nil)
				})
			},
			args: args{
				volName:    "test-volume",
				exportID:   123,
				accessZone: "System",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &isiService{
				endpoint: "http://localhost:8080",
				client: &isi.Client{
					API: &isimocks.Client{},
				},
			}

			if tt.setup != nil {
				tt.setup(svc)
			}

			ctx := context.Background()
			if err := svc.DeleteQuotaByExportIDWithZone(ctx, tt.args.volName, tt.args.exportID, tt.args.accessZone); (err != nil) != tt.wantErr {
				t.Errorf("isiService.DeleteQuotaByExportIDWithZone() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateQuotaSize(t *testing.T) {
	type args struct {
		ctx                  context.Context
		quotaID              string
		updatedSize          int64
		updatedSoftLimit     int64
		updatedAdvisoryLimit int64
		softGrace            int64
	}
	tests := []struct {
		name    string
		setup   func(svc *isiService)
		args    args
		wantErr bool
	}{
		{
			name: "success case",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Put", anyArgs...).Return(nil)
			},
			args: args{
				quotaID:              "test-quota-id",
				updatedSize:          100,
				updatedSoftLimit:     50,
				updatedAdvisoryLimit: 75,
				softGrace:            10,
			},
			wantErr: false,
		},
		{
			name: "failure case",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Put", anyArgs...).Return(errors.New("mock error"))
			},
			args: args{
				quotaID:              "test-quota-id",
				updatedSize:          100,
				updatedSoftLimit:     50,
				updatedAdvisoryLimit: 75,
				softGrace:            10,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &isiService{
				endpoint: "http://localhost:8080",
				client: &isi.Client{
					API: &isimocks.Client{},
				},
			}

			if tt.setup != nil {
				tt.setup(svc)
			}

			ctx := context.Background()
			if err := svc.UpdateQuotaSize(ctx, tt.args.quotaID, tt.args.updatedSize, tt.args.updatedSoftLimit, tt.args.updatedAdvisoryLimit, tt.args.softGrace); (err != nil) != tt.wantErr {
				t.Errorf("isiService.UpdateQuotaSize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUnexportByIDWithZone(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(svc *isiService)
		exportID   int
		accessZone string
		wantErr    bool
	}{
		{
			name: "success case",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Delete", anyArgs...).Return(nil)
			},
			exportID:   123,
			accessZone: "System",
			wantErr:    false,
		},
		{
			name: "failure case",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Delete", anyArgs...).Return(errors.New("mock error"))
			},
			exportID:   123,
			accessZone: "System",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &isiService{
				endpoint: "http://localhost:8080",
				client: &isi.Client{
					API: &isimocks.Client{},
				},
			}

			if tt.setup != nil {
				tt.setup(svc)
			}

			ctx := context.Background()
			if err := svc.UnexportByIDWithZone(ctx, tt.exportID, tt.accessZone); (err != nil) != tt.wantErr {
				t.Errorf("isiService.UnexportByIDWithZone() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeleteVolume(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(svc *isiService)
		isiPath string
		volName string
		wantErr bool
	}{
		{
			name: "success case",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Delete", anyArgs...).Return(nil)
			},
			isiPath: "/ifs/data",
			volName: "test_volume",
			wantErr: false,
		},
		{
			name: "failure case",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Delete", anyArgs...).Return(errors.New("mock error"))
			},
			isiPath: "/ifs/data",
			volName: "test_volume",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &isiService{
				endpoint: "http://localhost:8080",
				client: &isi.Client{
					API: &isimocks.Client{},
				},
			}

			if tt.setup != nil {
				tt.setup(svc)
			}

			ctx := context.Background()
			if err := svc.DeleteVolume(ctx, tt.isiPath, tt.volName); (err != nil) != tt.wantErr {
				t.Errorf("isiService.DeleteVolume() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClearQuotaByID(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(svc *isiService)
		quotaID string
		wantErr bool
	}{
		{
			name: "success case",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Delete", anyArgs...).Return(nil)
			},
			quotaID: "123",
			wantErr: false,
		},
		{
			name: "failure case",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Delete", anyArgs...).Return(errors.New("mock error"))
			},
			quotaID: "123",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &isiService{
				endpoint: "http://localhost:8080",
				client: &isi.Client{
					API: &isimocks.Client{},
				},
			}

			if tt.setup != nil {
				tt.setup(svc)
			}

			ctx := context.Background()
			if err := svc.ClearQuotaByID(ctx, tt.quotaID); (err != nil) != tt.wantErr {
				t.Errorf("isiService.ClearQuotaByID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTestConnection(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(svc *isiService)
		wantErr bool
	}{
		{
			name: "success case",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("User").Return("test-user")
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "failure case",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("User").Return("test-user")
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(errors.New("mock error"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &isiService{
				endpoint: "http://localhost:8080",
				client: &isi.Client{
					API: &isimocks.Client{},
				},
			}

			if tt.setup != nil {
				tt.setup(svc)
			}

			ctx := context.Background()
			if err := svc.TestConnection(ctx); (err != nil) != tt.wantErr {
				t.Errorf("isiService.TestConnection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetVolumeWithIsiPath(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(svc *isiService)
		isiPath string
		volID   string
		volName string
		want    *apiv1.IsiVolume
		wantErr bool
	}{
		{
			name: "success case",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(**apiv1.GetIsiVolumeAttributesResp)
					*resp = &apiv1.GetIsiVolumeAttributesResp{
						AttributeMap: []struct {
							Name  string      `json:"name"`
							Value interface{} `json:"value"`
						}{
							{
								Name:  "test1Name",
								Value: "test1Value",
							},
							{
								Name:  "test2Name",
								Value: "test2Value",
							},
						},
					}
				})
			},
			isiPath: "/ifs/data",
			volID:   "123",
			volName: "test_volume",
			want: &apiv1.IsiVolume{
				Name: "123",
				AttributeMap: []struct {
					Name  string      `json:"name"`
					Value interface{} `json:"value"`
				}{
					{
						Name:  "test1Name",
						Value: "test1Value",
					},
					{
						Name:  "test2Name",
						Value: "test2Value",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "failure case",
			setup: func(svc *isiService) {
				svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(errors.New("mock error"))
			},
			isiPath: "/ifs/data",
			volID:   "123",
			volName: "test_volume",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &isiService{
				endpoint: "http://localhost:8080",
				client: &isi.Client{
					API: &isimocks.Client{},
				},
			}

			if tt.setup != nil {
				tt.setup(svc)
			}

			ctx := context.Background()
			got, err := svc.GetVolumeWithIsiPath(ctx, tt.isiPath, tt.volID, tt.volName)
			if (err != nil) != tt.wantErr {
				t.Errorf("isiService.GetVolumeWithIsiPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if (err == nil) && !reflect.DeepEqual(got.Name, tt.want.Name) {
				t.Errorf("isiService.GetVolumeWithIsiPath() = '%v', want '%v'", got, tt.want)
			}
		})
	}
}

func TestRemoveExportClientByIPsWithZone(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: mockClient,
		},
	}

	// Define the test cases
	testCases := []struct {
		name                    string
		exportID                int
		accessZone              string
		clientIPs               []string
		ignoreUnresolvableHosts bool
		setup                   func(mockClient *isimocks.Client)
		expectedErr             error
	}{
		{
			name:                    "success case",
			exportID:                456,
			accessZone:              "System",
			clientIPs:               []string{"1.2.3.4", "5.6.7.8"},
			ignoreUnresolvableHosts: true,
			setup: func(mockClient *isimocks.Client) {
				ex := &apiv2.Export{
					ID:               456,
					Paths:            &[]string{"/export1"},
					Clients:          &[]string{"1.2.3.4", "5.6.7.8"},
					RootClients:      &[]string{},
					ReadOnlyClients:  &[]string{},
					ReadWriteClients: &[]string{},
				}
				mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(*apiv2.ExportList)
					*resp = apiv2.ExportList{ex}
				}).Once()
				mockClient.On("Put", anyArgs...).Return(nil).Once()
			},
			expectedErr: nil,
		},
		{
			name:                    "404 error - export not found",
			exportID:                456,
			accessZone:              "System",
			clientIPs:               []string{"1.2.3.4"},
			ignoreUnresolvableHosts: true,
			setup: func(mockClient *isimocks.Client) {
				mockClient.On("Get", anyArgs...).Return(&api.JSONError{StatusCode: 404}).Once()
			},
			expectedErr: nil, // 404 is handled gracefully
		},
		{
			name:                    "generic error",
			exportID:                456,
			accessZone:              "System",
			clientIPs:               []string{"1.2.3.4"},
			ignoreUnresolvableHosts: true,
			setup: func(mockClient *isimocks.Client) {
				mockClient.On("Get", anyArgs...).Return(errors.New("mock error")).Once()
			},
			expectedErr: errors.New("failed to remove clients from export '456' with access zone 'System' : 'mock error'"),
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setup != nil {
				tc.setup(mockClient)
			}

			ctx := context.Background()
			err := svc.RemoveExportClientByIPsWithZone(ctx, tc.exportID, tc.accessZone, tc.clientIPs, tc.ignoreUnresolvableHosts)

			if err != nil {
				if tc.expectedErr == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.expectedErr.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.expectedErr, err)
				}
			} else {
				if tc.expectedErr != nil {
					t.Errorf("Expected error '%v', but got nil", tc.expectedErr)
				}
			}
		})
	}
}

func TestAddExportClientsByIDWithZone(t *testing.T) {
	mockClient := &isimocks.Client{}

	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	testCases := []struct {
		name                    string
		exportID                int
		accessZone              string
		clientIPs               []string
		ignoreUnresolvableHosts bool
		expectedErr             error
	}{
		{
			name:                    "error case",
			exportID:                456,
			accessZone:              "System",
			clientIPs:               []string{"5.6.7.8"},
			ignoreUnresolvableHosts: true,
			expectedErr:             errors.New("failed to add clients to export id '456' with access zone 'System' : 'mock error'"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
			err := svc.AddExportClientsByIDWithZone(ctx, tc.exportID, tc.accessZone, tc.clientIPs, tc.ignoreUnresolvableHosts)

			if err != nil {
				if tc.expectedErr == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.expectedErr.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.expectedErr, err)
				}
			} else {
				if tc.expectedErr != nil {
					t.Errorf("Expected error '%v', but got nil", tc.expectedErr)
				}
			}
		})
	}
}

func TestAddExportRootClientsByIDWithZone(t *testing.T) {
	mockClient := &isimocks.Client{}

	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	testCases := []struct {
		name                    string
		exportID                int
		accessZone              string
		clientIPs               []string
		ignoreUnresolvableHosts bool
		expectedErr             error
	}{
		{
			name:                    "error case",
			exportID:                456,
			accessZone:              "System",
			clientIPs:               []string{"5.6.7.8"},
			ignoreUnresolvableHosts: true,
			expectedErr:             errors.New("failed to add clients to export id '456' with access zone 'System' : 'mock error'"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
			err := svc.AddExportRootClientsByIDWithZone(ctx, tc.exportID, tc.accessZone, tc.clientIPs, tc.ignoreUnresolvableHosts)

			if err != nil {
				if tc.expectedErr == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.expectedErr.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.expectedErr, err)
				}
			} else {
				if tc.expectedErr != nil {
					t.Errorf("Expected error '%v', but got nil", tc.expectedErr)
				}
			}
		})
	}
}

func TestAddExportReadOnlyClientsByIDWithZone(t *testing.T) {
	mockClient := &isimocks.Client{}

	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	testCases := []struct {
		name                    string
		exportID                int
		accessZone              string
		clientIPs               []string
		ignoreUnresolvableHosts bool
		expectedErr             error
	}{
		{
			name:                    "error case",
			exportID:                456,
			accessZone:              "System",
			clientIPs:               []string{"5.6.7.8"},
			ignoreUnresolvableHosts: true,
			expectedErr:             errors.New("failed to add read only clients to export id '456' with access zone 'System' : 'mock error'"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc.client.API.(*isimocks.Client).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
			err := svc.AddExportReadOnlyClientsByIDWithZone(ctx, tc.exportID, tc.accessZone, tc.clientIPs, tc.ignoreUnresolvableHosts)

			if err != nil {
				if tc.expectedErr == nil {
					t.Errorf("Unexpected error: %v", err)
				} else if err.Error() != tc.expectedErr.Error() {
					t.Errorf("Expected error '%v', but got '%v'", tc.expectedErr, err)
				}
			} else {
				if tc.expectedErr != nil {
					t.Errorf("Expected error '%v', but got nil", tc.expectedErr)
				}
			}
		})
	}
}

func TestAddExportClientsByIDWithZoneSuccess(t *testing.T) {
	mockClient := &isimocks.Client{}
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv2.ExportList)
		*resp = apiv2.ExportList{
			&apiv2.Export{
				ID:      456,
				Zone:    "System",
				Clients: &[]string{},
			},
		}
	}).Once()
	mockClient.On("Put", anyArgs...).Return(nil).Once()

	ctx := context.Background()
	err := svc.AddExportClientsByIDWithZone(ctx, 456, "System", []string{"5.6.7.8"}, false)
	assert.NoError(t, err)
}

func TestAddExportRootClientsByIDWithZoneSuccess(t *testing.T) {
	mockClient := &isimocks.Client{}
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv2.ExportList)
		*resp = apiv2.ExportList{
			&apiv2.Export{
				ID:          456,
				Zone:        "System",
				RootClients: &[]string{},
			},
		}
	}).Once()
	mockClient.On("Put", anyArgs...).Return(nil).Once()

	ctx := context.Background()
	err := svc.AddExportRootClientsByIDWithZone(ctx, 456, "System", []string{"5.6.7.8"}, false)
	assert.NoError(t, err)
}

func TestAddExportReadOnlyClientsByIDWithZoneSuccess(t *testing.T) {
	mockClient := &isimocks.Client{}
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv2.ExportList)
		*resp = apiv2.ExportList{
			&apiv2.Export{
				ID:              456,
				Zone:            "System",
				ReadOnlyClients: &[]string{},
			},
		}
	}).Once()
	mockClient.On("Put", anyArgs...).Return(nil).Once()

	ctx := context.Background()
	err := svc.AddExportReadOnlyClientsByIDWithZone(ctx, 456, "System", []string{"5.6.7.8"}, false)
	assert.NoError(t, err)
}

func TestGetExportsWithLimit_Error(t *testing.T) {
	mockClient := &isimocks.Client{}
	svc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: mockClient,
		},
	}
	mockClient.On("Get", anyArgs...).Return(errors.New("get exports failed")).Once()
	ctx := context.Background()
	exports, resume, err := svc.GetExportsWithLimit(ctx, "10")
	assert.Error(t, err)
	assert.Nil(t, exports)
	assert.Empty(t, resume)
}

func TestGetExportsWithLimit_Success(t *testing.T) {
	mockClient := &isimocks.Client{}
	svc := &isiService{
		endpoint: "http://localhost:8080",
		client: &isi.Client{
			API: mockClient,
		},
	}
	mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv2.Exports)
		*resp = apiv2.Exports{
			Exports: apiv2.ExportList{
				&apiv2.Export{ID: 1, Paths: &[]string{"/ifs/data/vol1"}},
				&apiv2.Export{ID: 2, Paths: &[]string{"/ifs/data/vol2"}},
			},
			Resume: "next-token",
		}
	}).Once()
	ctx := context.Background()
	exports, resume, err := svc.GetExportsWithLimit(ctx, "10")
	assert.NoError(t, err)
	assert.NotNil(t, exports)
	assert.Len(t, exports, 2)
	assert.Equal(t, "next-token", resume)
}

func TestGetFilesystemsWithLimit_InvalidLimit(t *testing.T) {
	mockClient := &isimocks.Client{}
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}
	ctx := context.Background()
	_, _, err := svc.GetFilesystemsWithLimit(ctx, "/ifs/data", "not-a-number")
	assert.Error(t, err)
}

func TestGetFilesystemsWithLimit_ListError(t *testing.T) {
	mockClient := &isimocks.Client{}
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}
	mockClient.On("Get", anyArgs...).Return(errors.New("list error")).Once()
	ctx := context.Background()
	_, _, err := svc.GetFilesystemsWithLimit(ctx, "/ifs/data", "10")
	assert.Error(t, err)
}

func TestGetSubDirectoryCount_VolumeNotFound(t *testing.T) {
	mockClient := &isimocks.Client{}
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}
	mockClient.On("Get", anyArgs...).Return(errors.New("not found")).Once()
	ctx := context.Background()
	_, err := svc.GetSubDirectoryCount(ctx, "/ifs/data", "nonexistent-vol")
	assert.Error(t, err)
}

func TestGetSubDirectoryCount_Success(t *testing.T) {
	mockClient := &isimocks.Client{}
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}
	ctx := context.Background()

	// First call to check if volume exists
	mockClient.On("Get", anyArgs...).Return(nil).Once()

	// Second call to get volume details with nlink attribute
	mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv1.GetIsiVolumeAttributesResp)
		*resp = &apiv1.GetIsiVolumeAttributesResp{
			AttributeMap: []struct {
				Name  string      `json:"name"`
				Value interface{} `json:"value"`
			}{
				{Name: "nlink", Value: float64(5)},
			},
		}
	}).Once()

	count, err := svc.GetSubDirectoryCount(ctx, "/ifs/data", "test-vol")
	assert.NoError(t, err)
	assert.Equal(t, int64(5), count)
}

func TestGetSubDirectoryCount_InvalidNlinkType(t *testing.T) {
	mockClient := &isimocks.Client{}
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}
	ctx := context.Background()

	// First call to check if volume exists
	mockClient.On("Get", anyArgs...).Return(nil).Once()

	// Second call to get volume details with invalid nlink attribute
	mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv1.GetIsiVolumeAttributesResp)
		*resp = &apiv1.GetIsiVolumeAttributesResp{
			AttributeMap: []struct {
				Name  string      `json:"name"`
				Value interface{} `json:"value"`
			}{
				{Name: "nlink", Value: "invalid"},
			},
		}
	}).Once()

	_, err := svc.GetSubDirectoryCount(ctx, "/ifs/data", "test-vol")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get total subdirectory count")
}

func TestGetSubDirectoryCount_NoNlinkAttribute(t *testing.T) {
	mockClient := &isimocks.Client{}
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}
	ctx := context.Background()

	// First call to check if volume exists
	mockClient.On("Get", anyArgs...).Return(nil).Once()

	// Second call to get volume details without nlink attribute
	mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv1.GetIsiVolumeAttributesResp)
		*resp = &apiv1.GetIsiVolumeAttributesResp{
			AttributeMap: []struct {
				Name  string      `json:"name"`
				Value interface{} `json:"value"`
			}{
				{Name: "size", Value: float64(1000)},
			},
		}
	}).Once()

	count, err := svc.GetSubDirectoryCount(ctx, "/ifs/data", "test-vol")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestGetSubDirectoryCount_GetVolumeError(t *testing.T) {
	mockClient := &isimocks.Client{}
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}
	ctx := context.Background()

	// First call to check if volume exists
	mockClient.On("Get", anyArgs...).Return(nil).Once()

	// Second call to get volume details returns error
	mockClient.On("Get", anyArgs...).Return(errors.New("API error")).Once()

	_, err := svc.GetSubDirectoryCount(ctx, "/ifs/data", "test-vol")
	assert.Error(t, err)
}

func TestSetVolumeGroupOwnershipByPath(t *testing.T) {
	tests := []struct {
		name            string
		isiPath         string
		volName         string
		gid             int
		existingGID     string
		putErr          error
		expectErr       bool
		expectChanged   bool
		expectErrSubstr string
	}{
		{
			name:          "success",
			isiPath:       "/ifs/k8s/shared",
			volName:       "csivol-abc123",
			gid:           2000,
			existingGID:   "",
			putErr:        nil,
			expectErr:     false,
			expectChanged: true,
		},
		{
			name:            "API error",
			isiPath:         "/ifs/k8s/shared",
			volName:         "csivol-abc123",
			gid:             2000,
			existingGID:     "",
			putErr:          errors.New("mock PUT error"),
			expectErr:       true,
			expectErrSubstr: "failed to set group ownership",
		},
		{
			name:          "group already matches",
			isiPath:       "/ifs/k8s/shared",
			volName:       "csivol-abc123",
			gid:           2000,
			existingGID:   "2000",
			putErr:        nil,
			expectErr:     false,
			expectChanged: false,
		},
		{
			name:            "fsGroup conflict",
			isiPath:         "/ifs/k8s/shared",
			volName:         "csivol-abc123",
			gid:             2000,
			existingGID:     "3000",
			putErr:          nil,
			expectErr:       true,
			expectErrSubstr: "fsGroup conflict",
		},
		{
			name:          "inherited root group allowed",
			isiPath:       "/ifs/k8s/shared",
			volName:       "csivol-abc123",
			gid:           2000,
			existingGID:   "0",
			putErr:        nil,
			expectErr:     false,
			expectChanged: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := &isimocks.Client{}
			svc := &isiService{
				client: &isi.Client{
					API: mockClient,
				},
			}
			isClone := false

			// Mock GetVolumeACL via the underlying API.Get call
			mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
				if len(tc.existingGID) > 0 {
					resp := args.Get(5).(*apiv2.ACL)
					*resp = apiv2.ACL{
						Group: &apiv2.Persona{
							ID: &apiv2.PersonaID{
								ID:   tc.existingGID,
								Type: apiv2.PersonaIDTypeGID,
							},
						},
					}
				}
			}).Once()

			// Mock Put for fresh volumes, API error cases, or inherited root group (0) being updated.
			if len(tc.existingGID) == 0 || tc.existingGID == "0" && strconv.Itoa(tc.gid) != "0" {
				mockClient.On("Put", anyArgs...).Return(tc.putErr).Once()
			}

			ctx := context.Background()
			result, err := svc.SetVolumeGroupOwnershipByPath(ctx, tc.isiPath, tc.volName, tc.gid, isClone)
			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), tc.expectErrSubstr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tc.expectChanged, result.Changed)
			}
		})
	}
}

func TestSetVolumeGroupOwnershipByPath_CloneOwnershipChange(t *testing.T) {
	mockClient := &isimocks.Client{}
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	// Mock Get to return existing GID 2000
	mockClient.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv2.ACL)
		*resp = apiv2.ACL{
			Group: &apiv2.Persona{
				ID: &apiv2.PersonaID{
					ID:   "2000",
					Type: apiv2.PersonaIDTypeGID,
				},
			},
		}
	}).Once()

	// Mock Put for ownership update
	mockClient.On("Put", anyArgs...).Return(nil).Once()

	ctx := context.Background()
	result, err := svc.SetVolumeGroupOwnershipByPath(ctx, "/ifs/k8s/shared", "csivol-clone123", 3000, true)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Changed, "Clone should allow ownership change from 2000 to 3000")
	assert.Equal(t, "2000", result.ExistingGID)
}

func TestSetVolumeGroupOwnershipRecursive_InvalidPath(t *testing.T) {
	mockClient := &isimocks.Client{}
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	ctx := context.Background()
	err := svc.SetVolumeGroupOwnershipRecursive(ctx, "invalid-path", "vol1", 1000, 8)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid volume path format")
}

func TestSetPathGroupOwnership_InvalidPath(t *testing.T) {
	mockClient := &isimocks.Client{}
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	ctx := context.Background()
	err := svc.setPathGroupOwnership(ctx, "invalid-path", 1000, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid path for group ownership update")
}

func TestSetPathGroupOwnership_APIError(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Mock the API to return an error
	mockClient.On("Put", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		&api.JSONError{StatusCode: 500},
	)

	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	ctx := context.Background()
	err := svc.setPathGroupOwnership(ctx, "/ifs/data/vol1/file.txt", 1000, false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to set group ownership")
	mockClient.AssertExpectations(t)
}

func TestSetPathGroupOwnership_SuccessFile(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Mock the API to succeed
	mockClient.On("Put", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	ctx := context.Background()
	err := svc.setPathGroupOwnership(ctx, "/ifs/data/vol1/file.txt", 1000, false)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestSetPathGroupOwnership_SuccessDirectory(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Mock the API to succeed
	mockClient.On("Put", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	ctx := context.Background()
	err := svc.setPathGroupOwnership(ctx, "/ifs/data/vol1/subdir", 1000, true)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestSetVolumeGroupOwnershipRecursive_NoChildren(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Mock the API to return empty children list
	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*resumeableContainerChildList)
		*resp = resumeableContainerChildList{
			Children: []*apiv2.ContainerChild{},
			Resume:   "",
		}
	})

	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	ctx := context.Background()
	err := svc.SetVolumeGroupOwnershipRecursive(ctx, "/ifs/data", "vol1", 1000, 8)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestSetVolumeGroupOwnershipRecursive_GetChildrenError(t *testing.T) {
	mockClient := &isimocks.Client{}

	// Mock the API to return an error
	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		&api.JSONError{StatusCode: 500},
	)

	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	ctx := context.Background()
	err := svc.SetVolumeGroupOwnershipRecursive(ctx, "/ifs/data", "vol1", 1000, 8)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to query volume children for recursive ownership")
	mockClient.AssertExpectations(t)
}

func TestSetVolumeGroupOwnershipRecursive_WithPagination(t *testing.T) {
	mockClient := &isimocks.Client{}

	path1 := "/ifs/data/vol1/file1.txt"
	name1 := "file1.txt"
	path2 := "/ifs/data/vol1/file2.txt"
	name2 := "file2.txt"
	path3 := "/ifs/data/vol1/file3.txt"
	name3 := "file3.txt"
	typeFile := "object"

	callCount := 0
	// Mock the API to return paginated results
	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*resumeableContainerChildList)
		callCount++
		if callCount == 1 {
			// First page with resume token
			*resp = resumeableContainerChildList{
				Children: []*apiv2.ContainerChild{
					{Path: &path1, Name: &name1, Type: &typeFile},
					{Path: &path2, Name: &name2, Type: &typeFile},
				},
				Resume: "token123",
			}
		} else {
			// Second page without resume token
			*resp = resumeableContainerChildList{
				Children: []*apiv2.ContainerChild{
					{Path: &path3, Name: &name3, Type: &typeFile},
				},
				Resume: "",
			}
		}
	})

	// Mock the Put calls for setting ownership
	mockClient.On("Put", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	ctx := context.Background()
	err := svc.SetVolumeGroupOwnershipRecursive(ctx, "/ifs/data", "vol1", 1000, 8)

	assert.NoError(t, err)
	assert.Equal(t, 2, callCount, "Should make 2 Get calls for pagination")
	mockClient.AssertExpectations(t)
}

func TestSetVolumeGroupOwnershipRecursive_SetOwnershipError(t *testing.T) {
	mockClient := &isimocks.Client{}

	path1 := "/ifs/data/vol1/file1.txt"
	name1 := "file1.txt"
	typeFile := "object"

	// Mock the API to return children
	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*resumeableContainerChildList)
		*resp = resumeableContainerChildList{
			Children: []*apiv2.ContainerChild{
				{Path: &path1, Name: &name1, Type: &typeFile},
			},
			Resume: "",
		}
	})

	// Mock the Put call to fail
	mockClient.On("Put", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		&api.JSONError{StatusCode: 500},
	)

	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	ctx := context.Background()
	err := svc.SetVolumeGroupOwnershipRecursive(ctx, "/ifs/data", "vol1", 1000, 8)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "recursive ownership failed")
	mockClient.AssertExpectations(t)
}

func TestSetVolumeGroupOwnershipRecursive_Success(t *testing.T) {
	mockClient := &isimocks.Client{}

	path1 := "/ifs/data/vol1/file1.txt"
	name1 := "file1.txt"
	path2 := "/ifs/data/vol1/subdir"
	name2 := "subdir"
	typeFile := "object"
	typeDir := "container"

	// Mock the API to return children with Type field
	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*resumeableContainerChildList)
		*resp = resumeableContainerChildList{
			Children: []*apiv2.ContainerChild{
				{Path: &path1, Name: &name1, Type: &typeFile},
				{Path: &path2, Name: &name2, Type: &typeDir},
			},
			Resume: "",
		}
	})

	// Mock the Put calls for setting ownership
	mockClient.On("Put", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	ctx := context.Background()
	err := svc.SetVolumeGroupOwnershipRecursive(ctx, "/ifs/data", "vol1", 1000, 8)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestSetVolumeGroupOwnershipRecursive_ChildrenWithNilFields(t *testing.T) {
	mockClient := &isimocks.Client{}

	path1 := "/ifs/data/vol1/file1.txt"
	name1 := "file1.txt"
	typeFile := "object"

	// Mock the API to return children with some nil fields (should be skipped)
	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*resumeableContainerChildList)
		*resp = resumeableContainerChildList{
			Children: []*apiv2.ContainerChild{
				{Path: &path1, Name: &name1, Type: &typeFile},
				{Path: nil, Name: &name1}, // Should be skipped
				{Path: &path1, Name: nil}, // Should be skipped
				{Path: nil, Name: nil},    // Should be skipped
			},
			Resume: "",
		}
	})

	// Mock the Put call (should only be called once for the valid child)
	mockClient.On("Put", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	ctx := context.Background()
	err := svc.SetVolumeGroupOwnershipRecursive(ctx, "/ifs/data", "vol1", 1000, 8)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestGetVolumeMetaData_InvalidPath_Dot(t *testing.T) {
	ctx := context.Background()
	svc := &isiService{}

	_, err := svc.GetVolumeMetaData(ctx, ".")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid volume path")
}

func TestGetVolumeMetaData_InvalidPath_Root(t *testing.T) {
	ctx := context.Background()
	svc := &isiService{}

	_, err := svc.GetVolumeMetaData(ctx, "/")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid volume path")
}

func TestGetSnapshotTrackingDirName(t *testing.T) {
	svc := &isiService{}

	result := svc.GetSnapshotTrackingDirName("test-snapshot")
	assert.Equal(t, ".csi-test-snapshot-tracking-dir", result)
}

func TestGetVolumeMetaData_GetVolumeError(t *testing.T) {
	ctx := context.Background()
	mockClient := &isimocks.Client{}

	svc := &isiService{
		client: &isi.Client{API: mockClient},
	}

	// Mock GetVolumeWithIsiPath to return error
	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("volume not found"))

	_, err := svc.GetVolumeMetaData(ctx, "/ifs/data/test-vol")
	assert.Error(t, err)
}

func TestGetSnapshotSourceVolumeIsiPath(t *testing.T) {
	ctx := context.Background()
	mockClient := &isimocks.Client{}

	svc := &isiService{
		client: &isi.Client{API: mockClient},
	}

	t.Run("success", func(t *testing.T) {
		mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(**apiv1.GetIsiSnapshotsResp)
			*resp = &apiv1.GetIsiSnapshotsResp{
				SnapshotList: []*apiv1.IsiSnapshot{
					{
						Path: "/ifs/data/volume1",
					},
				},
			}
		}).Once()

		result, err := svc.GetSnapshotSourceVolumeIsiPath(ctx, "snap1")
		assert.NoError(t, err)
		assert.Equal(t, "/ifs/data", result)
		mockClient.ExpectedCalls = nil
	})

	t.Run("get snapshot error", func(t *testing.T) {
		mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(errors.New("snapshot not found"))

		_, err := svc.GetSnapshotSourceVolumeIsiPath(ctx, "snap1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get snapshot id")
		mockClient.ExpectedCalls = nil
	})
}

func TestDeleteWritableSnapshot(t *testing.T) {
	ctx := context.Background()
	mockClient := &isimocks.Client{}

	svc := &isiService{
		client: &isi.Client{API: mockClient},
	}

	t.Run("success", func(t *testing.T) {
		mockClient.On("Delete", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).Once()

		err := svc.DeleteWritableSnapshot(ctx, "/ifs/data/writable-vol-1")
		assert.NoError(t, err)
		mockClient.ExpectedCalls = nil
	})

	t.Run("delete error", func(t *testing.T) {
		mockClient.On("Delete", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(errors.New("delete failed")).Once()

		err := svc.DeleteWritableSnapshot(ctx, "/ifs/data/writable-vol-1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "delete failed")
		mockClient.ExpectedCalls = nil
	})
}
