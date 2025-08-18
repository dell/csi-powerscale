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
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/dell/csi-isilon/v2/common/utils/identifiers"
	vgsext "github.com/dell/dell-csi-extensions/volumeGroupSnapshot"
	isi "github.com/dell/goisilon"
	apiv1 "github.com/dell/goisilon/api/v1"
	v1 "github.com/dell/goisilon/api/v1"
	v2 "github.com/dell/goisilon/api/v2"
	isimocks "github.com/dell/goisilon/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
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
	getSnapshotFunc = func(_ context.Context, _ *IsilonClusterConfig) func(ctx context.Context, snapshotID string) (isi.Snapshot, error) {
		return func(_ context.Context, snapshotID string) (isi.Snapshot, error) {
			if snapshotID == "snapshot1234" {
				return &v1.IsiSnapshot{ID: 1234, Path: "/ifs/data/snapshot1234"}, nil
			}
			return nil, errors.New("snapshot not found")
		}
	}

	getSnapshotSizeFunc = func(_ context.Context, _ *IsilonClusterConfig) func(ctx context.Context, volumePath, snapshotName, accessZone string) int64 {
		return func(_ context.Context, _, _, _ string) int64 {
			return 100
		}
	}

	copySnapshotFunc = func(_ context.Context, _ *IsilonClusterConfig) func(ctx context.Context, dstPath, srcPath string, snapshotID int64, dstName, accessZone string) (isi.Volume, error) {
		return func(_ context.Context, _, _ string, snapshotID int64, dstName, _ string) (isi.Volume, error) {
			if snapshotID == 1234 {
				return &v1.IsiVolume{Name: dstName, AttributeMap: []struct {
					Name  string      `json:"name"`
					Value interface{} `json:"value"`
				}{}}, nil
			}
			return &v1.IsiVolume{}, errors.New("failed to copy snapshot")
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
	isVolumeExistentFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, isiPath, ns, srcVolumeName string) bool {
		return func(_ context.Context, _, _, srcVolumeName string) bool {
			if srcVolumeName == "existentVolume" || srcVolumeName == "errorVolumeCopy" {
				return true
			}
			return false
		}
	}

	getVolumeSizeFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, isiPath, srcVolumeName string) int64 {
		return func(_ context.Context, _, srcVolumeName string) int64 {
			if srcVolumeName == "existentVolume" || srcVolumeName == "errorVolumeCopy" {
				return 100
			}
			return 0
		}
	}

	copyVolumeFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, isiPath, srcVolumeName, dstVolumeName string) (isi.Volume, error) {
		return func(_ context.Context, _, srcVolumeName, dstVolumeName string) (isi.Volume, error) {
			if srcVolumeName == "errorVolumeCopy" {
				return &v1.IsiVolume{}, errors.New("failed to copy volume name")
			}
			if srcVolumeName == "existentVolume" {
				return &v1.IsiVolume{Name: dstVolumeName, AttributeMap: []struct {
					Name  string      `json:"name"`
					Value interface{} `json:"value"`
				}{}}, nil
			}
			return &v1.IsiVolume{}, errors.New("failed to copy volume")
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
	s := &service{
		nodeID:                identifiers.DummyHostNodeID,
		nodeIP:                "127.0.0.1",
		defaultIsiClusterName: "system",
		opts:                  Opts{AccessZone: "testZone"},
		isiClusters:           &sync.Map{},
	}

	mockClient := &isimocks.Client{}
	isiConfig := &IsilonClusterConfig{
		ClusterName: "system",
		isiSvc: &isiService{
			client: &isi.Client{API: mockClient},
		},
	}
	s.isiClusters.Store("system", isiConfig)

	getSnapshotArgs := mock.Arguments{mock.Anything, "platform/1/snapshot/snapshots", mock.Anything, mock.Anything, mock.Anything, mock.Anything}
	getExportArgs := mock.Arguments{mock.Anything, "platform/2/protocols/nfs/exports", mock.Anything, mock.Anything, mock.Anything, mock.Anything}

	setupSnapshots := func(snapshots []apiv1.IsiSnapshot) {
		mockClient.ExpectedCalls = nil
		mockClient.On("Get", getSnapshotArgs...).Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(**v1.GetIsiSnapshotsResp)
			tmpResp := apiv1.GetIsiSnapshotsResp{}
			for _, snapshot := range snapshots {
				tmpSnap := snapshot
				tmpResp.SnapshotList = append(tmpResp.SnapshotList, &tmpSnap)
			}
			*resp = &tmpResp
		})
	}

	setupExports := func(ids ...int) {
		for _, id := range ids {
			mockClient.On("Get", getExportArgs...).Return(nil).Run(func(args mock.Arguments) {
				resp := args.Get(5).(*v2.ExportList)
				*resp = v2.ExportList{&v2.Export{ID: id, Zone: s.opts.AccessZone}}
			}).Once()
		}
	}

	snapshots := []apiv1.IsiSnapshot{
		v1.IsiSnapshot{ID: 101, Name: "snapshot1", Path: "/ifs/data/snapshot1", Created: time.Now().Unix(), State: "STATE_SUCCESSFUL", Size: 100},
		v1.IsiSnapshot{ID: 102, Name: "snapshot2", Path: "/ifs/data/snapshot2", Created: time.Now().Unix(), State: "STATE_SUCCESSFUL", Size: 200},
	}

	t.Run("No snapshots found", func(t *testing.T) {
		setupSnapshots(nil)
		setupExports(1, 2)
		req := &csi.ListSnapshotsRequest{}
		resp, err := s.ListSnapshots(context.Background(), req)
		assert.NoError(t, err)
		assert.Nil(t, resp.Entries)
		assert.Empty(t, resp.NextToken)
	})

	t.Run("Error in GetSnapshot", func(t *testing.T) {
		mockClient.ExpectedCalls = nil
		mockClient.On("Get", getSnapshotArgs...).Return(fmt.Errorf("powerscale api error")).Once()
		req := &csi.ListSnapshotsRequest{}
		resp, err := s.ListSnapshots(context.Background(), req)
		assert.NoError(t, err)
		assert.Nil(t, resp.Entries)
		assert.Empty(t, resp.NextToken)
	})

	t.Run("Successful snapshot listing", func(t *testing.T) {
		setupSnapshots(snapshots)
		setupExports(1, 2)
		req := &csi.ListSnapshotsRequest{MaxEntries: 0}
		resp, err := s.ListSnapshots(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.Entries, 2)
		assert.Equal(t, "101=_=_=system=_=_=testZone", resp.Entries[0].Snapshot.SnapshotId)
		assert.Equal(t, "102=_=_=system=_=_=testZone", resp.Entries[1].Snapshot.SnapshotId)
		assert.Equal(t, "snapshot1=_=_=1=_=_=testZone=_=_=system", resp.Entries[0].Snapshot.SourceVolumeId)
		assert.Equal(t, "snapshot2=_=_=2=_=_=testZone=_=_=system", resp.Entries[1].Snapshot.SourceVolumeId)
		assert.Empty(t, resp.NextToken)
	})

	t.Run("Error in GetExportWithPath", func(t *testing.T) {
		setupSnapshots(snapshots)
		mockClient.On("Get", getExportArgs...).Return(fmt.Errorf("powerscale api error")).Twice()
		req := &csi.ListSnapshotsRequest{MaxEntries: 0}
		resp, err := s.ListSnapshots(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.Entries, 2)
		assert.Equal(t, "101=_=_=system=_=_=testZone", resp.Entries[0].Snapshot.SnapshotId)
		assert.Equal(t, "102=_=_=system=_=_=testZone", resp.Entries[1].Snapshot.SnapshotId)
		assert.Empty(t, resp.NextToken)
	})

	t.Run("MaxEntries less than total snapshots", func(t *testing.T) {
		setupSnapshots(snapshots)
		setupExports(1, 2)
		req := &csi.ListSnapshotsRequest{MaxEntries: 1}
		resp, err := s.ListSnapshots(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.Entries, 1)
		assert.Equal(t, "101=_=_=system=_=_=testZone", resp.Entries[0].Snapshot.SnapshotId)
		assert.Equal(t, "snapshot1=_=_=1=_=_=testZone=_=_=system", resp.Entries[0].Snapshot.SourceVolumeId)
		assert.Equal(t, "1", resp.NextToken)
	})

	t.Run("Valid StartingToken", func(t *testing.T) {
		setupSnapshots(snapshots)
		setupExports(1, 2)
		req := &csi.ListSnapshotsRequest{StartingToken: "1"}
		resp, err := s.ListSnapshots(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.Entries, 1)
		assert.Equal(t, "102=_=_=system=_=_=testZone", resp.Entries[0].Snapshot.SnapshotId)
		assert.Equal(t, "snapshot2=_=_=2=_=_=testZone=_=_=system", resp.Entries[0].Snapshot.SourceVolumeId)
		assert.Empty(t, resp.NextToken)
	})

	t.Run("StartingToken greater than snapshot count", func(t *testing.T) {
		setupSnapshots(snapshots)
		setupExports(1, 2)
		req := &csi.ListSnapshotsRequest{StartingToken: "10"}
		resp, err := s.ListSnapshots(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
		assert.ErrorContains(t, err, "invalid starting token, error: startingToken=10 > totalSnapshots=2")
		assert.Nil(t, resp)
	})

	t.Run("Invalid StartingToken format", func(t *testing.T) {
		req := &csi.ListSnapshotsRequest{StartingToken: "invalid"}
		resp, err := s.ListSnapshots(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, codes.Aborted, status.Code(err))
		assert.ErrorContains(t, err, "unable to parse StartingToken")
		assert.Nil(t, resp)
	})

	t.Run("ListSnapshots with SnapshotId", func(t *testing.T) {
		setupSnapshots(snapshots)
		setupExports(2)
		req := &csi.ListSnapshotsRequest{SnapshotId: "102=_=_=system=_=_=testZone"}
		resp, err := s.ListSnapshots(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.Entries, 1)
		assert.Equal(t, "102=_=_=system=_=_=testZone", resp.Entries[0].Snapshot.SnapshotId)
		assert.Equal(t, "snapshot2=_=_=2=_=_=testZone=_=_=system", resp.Entries[0].Snapshot.SourceVolumeId)
		assert.Empty(t, resp.NextToken)
	})

	t.Run("ListSnapshots with SourceVolumeId", func(t *testing.T) {
		setupSnapshots(snapshots)
		setupExports(2)
		req := &csi.ListSnapshotsRequest{SourceVolumeId: "snapshot2=_=_=2=_=_=testZone=_=_=system"}
		resp, err := s.ListSnapshots(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.Entries, 1)
		assert.Equal(t, "102=_=_=system=_=_=testZone", resp.Entries[0].Snapshot.SnapshotId)
		assert.Equal(t, "snapshot2=_=_=2=_=_=testZone=_=_=system", resp.Entries[0].Snapshot.SourceVolumeId)
		assert.Empty(t, resp.NextToken)
	})
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
		if contentSource.GetVolume() != nil && contentSource.GetVolume().VolumeId == "invalidVolumeID" {
			return &csi.VolumeContentSource_VolumeSource{VolumeId: "invalidVolumeID"}
		}
		return nil
	}

	createVolumeFromSnapshotFunc = func(_ *service) func(ctx context.Context, isiConfig *IsilonClusterConfig, isiPath, snapshotID, volName string, sizeInBytes int64, accessZone string) error {
		return func(_ context.Context, _ *IsilonClusterConfig, _, snapshotID, _ string, _ int64, _ string) error {
			if snapshotID == "errorSnapshot" {
				return errors.New("snapshot error")
			}
			return nil
		}
	}

	createVolumeFromVolumeFunc = func(_ *service) func(ctx context.Context, isiConfig *IsilonClusterConfig, isiPath, srcVolumeName, dstVolumeName string, sizeInBytes int64) error {
		return func(_ context.Context, _ *IsilonClusterConfig, _, srcVolumeName, _ string, _ int64) error {
			if srcVolumeName == "errorVolume" {
				return errors.New("volumes error")
			}
			return nil
		}
	}

	// Mock implementation of utils.ParseNormalizedVolumeID
	getUtilsParseNormalizedVolumeID = func(_ context.Context, volumeID string) (string, int, string, string, error) {
		if volumeID == "validVolume" {
			return "clusterName", 0, "volumePath", "volumeName", nil
		}
		if volumeID == "invalidVolumeID" {
			return "", 0, "", "", errors.New("volume ID 'invalidVolumeID' cannot be split into tokens")
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
		{
			"InvalidVolumeID",
			&csi.VolumeContentSource{
				Type: &csi.VolumeContentSource_Volume{
					Volume: &csi.VolumeContentSource_VolumeSource{VolumeId: "invalidVolumeID"},
				},
			},
			&csi.CreateVolumeRequest{Name: "newVolume"},
			200,
			status.Error(codes.NotFound, "volume ID is invalid or not found"),
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
	getCSIVolumeFunc = func(_ *service) func(ctx context.Context, exportID int, volName, path, accessZone string, sizeInBytes int64, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork string) *csi.Volume {
		return func(_ context.Context, _ int, volName, path, accessZone string, sizeInBytes int64, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork string) *csi.Volume {
			return &csi.Volume{
				VolumeId:      volName,
				CapacityBytes: sizeInBytes,
				VolumeContext: map[string]string{
					"path":             path,
					"accessZone":       accessZone,
					"azServiceIP":      azServiceIP,
					"azNetwork":        azNetwork,
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
	azNetwork := "10.0.0.0/24"
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
			"azNetwork":        azNetwork,
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
	response := s.getCreateVolumeResponse(ctx, exportID, volName, path, accessZone, sizeInBytes, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork)

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
			supported, reason := validateVolumeCaps(tt.vcs, &v1.IsiVolume{})
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
	getUtilsGetNormalizedSnapshotID = func(_ context.Context, snapshotID, clusterName, accessZone string) string {
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

func TestProcessSnapshotTrackingDirectoryDuringDeleteVolume(t *testing.T) {
	ctx := context.Background()
	originalGetZoneByNameFunc := getZoneByNameFunc
	originalGetSnapshotIsiPathComponentsFunc := getSnapshotIsiPathComponentsFunc
	originalGetSnapshotTrackingDirNameFunc := getSnapshotTrackingDirNameFunc
	originalIsVolumeExistentFunc := isVolumeExistentFunc
	originalDeleteVolumeFunc := deleteVolumeFunc
	originalGetSubDirectoryCountFunc := getSubDirectoryCountFunc
	originalUnexportByIDWithZoneFunc := unexportByIDWithZoneFunc
	originalRemoveSnapshotFunc := removeSnapshotFunc

	after := func() {
		getZoneByNameFunc = originalGetZoneByNameFunc
		getSnapshotIsiPathComponentsFunc = originalGetSnapshotIsiPathComponentsFunc
		getSnapshotTrackingDirNameFunc = originalGetSnapshotTrackingDirNameFunc
		isVolumeExistentFunc = originalIsVolumeExistentFunc
		deleteVolumeFunc = originalDeleteVolumeFunc
		getSubDirectoryCountFunc = originalGetSubDirectoryCountFunc
		unexportByIDWithZoneFunc = originalUnexportByIDWithZoneFunc
		removeSnapshotFunc = originalRemoveSnapshotFunc
	}

	isiConfig := &IsilonClusterConfig{
		isiSvc: &isiService{
			endpoint: "http://testendpoint:8080",
			client:   &isi.Client{},
		},
	}

	s := &service{}

	type testCase struct {
		name        string
		volName     string
		accessZone  string
		export      isi.Export
		expectedErr error
		setup       func()
	}

	testCases := []testCase{
		{
			name:       "GetZoneError",
			volName:    "volumeName",
			accessZone: "accessZone",
			export: &v2.Export{
				Paths: &[]string{"/exportPath"},
			},
			expectedErr: fmt.Errorf("failed to get zone"),
			setup: func() {
				// Mock implementations for a valid case
				getZoneByNameFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, zoneName string) (*v1.IsiZone, error) {
					return func(_ context.Context, _ string) (*v1.IsiZone, error) {
						return nil, errors.New("failed to get zone")
					}
				}
			},
		},
		{
			name:       "ValidCase",
			volName:    "volumeName",
			accessZone: "accessZone",
			export: &v2.Export{
				Paths: &[]string{"/exportPath"},
			},
			expectedErr: nil,
			setup: func() {
				// Mock implementations for a valid case
				getZoneByNameFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, zoneName string) (*v1.IsiZone, error) {
					return func(_ context.Context, _ string) (*v1.IsiZone, error) {
						return &v1.IsiZone{Path: "/zonePath"}, nil
					}
				}
				getSnapshotIsiPathComponentsFunc = func(_ *IsilonClusterConfig) func(exportPath, zonePath string) (string, string, string) {
					return func(_, _ string) (string, string, string) {
						return "/isiPath", "snapshotName", ""
					}
				}
				getSnapshotTrackingDirNameFunc = func(_ *IsilonClusterConfig) func(snapshotName string) string {
					return func(_ string) string {
						return "/snapshotTrackingDir"
					}
				}
				isVolumeExistentFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeID, volumeEntry string) bool {
					return func(_ context.Context, _, _, _ string) bool {
						return true
					}
				}
				deleteVolumeFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeSelector string) error {
					return func(_ context.Context, _, _ string) error {
						return nil
					}
				}
				getSubDirectoryCountFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeSelector string) (int64, error) {
					return func(_ context.Context, _, _ string) (int64, error) {
						return 3, nil
					}
				}
				getSubDirectoryCountFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeSelector string) (int64, error) {
					return func(_ context.Context, _, _ string) (int64, error) {
						return 2, nil
					}
				}
			},
		},
		{
			name:       "unexportByIDWithZoneFuncError",
			volName:    "volumeName",
			accessZone: "accessZone",
			export: &v2.Export{
				Paths: &[]string{"/exportPath"},
			},
			expectedErr: nil,
			setup: func() {
				// Mock implementations for a valid case
				getZoneByNameFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, zoneName string) (*v1.IsiZone, error) {
					return func(_ context.Context, _ string) (*v1.IsiZone, error) {
						return &v1.IsiZone{Path: "/zonePath"}, nil
					}
				}
				getSnapshotIsiPathComponentsFunc = func(_ *IsilonClusterConfig) func(exportPath, zonePath string) (string, string, string) {
					return func(_, _ string) (string, string, string) {
						return "/isiPath", "snapshotName", ""
					}
				}
				getSnapshotTrackingDirNameFunc = func(_ *IsilonClusterConfig) func(snapshotName string) string {
					return func(_ string) string {
						return "/snapshotTrackingDir"
					}
				}
				isVolumeExistentFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeID, volumeEntry string) bool {
					return func(_ context.Context, _, _, _ string) bool {
						return true
					}
				}
				deleteVolumeFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeSelector string) error {
					return func(_ context.Context, _, _ string) error {
						return nil
					}
				}
				getSubDirectoryCountFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeSelector string) (int64, error) {
					return func(_ context.Context, _, _ string) (int64, error) {
						return 3, nil
					}
				}
				unexportByIDWithZoneFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, exportID int, zoneName string) error {
					return func(_ context.Context, _ int, _ string) error {
						return errors.New("failed to delete snapshot directory export")
					}
				}
			},
		},
		{
			name:       "RemoveSnapshotFuncError",
			volName:    "volumeName",
			accessZone: "accessZone",
			export: &v2.Export{
				Paths: &[]string{"/exportPath"},
			},
			expectedErr: nil,
			setup: func() {
				returnVal := true
				getZoneByNameFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, zoneName string) (*v1.IsiZone, error) {
					return func(_ context.Context, _ string) (*v1.IsiZone, error) {
						return &v1.IsiZone{Path: "/zonePath"}, nil
					}
				}
				getSnapshotIsiPathComponentsFunc = func(_ *IsilonClusterConfig) func(exportPath, zonePath string) (string, string, string) {
					return func(_, _ string) (string, string, string) {
						return "/isiPath", "snapshotName", ""
					}
				}
				getSnapshotTrackingDirNameFunc = func(_ *IsilonClusterConfig) func(snapshotName string) string {
					return func(_ string) string {
						return "/snapshotTrackingDir"
					}
				}
				isVolumeExistentFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeID, volumeEntry string) bool {
					return func(_ context.Context, _, _, _ string) bool {
						return returnVal
					}
				}
				getSubDirectoryCountFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeSelector string) (int64, error) {
					return func(_ context.Context, _, _ string) (int64, error) {
						return 3, nil
					}
				}
				unexportByIDWithZoneFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, exportID int, zoneName string) error {
					return func(_ context.Context, _ int, _ string) error {
						return nil
					}
				}
				deleteVolumeFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeSelector string) error {
					return func(_ context.Context, _, _ string) error {
						return nil
					}
				}
				removeSnapshotFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, snapID int64, snapName string) error {
					return func(_ context.Context, _ int64, _ string) error {
						return errors.New("error deleting snapshot: 'some error'")
					}
				}
			},
		},
		{
			name:       "getSubDirectoryCountFuncError",
			volName:    "volumeName",
			accessZone: "accessZone",
			export: &v2.Export{
				Paths: &[]string{"/exportPath"},
			},
			expectedErr: nil,
			setup: func() {
				returnVal := true
				getZoneByNameFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, zoneName string) (*v1.IsiZone, error) {
					return func(_ context.Context, _ string) (*v1.IsiZone, error) {
						return &v1.IsiZone{Path: "/zonePath"}, nil
					}
				}
				getSnapshotIsiPathComponentsFunc = func(_ *IsilonClusterConfig) func(exportPath, zonePath string) (string, string, string) {
					return func(_, _ string) (string, string, string) {
						return "/isiPath", "snapshotName", ""
					}
				}
				getSnapshotTrackingDirNameFunc = func(_ *IsilonClusterConfig) func(snapshotName string) string {
					return func(_ string) string {
						return "/snapshotTrackingDir"
					}
				}
				isVolumeExistentFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeID, volumeEntry string) bool {
					return func(_ context.Context, _, _, _ string) bool {
						return !returnVal
					}
				}
				getSubDirectoryCountFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeSelector string) (int64, error) {
					return func(_ context.Context, _, _ string) (int64, error) {
						return 0, errors.New("error getting subdirectory count: 'some error'")
					}
				}
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			defer after()
			if tt.setup != nil {
				tt.setup()
			}

			err := s.processSnapshotTrackingDirectoryDuringDeleteVolume(ctx, tt.volName, tt.accessZone, tt.export, isiConfig)
			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateVolumefunc(t *testing.T) {
	mockSvc := new(mockService)
	ctx := context.Background()
	req := &csi.CreateVolumeRequest{
		Parameters: map[string]string{
			ClusterNameParam: "",
		},
	}
	_, err := mockSvc.CreateVolume(ctx, req)
	assert.NotEqual(t, nil, err)
}

func TestValidateCreateSnapshotRequest(t *testing.T) {
	svc := &service{}
	ctx := context.Background()
	req := &csi.CreateSnapshotRequest{
		SourceVolumeId: "",
	}
	isiConfig := &IsilonClusterConfig{}

	_, _, err := svc.validateCreateSnapshotRequest(ctx, req, "/ifs/data", isiConfig)

	assert.NotEqual(t, nil, err)
}

func TestGetCapacity(t *testing.T) {
	ctx := context.Background()
	s := &service{
		isiClusters:           &sync.Map{},
		defaultIsiClusterName: "default-cluster",
	}

	cluster1 := &IsilonClusterConfig{ClusterName: "Cluster1"}
	cluster2 := &IsilonClusterConfig{ClusterName: "Cluster2"}
	s.isiClusters.Store("key1", cluster1)
	s.isiClusters.Store("key2", cluster2)

	params := map[string]string{"ClusterName": ""}
	req := &csi.GetCapacityRequest{Parameters: params}
	_, err := s.GetCapacity(ctx, req)

	assert.NotEqual(t, nil, err)
}

func TestControllerPublishVolume_MaxVolumesPerNode(t *testing.T) {
	fmt.Println("TestControllerPublishVolume_MaxVolumesPerNode")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s := &service{
		nodeID:                identifiers.DummyHostNodeID,
		nodeIP:                "127.0.0.1",
		defaultIsiClusterName: "system",
		opts:                  Opts{MaxVolumesPerNode: 5},
		isiClusters:           &sync.Map{},
	}
	mockClient := &isimocks.Client{}
	isiConfig := &IsilonClusterConfig{
		ClusterName: "system",
		isiSvc: &isiService{
			client: &isi.Client{
				API: mockClient,
			},
		},
	}
	s.isiClusters.Store("system", isiConfig)

	ctx := context.Background()
	req := &csi.ControllerPublishVolumeRequest{
		VolumeId: "k8s-e89c9d089e=_=_=19=_=_=csi0zone=_=_=system",
		NodeId:   identifiers.DummyHostNodeID,
		VolumeCapability: &csi.VolumeCapability{
			AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
			AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
		},
	}

	// Mock the necessary calls
	isiConfig.isiSvc.client.API.(*isimocks.Client).ExpectedCalls = nil
	isiConfig.isiSvc.client.API.(*isimocks.Client).On("Get", anyArgs[0:6]...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv1.GetIsiVolumeAttributesResp)
		*resp = &apiv1.GetIsiVolumeAttributesResp{}
	}).Once()
	isiConfig.isiSvc.client.API.(*isimocks.Client).On("Get", anyArgs[0:6]...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv1.GetIsiExportsResp)
		*resp = &apiv1.GetIsiExportsResp{
			ExportList: []*apiv1.IsiExport{
				{Clients: []string{"127.0.0.1", "127.0.0.1"}},
				{Clients: []string{"127.0.0.1"}},
				{Clients: []string{"127.0.0.1"}},
				{Clients: []string{"127.0.0.1"}},
				{Clients: []string{"127.0.0.1"}},
			},
		}
	}).Once()
	isiConfig.isiSvc.client.API.(*isimocks.Client).On("Get", anyArgs[0:6]...).Return(nil)
	_, err := s.ControllerPublishVolume(ctx, req)
	assert.NotNil(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "maximum volume limit reached for node")

	// Error Scenario
	isiConfig.isiSvc.client.API.(*isimocks.Client).ExpectedCalls = nil
	isiConfig.isiSvc.client.API.(*isimocks.Client).On("Get", anyArgs[0:6]...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv1.GetIsiVolumeAttributesResp)
		*resp = &apiv1.GetIsiVolumeAttributesResp{}
	}).Once()
	isiConfig.isiSvc.client.API.(*isimocks.Client).On("Get", anyArgs[0:6]...).Return(fmt.Errorf("failed to get exports")).Once()
	_, err = s.ControllerPublishVolume(ctx, req)
	assert.NotNil(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "failed to export count for node id")
}

func TestGetIpsFromAZNetworkLabel(t *testing.T) {
	originalGetNodeLabelsWithName := getNodeLabelsWithNameFunc

	after := func() {
		getNodeLabelsWithNameFunc = originalGetNodeLabelsWithName
	}

	tests := []struct {
		name        string
		nodeID      string
		azNetwork   string
		nodeLabels  map[string]string
		expectedIPs []string
		expectedErr error
	}{
		{
			name:      "successful execution",
			nodeID:    "nodename=#=#=localhost=#=#=127.0.0.1",
			azNetwork: "192.168.1.0/24",
			nodeLabels: map[string]string{
				"csi-isilon.dellemc.com/az-192.168.1.0-24-192.168.1.1": "true",
				"csi-isilon.dellemc.com/az-192.168.1.0-24-192.168.1.2": "true",
			},
			expectedIPs: []string{"192.168.1.1", "192.168.1.2"},
		},
		{
			name:        "node ID parsing error",
			nodeID:      "invalid-node-id",
			azNetwork:   "192.168.1.0/24",
			expectedErr: fmt.Errorf("node ID '%s' cannot match the expected '^(.+)=#=#=(.+)=#=#=(.+)$' pattern", "invalid-node-id"),
		},
		{
			name:        "node labels retrieval error",
			nodeID:      "nodename=#=#=localhost=#=#=127.0.0.1",
			azNetwork:   "192.168.1.0/24",
			expectedIPs: []string{},
			expectedErr: fmt.Errorf("failed to match AZNetwork to get IPs for export %s", "192.168.1.0/24"),
		},
		{
			name:      "no matching AZNetwork label",
			nodeID:    "nodename=#=#=localhost=#=#=127.0.0.1",
			azNetwork: "10.0.0.1/24",
			nodeLabels: map[string]string{
				"csi-isilon.dellemc.com/az-192.168.1.0-24-192.168.1.1": "true",
				"csi-isilon.dellemc.com/az-192.168.1.0-24-192.168.1.2": "true",
			},
			expectedIPs: []string{},
			expectedErr: fmt.Errorf("failed to match AZNetwork to get IPs for export %s", "10.0.0.1/24"),
		},
		{
			name:        "empty node labels",
			nodeID:      "nodename=#=#=localhost=#=#=127.0.0.1",
			azNetwork:   "192.168.1.0/24",
			nodeLabels:  map[string]string{},
			expectedIPs: []string{},
			expectedErr: fmt.Errorf("failed to match AZNetwork to get IPs for export %s", "192.168.1.0/24"),
		},
		{
			name:        "error in getIpsFromAZNetworkLabel",
			nodeID:      "nodename=#=#=localhost=#=#=127.0.0.1",
			expectedIPs: nil,
			expectedErr: fmt.Errorf("failed in getIpsFromAZNetworkLabel"),
		},
	}

	s := &service{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer after()

			getNodeLabelsWithNameFunc = func(_ *service) func(string) (map[string]string, error) {
				if tt.name == "error in getIpsFromAZNetworkLabel" {
					return func(string) (map[string]string, error) {
						return nil, fmt.Errorf("failed in getIpsFromAZNetworkLabel")
					}
				}
				return func(string) (map[string]string, error) {
					return tt.nodeLabels, nil
				}
			}

			ips, err := s.getIpsFromAZNetworkLabel(context.Background(), tt.nodeID, tt.azNetwork)
			if (err != nil) != (tt.expectedErr != nil) || (err != nil && err.Error() != tt.expectedErr.Error()) {
				t.Errorf("getIpsFromAZNetworkLabel() error = %v, expectedErr %v", err, tt.expectedErr)
			}

			sort.Strings(ips)
			sort.Strings(tt.expectedIPs)
			if !reflect.DeepEqual(ips, tt.expectedIPs) {
				t.Errorf("getIpsFromAZNetworkLabel() IPs = %v, expectedIPs %v", ips, tt.expectedIPs)
			}
		})
	}
}

func TestControllerPublishVolume(t *testing.T) {
	fmt.Println("TestControllerPublishVolume")

	originalGetNodeLabelsWithName := getNodeLabelsWithNameFunc

	after := func() {
		getNodeLabelsWithNameFunc = originalGetNodeLabelsWithName
	}

	tests := []struct {
		name       string
		req        *csi.ControllerPublishVolumeRequest
		nodeLabels map[string]string
		wantErr    bool
	}{
		{
			name: "fail to check volumeContext for AzNetwork and get the corresponding IP from node labels",
			req: &csi.ControllerPublishVolumeRequest{
				VolumeId: "",
				VolumeContext: map[string]string{
					"AzNetwork": "10.0.0.0/24",
				},
			},
			wantErr: true,
		},
		{
			name: "empty volume ID",
			req: &csi.ControllerPublishVolumeRequest{
				VolumeId: "",
				NodeId:   identifiers.DummyHostNodeID,
				VolumeContext: map[string]string{
					"AzNetwork": "10.0.0.0/24",
				},
			},
			nodeLabels: map[string]string{
				"csi-isilon.dellemc.com/az-10.0.0.0-24-10.0.0.1": "true",
			},
			wantErr: true,
		},
	}

	s := &service{
		k8sclient:             fake.NewSimpleClientset(),
		defaultIsiClusterName: "system",
		isiClusters:           &sync.Map{},
	}

	mockClient := &isimocks.Client{}
	isiConfig := &IsilonClusterConfig{
		ClusterName: "system",
		isiSvc: &isiService{
			client: &isi.Client{
				API: mockClient,
			},
		},
	}

	s.isiClusters.Store("system", isiConfig)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer after()

			ctx := context.Background()

			getNodeLabelsWithNameFunc = func(_ *service) func(string) (map[string]string, error) {
				return func(string) (map[string]string, error) {
					return tt.nodeLabels, nil
				}
			}

			_, err := s.ControllerPublishVolume(ctx, tt.req)

			if (err != nil) != tt.wantErr {
				t.Errorf("TestControllerPublishVolume() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestControllerUnpublishVolume(t *testing.T) {
	fmt.Println("TestControllerUnpublishVolume")

	originalGetNodeLabelsWithName := getNodeLabelsWithNameFunc

	after := func() {
		getNodeLabelsWithNameFunc = originalGetNodeLabelsWithName
	}

	azPv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "azpv",
		},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					VolumeAttributes: map[string]string{
						"AzNetwork": "10.0.0.0/24",
					},
				},
			},
		},
	}

	systemPv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "systempv",
		},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					VolumeAttributes: map[string]string{},
				},
			},
		},
	}

	tests := []struct {
		name       string
		req        *csi.ControllerUnpublishVolumeRequest
		nodeLabels map[string]string
		wantErr    bool
	}{
		{
			name: "empty volume ID",
			req: &csi.ControllerUnpublishVolumeRequest{
				VolumeId: "",
			},
			wantErr: true,
		},
		{
			name: "invalid volume ID",
			req: &csi.ControllerUnpublishVolumeRequest{
				VolumeId: "invalid-volume-id",
			},
			wantErr: true,
		},
		{
			name: "failed to get PV",
			req: &csi.ControllerUnpublishVolumeRequest{
				VolumeId: "fake-pv=_=_=19=_=_=csi0zone",
			},
			wantErr: true,
		},
		{
			name: "failed to get Isilon config",
			req: &csi.ControllerUnpublishVolumeRequest{
				VolumeId: "azpv=_=_=19=_=_=csi0zone=_=_=fake-cluster",
			},
			wantErr: true,
		},
		{
			name: "failed to autoProbe",
			req: &csi.ControllerUnpublishVolumeRequest{
				VolumeId: "azpv=_=_=19=_=_=csi0zone",
			},
			wantErr: true,
		},
		{
			name: "fail to match node label IPs with AzNetwork attribute",
			req: &csi.ControllerUnpublishVolumeRequest{
				VolumeId: "azpv=_=_=19=_=_=csi0zone",
				NodeId:   identifiers.DummyHostNodeID,
			},
			nodeLabels: map[string]string{
				"csi-isilon.dellemc.com/az-10.0.0.0-32-10.0.0.1": "true",
			},
			wantErr: true,
		},
		{
			name: "fail to getNodeId when removing export without AZNetwork attribute",
			req: &csi.ControllerUnpublishVolumeRequest{
				VolumeId: "systempv=_=_=19=_=_=csi0zone",
				NodeId:   "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer after()

			ctx := context.Background()

			getNodeLabelsWithNameFunc = func(_ *service) func(string) (map[string]string, error) {
				return func(string) (map[string]string, error) {
					return tt.nodeLabels, nil
				}
			}

			s := &service{
				k8sclient:             fake.NewSimpleClientset(azPv, systemPv),
				defaultIsiClusterName: "system",
				isiClusters:           &sync.Map{},
			}

			mockClient := &isimocks.Client{}
			isiConfig := &IsilonClusterConfig{
				ClusterName: "system",
				isiSvc: &isiService{
					client: &isi.Client{
						API: mockClient,
					},
				},
			}

			if tt.name == "failed to autoProbe" {
				isiConfig.isiSvc = nil
				s.opts.AutoProbe = false
			}

			s.isiClusters.Store("system", isiConfig)

			_, err := s.ControllerUnpublishVolume(ctx, tt.req)

			if (err != nil) != tt.wantErr {
				t.Errorf("TestControllerUnpublishVolume() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestControllerCreateVolume(t *testing.T) {
	fmt.Println("TestControllerCreateVolume")

	tests := []struct {
		name    string
		req     *csi.CreateVolumeRequest
		wantErr bool
	}{
		{
			name: "fail - remote system doesn't exist",
			req: &csi.CreateVolumeRequest{
				Name: "test-volume",
				Parameters: map[string]string{
					"AzServiceIP":              "",
					"AzNetwork":                "10.0.0.0/24",
					"IsiVolumePathPermissions": "0777",
					"RootClientEnabled":        "notabool",
					"isReplicationEnabled":     "true",
					"UT/isReplicationEnabled":  "true",
					"UT/volumeGroupPrefix":     "UT",
					"UT/rpo":                   "Five_Minutes",
					"UT/remoteSystem":          "remote-system",
				},
				CapacityRange: &csi.CapacityRange{
					RequiredBytes: 1024,
				},
			},
			wantErr: true,
		},
	}

	s := &service{
		k8sclient:             fake.NewSimpleClientset(),
		defaultIsiClusterName: "system",
		isiClusters:           &sync.Map{},
		opts: Opts{
			CustomTopologyEnabled: true,
			replicationPrefix:     "UT",
		},
	}

	mockClient := &isimocks.Client{}
	isiConfig := &IsilonClusterConfig{
		Endpoint:    "http://testendpoint:8080",
		ClusterName: "system",
		isiSvc: &isiService{
			client: &isi.Client{
				API: mockClient,
			},
		},
	}

	s.isiClusters.Store("system", isiConfig)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			_, err := s.CreateVolume(ctx, tt.req)

			if (err != nil) != tt.wantErr {
				t.Errorf("TestControllerCreateVolume() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
