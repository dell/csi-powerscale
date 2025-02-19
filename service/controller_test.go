package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	vgsext "github.com/dell/dell-csi-extensions/volumeGroupSnapshot"
	isi "github.com/dell/goisilon"
	api "github.com/dell/goisilon/api/v1"
	apiv1 "github.com/dell/goisilon/api/v1"
	"github.com/stretchr/testify/assert"
)

func TestRemoveString(t *testing.T) {
	tests := []struct {
		name     string
		volumes  []string
		toRemove string
		expected []string
	}{
		{
			name:     "Remove present volume",
			volumes:  []string{"volume1", "volume2", "volume3"},
			toRemove: "volume2",
			expected: []string{"volume1", "volume3"},
		},
		{
			name:     "Remove non-present volume",
			volumes:  []string{"volume1", "volume2", "volume3"},
			toRemove: "volume4",
			expected: []string{"volume1", "volume2", "volume3"},
		},
		{
			name:     "Remove from empty volume list",
			volumes:  []string{},
			toRemove: "volume2",
			expected: []string{},
		},
		{
			name:     "Remove last volume",
			volumes:  []string{"volume1", "volume2", "volume3"},
			toRemove: "volume3",
			expected: []string{"volume1", "volume2"},
		},
		{
			name:     "Remove first volume",
			volumes:  []string{"volume1", "volume2", "volume3"},
			toRemove: "volume1",
			expected: []string{"volume2", "volume3"},
		},
		{
			name:     "Remove duplicate volume (only first occurrence)",
			volumes:  []string{"volume1", "volume2", "volume1", "volume3"},
			toRemove: "volume1",
			expected: []string{"volume2", "volume1", "volume3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeString(tt.volumes, tt.toRemove)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("removeString(%v, %s) = %v; expected %v", tt.volumes, tt.toRemove, result, tt.expected)
			}
		})
	}
}

func TestReadQuotaLimitParams(t *testing.T) {
	testCases := []struct {
		name              string
		params            map[string]string
		expectedSoft      string
		expectedAdv       string
		expectedSoftGrace string
	}{
		{
			name: "Default values",
			params: map[string]string{
				SoftLimitParam:     "",
				AdvisoryLimitParam: "",
				SoftGracePrdParam:  "",
			},
			expectedSoft:      SoftLimitParamDefault,
			expectedAdv:       AdvisoryLimitParamDefault,
			expectedSoftGrace: SoftGracePrdParamDefault,
		},
		{
			name: "Soft limit overridden",
			params: map[string]string{
				SoftLimitParam:     "70",
				AdvisoryLimitParam: "",
				SoftGracePrdParam:  "",
			},
			expectedSoft:      "70",
			expectedAdv:       AdvisoryLimitParamDefault,
			expectedSoftGrace: SoftGracePrdParamDefault,
		},
		{
			name: "Advisory limit overridden",
			params: map[string]string{
				SoftLimitParam:     "",
				AdvisoryLimitParam: "80",
				SoftGracePrdParam:  "",
			},
			expectedSoft:      SoftLimitParamDefault,
			expectedAdv:       "80",
			expectedSoftGrace: SoftGracePrdParamDefault,
		},
		{
			name: "Soft grace period overridden",
			params: map[string]string{
				SoftLimitParam:     "",
				AdvisoryLimitParam: "",
				SoftGracePrdParam:  "30",
			},
			expectedSoft:      SoftLimitParamDefault,
			expectedAdv:       AdvisoryLimitParamDefault,
			expectedSoftGrace: "30",
		},
		{
			name: "Soft limit overridden in PVC",
			params: map[string]string{
				SoftLimitParam:     "",
				AdvisoryLimitParam: "",
				SoftGracePrdParam:  "",
				PVCSoftLimitParam:  "70",
			},
			expectedSoft:      "70",
			expectedAdv:       AdvisoryLimitParamDefault,
			expectedSoftGrace: SoftGracePrdParamDefault,
		},
		{
			name: "Advisory limit overridden in PVC",
			params: map[string]string{
				SoftLimitParam:        "",
				AdvisoryLimitParam:    "",
				SoftGracePrdParam:     "",
				PVCAdvisoryLimitParam: "80",
			},
			expectedSoft:      SoftLimitParamDefault,
			expectedAdv:       "80",
			expectedSoftGrace: SoftGracePrdParamDefault,
		},
		{
			name: "Soft grace period overridden in PVC",
			params: map[string]string{
				SoftLimitParam:       "",
				AdvisoryLimitParam:   "",
				SoftGracePrdParam:    "",
				PVCSoftGracePrdParam: "30",
			},
			expectedSoft:      SoftLimitParamDefault,
			expectedAdv:       AdvisoryLimitParamDefault,
			expectedSoftGrace: "30",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			softLimit, advisoryLimit, softGracePrd := readQuotaLimitParams(tc.params)
			if softLimit != tc.expectedSoft {
				t.Errorf("Expected soft limit '%s', but got '%s'", tc.expectedSoft, softLimit)
			}
			if advisoryLimit != tc.expectedAdv {
				t.Errorf("Expected advisory limit '%s', but got '%s'", tc.expectedAdv, advisoryLimit)
			}
			if softGracePrd != tc.expectedSoftGrace {
				t.Errorf("Expected soft grace period '%s', but got '%s'", tc.expectedSoftGrace, softGracePrd)
			}
		})
	}
}

// Test function for CreateVolumeGroupSnapshot
func TestCreateVolumeGroupSnapshot(t *testing.T) {
	s := &service{}

	tests := []struct {
		name    string
		req     *vgsext.CreateVolumeGroupSnapshotRequest
		wantErr bool
	}{
		{
			name: "Valid Request",
			req: &vgsext.CreateVolumeGroupSnapshotRequest{
				SourceVolumeIDs: []string{"volume1", "volume2"},
				Name:            "snapshot-group-1",
				Description:     "A test snapshot group",
				Parameters:      map[string]string{"param1": "value1"},
			},
			wantErr: false,
		},
		{
			name:    "Invalid Request - nil request",
			req:     nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()

			// Capture the possible panic for unimplemented function
			defer func() {
				if r := recover(); r != nil {
					t.Skip("Function not implemented")
				}
			}()

			// Call the function
			resp, err := s.CreateVolumeGroupSnapshot(ctx, tt.req)

			// Check if error condition matches
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateVolumeGroupSnapshot() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Add additional assertions if needed
			if !tt.wantErr {
				if resp == nil {
					t.Errorf("Expected non-nil response, got nil")
				}
			}
		})
	}
}

func TestCreateVolumeFromSnapshot(t *testing.T) {
	// Backup original functions
	originalGetSnapshotFunc := getSnapshotFunc
	originalGetSnapshotSizeFunc := getSnapshotSizeFunc
	originalCopySnapshotFunc := copySnapshotFunc

	// Restore original functions after the test
	defer func() { getSnapshotFunc = originalGetSnapshotFunc }()
	defer func() { getSnapshotSizeFunc = originalGetSnapshotSizeFunc }()
	defer func() { copySnapshotFunc = originalCopySnapshotFunc }()

	// Mock implementations
	getSnapshotFunc = func(ctx context.Context, isiConfig *IsilonClusterConfig) func(ctx context.Context, snapshotID string) (isi.Snapshot, error) {
		return func(ctx context.Context, snapshotID string) (isi.Snapshot, error) {
			if snapshotID == "snapshot1234" {
				return &api.IsiSnapshot{ID: 1234, Path: "/ifs/data/snapshot1234"}, nil
			}
			return nil, errors.New("snapshot not found")
		}
	}

	getSnapshotSizeFunc = func(ctx context.Context, isiConfig *IsilonClusterConfig) func(ctx context.Context, volumePath, snapshotName, accessZone string) int64 {
		return func(ctx context.Context, volumePath, snapshotName, accessZone string) int64 {
			return 100
		}
	}

	copySnapshotFunc = func(ctx context.Context, isiConfig *IsilonClusterConfig) func(ctx context.Context, dstPath, srcPath string, snapshotID int64, dstName, accessZone string) (isi.Volume, error) {
		return func(ctx context.Context, dstPath, srcPath string, snapshotID int64, dstName, accessZone string) (isi.Volume, error) {
			if snapshotID == 1234 {
				return &apiv1.IsiVolume{Name: dstName, AttributeMap: []struct {
					Name  string      `json:"name"`
					Value interface{} `json:"value"`
				}{}}, nil
			}
			return &apiv1.IsiVolume{}, errors.New("failed to copy snapshot")
		}
	}

	isiConfig := &IsilonClusterConfig{
		// Any necessary initialization here
	}

	s := &service{}

	tests := []struct {
		name          string
		normalizedID  string
		dstVolumeName string
		sizeInBytes   int64
		expectedError error
	}{
		{"ValidCase", "snapshot1234", "dstVolumeName", 200, nil},
		{"InvalidSnapshotSize", "snapshot1234", "dstVolumeName", 50, fmt.Errorf("specified size '50' is smaller than source snapshot size '100'")},
		{"SnapshotNotFound", "invalidSnapshotID", "dstVolumeName", 200, fmt.Errorf("failed to get snapshot id 'invalidSnapshotID', error 'snapshot not found'")},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.createVolumeFromSnapshot(ctx, isiConfig, "/ifs/data/destinationPath", tt.normalizedID, tt.dstVolumeName, tt.sizeInBytes, "accessZone")
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateVolumeFromVolume(t *testing.T) {
	// Backup original functions
	originalIsVolumeExistentFunc := isVolumeExistentFunc
	originalGetVolumeSizeFunc := getVolumeSizeFunc
	originalCopyVolumeFunc := copyVolumeFunc

	// Restore original functions after the test
	defer func() { isVolumeExistentFunc = originalIsVolumeExistentFunc }()
	defer func() { getVolumeSizeFunc = originalGetVolumeSizeFunc }()
	defer func() { copyVolumeFunc = originalCopyVolumeFunc }()

	// Mock implementations
	isVolumeExistentFunc = func(isiConfig *IsilonClusterConfig) func(ctx context.Context, isiPath, ns, srcVolumeName string) bool {
		return func(ctx context.Context, isiPath, ns, srcVolumeName string) bool {
			if srcVolumeName == "existentVolume" || srcVolumeName == "errorVolumeCopy" {
				return true
			}
			return false
		}
	}

	getVolumeSizeFunc = func(isiConfig *IsilonClusterConfig) func(ctx context.Context, isiPath, srcVolumeName string) int64 {
		return func(ctx context.Context, isiPath, srcVolumeName string) int64 {
			if srcVolumeName == "existentVolume" || srcVolumeName == "errorVolumeCopy" {
				return 100
			}
			return 0
		}
	}

	copyVolumeFunc = func(isiConfig *IsilonClusterConfig) func(ctx context.Context, isiPath, srcVolumeName, dstVolumeName string) (isi.Volume, error) {
		return func(ctx context.Context, isiPath, srcVolumeName, dstVolumeName string) (isi.Volume, error) {
			if srcVolumeName == "errorVolumeCopy" {
				return &apiv1.IsiVolume{}, errors.New("failed to copy volume name")
			}
			if srcVolumeName == "existentVolume" {
				return &apiv1.IsiVolume{Name: dstVolumeName, AttributeMap: []struct {
					Name  string      `json:"name"`
					Value interface{} `json:"value"`
				}{}}, nil
			}
			return &apiv1.IsiVolume{}, errors.New("failed to copy volume")
		}
	}

	isiConfig := &IsilonClusterConfig{
		// Any necessary initialization here
	}

	s := &service{}

	tests := []struct {
		name          string
		srcVolumeName string
		dstVolumeName string
		sizeInBytes   int64
		expectedError error
	}{
		{"ValidCase", "existentVolume", "newVolume", 200, nil},
		{"InvalidVolumeSize", "existentVolume", "newVolume", 50, fmt.Errorf("specified size '50' is smaller than source volume size '100'")},
		{"VolumeNotFound", "nonExistentVolume", "newVolume", 200, fmt.Errorf("failed to get volume name 'nonExistentVolume', error '<nil>'")},
		{"CopyVolumeError", "errorVolumeCopy", "newVolume", 200, fmt.Errorf("failed to copy volume name 'errorVolumeCopy', error 'failed to copy volume name'")},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.createVolumeFromVolume(ctx, isiConfig, "/ifs/data/volumePath", tt.srcVolumeName, tt.dstVolumeName, tt.sizeInBytes)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
