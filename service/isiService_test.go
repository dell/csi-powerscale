package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	isi "github.com/dell/goisilon"
	"github.com/dell/goisilon/api"
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

func (m *MockClient) SetAuthToken(token string) {

}

func (m *MockClient) SetCSRFToken(token string) {

}

func (m *MockClient) SetReferer(token string) {

}

func (m *MockClient) VolumePath(token string) string {
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

func (m *MockClient) Delete(
	ctx context.Context,
	path, id string,
	params api.OrderedValues, headers map[string]string,
	resp interface{},
) error {
	return nil
}

func (m *MockClient) Do(
	ctx context.Context,
	method, path, id string,
	params api.OrderedValues,
	body, resp interface{},
) error {
	return nil
}

func (m *MockClient) DoWithHeaders(
	ctx context.Context,
	method, path, id string,
	params api.OrderedValues, headers map[string]string,
	body, resp interface{},
) error {
	return nil
}

func (m *MockClient) Get(
	ctx context.Context,
	path, id string,
	params api.OrderedValues, headers map[string]string,
	resp interface{},
) error {
	return errors.New("mock error")
}

func (m *MockClient) Post(
	ctx context.Context,
	path, id string,
	params api.OrderedValues, headers map[string]string,
	body, resp interface{},
) error {
	return nil
}

func (m *MockClient) Put(
	ctx context.Context,
	path, id string,
	params api.OrderedValues, headers map[string]string,
	body, resp interface{},
) error {
	return errors.New("mock error")
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
			name:                        "Error case",
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
			name:          "Error case",
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
			name:                     "Error case",
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
			name:                     "Error case",
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
		volName      string
		exportID     int
		accessZone   string
		expectedQuot isi.Quota
		expectedErr  error
	}{
		{
			name:         "Error case",
			volName:      "test_volume",
			exportID:     456,
			accessZone:   "System",
			expectedQuot: nil,
			expectedErr:  errors.New("failed to get export 'test_volume':'456' with access zone 'System', error: 'mock error'"),
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

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
			name:            "Invalid advisory limit",
			isiPath:         "/ifs/data/csi-isilon",
			volName:         "volume3",
			softLimit:       "70",
			advisoryLimit:   "invalid",
			softGracePrd:    "30",
			sizeInBytes:     100,
			quotaEnabled:    true,
			expectedQuotaID: "",
			expectedError:   fmt.Errorf("invalid advisory limit"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			mockClient := &MockClient{}

			svc := &isiService{
				endpoint: "http://localhost:8080",
				client: &isi.Client{
					API: mockClient,
				},
			}

			_, err := svc.CreateQuota(ctx, tc.isiPath, tc.volName, tc.softLimit, tc.advisoryLimit, tc.softGracePrd, tc.sizeInBytes, tc.quotaEnabled)
			assert.NoError(t, err)
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
			name: "Error case",
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
			name:         "Error case",
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
			name:            "Error case",
			expectedClients: nil,
			expectedErr:     errors.New("mock error"),
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

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
			name:                    "Error case",
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
			ctx := context.Background()

			err := svc.AddExportClientNetworkIdentifierByIDWithZone(ctx, tc.clusterName, tc.exportID, tc.accessZone, tc.nodeID, tc.ignoreUnresolvableHosts, func(ctx context.Context, exportID int, accessZone, clientIP string, ignoreUnresolvableHosts bool) error {
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
			name:                    "Error case",
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
			name:        "Error case",
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
			name:                    "Error case",
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
			name:                    "Error case",
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

			result := svc.IsHostAlreadyAdded(ctx, tc.exportID, tc.accessZone, tc.nodeID)

			if result != tc.expectedBool {
				t.Errorf("Expected '%v', but got '%v'", tc.expectedBool, result)
			}
		})
	}
}
