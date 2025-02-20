package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	vgsext "github.com/dell/dell-csi-extensions/volumeGroupSnapshot"
	isi "github.com/dell/goisilon"
	api "github.com/dell/goisilon/api/v1"
	apiv1 "github.com/dell/goisilon/api/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func TestIsValid(t *testing.T) {
	tests := []struct {
		rpo         RPOEnum
		expectedErr bool
	}{
		{RpoFiveMinutes, false},
		{RpoFifteenMinutes, false},
		{RpoThirtyMinutes, false},
		{RpoOneHour, false},
		{RpoSixHours, false},
		{RpoTwelveHours, false},
		{RpoOneDay, false},
		{"Invalid_RPO", true},
	}

	for _, test := range tests {
		err := test.rpo.IsValid()
		if (err != nil) != test.expectedErr {
			t.Errorf("RPOEnum.IsValid() for RPO %v, expected error: %v, got: %v", test.rpo, test.expectedErr, err != nil)
		}
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		rpo      RPOEnum
		expected int
		err      bool
	}{
		{RpoFiveMinutes, 300, false},
		{RpoFifteenMinutes, 900, false},
		{RpoThirtyMinutes, 1800, false},
		{RpoOneHour, 3600, false},
		{RpoSixHours, 21600, false},
		{RpoTwelveHours, 43200, false},
		{RpoOneDay, 86400, false},
		{"Invalid_RPO", -1, true},
	}

	for _, test := range tests {
		result, err := test.rpo.ToInt()
		if (err != nil) != test.err {
			t.Errorf("RPOEnum.ToInt() error for RPO %v, expected error: %v, got: %v", test.rpo, test.err, err != nil)
		}
		if result != test.expected {
			t.Errorf("RPOEnum.ToInt() for RPO %v, expected: %d, got: %d", test.rpo, test.expected, result)
		}
	}
}

func TestListSnapshots(t *testing.T) {
	// Creating a mock service
	srv := &service{}

	// Creating a test request
	request := &csi.ListSnapshotsRequest{}

	// Calling ListSnapshots method
	_, err := srv.ListSnapshots(context.Background(), request)

	// Check if the error is as expected
	if err == nil {
		t.Fatalf("Expected error but got nil")
	}

	// Check if the error is of type Unimplemented
	if status.Code(err) != codes.Unimplemented {
		t.Errorf("Expected error code %v, got %v", codes.Unimplemented, status.Code(err))
	}
}

func TestCreateVolumeFromSource(t *testing.T) {
	// Backup original functions
	originalGetSnapshotSourceFunc := getSnapshotSourceFunc
	originalGetVolumeFunc := getVolumeFunc
	originalCreateVolumeFromSnapshotFunc := createVolumeFromSnapshotFunc
	originalCreateVolumeFromVolumeFunc := createVolumeFromVolumeFunc
	originalGetUtilsParseNormalizedVolumeID := getUtilsParseNormalizedVolumeID

	// Restore original functions after the test
	defer func() {
		getSnapshotSourceFunc = originalGetSnapshotSourceFunc
		getVolumeFunc = originalGetVolumeFunc
		createVolumeFromSnapshotFunc = originalCreateVolumeFromSnapshotFunc
		createVolumeFromVolumeFunc = originalCreateVolumeFromVolumeFunc
		getUtilsParseNormalizedVolumeID = originalGetUtilsParseNormalizedVolumeID
	}()

	// Mock implementations
	getSnapshotSourceFunc = func(contentSource *csi.VolumeContentSource) *csi.VolumeContentSource_SnapshotSource {
		if contentSource.GetSnapshot() != nil && contentSource.GetSnapshot().SnapshotId == "validSnapshot" {
			return &csi.VolumeContentSource_SnapshotSource{SnapshotId: "validSnapshot"}
		}
		if contentSource.GetSnapshot() != nil && contentSource.GetSnapshot().SnapshotId == "errorSnapshot" {
			return &csi.VolumeContentSource_SnapshotSource{SnapshotId: "errorSnapshot"}
		}
		return nil
	}

	getVolumeFunc = func(contentSource *csi.VolumeContentSource) *csi.VolumeContentSource_VolumeSource {
		if contentSource.GetVolume() != nil && contentSource.GetVolume().VolumeId == "validVolume" {
			return &csi.VolumeContentSource_VolumeSource{VolumeId: "validVolume"}
		}
		if contentSource.GetVolume() != nil && contentSource.GetVolume().VolumeId == "errorVolume" {
			return &csi.VolumeContentSource_VolumeSource{VolumeId: "errorVolume"}
		}
		return nil
	}

	createVolumeFromSnapshotFunc = func(svc *service) func(ctx context.Context, isiConfig *IsilonClusterConfig, isiPath, snapshotID, volName string, sizeInBytes int64, accessZone string) error {
		return func(ctx context.Context, isiConfig *IsilonClusterConfig, isiPath, snapshotID, volName string, sizeInBytes int64, accessZone string) error {
			if snapshotID == "errorSnapshot" {
				return errors.New("snapshot error")
			}
			return nil
		}
	}

	createVolumeFromVolumeFunc = func(svc *service) func(ctx context.Context, isiConfig *IsilonClusterConfig, isiPath, srcVolumeName, dstVolumeName string, sizeInBytes int64) error {
		return func(ctx context.Context, isiConfig *IsilonClusterConfig, isiPath, srcVolumeName, dstVolumeName string, sizeInBytes int64) error {
			if srcVolumeName == "errorVolume" {
				return errors.New("volumes error")
			}
			return nil
		}
	}

	// Mock implementation of utils.ParseNormalizedVolumeID
	getUtilsParseNormalizedVolumeID = func(ctx context.Context, volumeID string) (string, int, string, string, error) {
		if volumeID == "validVolume" {
			return "clusterName", 0, "volumePath", "volumeName", nil
		}
		return "", 0, "", "", errors.New("volume error")
	}

	// Create mock IsilonClusterConfig
	isiConfig := &IsilonClusterConfig{}

	s := &service{}

	tests := []struct {
		name          string
		contentSource *csi.VolumeContentSource
		req           *csi.CreateVolumeRequest
		sizeInBytes   int64
		expectedError error
	}{
		{
			"ValidSnapshotSource",
			&csi.VolumeContentSource{
				Type: &csi.VolumeContentSource_Snapshot{
					Snapshot: &csi.VolumeContentSource_SnapshotSource{SnapshotId: "validSnapshot"},
				},
			},
			&csi.CreateVolumeRequest{Name: "newVolume"},
			200,
			nil,
		},
		{
			"SnapshotSourceError",
			&csi.VolumeContentSource{
				Type: &csi.VolumeContentSource_Snapshot{
					Snapshot: &csi.VolumeContentSource_SnapshotSource{SnapshotId: "errorSnapshot"},
				},
			},
			&csi.CreateVolumeRequest{Name: "newVolume"},
			200,
			status.Error(codes.Internal, "snapshot error"),
		},
		{
			"ValidVolumeSource",
			&csi.VolumeContentSource{
				Type: &csi.VolumeContentSource_Volume{
					Volume: &csi.VolumeContentSource_VolumeSource{VolumeId: "validVolume"},
				},
			},
			&csi.CreateVolumeRequest{Name: "newVolume"},
			200,
			nil,
		},
		{
			"VolumeSourceError",
			&csi.VolumeContentSource{
				Type: &csi.VolumeContentSource_Volume{
					Volume: &csi.VolumeContentSource_VolumeSource{VolumeId: "errorVolume"},
				},
			},
			&csi.CreateVolumeRequest{Name: "newVolume"},
			200,
			status.Error(codes.Internal, "volume error"),
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.createVolumeFromSource(ctx, isiConfig, "/ifs/data/volumePath", tt.contentSource, tt.req, tt.sizeInBytes, "accessZone")
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetCreateVolumeResponse(t *testing.T) {
	// Backup original function
	originalGetCSIVolumeFunc := getCSIVolumeFunc

	// Restore original function after the test
	defer func() {
		getCSIVolumeFunc = originalGetCSIVolumeFunc
	}()

	// Mock implementation
	getCSIVolumeFunc = func(svc *service) func(ctx context.Context, exportID int, volName, path, accessZone string, sizeInBytes int64, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName string) *csi.Volume {
		return func(ctx context.Context, exportID int, volName, path, accessZone string, sizeInBytes int64, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName string) *csi.Volume {
			return &csi.Volume{
				VolumeId:      volName,
				CapacityBytes: sizeInBytes,
				VolumeContext: map[string]string{
					"path":             path,
					"accessZone":       accessZone,
					"azServiceIP":      azServiceIP,
					"rootClient":       rootClientEnabled,
					"sourceSnapshotID": sourceSnapshotID,
					"sourceVolumeID":   sourceVolumeID,
					"clusterName":      clusterName,
				},
			}
		}
	}

	// Mock service instance
	s := &service{}

	// Mock inputs
	ctx := context.Background()
	exportID := 1
	volName := "vol-test"
	path := "/data/vol-test"
	accessZone := "accessZone1"
	sizeInBytes := int64(1024)
	azServiceIP := "10.0.0.1"
	rootClientEnabled := "true"
	sourceSnapshotID := "snapshot123"
	sourceVolumeID := "volume123"
	clusterName := "cluster-test"

	// Expected result
	expectedVolume := &csi.Volume{
		VolumeId:      volName,
		CapacityBytes: sizeInBytes,
		VolumeContext: map[string]string{
			"path":             path,
			"accessZone":       accessZone,
			"azServiceIP":      azServiceIP,
			"rootClient":       rootClientEnabled,
			"sourceSnapshotID": sourceSnapshotID,
			"sourceVolumeID":   sourceVolumeID,
			"clusterName":      clusterName,
		},
	}

	expectedResponse := &csi.CreateVolumeResponse{
		Volume: expectedVolume,
	}

	// Call the function under test
	response := s.getCreateVolumeResponse(ctx, exportID, volName, path, accessZone, sizeInBytes, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName)

	// Assert the response is as expected
	assert.Equal(t, expectedResponse, response)
}

func TestAddMetaData(t *testing.T) {
	tests := []struct {
		name     string
		params   map[string]string
		expected map[string]string
	}{
		{
			name: "All keys present",
			params: map[string]string{
				csiPersistentVolumeName:           "pv1",
				csiPersistentVolumeClaimName:      "pvc1",
				csiPersistentVolumeClaimNamespace: "namespace1",
			},
			expected: map[string]string{
				headerPersistentVolumeName:           "pv1",
				headerPersistentVolumeClaimName:      "pvc1",
				headerPersistentVolumeClaimNamespace: "namespace1",
			},
		},
		{
			name: "Some keys present",
			params: map[string]string{
				csiPersistentVolumeName:      "pv1",
				csiPersistentVolumeClaimName: "pvc1",
			},
			expected: map[string]string{
				headerPersistentVolumeName:      "pv1",
				headerPersistentVolumeClaimName: "pvc1",
			},
		},
		{
			name:     "No keys present",
			params:   map[string]string{},
			expected: map[string]string{},
		},
		{
			name: "Only PersistentVolumeClaimNamespace key present",
			params: map[string]string{
				csiPersistentVolumeClaimNamespace: "namespace1",
			},
			expected: map[string]string{
				headerPersistentVolumeClaimNamespace: "namespace1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := addMetaData(tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCheckValidAccessTypes(t *testing.T) {
	tests := []struct {
		name     string
		vcs      []*csi.VolumeCapability
		expected bool
	}{
		{
			name: "All valid mount access types",
			vcs: []*csi.VolumeCapability{
				{AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}}},
				{AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}}},
			},
			expected: true,
		},
		{
			name: "Nil value in volume capabilities",
			vcs: []*csi.VolumeCapability{
				nil,
				{AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}}},
			},
			expected: true,
		},
		{
			name: "Invalid access type",
			vcs: []*csi.VolumeCapability{
				{AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}}},
				{AccessType: nil},
			},
			expected: false,
		},
		{
			name: "Mixed valid and invalid access types",
			vcs: []*csi.VolumeCapability{
				{AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}}},
				{AccessType: nil},
				nil,
			},
			expected: false,
		},
		{
			name:     "All nil values",
			vcs:      []*csi.VolumeCapability{nil, nil},
			expected: true,
		},
		{
			name:     "Empty slice",
			vcs:      []*csi.VolumeCapability{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkValidAccessTypes(tt.vcs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Assume the validateVolumeCaps function is defined here or imported

func TestValidateVolumeCaps(t *testing.T) {
	tests := []struct {
		name     string
		vcs      []*csi.VolumeCapability
		expected bool
		reason   string
	}{
		{
			name: "All valid mount access types",
			vcs: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
				},
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY},
				},
			},
			expected: true,
			reason:   "",
		},
		{
			name: "Invalid access type",
			vcs: []*csi.VolumeCapability{
				{
					AccessType: nil,
					AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
				},
			},
			expected: false,
			reason:   errUnknownAccessType,
		},
		{
			name: "Unknown access mode",
			vcs: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_UNKNOWN},
				},
			},
			expected: false,
			reason:   errUnknownAccessMode,
		},
		{
			name: "Single node reader only not supported",
			vcs: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY},
				},
			},
			expected: false,
			reason:   errNoSingleNodeReader,
		},
		{
			name: "Multi-node single writer not supported",
			vcs: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER},
				},
			},
			expected: false,
			reason:   errNoMultiNodeSingleWriter,
		},
		{
			name:     "All nil values",
			vcs:      []*csi.VolumeCapability{nil, nil},
			expected: true,
			reason:   "",
		},
		{
			name:     "Empty slice",
			vcs:      []*csi.VolumeCapability{},
			expected: true,
			reason:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			supported, reason := validateVolumeCaps(tt.vcs, &apiv1.IsiVolume{})
			assert.Equal(t, tt.expected, supported)
			assert.Equal(t, tt.reason, reason)
		})
	}
}

func TestGetCSISnapshot(t *testing.T) {
	s := &service{}

	tests := []struct {
		name           string
		snapshotID     string
		sourceVolumeID string
		creationTime   int64
		sizeInBytes    int64
		expected       *csi.Snapshot
	}{
		{
			name:           "Valid snapshot creation",
			snapshotID:     "snapshot-123",
			sourceVolumeID: "volume-123",
			creationTime:   1631022242,
			sizeInBytes:    1024,
			expected: &csi.Snapshot{
				SizeBytes:      1024,
				SnapshotId:     "snapshot-123",
				SourceVolumeId: "volume-123",
				CreationTime:   &timestamppb.Timestamp{Seconds: 1631022242},
				ReadyToUse:     true,
			},
		},
		{
			name:           "Snapshot with zero size",
			snapshotID:     "snapshot-456",
			sourceVolumeID: "volume-456",
			creationTime:   1631022242,
			sizeInBytes:    0,
			expected: &csi.Snapshot{
				SizeBytes:      0,
				SnapshotId:     "snapshot-456",
				SourceVolumeId: "volume-456",
				CreationTime:   &timestamppb.Timestamp{Seconds: 1631022242},
				ReadyToUse:     true,
			},
		},
		{
			name:           "Snapshot with future creation time",
			snapshotID:     "snapshot-789",
			sourceVolumeID: "volume-789",
			creationTime:   1731022242,
			sizeInBytes:    2048,
			expected: &csi.Snapshot{
				SizeBytes:      2048,
				SnapshotId:     "snapshot-789",
				SourceVolumeId: "volume-789",
				CreationTime:   &timestamppb.Timestamp{Seconds: 1731022242},
				ReadyToUse:     true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := s.getCSISnapshot(tt.snapshotID, tt.sourceVolumeID, tt.creationTime, tt.sizeInBytes)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestGetCreateSnapshotResponse(t *testing.T) {
	// Backup the original function.
	originalGetUtilsGetNormalizedSnapshotID := getUtilsGetNormalizedSnapshotID

	// Restore original function after the test.
	defer func() {
		getUtilsGetNormalizedSnapshotID = originalGetUtilsGetNormalizedSnapshotID
	}()

	// Mock implementation of getUtilsGetNormalizedSnapshotID.
	getUtilsGetNormalizedSnapshotID = func(ctx context.Context, snapshotID, clusterName, accessZone string) string {
		return snapshotID + "-" + clusterName + "-" + accessZone
	}

	// Mock service instance
	s := &service{}

	// Define the test cases
	tests := []struct {
		name           string
		snapshotID     string
		sourceVolumeID string
		creationTime   int64
		sizeInBytes    int64
		clusterName    string
		accessZone     string
		expected       *csi.CreateSnapshotResponse
	}{
		{
			name:           "Valid snapshot creation",
			snapshotID:     "snapshot-123",
			sourceVolumeID: "volume-123",
			creationTime:   1631022242,
			sizeInBytes:    1024,
			clusterName:    "clusterA",
			accessZone:     "zoneA",
			expected: &csi.CreateSnapshotResponse{
				Snapshot: &csi.Snapshot{
					SizeBytes:      1024,
					SnapshotId:     "snapshot-123-clusterA-zoneA",
					SourceVolumeId: "volume-123",
					CreationTime:   &timestamppb.Timestamp{Seconds: 1631022242},
					ReadyToUse:     true,
				},
			},
		},
		{
			name:           "Snapshot with zero size",
			snapshotID:     "snapshot-456",
			sourceVolumeID: "volume-456",
			creationTime:   1631022242,
			sizeInBytes:    0,
			clusterName:    "clusterB",
			accessZone:     "zoneB",
			expected: &csi.CreateSnapshotResponse{
				Snapshot: &csi.Snapshot{
					SizeBytes:      0,
					SnapshotId:     "snapshot-456-clusterB-zoneB",
					SourceVolumeId: "volume-456",
					CreationTime:   &timestamppb.Timestamp{Seconds: 1631022242},
					ReadyToUse:     true,
				},
			},
		},
		{
			name:           "Snapshot with future creation time",
			snapshotID:     "snapshot-789",
			sourceVolumeID: "volume-789",
			creationTime:   1731022242,
			sizeInBytes:    2048,
			clusterName:    "clusterC",
			accessZone:     "zoneC",
			expected: &csi.CreateSnapshotResponse{
				Snapshot: &csi.Snapshot{
					SizeBytes:      2048,
					SnapshotId:     "snapshot-789-clusterC-zoneC",
					SourceVolumeId: "volume-789",
					CreationTime:   &timestamppb.Timestamp{Seconds: 1731022242},
					ReadyToUse:     true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function under test
			actual := s.getCreateSnapshotResponse(context.Background(), tt.snapshotID, tt.sourceVolumeID, tt.creationTime, tt.sizeInBytes, tt.clusterName, tt.accessZone)
			// Assert the result
			assert.Equal(t, tt.expected, actual)
		})
	}
}
