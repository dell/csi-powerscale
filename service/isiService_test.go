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
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	isi "github.com/dell/goisilon"
	"github.com/dell/goisilon/api"
	apiv1 "github.com/dell/goisilon/api/v1"
	apiv2 "github.com/dell/goisilon/api/v2"
	apiv5 "github.com/dell/goisilon/api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockClient struct {
	mock.Mock
}

func (m *MockClient) APIVersion() uint8 {
	// Implement the logic for the APIVersion method
	// Return the desired API version and error
	return uint8(2)
}

func (m *MockClient) GetAuthToken() string {
	// Implement the logic for the APIVersion method
	// Return the desired API version and error
	return ""
}

func (m *MockClient) GetCSRFToken() string {
	// Implement the logic for the APIVersion method
	// Return the desired API version and error
	return ""
}

func (m *MockClient) GetReferer() string {
	// Implement the logic for the APIVersion method
	// Return the desired API version and error
	return ""
}

func (m *MockClient) SetAuthToken(_ string) {
}

func (m *MockClient) SetCSRFToken(_ string) {
}

func (m *MockClient) SetReferer(_ string) {
}

func (m *MockClient) VolumePath(_ string) string {
	return ""
}

func (m *MockClient) User() string {
	return ""
}

func (m *MockClient) VolumesPath() string {
	return ""
}

func (m *MockClient) Group() string {
	return ""
}

func (m *MockClient) Delete(ctx context.Context, path string, id string, params api.OrderedValues, headers map[string]string, resp interface{}) error {
	ret := m.Called(ctx, path, id, params, headers, resp)

	if len(ret) == 0 {
		panic("no return value specified for Delete")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, api.OrderedValues, map[string]string, interface{}) error); ok {
		r0 = rf(ctx, path, id, params, headers, resp)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (m *MockClient) Do(
	_ context.Context,
	_, _, _ string,
	_ api.OrderedValues,
	_, _ interface{},
) error {
	return nil
}

func (m *MockClient) DoWithHeaders(
	_ context.Context,
	_, _, _ string,
	_ api.OrderedValues, _ map[string]string,
	_, _ interface{},
) error {
	return nil
}

func (m *MockClient) Get(ctx context.Context, path string, id string, params api.OrderedValues, headers map[string]string, resp interface{}) error {
	ret := m.Called(ctx, path, id, params, headers, resp)

	if len(ret) == 0 {
		panic("no return value specified for Get")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, api.OrderedValues, map[string]string, interface{}) error); ok {
		r0 = rf(ctx, path, id, params, headers, resp)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (m *MockClient) Post(ctx context.Context, path string, id string, params api.OrderedValues, headers map[string]string, body interface{}, resp interface{}) error {
	ret := m.Called(ctx, path, id, params, headers, body, resp)

	if len(ret) == 0 {
		panic("no return value specified for Post")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, api.OrderedValues, map[string]string, interface{}, interface{}) error); ok {
		r0 = rf(ctx, path, id, params, headers, body, resp)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func (m *MockClient) Put(ctx context.Context, path string, id string, params api.OrderedValues, headers map[string]string, body interface{}, resp interface{}) error {
	ret := m.Called(ctx, path, id, params, headers, body, resp)

	if len(ret) == 0 {
		panic("no return value specified for Put")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, api.OrderedValues, map[string]string, interface{}, interface{}) error); ok {
		r0 = rf(ctx, path, id, params, headers, body, resp)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

func TestCopySnapshot(t *testing.T) {
	mockClient := &MockClient{}

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
			svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
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
	mockClient := &MockClient{}

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
			svc.client.API.(*MockClient).On("Put", anyArgs...).Return(errors.New("mock error")).Once()
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
	mockClient := &MockClient{}

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
			svc.client.API.(*MockClient).On("Put", anyArgs...).Return(errors.New("mock error")).Once()
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
	mockClient := &MockClient{}

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
			svc.client.API.(*MockClient).On("Put", anyArgs...).Return(errors.New("mock error")).Once()
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
				svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
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
				svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Once()
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
				svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
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
				svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
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
					API: &MockClient{},
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
				svc.client.API.(*MockClient).On("Get", anyArgs...).RunFn = func(_ mock.Arguments) {
					panic("should not be called")
				}
			},
			quotaEnabled: false,
		},
		{
			name: "invalid smart quota value skip create",
			setup: func(svc *isiService) {
				svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
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
				svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(*apiv5.QuotaLicense)
					*resp = apiv5.QuotaLicense{
						STATUS: "Licensed",
					}
				}).Once()

				svc.client.API.(*MockClient).On("Post", anyArgs...).Return(errors.New("mock error"))
			},
			sizeInBytes:   100,
			quotaEnabled:  true,
			expectedError: errors.New("SmartQuotas is activated, but creating quota failed with error: 'mock error'"),
		},
		{
			name: "invalid soft grace period use default",
			setup: func(svc *isiService) {
				svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(*apiv5.QuotaLicense)
					*resp = apiv5.QuotaLicense{
						STATUS: "Licensed",
					}
				}).Once()

				svc.client.API.(*MockClient).On("Post", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
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
				svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Once()
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
				svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Once()
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
				svc.client.API.(*MockClient).On("Get", anyArgs...).RunFn = func(_ mock.Arguments) {
					panic("should not be called")
				}
			},
			sizeInBytes:  0,
			quotaEnabled: true,
		},
		{
			name: "size negative skip creating quotas",
			setup: func(svc *isiService) {
				svc.client.API.(*MockClient).On("Get", anyArgs...).RunFn = func(_ mock.Arguments) {
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
					API: &MockClient{},
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
	mockClient := &MockClient{}

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
			svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
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

func TestGetVolumeSize(t *testing.T) {
	mockClient := &MockClient{}

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
			svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
			size := svc.GetVolumeSize(ctx, tc.isiPath, tc.volName)
			assert.Equal(t, tc.expectedSize, size)
		})
	}
}

func TestIsIOInProgress(t *testing.T) {
	mockClient := &MockClient{}

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
			svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
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
	mockClient := &MockClient{}

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
		exportID     int
		accessZone   string
		nodeID       string
		expectedBool bool
	}{
		{
			name:         "Export is nil",
			exportID:     456,
			accessZone:   "System",
			nodeID:       "node2",
			expectedBool: true,
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
			result := svc.OtherClientsAlreadyAdded(ctx, tc.exportID, tc.accessZone, tc.nodeID)

			if result != tc.expectedBool {
				t.Errorf("Expected '%v', but got '%v'", tc.expectedBool, result)
			}
		})
	}
}

func TestAddExportClientNetworkIdentifierByIDWithZone(t *testing.T) {
	mockClient := &MockClient{}

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
		expectedErr             error
	}{
		{
			name:                    "error case",
			clusterName:             "cluster2",
			exportID:                456,
			accessZone:              "System",
			nodeID:                  "!@$%~^",
			ignoreUnresolvableHosts: true,
			expectedErr:             errors.New("node ID '!@$%~^' cannot match the expected '^(.+)=#=#=(.+)=#=#=(.+)$' pattern"),
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("node ID '!@$%~^' cannot match the expected '^(.+)=#=#=(.+)=#=#=(.+)$' pattern")).Once()
			err := svc.AddExportClientNetworkIdentifierByIDWithZone(context.Background(), tc.clusterName, tc.exportID, tc.accessZone, tc.nodeID, tc.ignoreUnresolvableHosts, func(_ context.Context, _ int, _, _ string, _ bool) error {
				// Simulate the addClientFunc behavior
				if tc.expectedErr != nil {
					return tc.expectedErr
				}
				return nil
			})

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
	mockClient := &MockClient{}

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
			svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
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
	mockClient := &MockClient{}

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
			svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
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
	mockClient := &MockClient{}

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
			svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
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
					API: &MockClient{},
				},
			}

			err := svc.AddExportClientByIPWithZone(ctx, tc.clusterName, tc.exportID, tc.accessZone, tc.nodeID, tc.clientIPs, tc.addClientFunc)

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

func TestRemoveExportClientByIDWithZone(t *testing.T) {
	mockClient := &MockClient{}

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
			svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
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
				svc.client.API.(*MockClient).On("Post", anyArgs...).Return(nil)
			},
		},
		{
			name:         "failure case",
			path:         "/ifs/data/csi-isilon/volume2",
			snapshotName: "ut-snapshot",
			setup: func(svc *isiService) {
				svc.client.API.(*MockClient).On("Post", anyArgs...).Return(errors.New("mock error"))
			},
			wantErr: errors.New("mock error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &isiService{
				endpoint: "http://localhost:8080",
				client: &isi.Client{
					API: &MockClient{},
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
	mockClient := &MockClient{}

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
			svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Twice()
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
	mockClient := &MockClient{}

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
	mockClient := &MockClient{}

	// Create a new instance of the isiService struct
	svc := &isiService{
		client: &isi.Client{
			API: mockClient,
		},
	}

	// Define the test cases
	testCases := []struct {
		name         string
		exportID     int
		accessZone   string
		nodeID       string
		expectedBool bool
	}{
		{
			name:         "Node ID is in client fields",
			exportID:     789,
			accessZone:   "System",
			nodeID:       "node2",
			expectedBool: true,
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error")).Once()
			result := svc.IsHostAlreadyAdded(ctx, tc.exportID, tc.accessZone, tc.nodeID)

			if result != tc.expectedBool {
				t.Errorf("Expected '%v', but got '%v'", tc.expectedBool, result)
			}
		})
	}
}

func TestGetExports(t *testing.T) {
	type fields struct {
		endpoint string
		client   *isi.Client
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		setup   func(svc *isiService)
		want    isi.ExportList
		wantErr bool
	}{
		{
			name: "Success case",
			setup: func(svc *isiService) {
				svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "error case",
			setup: func(svc *isiService) {
				svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &isiService{
				endpoint: "http://localhost:8080",
				client: &isi.Client{
					API: &MockClient{},
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
				svc.client.API.(*MockClient).On("Post", anyArgs...).Return(nil)
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
				svc.client.API.(*MockClient).On("Post", anyArgs...).Return(errors.New("mock error"))
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
					API: &MockClient{},
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
				svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error"))
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
				svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
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
				svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
					resp := args.Get(5).(*apiv2.ExportList)
					*resp = apiv2.ExportList{
						&apiv2.Export{
							ID:          123,
							Description: fmt.Sprintf("CSI_QUOTA_ID:%d", 123),
						},
					}
					svc.client.API.(*MockClient).On("Delete", anyArgs...).Return(nil)
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
					API: &MockClient{},
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
				svc.client.API.(*MockClient).On("Put", anyArgs...).Return(nil)
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
				svc.client.API.(*MockClient).On("Put", anyArgs...).Return(errors.New("mock error"))
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
					API: &MockClient{},
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
				svc.client.API.(*MockClient).On("Delete", anyArgs...).Return(nil)
			},
			exportID:   123,
			accessZone: "System",
			wantErr:    false,
		},
		{
			name: "failure case",
			setup: func(svc *isiService) {
				svc.client.API.(*MockClient).On("Delete", anyArgs...).Return(errors.New("mock error"))
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
					API: &MockClient{},
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
				svc.client.API.(*MockClient).On("Delete", anyArgs...).Return(nil)
			},
			isiPath: "/ifs/data",
			volName: "test_volume",
			wantErr: false,
		},
		{
			name: "failure case",
			setup: func(svc *isiService) {
				svc.client.API.(*MockClient).On("Delete", anyArgs...).Return(errors.New("mock error"))
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
					API: &MockClient{},
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
				svc.client.API.(*MockClient).On("Delete", anyArgs...).Return(nil)
			},
			quotaID: "123",
			wantErr: false,
		},
		{
			name: "failure case",
			setup: func(svc *isiService) {
				svc.client.API.(*MockClient).On("Delete", anyArgs...).Return(errors.New("mock error"))
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
					API: &MockClient{},
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
				svc.client.API.(*MockClient).On("Get", anyArgs...).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "failure case",
			setup: func(svc *isiService) {
				svc.client.API.(*MockClient).On("Get", anyArgs...).Return(errors.New("mock error"))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &isiService{
				endpoint: "http://localhost:8080",
				client: &isi.Client{
					API: &MockClient{},
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
