// Copyright © 2025-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/constants"
	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/utils/identifiers"
	isi "github.com/Ecosystems/container-storage-modules/src/gopowerscale"
	isiapi "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api"
	apiv1 "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api/v1"
	v1 "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api/v1"
	apiv17 "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api/v17"
	v2 "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api/v2"
	isimocks "github.com/Ecosystems/container-storage-modules/src/gopowerscale/mocks"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
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
		mutableParams     map[string]string
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
		{
			name: "Mutable params override PVC and SC",
			params: map[string]string{
				SoftLimitParam:        "",
				AdvisoryLimitParam:    "",
				SoftGracePrdParam:     "",
				PVCSoftLimitParam:     "70",
				PVCAdvisoryLimitParam: "80",
				PVCSoftGracePrdParam:  "30",
			},
			mutableParams: map[string]string{
				SoftLimitParam:     "90",
				AdvisoryLimitParam: "85",
				SoftGracePrdParam:  "40",
			},
			expectedSoft:      "90",
			expectedAdv:       "85",
			expectedSoftGrace: "40",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			softLimit, advisoryLimit, softGracePrd := readQuotaLimitParams(tc.params, tc.mutableParams)
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

func TestIsIgnorableWritableSnapshotErrors(t *testing.T) {
	tests := []struct {
		name                string
		err                 error
		expectLookupIgnored bool
		expectDeleteIgnored bool
	}{
		{
			name:                "Nil error",
			err:                 nil,
			expectLookupIgnored: false,
			expectDeleteIgnored: false,
		},
		{
			name:                "JSON 404",
			err:                 &isiapi.JSONError{StatusCode: 404},
			expectLookupIgnored: true,
			expectDeleteIgnored: true,
		},
		{
			name:                "Not a writable snapshot member",
			err:                 errors.New("Not a member of WritableSnapshot domain"),
			expectLookupIgnored: true,
			expectDeleteIgnored: true,
		},
		{
			name:                "Writable snapshot not found",
			err:                 errors.New("Writable snapshot not found"),
			expectLookupIgnored: true,
			expectDeleteIgnored: true,
		},
		{
			name:                "Failed to open path",
			err:                 errors.New("failed to open path"),
			expectLookupIgnored: true,
			expectDeleteIgnored: true,
		},
		{
			name:                "Non ignorable error",
			err:                 errors.New("backend timeout"),
			expectLookupIgnored: false,
			expectDeleteIgnored: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectLookupIgnored, isIgnorableWritableSnapshotLookupError(tt.err))
			assert.Equal(t, tt.expectDeleteIgnored, isIgnorableWritableSnapshotDeleteError(tt.err))
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
	getSnapshotFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, snapshotID string) (isi.Snapshot, error) {
		return func(_ context.Context, snapshotID string) (isi.Snapshot, error) {
			if snapshotID == "snapshot1234" {
				return &v1.IsiSnapshot{ID: 1234, Path: "/ifs/data/snapshot1234"}, nil
			}
			return nil, errors.New("snapshot not found")
		}
	}

	getSnapshotSizeFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, snapshotName, accessZone string) int64 {
		return func(_ context.Context, _, _, _ string) int64 {
			return 100
		}
	}

	copySnapshotFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, dstPath, srcPath string, snapshotID int64, dstName, accessZone string) (isi.Volume, error) {
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

func TestValidateSnapshotIQLicense(t *testing.T) {
	originalGetSnapshotIQLicenseStatusFunc := getSnapshotIQLicenseStatusFunc
	defer func() {
		getSnapshotIQLicenseStatusFunc = originalGetSnapshotIQLicenseStatusFunc
	}()

	isiConfig := &IsilonClusterConfig{isiSvc: &isiService{}}
	ctx := context.Background()

	tests := []struct {
		name           string
		mockStatus     string
		mockErr        error
		expectCode     codes.Code
		expectContains string
	}{
		{
			name:       "licensed status",
			mockStatus: "Licensed",
			expectCode: codes.OK,
		},
		{
			name:           "license lookup failed",
			mockErr:        errors.New("no license found"),
			expectCode:     codes.FailedPrecondition,
			expectContains: "SnapshotIQ license is not activated or available",
		},
		{
			name:       "license api unavailable fallback",
			mockErr:    errors.New("json: cannot unmarshal number into Go value of type api.JSONError"),
			expectCode: codes.OK,
		},
		{
			name:           "license not active",
			mockStatus:     "Expired",
			expectCode:     codes.FailedPrecondition,
			expectContains: "SnapshotIQ license status is 'Expired'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getSnapshotIQLicenseStatusFunc = func(context.Context, *IsilonClusterConfig) (string, error) {
				return tt.mockStatus, tt.mockErr
			}

			err := validateSnapshotIQLicense(ctx, isiConfig)
			if tt.expectCode == codes.OK {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)
			st, ok := status.FromError(err)
			assert.True(t, ok)
			assert.Equal(t, tt.expectCode, st.Code())
			assert.Contains(t, st.Message(), tt.expectContains)
		})
	}
}

func TestGetSnapshotIQLicenseStatusFunc(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error when service is not initialized", func(t *testing.T) {
		status, err := getSnapshotIQLicenseStatusFunc(ctx, nil)
		assert.Error(t, err)
		assert.Equal(t, "", status)
		assert.Contains(t, err.Error(), "isilon service is not initialized")
	})

	t.Run("returns error from backend lookup", func(t *testing.T) {
		mockClient := &isimocks.Client{}
		mockClient.On("Get", mock.Anything, "platform/17/license/licenses", SnapshotIQLicenseID, mock.Anything, mock.Anything, mock.Anything).
			Return(errors.New("mock license lookup error")).Once()

		isiConfig := &IsilonClusterConfig{
			isiSvc: &isiService{
				client: &isi.Client{API: mockClient},
			},
		}

		status, err := getSnapshotIQLicenseStatusFunc(ctx, isiConfig)
		assert.Error(t, err)
		assert.Equal(t, "", status)
		assert.Contains(t, err.Error(), "mock license lookup error")
		mockClient.AssertExpectations(t)
	})

	t.Run("returns license status when lookup succeeds", func(t *testing.T) {
		mockClient := &isimocks.Client{}
		mockClient.On("Get", mock.Anything, "platform/17/license/licenses", SnapshotIQLicenseID, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				resp := args.Get(5).(*apiv17.LicensesResponse)
				resp.Licenses = []apiv17.License{{ID: SnapshotIQLicenseID, Status: "Evaluation"}}
			}).
			Return(nil).Once()

		isiConfig := &IsilonClusterConfig{
			isiSvc: &isiService{
				client: &isi.Client{API: mockClient},
			},
		}

		status, err := getSnapshotIQLicenseStatusFunc(ctx, isiConfig)
		assert.NoError(t, err)
		assert.Equal(t, "Evaluation", status)
		mockClient.AssertExpectations(t)
	})
}

func TestCreateVolumeFromSnapshotErrorCodeMapping(t *testing.T) {
	originalGetSnapshotFunc := getSnapshotFunc
	originalGetSnapshotSizeFunc := getSnapshotSizeFunc
	originalCopySnapshotFunc := copySnapshotFunc
	defer func() {
		getSnapshotFunc = originalGetSnapshotFunc
		getSnapshotSizeFunc = originalGetSnapshotSizeFunc
		copySnapshotFunc = originalCopySnapshotFunc
	}()

	getSnapshotFunc = func(_ *IsilonClusterConfig) func(context.Context, string) (isi.Snapshot, error) {
		return func(_ context.Context, _ string) (isi.Snapshot, error) {
			return &apiv1.IsiSnapshot{
				ID:   1234,
				Name: "snap1",
				Path: "/ifs/data/src/snap1",
			}, nil
		}
	}
	getSnapshotSizeFunc = func(_ *IsilonClusterConfig) func(context.Context, string, string, string) int64 {
		return func(_ context.Context, _, _, _ string) int64 { return 1 }
	}

	tests := []struct {
		name       string
		copyErr    error
		expectCode codes.Code
	}{
		{
			name:       "snapshotiq license-like error returns internal",
			copyErr:    errors.New("SnapshotIQ is not licensed"),
			expectCode: codes.Internal,
		},
		{
			name:       "generic copy error returns internal",
			copyErr:    errors.New("copy snapshot failed"),
			expectCode: codes.Internal,
		},
	}

	ctx := context.Background()
	s := &service{}
	isiConfig := &IsilonClusterConfig{}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			copySnapshotFunc = func(_ *IsilonClusterConfig) func(context.Context, string, string, int64, string, string) (isi.Volume, error) {
				return func(_ context.Context, _, _ string, _ int64, _, _ string) (isi.Volume, error) {
					return nil, tc.copyErr
				}
			}

			err := s.createVolumeFromSnapshot(ctx, isiConfig, "/ifs/data", "snapshot1234", "vol-test", 2, "System")
			assert.Error(t, err)
			st, ok := status.FromError(err)
			assert.True(t, ok)
			assert.Equal(t, tc.expectCode, st.Code())
		})
	}
}

func TestCreateVolumeFromWritableSnapshotErrorCodeMapping(t *testing.T) {
	originalGetSnapshotFunc := getSnapshotFunc
	originalGetSnapshotSizeFunc := getSnapshotSizeFunc
	originalGetWritableSnapshotFunc := getWritableSnapshotFunc
	originalCreateWritableSnapshotFunc := createWritableSnapshotFunc
	defer func() {
		getSnapshotFunc = originalGetSnapshotFunc
		getSnapshotSizeFunc = originalGetSnapshotSizeFunc
		getWritableSnapshotFunc = originalGetWritableSnapshotFunc
		createWritableSnapshotFunc = originalCreateWritableSnapshotFunc
	}()

	getSnapshotFunc = func(_ *IsilonClusterConfig) func(context.Context, string) (isi.Snapshot, error) {
		return func(_ context.Context, _ string) (isi.Snapshot, error) {
			return &apiv1.IsiSnapshot{
				ID:   1234,
				Name: "snap1",
				Path: "/ifs/data/src/snap1",
			}, nil
		}
	}
	getSnapshotSizeFunc = func(_ *IsilonClusterConfig) func(context.Context, string, string, string) int64 {
		return func(_ context.Context, _, _, _ string) int64 { return 1 }
	}
	// Force create path by making idempotency lookup fail.
	getWritableSnapshotFunc = func(_ *IsilonClusterConfig) func(context.Context, string) (isi.WritableSnapshot, error) {
		return func(_ context.Context, _ string) (isi.WritableSnapshot, error) {
			return nil, errors.New("not found")
		}
	}

	tests := []struct {
		name       string
		createErr  error
		expectCode codes.Code
	}{
		{
			name:       "snapshotiq license-like error returns internal",
			createErr:  errors.New("snapshot feature is not licensed"),
			expectCode: codes.Internal,
		},
		{
			name:       "generic create error returns internal",
			createErr:  errors.New("create writable snapshot failed"),
			expectCode: codes.Internal,
		},
	}

	ctx := context.Background()
	s := &service{}
	isiConfig := &IsilonClusterConfig{}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			createWritableSnapshotFunc = func(_ *IsilonClusterConfig) func(context.Context, string, string, string, string, string) (isi.Volume, error) {
				return func(_ context.Context, _, _, _, _, _ string) (isi.Volume, error) {
					return nil, tc.createErr
				}
			}

			err := s.createVolumeFromWritableSnapshot(ctx, isiConfig, "/ifs/data", "snapshot1234", "vol-test", 2, "System")
			assert.Error(t, err)
			st, ok := status.FromError(err)
			assert.True(t, ok)
			assert.Equal(t, tc.expectCode, st.Code())
		})
	}
}

func TestEmitSnapshotDependencyEvent(t *testing.T) {
	ctx := context.Background()

	t.Run("returns when client is nil", func(_ *testing.T) {
		s := &service{}
		s.emitSnapshotDependencyEvent(ctx, "snap-1=cluster=System", "dependency exists")
	})

	t.Run("returns when namespace is missing", func(t *testing.T) {
		s := &service{k8sclient: fake.NewSimpleClientset()}
		_ = os.Unsetenv("POD_NAMESPACE")
		_ = os.Unsetenv("X_CSI_DRIVER_NAMESPACE")
		s.emitSnapshotDependencyEvent(ctx, "snap-2=cluster=System", "dependency exists")
		events, err := s.k8sclient.CoreV1().Events("").List(ctx, metav1.ListOptions{})
		assert.NoError(t, err)
		assert.Len(t, events.Items, 0)
	})

	t.Run("creates warning event when namespace is set", func(t *testing.T) {
		s := &service{k8sclient: fake.NewSimpleClientset()}
		t.Setenv("POD_NAMESPACE", "csi-powerscale")
		s.emitSnapshotDependencyEvent(ctx, "snap-3=cluster=System", "snapshot has dependents")

		events, err := s.k8sclient.CoreV1().Events("csi-powerscale").List(ctx, metav1.ListOptions{})
		assert.NoError(t, err)
		if assert.Len(t, events.Items, 1) {
			assert.Equal(t, "VolumeSnapshot", events.Items[0].InvolvedObject.Kind)
			assert.Equal(t, "snap-3", events.Items[0].InvolvedObject.Name)
			assert.Equal(t, corev1.EventTypeWarning, events.Items[0].Type)
			assert.Equal(t, "SnapshotHasDependents", events.Items[0].Reason)
		}
	})
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
			err := s.createVolumeFromSource(ctx, isiConfig, "/ifs/data/volumePath", tt.contentSource, tt.req, tt.sizeInBytes, "accessZone", false)
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
	getCSIVolumeFunc = func(_ *service) func(ctx context.Context, exportID int, volName, path, accessZone string, sizeInBytes int64, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork string, directoryBacked bool, sharedExportPath, smartConnectZoneFQDN, nfsTransportSecurity string) *csi.Volume {
		return func(_ context.Context, _ int, volName, path, accessZone string, sizeInBytes int64, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork string, directoryBacked bool, sharedExportPath, _, _ string) *csi.Volume {
			volumeContext := map[string]string{
				"path":             path,
				"accessZone":       accessZone,
				"azServiceIP":      azServiceIP,
				"azNetwork":        azNetwork,
				"rootClient":       rootClientEnabled,
				"sourceSnapshotID": sourceSnapshotID,
				"sourceVolumeID":   sourceVolumeID,
				"clusterName":      clusterName,
			}
			if directoryBacked {
				volumeContext["ProvisioningMode"] = "directory"
				volumeContext["DirectoryPath"] = volName
				volumeContext["SharedExportPath"] = sharedExportPath
			} else {
				volumeContext["ProvisioningMode"] = "export"
			}
			return &csi.Volume{
				VolumeId:      volName,
				CapacityBytes: sizeInBytes,
				VolumeContext: volumeContext,
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

	// Expected result for export-backed volume
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
			"ProvisioningMode": "export",
		},
	}

	expectedResponse := &csi.CreateVolumeResponse{
		Volume: expectedVolume,
	}

	// Call the function under test - export-backed mode (directoryBacked=false)
	response := s.getCreateVolumeResponse(ctx, exportID, volName, path, accessZone, sizeInBytes, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork, false, "", "", "")

	// Assert the response is as expected
	assert.Equal(t, expectedResponse, response)
}

func TestGetCreateVolumeResponseWithMTLS(t *testing.T) {
	// Backup original function
	originalGetCSIVolumeFunc := getCSIVolumeFunc

	// Restore original function after the test
	defer func() {
		getCSIVolumeFunc = originalGetCSIVolumeFunc
	}()

	// Use the real getCSIVolume function to test mTLS parameter handling
	getCSIVolumeFunc = func(svc *service) func(ctx context.Context, exportID int, volName, path, accessZone string, sizeInBytes int64, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork string, directoryBacked bool, sharedExportPath, smartConnectZoneFQDN, nfsTransportSecurity string) *csi.Volume {
		return svc.getCSIVolume
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
	sourceSnapshotID := ""
	sourceVolumeID := ""
	clusterName := "cluster-test"
	smartConnectZoneFQDN := "zone1.smartconnect.example.com"
	nfsTransportSecurity := "mtls"

	// Call the function under test
	response := s.getCreateVolumeResponse(ctx, exportID, volName, path, accessZone, sizeInBytes, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork, false, "", smartConnectZoneFQDN, nfsTransportSecurity)

	// Assert the mTLS parameters are in the volume context
	assert.NotNil(t, response)
	assert.NotNil(t, response.Volume)
	assert.Equal(t, smartConnectZoneFQDN, response.Volume.VolumeContext[constants.SmartConnectZoneFQDNParam])
	assert.Equal(t, nfsTransportSecurity, response.Volume.VolumeContext[constants.NFSTransportSecurityParam])
}

func TestResolveCreateVolumeMTLSSettings(t *testing.T) {
	tests := []struct {
		name         string
		storageClass map[string]string
		clusterFQDN  string
		envFQDN      string
		envMode      string
		expectedFQDN string
		expectedMode string
	}{
		{
			name:         "cluster FQDN overrides environment without inheriting transport security",
			storageClass: map[string]string{},
			clusterFQDN:  "cluster.smartconnect.example.com",
			envFQDN:      "env.smartconnect.example.com",
			envMode:      "tls",
			expectedFQDN: "cluster.smartconnect.example.com",
			expectedMode: "",
		},
		{
			name:         "environment FQDN is used without inheriting transport security",
			storageClass: map[string]string{},
			envFQDN:      "env.smartconnect.example.com",
			envMode:      "mtls",
			expectedFQDN: "env.smartconnect.example.com",
			expectedMode: "",
		},
		{
			name: "StorageClass overrides secret and environment",
			storageClass: map[string]string{
				constants.SmartConnectZoneFQDNParam: "sc.smartconnect.example.com",
				constants.NFSTransportSecurityParam: "none",
			},
			clusterFQDN:  "cluster.smartconnect.example.com",
			envFQDN:      "env.smartconnect.example.com",
			envMode:      "tls",
			expectedFQDN: "sc.smartconnect.example.com",
			expectedMode: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(constants.EnvNFSMountFQDN, tt.envFQDN)
			t.Setenv("X_CSI_ISI_NFS_TRANSPORT_SECURITY", tt.envMode)

			fqdn, mode := resolveCreateVolumeMTLSSettings(tt.storageClass, &IsilonClusterConfig{
				NFSMountFQDN: tt.clusterFQDN,
			})

			assert.Equal(t, tt.expectedFQDN, fqdn)
			assert.Equal(t, tt.expectedMode, mode)
		})
	}
}

func TestGetCreateVolumeResponseWithoutMTLS(t *testing.T) {
	// Backup original function
	originalGetCSIVolumeFunc := getCSIVolumeFunc

	// Restore original function after the test
	defer func() {
		getCSIVolumeFunc = originalGetCSIVolumeFunc
	}()

	// Use the real getCSIVolume function to test backward compatibility
	getCSIVolumeFunc = func(svc *service) func(ctx context.Context, exportID int, volName, path, accessZone string, sizeInBytes int64, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork string, directoryBacked bool, sharedExportPath, smartConnectZoneFQDN, nfsTransportSecurity string) *csi.Volume {
		return svc.getCSIVolume
	}

	// Mock service instance
	s := &service{}

	// Mock inputs without mTLS parameters
	ctx := context.Background()
	exportID := 1
	volName := "vol-test"
	path := "/data/vol-test"
	accessZone := "accessZone1"
	sizeInBytes := int64(1024)
	azServiceIP := "10.0.0.1"
	azNetwork := "10.0.0.0/24"
	rootClientEnabled := "true"
	sourceSnapshotID := ""
	sourceVolumeID := ""
	clusterName := "cluster-test"
	smartConnectZoneFQDN := "" // Empty - not configured
	nfsTransportSecurity := "" // Empty - not configured

	// Call the function under test
	response := s.getCreateVolumeResponse(ctx, exportID, volName, path, accessZone, sizeInBytes, azServiceIP, rootClientEnabled, sourceSnapshotID, sourceVolumeID, clusterName, azNetwork, false, "", smartConnectZoneFQDN, nfsTransportSecurity)

	// Assert the mTLS parameters are NOT in the volume context when empty
	assert.NotNil(t, response)
	assert.NotNil(t, response.Volume)
	_, hasFQDN := response.Volume.VolumeContext[constants.SmartConnectZoneFQDNParam]
	_, hasSecurity := response.Volume.VolumeContext[constants.NFSTransportSecurityParam]
	assert.False(t, hasFQDN, "SmartConnectZoneFQDN should not be in volume context when empty")
	assert.False(t, hasSecurity, "NFSTransportSecurity should not be in volume context when empty")
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
			name: "Single node single writer supported",
			vcs: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER},
				},
			},
			expected: true,
			reason:   "",
		},
		{
			name: "Single node multi writer supported",
			vcs: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER},
				},
			},
			expected: true,
			reason:   "",
		},
		{
			name: "Multi node multi writer supported",
			vcs: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER},
				},
			},
			expected: true,
			reason:   "",
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
		{
			name: "Nil access mode",
			vcs: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: nil,
				},
			},
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

// TestProcessSnapshotTrackingDirVolumeNameConsistency is a regression test for CSME-235.
// It verifies that processSnapshotTrackingDirectoryDuringDeleteVolume correctly looks up
// the tracking directory entry using the volume name from the volume ID. Before the fix,
// CreateVolume for RO snapshot volumes would return a volume ID containing the source
// volume name (from the export path) instead of the requested volume name, causing
// the tracking entry lookup to fail during DeleteVolume.
func TestProcessSnapshotTrackingDirVolumeNameConsistency(t *testing.T) {
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
	defer after()

	isiConfig := &IsilonClusterConfig{
		isiSvc: &isiService{
			endpoint: "http://testendpoint:8080",
			client:   &isi.Client{},
		},
	}
	s := &service{}

	// Simulate the scenario from CSME-235:
	// - Source volume: "sourceVol" (name in the snapshot export path)
	// - Restored volume: "restoredVol" (req.GetName() used for tracking dir entry)
	// - The volume ID should contain "restoredVol" so that DeleteVolume can find the tracking entry
	//
	// The tracking dir entry is: snapshotTrackingDir/restoredVol
	// The volName passed to processSnapshotTrackingDirectoryDuringDeleteVolume comes from
	// parsing the volume ID, so it must be "restoredVol" (not "sourceVol").
	restoredVolName := "restoredVol"
	snapshotTrackingDir := ".csi-snapshot-abc-tracking-dir"
	expectedTrackingEntry := snapshotTrackingDir + "/" + restoredVolName

	var deletedEntries []string
	var checkedEntries []string

	getZoneByNameFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, zoneName string) (*v1.IsiZone, error) {
		return func(_ context.Context, _ string) (*v1.IsiZone, error) {
			return &v1.IsiZone{Path: "/ifs"}, nil
		}
	}
	getSnapshotIsiPathComponentsFunc = func(_ *IsilonClusterConfig) func(exportPath, zonePath string) (string, string, string) {
		return func(_, _ string) (string, string, string) {
			return "/ifs/data/csi", "snapshot-abc", "sourceVol"
		}
	}
	getSnapshotTrackingDirNameFunc = func(_ *IsilonClusterConfig) func(snapshotName string) string {
		return func(_ string) string {
			return snapshotTrackingDir
		}
	}
	isVolumeExistentFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeID, volumeEntry string) bool {
		return func(_ context.Context, _, _, entry string) bool {
			checkedEntries = append(checkedEntries, entry)
			// Only the correct tracking entry (using restoredVol) should exist
			return entry == expectedTrackingEntry
		}
	}
	deleteVolumeFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeSelector string) error {
		return func(_ context.Context, _, selector string) error {
			deletedEntries = append(deletedEntries, selector)
			return nil
		}
	}
	getSubDirectoryCountFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeSelector string) (int64, error) {
		return func(_ context.Context, _, _ string) (int64, error) {
			// After deleting the entry: only ., .. remain
			return 2, nil
		}
	}
	unexportByIDWithZoneFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, exportID int, zoneName string) error {
		return func(_ context.Context, _ int, _ string) error {
			return nil
		}
	}
	removeSnapshotFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, snapID int64, snapName string) error {
		return func(_ context.Context, _ int64, _ string) error {
			return nil
		}
	}

	export := &v2.Export{
		Paths: &[]string{"/ifs/.snapshot/snapshot-abc/data/csi/sourceVol"},
	}

	err := s.processSnapshotTrackingDirectoryDuringDeleteVolume(ctx, restoredVolName, "System", export, isiConfig)
	assert.NoError(t, err)

	// Verify the tracking entry was looked up using the restored volume name (not the source)
	assert.Contains(t, checkedEntries, expectedTrackingEntry,
		"tracking dir entry should be checked using the restored volume name from the volume ID")

	// Verify the correct entry was deleted
	assert.Contains(t, deletedEntries, expectedTrackingEntry,
		"tracking dir entry for restored volume should be deleted")

	// Verify that no entry using sourceVol was looked up
	sourceEntry := snapshotTrackingDir + "/sourceVol"
	assert.NotContains(t, checkedEntries, sourceEntry,
		"should NOT look up tracking entry using source volume name")
}

// TestCSME244_DeleteVolumePassesAccessZoneToUnexport is a regression test for CSME-244.
// It verifies that processSnapshotTrackingDirectoryDuringDeleteVolume forwards the
// accessZone argument to UnexportByIDWithZone rather than passing an empty string.
func TestCSME244_DeleteVolumePassesAccessZoneToUnexport(t *testing.T) {
	ctx := context.Background()

	originalGetZoneByNameFunc := getZoneByNameFunc
	originalGetSnapshotIsiPathComponentsFunc := getSnapshotIsiPathComponentsFunc
	originalGetSnapshotTrackingDirNameFunc := getSnapshotTrackingDirNameFunc
	originalIsVolumeExistentFunc := isVolumeExistentFunc
	originalDeleteVolumeFunc := deleteVolumeFunc
	originalGetSubDirectoryCountFunc := getSubDirectoryCountFunc
	originalUnexportByIDWithZoneFunc := unexportByIDWithZoneFunc
	originalRemoveSnapshotFunc := removeSnapshotFunc
	defer func() {
		getZoneByNameFunc = originalGetZoneByNameFunc
		getSnapshotIsiPathComponentsFunc = originalGetSnapshotIsiPathComponentsFunc
		getSnapshotTrackingDirNameFunc = originalGetSnapshotTrackingDirNameFunc
		isVolumeExistentFunc = originalIsVolumeExistentFunc
		deleteVolumeFunc = originalDeleteVolumeFunc
		getSubDirectoryCountFunc = originalGetSubDirectoryCountFunc
		unexportByIDWithZoneFunc = originalUnexportByIDWithZoneFunc
		removeSnapshotFunc = originalRemoveSnapshotFunc
	}()

	isiConfig := &IsilonClusterConfig{
		isiSvc: &isiService{
			endpoint: "http://testendpoint:8080",
			client:   &isi.Client{},
		},
	}
	s := &service{}
	const testAccessZone = "az-sust070a-tst"

	getZoneByNameFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, zoneName string) (*v1.IsiZone, error) {
		return func(_ context.Context, _ string) (*v1.IsiZone, error) {
			return &v1.IsiZone{Path: "/ifs/az-sust070a-tst"}, nil
		}
	}
	getSnapshotIsiPathComponentsFunc = func(_ *IsilonClusterConfig) func(exportPath, zonePath string) (string, string, string) {
		return func(_, _ string) (string, string, string) {
			return "/ifs/az-sust070a-tst", "snapshot-c998475a", ""
		}
	}
	getSnapshotTrackingDirNameFunc = func(_ *IsilonClusterConfig) func(snapshotName string) string {
		return func(_ string) string {
			return ".csi-snapshot-c998475a-tracking-dir"
		}
	}
	isVolumeExistentFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeID, volumeEntry string) bool {
		return func(_ context.Context, _, _, _ string) bool { return true }
	}
	deleteVolumeFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeSelector string) error {
		return func(_ context.Context, _, _ string) error { return nil }
	}
	getSubDirectoryCountFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, volumePath, volumeSelector string) (int64, error) {
		return func(_ context.Context, _, _ string) (int64, error) { return 3, nil }
	}

	var capturedZone string
	unexportByIDWithZoneFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, exportID int, zoneName string) error {
		return func(_ context.Context, _ int, zone string) error {
			capturedZone = zone
			return nil
		}
	}
	removeSnapshotFunc = func(_ *IsilonClusterConfig) func(ctx context.Context, snapID int64, snapName string) error {
		return func(_ context.Context, _ int64, _ string) error { return nil }
	}

	export := &v2.Export{
		ID:    289846,
		Paths: &[]string{"/ifs/az-sust070a-tst/.snapshot/snapshot-c998475a/tst-dr-off-csi-sebshift/infraver01-15a91bd3df"},
	}

	err := s.processSnapshotTrackingDirectoryDuringDeleteVolume(ctx, "infraver01-b033196589", testAccessZone, export, isiConfig)
	assert.NoError(t, err)
	assert.Equal(t, testAccessZone, capturedZone,
		"CSME-244 regression: UnexportByIDWithZone must be called with accessZone, not empty string")
}

// TestCSME244_DeleteSnapshotPassesAccessZoneToUnexport is a regression test for CSME-244.
// It verifies that processSnapshotTrackingDirectoryDuringDeleteSnapshot forwards the
// accessZone argument to UnexportByIDWithZone rather than passing an empty string.
func TestCSME244_DeleteSnapshotPassesAccessZoneToUnexport(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const testAccessZone = "az-sust070a-tst"
	const exportID = 289846

	mockAPIClient := &isimocks.Client{}
	isiConfig := &IsilonClusterConfig{
		isiSvc: &isiService{
			client: &isi.Client{API: mockAPIClient},
		},
	}
	s := &service{}

	mockAPIClient.On(
		"Get",
		mock.Anything, "platform/1/zones", testAccessZone, mock.Anything, mock.Anything, mock.Anything,
	).Return(nil).Once().Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv1.GetIsiZonesResp)
		zone := apiv1.IsiZone{Name: testAccessZone, Path: "/ifs/az-sust070a-tst"}
		resp.Zones = []*apiv1.IsiZone{&zone}
	})

	mockAPIClient.On(
		"Get",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(errors.New("not found"))

	var capturedZone string
	mockAPIClient.On(
		"Delete",
		mock.Anything, mock.Anything, strconv.Itoa(exportID), mock.Anything, mock.Anything, mock.Anything,
	).Return(nil).Run(func(args mock.Arguments) {
		if params, ok := args.Get(3).(isiapi.OrderedValues); ok {
			for _, kv := range params {
				if string(kv[0]) == "zone" {
					capturedZone = string(kv[1])
				}
			}
		}
	})
	mockAPIClient.On(
		"Delete",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(nil)

	snapshotIsiPath := "/ifs/az-sust070a-tst/.snapshot/snapshot-c998475a-cf7b-4313-88f0-8c341f1d44f8/tst-dr-off-csi-sebshift/infraver01-15a91bd3df"
	export := &v2.Export{
		ID:    exportID,
		Paths: &[]string{snapshotIsiPath},
	}
	deleteSnapshot := true

	err := s.processSnapshotTrackingDirectoryDuringDeleteSnapshot(ctx, export, snapshotIsiPath, testAccessZone, &deleteSnapshot, isiConfig)
	assert.NoError(t, err)
	assert.Equal(t, testAccessZone, capturedZone,
		"CSME-244 regression: UnexportByIDWithZone must be called with accessZone, not empty string")
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
	isiConfig.isiSvc.client.API.(*isimocks.Client).On("Get", anyArgs[0:6]...).Return(errors.New("mocked export lookup failure")).Once()
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
	isiConfig.isiSvc.client.API.(*isimocks.Client).On("Get", anyArgs[0:6]...).Return(errors.New("mocked export lookup failure")).Once()
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
			name:      "successful execution with IPInCIDR check",
			nodeID:    "nodename=#=#=localhost=#=#=127.0.0.1",
			azNetwork: "192.168.0.0/16",
			nodeLabels: map[string]string{
				"csi-isilon.dellemc.com/az-192.168.1.0-24-192.168.5.100": "true",
				"csi-isilon.dellemc.com/az-192.168.1.0-24-192.168.6.101": "true",
			},
			expectedIPs: []string{"192.168.5.100", "192.168.6.101"},
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
				"csi-isilon.dellemc.com/az-10.0.0.0-32-100.0.0.1": "true",
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

func TestControllerModifyVolume(t *testing.T) {
	fmt.Println("TestControllerModifyVolume")

	getExportArgsForQuota := mock.Arguments{mock.Anything, "platform/2/protocols/nfs/exports", mock.Anything, mock.Anything, mock.Anything, mock.Anything}
	getQuotaArgs := mock.Arguments{mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything}
	doWithHeadersArgs := []interface{}{mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything}

	// newService builds a fresh service + mock client for each subtest to avoid cross-test mock state leakage.
	newService := func(quotaEnabled bool) (*service, *isimocks.Client) {
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
		s := &service{
			k8sclient:             fake.NewSimpleClientset(),
			defaultIsiClusterName: "system",
			isiClusters:           &sync.Map{},
			opts: Opts{
				QuotaEnabled: quotaEnabled,
				AutoProbe:    true,
			},
		}
		s.isiClusters.Store("system", isiConfig)
		return s, mockClient
	}

	// setupQuotaMocks configures the mock client so that GetVolumeQuota succeeds and returns
	// a quota with the given thresholds, keyed off export description "CSI_QUOTA_ID:<quotaID>".
	setupQuotaMocks := func(mockClient *isimocks.Client, quotaID string, advisory, soft, softGrace, hard int64) {
		mockClient.On("Get", getExportArgsForQuota...).Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(*v2.ExportList)
			*resp = v2.ExportList{
				&v2.Export{
					ID:          19,
					Description: "CSI_QUOTA_ID:" + quotaID,
				},
			}
		}).Once()
		mockClient.On("Get", getQuotaArgs...).Return(nil).Run(func(args mock.Arguments) {
			resp := args.Get(5).(*apiv1.IsiQuotaListResp)
			*resp = apiv1.IsiQuotaListResp{
				Quotas: []apiv1.IsiQuota{
					{
						ID: quotaID,
						Thresholds: struct {
							Advisory             int64       `json:"advisory"`
							AdvisoryExceeded     bool        `json:"advisory_exceeded"`
							AdvisoryLastExceeded interface{} `json:"advisory_last_exceeded"`
							Hard                 int64       `json:"hard"`
							HardExceeded         bool        `json:"hard_exceeded"`
							HardLastExceeded     interface{} `json:"hard_last_exceeded"`
							Soft                 int64       `json:"soft"`
							SoftExceeded         bool        `json:"soft_exceeded"`
							SoftLastExceeded     interface{} `json:"soft_last_exceeded"`
							SoftGrace            int64       `json:"soft_grace"`
						}{
							Advisory:  advisory,
							Soft:      soft,
							SoftGrace: softGrace,
							Hard:      hard,
						},
					},
				},
			}
		}).Once()
	}

	const volID = "volume1=_=_=19=_=_=System"

	t.Run("empty volume ID", func(t *testing.T) {
		s, _ := newService(true)
		req := &csi.ControllerModifyVolumeRequest{
			VolumeId: "",
			MutableParameters: map[string]string{
				"AdvisoryLimit": "50",
			},
		}
		_, err := s.ControllerModifyVolume(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("empty mutable parameters", func(t *testing.T) {
		s, mockClient := newService(true)
		setupQuotaMocks(mockClient, "quota-1", 500, 1000, 100, 2000)
		req := &csi.ControllerModifyVolumeRequest{
			VolumeId:          volID,
			MutableParameters: map[string]string{},
		}
		resp, err := s.ControllerModifyVolume(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("invalid volume ID format", func(t *testing.T) {
		s, _ := newService(true)
		req := &csi.ControllerModifyVolumeRequest{
			VolumeId: "invalid-volume-id",
			MutableParameters: map[string]string{
				"AdvisoryLimit": "50",
			},
		}
		_, err := s.ControllerModifyVolume(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("cluster config not found", func(t *testing.T) {
		s, _ := newService(true)
		req := &csi.ControllerModifyVolumeRequest{
			VolumeId: "volume1=_=_=19=_=_=System=_=_=unknownCluster",
			MutableParameters: map[string]string{
				"AdvisoryLimit": "50",
			},
		}
		_, err := s.ControllerModifyVolume(context.Background(), req)
		assert.Error(t, err)
	})

	t.Run("auto probe fails", func(t *testing.T) {
		isiConfig := &IsilonClusterConfig{
			Endpoint:    "http://testendpoint:8080",
			ClusterName: "system",
			isiSvc:      nil,
		}
		s := &service{
			k8sclient:             fake.NewSimpleClientset(),
			defaultIsiClusterName: "system",
			isiClusters:           &sync.Map{},
			opts: Opts{
				QuotaEnabled: true,
				AutoProbe:    false,
			},
		}
		s.isiClusters.Store("system", isiConfig)
		req := &csi.ControllerModifyVolumeRequest{
			VolumeId: volID,
			MutableParameters: map[string]string{
				"AdvisoryLimit": "50",
			},
		}
		_, err := s.ControllerModifyVolume(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	})

	t.Run("quota not enabled", func(t *testing.T) {
		s, _ := newService(false)
		req := &csi.ControllerModifyVolumeRequest{
			VolumeId: volID,
			MutableParameters: map[string]string{
				"AdvisoryLimit": "50",
			},
		}
		_, err := s.ControllerModifyVolume(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	})

	t.Run("get volume quota fails", func(t *testing.T) {
		s, mockClient := newService(true)
		mockClient.On("Get", getExportArgsForQuota...).Return(errors.New("mock export lookup error")).Once()
		req := &csi.ControllerModifyVolumeRequest{
			VolumeId: volID,
			MutableParameters: map[string]string{
				"AdvisoryLimit": "50",
			},
		}
		_, err := s.ControllerModifyVolume(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("unsupported mutable parameter", func(t *testing.T) {
		s, mockClient := newService(true)
		setupQuotaMocks(mockClient, "quota-1", 500, 800, 100, 1000)
		req := &csi.ControllerModifyVolumeRequest{
			VolumeId: volID,
			MutableParameters: map[string]string{
				"UnsupportedParam": "50",
			},
		}
		_, err := s.ControllerModifyVolume(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("non-integer parameter value", func(t *testing.T) {
		s, mockClient := newService(true)
		setupQuotaMocks(mockClient, "quota-1", 500, 800, 100, 1000)
		req := &csi.ControllerModifyVolumeRequest{
			VolumeId: volID,
			MutableParameters: map[string]string{
				"AdvisoryLimit": "not-a-number",
			},
		}
		_, err := s.ControllerModifyVolume(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("negative parameter value", func(t *testing.T) {
		s, mockClient := newService(true)
		setupQuotaMocks(mockClient, "quota-1", 500, 800, 100, 1000)
		req := &csi.ControllerModifyVolumeRequest{
			VolumeId: volID,
			MutableParameters: map[string]string{
				"SoftGracePrd": "-1",
			},
		}
		_, err := s.ControllerModifyVolume(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("AdvisoryLimit greater than 100", func(t *testing.T) {
		s, mockClient := newService(true)
		setupQuotaMocks(mockClient, "quota-1", 500, 800, 100, 1000)
		req := &csi.ControllerModifyVolumeRequest{
			VolumeId: volID,
			MutableParameters: map[string]string{
				"AdvisoryLimit": "101",
			},
		}
		_, err := s.ControllerModifyVolume(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Contains(t, err.Error(), "mutable parameter AdvisoryLimit must be between 0 and 100, got 101")
	})

	t.Run("SoftLimit greater than 100", func(t *testing.T) {
		s, mockClient := newService(true)
		setupQuotaMocks(mockClient, "quota-1", 500, 800, 100, 1000)
		req := &csi.ControllerModifyVolumeRequest{
			VolumeId: volID,
			MutableParameters: map[string]string{
				"SoftLimit": "101",
			},
		}
		_, err := s.ControllerModifyVolume(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Contains(t, err.Error(), "mutable parameter SoftLimit must be between 0 and 100, got 101")
	})

	t.Run("hard quota is 0", func(t *testing.T) {
		s, mockClient := newService(true)
		setupQuotaMocks(mockClient, "quota-1", 500, 800, 100, 0)
		mockClient.On("DoWithHeaders", doWithHeadersArgs...).Return(nil)
		req := &csi.ControllerModifyVolumeRequest{
			VolumeId: volID,
			MutableParameters: map[string]string{
				"AdvisoryLimit": "60",
			},
		}
		_, err := s.ControllerModifyVolume(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, codes.FailedPrecondition, status.Code(err))
		assert.Contains(t, err.Error(), "hard")
		mockClient.AssertNotCalled(t, "DoWithHeaders", doWithHeadersArgs...)
	})

	t.Run("idempotent no changes needed", func(t *testing.T) {
		s, mockClient := newService(true)
		// Hard=1000, requested AdvisoryLimit "50" (%) => 500 which matches current Advisory.
		setupQuotaMocks(mockClient, "quota-1", 500, 800, 100, 1000)
		req := &csi.ControllerModifyVolumeRequest{
			VolumeId: volID,
			MutableParameters: map[string]string{
				"AdvisoryLimit": "50",
			},
		}
		resp, err := s.ControllerModifyVolume(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		// ModifyQuota (DoWithHeaders) must not have been called since no update was needed.
		mockClient.AssertNotCalled(t, "DoWithHeaders", doWithHeadersArgs...)
	})

	t.Run("successful modification", func(t *testing.T) {
		s, mockClient := newService(true)
		// Hard=1000, requested AdvisoryLimit "60" (%) => 600 which differs from current Advisory (500).
		setupQuotaMocks(mockClient, "quota-1", 500, 800, 100, 1000)
		mockClient.On("DoWithHeaders", doWithHeadersArgs...).Return(nil).Once()
		req := &csi.ControllerModifyVolumeRequest{
			VolumeId: volID,
			MutableParameters: map[string]string{
				"AdvisoryLimit": "60",
			},
		}
		resp, err := s.ControllerModifyVolume(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		mockClient.AssertCalled(t, "DoWithHeaders", doWithHeadersArgs...)
	})

	t.Run("successful modification of SoftLimit and SoftGracePrd", func(t *testing.T) {
		s, mockClient := newService(true)
		// Hard=1000, requested SoftLimit "90" (%) => 900 which differs from current Soft (800).
		// SoftGracePrd "200" differs from current SoftGrace (100).
		setupQuotaMocks(mockClient, "quota-1", 500, 800, 100, 1000)
		mockClient.On("DoWithHeaders", doWithHeadersArgs...).Return(nil).Once()
		req := &csi.ControllerModifyVolumeRequest{
			VolumeId: volID,
			MutableParameters: map[string]string{
				"SoftLimit":    "90",
				"SoftGracePrd": "200",
			},
		}
		resp, err := s.ControllerModifyVolume(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		mockClient.AssertCalled(t, "DoWithHeaders", doWithHeadersArgs...)
	})

	t.Run("modify quota fails", func(t *testing.T) {
		s, mockClient := newService(true)
		setupQuotaMocks(mockClient, "quota-1", 500, 800, 100, 1000)
		mockClient.On("DoWithHeaders", doWithHeadersArgs...).Return(errors.New("mock modify error")).Once()
		req := &csi.ControllerModifyVolumeRequest{
			VolumeId: volID,
			MutableParameters: map[string]string{
				"AdvisoryLimit": "60",
			},
		}
		_, err := s.ControllerModifyVolume(context.Background(), req)
		assert.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})
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

func TestCreateVolumeWritableFromSnapshotRequiresSnapshotSource(t *testing.T) {
	newService := func() *service {
		s := &service{
			k8sclient:             fake.NewSimpleClientset(),
			defaultIsiClusterName: "system",
			isiClusters:           &sync.Map{},
			opts: Opts{
				AutoProbe: false,
			},
		}
		s.isiClusters.Store("system", &IsilonClusterConfig{
			Endpoint:    "http://testendpoint:8080",
			ClusterName: "system",
			isiSvc:      &isiService{},
		})
		return s
	}

	tests := []struct {
		name    string
		source  *csi.VolumeContentSource
		wantMsg string
	}{
		{
			name:    "missing volume content source",
			source:  nil,
			wantMsg: "writable-from-snapshot parameter requires volume content source snapshot",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newService()
			req := &csi.CreateVolumeRequest{
				Name: "test-vol",
				Parameters: map[string]string{
					ClusterNameParam:          "system",
					WritableFromSnapshotParam: "true",
				},
				CapacityRange:       &csi.CapacityRange{RequiredBytes: 1024},
				VolumeContentSource: tt.source,
			}

			_, err := s.CreateVolume(context.Background(), req)
			assert.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
			assert.Contains(t, err.Error(), tt.wantMsg)
		})
	}
}

func TestCreateVolumeFailedProvisioningRollsBackCreatedVolume(t *testing.T) {
	originalGetIsVolumeExistentFunc := getIsVolumeExistentFunc
	originalGetExportWithPathAndZoneFunc := getGetExportWithPathAndZoneFunc
	t.Cleanup(func() {
		getIsVolumeExistentFunc = originalGetIsVolumeExistentFunc
		getGetExportWithPathAndZoneFunc = originalGetExportWithPathAndZoneFunc
	})

	getIsVolumeExistentFunc = func(_ *IsilonClusterConfig) func(context.Context, string, string, string) bool {
		return func(_ context.Context, _, _, _ string) bool {
			return false
		}
	}
	getGetExportWithPathAndZoneFunc = func(_ *IsilonClusterConfig) func(context.Context, string, string) (isi.Export, error) {
		return func(_ context.Context, _, _ string) (isi.Export, error) {
			return nil, nil
		}
	}

	mockClient := &isimocks.Client{}
	isiConfig := &IsilonClusterConfig{
		Endpoint:                 "http://testendpoint:8080",
		ClusterName:              "system",
		IsiPath:                  "/ifs/data",
		IsiVolumePathPermissions: "0777",
		isiSvc: &isiService{
			client: &isi.Client{
				API: mockClient,
			},
		},
	}
	s := &service{
		k8sclient:             fake.NewSimpleClientset(),
		defaultIsiClusterName: "system",
		isiClusters:           &sync.Map{},
		opts: Opts{
			AccessZone:   "System",
			AutoProbe:    false,
			QuotaEnabled: false,
		},
	}
	s.isiClusters.Store("system", isiConfig)

	req := &csi.CreateVolumeRequest{
		Name: "rollback-vol",
		Parameters: map[string]string{
			ClusterNameParam: "system",
		},
		CapacityRange: &csi.CapacityRange{RequiredBytes: 1024},
	}

	mockClient.On("Put", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	mockClient.On("Post", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("export create failed")).Once()

	deleteCalled := false
	mockClient.On("Delete", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(_ mock.Arguments) {
		deleteCalled = true
	}).Once()

	_, err := s.CreateVolume(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "export create failed")
	assert.True(t, deleteCalled, "expected created volume to be deleted when provisioning fails")
	mockClient.AssertExpectations(t)
}

func TestCreateVolumeRetryIsIdempotentWithoutDuplicateProvisioning(t *testing.T) {
	originalGetIsVolumeExistentFunc := getIsVolumeExistentFunc
	originalGetExportWithPathAndZoneFunc := getGetExportWithPathAndZoneFunc
	t.Cleanup(func() {
		getIsVolumeExistentFunc = originalGetIsVolumeExistentFunc
		getGetExportWithPathAndZoneFunc = originalGetExportWithPathAndZoneFunc
	})

	volumeExistsChecks := 0
	getIsVolumeExistentFunc = func(_ *IsilonClusterConfig) func(context.Context, string, string, string) bool {
		return func(_ context.Context, _, _, _ string) bool {
			volumeExistsChecks++
			return volumeExistsChecks > 1
		}
	}

	exportLookupChecks := 0
	getGetExportWithPathAndZoneFunc = func(_ *IsilonClusterConfig) func(context.Context, string, string) (isi.Export, error) {
		return func(_ context.Context, _, _ string) (isi.Export, error) {
			exportLookupChecks++
			if exportLookupChecks == 1 {
				return nil, nil
			}
			paths := []string{"/ifs/data/retry-vol"}
			return &v2.Export{ID: 77, Zone: "System", Paths: &paths}, nil
		}
	}

	mockClient := &isimocks.Client{}
	isiConfig := &IsilonClusterConfig{
		Endpoint:                 "http://testendpoint:8080",
		ClusterName:              "system",
		IsiPath:                  "/ifs/data",
		IsiVolumePathPermissions: "0777",
		isiSvc: &isiService{
			client: &isi.Client{
				API: mockClient,
			},
		},
	}
	s := &service{
		k8sclient:             fake.NewSimpleClientset(),
		defaultIsiClusterName: "system",
		isiClusters:           &sync.Map{},
		opts: Opts{
			AccessZone:   "System",
			AutoProbe:    false,
			QuotaEnabled: false,
		},
	}
	s.isiClusters.Store("system", isiConfig)

	req := &csi.CreateVolumeRequest{
		Name: "retry-vol",
		Parameters: map[string]string{
			ClusterNameParam: "system",
		},
		CapacityRange: &csi.CapacityRange{RequiredBytes: 1024},
	}

	mockClient.On("Put", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	mockClient.On("Post", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(6).(*v2.Export)
		resp.ID = 77
		resp.Zone = "System"
		paths := []string{"/ifs/data/retry-vol"}
		resp.Paths = &paths
	}).Once()
	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*v2.ExportList)
		paths := []string{"/ifs/data/retry-vol"}
		empty := []string{}
		*resp = v2.ExportList{
			&v2.Export{
				ID:               77,
				Zone:             "System",
				Paths:            &paths,
				Clients:          &empty,
				ReadOnlyClients:  &empty,
				ReadWriteClients: &empty,
				RootClients:      &empty,
			},
		}
	}).Once()
	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("export not found for host check")).Once()

	resp1, err1 := s.CreateVolume(context.Background(), req)
	assert.NoError(t, err1)
	assert.NotNil(t, resp1)

	resp2, err2 := s.CreateVolume(context.Background(), req)
	assert.NoError(t, err2)
	assert.NotNil(t, resp2)
	assert.Equal(t, resp1.Volume.VolumeId, resp2.Volume.VolumeId)

	mockClient.AssertNumberOfCalls(t, "Put", 1)
	mockClient.AssertNumberOfCalls(t, "Post", 1)
	mockClient.AssertExpectations(t)
}

func TestListVolumes(t *testing.T) {
	fmt.Println("TestListVolumes")
	originalWorkerCount := listVolumesWorkerCount
	listVolumesWorkerCount = 1
	t.Cleanup(func() {
		listVolumesWorkerCount = originalWorkerCount
	})

	newService := func() (*service, *isimocks.Client) {
		mockClient := &isimocks.Client{}
		isiConfig := &IsilonClusterConfig{
			Endpoint:    "http://testendpoint:8080",
			ClusterName: "system",
			IsiPath:     "/ifs/data",
			isiSvc: &isiService{
				client: &isi.Client{
					API: mockClient,
				},
			},
		}
		s := &service{
			k8sclient:             fake.NewSimpleClientset(),
			defaultIsiClusterName: "system",
			isiClusters:           &sync.Map{},
			opts: Opts{
				AutoProbe: true,
			},
		}
		s.isiClusters.Store("system", isiConfig)
		return s, mockClient
	}

	filesystemListMatcher := mock.MatchedBy(func(resp interface{}) bool {
		_, ok := resp.(*v2.ResumeableContainerChildList)
		return ok
	})
	exportListMatcher := mock.MatchedBy(func(resp interface{}) bool {
		_, ok := resp.(*v2.ExportList)
		return ok
	})
	clientSlice := func(clients ...string) *[]string {
		copied := append([]string(nil), clients...)
		return &copied
	}

	setupFilesystemList := func(mockClient *isimocks.Client, children []*v2.ContainerChild, resume string, listErr error) {
		call := mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, filesystemListMatcher).
			Return(listErr).Once()
		if listErr == nil {
			call.Run(func(args mock.Arguments) {
				resp := args.Get(5).(*v2.ResumeableContainerChildList)
				*resp = v2.ResumeableContainerChildList{
					Children: children,
					Resume:   resume,
				}
			})
		}
	}

	setupExportsList := func(mockClient *isimocks.Client, exports v2.ExportList, exportErr error) {
		call := mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, exportListMatcher).
			Return(exportErr).Once()
		if exportErr == nil {
			call.Run(func(args mock.Arguments) {
				resp := args.Get(5).(*v2.ExportList)
				*resp = exports
			})
		}
	}
	metadataMatcher := mock.MatchedBy(func(resp interface{}) bool {
		_, ok := resp.(**apiv1.GetIsiVolumeAttributesResp)
		return ok
	})
	setupVolumeMetadata := func(mockClient *isimocks.Client, metadata map[string]string, metadataErr error) {
		call := mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, metadataMatcher).
			Return(metadataErr).Once()
		if metadataErr == nil {
			call.Run(func(args mock.Arguments) {
				resp := args.Get(5).(**apiv1.GetIsiVolumeAttributesResp)
				attrs := make([]struct {
					Name  string      `json:"name"`
					Value interface{} `json:"value"`
				}, 0, len(metadata))
				for key, value := range metadata {
					attrs = append(attrs, struct {
						Name  string      `json:"name"`
						Value interface{} `json:"value"`
					}{Name: key, Value: value})
				}
				*resp = &apiv1.GetIsiVolumeAttributesResp{AttributeMap: attrs}
			})
		}
	}

	t.Run("ListVolumes returns only exported CSI volumes with export-based IDs", func(t *testing.T) {
		s, mockClient := newService()
		setupFilesystemList(mockClient, []*v2.ContainerChild{
			{Name: strPtr("vol1"), Path: strPtr("/ifs/data"), Size: intPtr(100), Type: strPtr("container"), Owner: strPtr("root"), Group: strPtr("wheel")},
			{Name: strPtr("vol2"), Path: strPtr("/ifs/data"), Size: intPtr(200), Type: strPtr("container")},
			{Name: strPtr("unexported"), Path: strPtr("/ifs/data"), Size: intPtr(300), Type: strPtr("container")},
		}, "", nil)
		setupExportsList(mockClient, v2.ExportList{
			{ID: 11, Zone: "System", Paths: clientSlice("/ifs/data/vol1")},
			{ID: 22, Zone: "System", Paths: clientSlice("/ifs/data/vol2")},
		}, nil)
		setupVolumeMetadata(mockClient, map[string]string{headerPersistentVolumeName: "pv-vol1"}, nil)
		setupVolumeMetadata(mockClient, map[string]string{headerPersistentVolumeClaimName: "pvc-vol2"}, nil)

		resp, err := s.ListVolumes(context.Background(), &csi.ListVolumesRequest{})
		assert.NoError(t, err)
		assert.Equal(t, 2, len(resp.Entries))
		assert.Equal(t, "", resp.NextToken)
		assert.Equal(t, identifiers.GetNormalizedVolumeID(context.Background(), "vol1", 11, "System", "system"), resp.Entries[0].Volume.VolumeId)
		assert.Equal(t, "11", resp.Entries[0].Volume.VolumeContext["ID"])
		assert.Equal(t, "System", resp.Entries[0].Volume.VolumeContext["AccessZone"])
	})

	t.Run("ListVolumes with pagination first page", func(t *testing.T) {
		s, mockClient := newService()
		setupFilesystemList(mockClient, []*v2.ContainerChild{
			{Name: strPtr("vol1"), Path: strPtr("/ifs/data"), Size: intPtr(100), Type: strPtr("container")},
		}, "resume-token-1", nil)
		setupExportsList(mockClient, v2.ExportList{
			{ID: 11, Zone: "System", Paths: clientSlice("/ifs/data/vol1")},
		}, nil)
		setupVolumeMetadata(mockClient, map[string]string{headerPersistentVolumeName: "pv-vol1"}, nil)

		resp, err := s.ListVolumes(context.Background(), &csi.ListVolumesRequest{MaxEntries: 1})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(resp.Entries))
		assert.Equal(t, "resume-token-1", resp.NextToken)
	})

	t.Run("ListVolumes with pagination resume", func(t *testing.T) {
		s, mockClient := newService()
		setupFilesystemList(mockClient, []*v2.ContainerChild{
			{Name: strPtr("vol2"), Path: strPtr("/ifs/data"), Size: intPtr(200), Type: strPtr("container")},
		}, "", nil)
		setupExportsList(mockClient, v2.ExportList{
			{ID: 22, Zone: "System", Paths: clientSlice("/ifs/data/vol2")},
		}, nil)
		setupVolumeMetadata(mockClient, map[string]string{headerPersistentVolumeClaimNamespace: "default"}, nil)

		resp, err := s.ListVolumes(context.Background(), &csi.ListVolumesRequest{
			MaxEntries:    1,
			StartingToken: "resume-token-1",
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(resp.Entries))
		assert.Equal(t, "", resp.NextToken)
	})

	t.Run("ListVolumes with invalid MaxEntries", func(t *testing.T) {
		s, _ := newService()
		_, err := s.ListVolumes(context.Background(), &csi.ListVolumesRequest{MaxEntries: -1})
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("ListVolumes with invalid StartingToken", func(t *testing.T) {
		s, mockClient := newService()
		setupFilesystemList(mockClient, nil, "", &isiapi.JSONError{StatusCode: 400, Err: []isiapi.Error{{Message: "invalid resume token"}}})

		_, err := s.ListVolumes(context.Background(), &csi.ListVolumesRequest{
			MaxEntries:    1,
			StartingToken: "invalid-token",
		})
		assert.Error(t, err)
		assert.Equal(t, codes.Aborted, status.Code(err))
	})

	t.Run("ListVolumes with resume API internal error", func(t *testing.T) {
		s, mockClient := newService()
		setupFilesystemList(mockClient, nil, "", assert.AnError)

		_, err := s.ListVolumes(context.Background(), &csi.ListVolumesRequest{
			MaxEntries:    1,
			StartingToken: "resume-token-1",
		})
		assert.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("ListVolumes with nil filesystem entries and nil Name field", func(t *testing.T) {
		s, mockClient := newService()
		setupFilesystemList(mockClient, []*v2.ContainerChild{
			nil,
			{Name: nil},
			{Name: strPtr("vol1"), Path: strPtr("/ifs/data")},
		}, "", nil)
		setupExportsList(mockClient, v2.ExportList{
			{ID: 11, Zone: "System", Paths: clientSlice("/ifs/data/vol1")},
		}, nil)
		setupVolumeMetadata(mockClient, map[string]string{headerPersistentVolumeName: "pv-vol1"}, nil)

		resp, err := s.ListVolumes(context.Background(), &csi.ListVolumesRequest{})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(resp.Entries))
	})

	t.Run("ListVolumes excludes exported volumes with no CSI fingerprints", func(t *testing.T) {
		s, mockClient := newService()
		setupFilesystemList(mockClient, []*v2.ContainerChild{
			{Name: strPtr("manual-vol"), Path: strPtr("/ifs/data")},
		}, "", nil)
		setupExportsList(mockClient, v2.ExportList{&v2.Export{
			ID:               99,
			Zone:             "System",
			Paths:            clientSlice("/ifs/data/manual-vol"),
			Clients:          clientSlice(),
			ReadOnlyClients:  clientSlice(),
			ReadWriteClients: clientSlice(),
			RootClients:      clientSlice(),
		}}, nil)
		setupVolumeMetadata(mockClient, map[string]string{"owner": "root"}, nil)

		resp, err := s.ListVolumes(context.Background(), &csi.ListVolumesRequest{})
		assert.NoError(t, err)
		assert.Equal(t, 0, len(resp.Entries))
	})

	t.Run("ListVolumes includes volumes with CSI-tagged export description", func(t *testing.T) {
		s, mockClient := newService()
		setupFilesystemList(mockClient, []*v2.ContainerChild{
			{Name: strPtr("vol-with-csi-quota-tag"), Path: strPtr("/ifs/data")},
		}, "", nil)
		setupExportsList(mockClient, v2.ExportList{
			{ID: 11, Zone: "System", Description: "CSI_QUOTA_ID:AABpAQE", Paths: clientSlice("/ifs/data/vol-with-csi-quota-tag")},
		}, nil)

		resp, err := s.ListVolumes(context.Background(), &csi.ListVolumesRequest{})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(resp.Entries))
		assert.Equal(t, "vol-with-csi-quota-tag", resp.Entries[0].Volume.VolumeContext["Name"])
	})

	t.Run("ListVolumes skips filesystems with no matching export", func(t *testing.T) {
		s, mockClient := newService()
		setupFilesystemList(mockClient, []*v2.ContainerChild{
			{Name: strPtr("vol1"), Path: strPtr("/ifs/data")},
		}, "", nil)
		setupExportsList(mockClient, v2.ExportList{}, nil)

		resp, err := s.ListVolumes(context.Background(), &csi.ListVolumesRequest{})
		assert.NoError(t, err)
		assert.Equal(t, 0, len(resp.Entries))
	})

	t.Run("ListVolumes includes volumes with dummy localhost export client fingerprint", func(t *testing.T) {
		s, mockClient := newService()
		setupFilesystemList(mockClient, []*v2.ContainerChild{
			{Name: strPtr("vol-with-dummyhost"), Path: strPtr("/ifs/data")},
		}, "", nil)
		setupExportsList(mockClient, v2.ExportList{&v2.Export{
			ID:               11,
			Zone:             "System",
			Paths:            clientSlice("/ifs/data/vol-with-dummyhost"),
			Clients:          clientSlice("127.0.0.1"),
			ReadOnlyClients:  clientSlice(),
			ReadWriteClients: clientSlice(),
			RootClients:      clientSlice(),
		}}, nil)

		resp, err := s.ListVolumes(context.Background(), &csi.ListVolumesRequest{})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(resp.Entries))
		assert.Equal(t, "vol-with-dummyhost", resp.Entries[0].Volume.VolumeContext["Name"])
	})

	t.Run("ListVolumes uses default containerPath /ifs when not configured", func(t *testing.T) {
		s, mockClient := newService()
		s.isiClusters.Range(func(_, value interface{}) bool {
			config := value.(*IsilonClusterConfig)
			config.IsiPath = ""
			return true
		})
		setupFilesystemList(mockClient, []*v2.ContainerChild{
			{Name: strPtr("vol1"), Path: strPtr("/ifs")},
		}, "", nil)
		setupExportsList(mockClient, v2.ExportList{
			{ID: 44, Zone: "System", Paths: clientSlice("/ifs/vol1")},
		}, nil)
		setupVolumeMetadata(mockClient, map[string]string{headerPersistentVolumeClaimName: "pvc-vol1"}, nil)

		resp, err := s.ListVolumes(context.Background(), &csi.ListVolumesRequest{})
		assert.NoError(t, err)
		assert.Equal(t, 1, len(resp.Entries))
		assert.Equal(t, identifiers.GetNormalizedVolumeID(context.Background(), "vol1", 44, "System", "system"), resp.Entries[0].Volume.VolumeId)
	})

	t.Run("ListVolumes filesystems API error", func(t *testing.T) {
		s, mockClient := newService()
		setupFilesystemList(mockClient, nil, "", assert.AnError)

		_, err := s.ListVolumes(context.Background(), &csi.ListVolumesRequest{})
		assert.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("ListVolumes export list API error", func(t *testing.T) {
		s, mockClient := newService()
		setupFilesystemList(mockClient, []*v2.ContainerChild{
			{Name: strPtr("vol1"), Path: strPtr("/ifs/data")},
		}, "", nil)
		setupExportsList(mockClient, nil, assert.AnError)

		_, err := s.ListVolumes(context.Background(), &csi.ListVolumesRequest{})
		assert.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("ListVolumes getIsilonConfig error", func(t *testing.T) {
		s, _ := newService()
		s.isiClusters.Delete("system")

		_, err := s.ListVolumes(context.Background(), &csi.ListVolumesRequest{})
		assert.Error(t, err)
	})

	t.Run("ListVolumes autoProbe error", func(t *testing.T) {
		s, mockClient := newService()
		s.opts.AutoProbe = false
		setupFilesystemList(mockClient, nil, "", assert.AnError)

		_, err := s.ListVolumes(context.Background(), &csi.ListVolumesRequest{})
		assert.Error(t, err)
	})
}

func TestGetIpsFromAllowedNetworks(t *testing.T) {
	originalGetNodeLabelsWithName := getNodeLabelsWithNameFunc
	after := func() {
		getNodeLabelsWithNameFunc = originalGetNodeLabelsWithName
	}
	defer after()

	tests := []struct {
		name            string
		nodeID          string
		allowedNetworks []string
		nodeLabels      map[string]string
		labelsErr       error
		expectedIPs     []string
		wantErr         bool
		errContains     string
	}{
		{
			// Scenario: Node labels match CIDRs (AC-003, AC-005)
			name:            "node labels match CIDRs",
			nodeID:          "worker1=#=#=worker1.domain=#=#=127.0.0.1",
			allowedNetworks: []string{"10.0.0.0/24"},
			nodeLabels: map[string]string{
				"csi-isilon.dellemc.com/az-10.0.0.0-24-10.0.0.1": "true",
				"csi-isilon.dellemc.com/az-10.0.0.0-24-10.0.0.2": "true",
			},
			expectedIPs: []string{"10.0.0.1", "10.0.0.2"},
		},
		{
			// Scenario: Partial match across multiple CIDRs (AC-004)
			name:            "partial match across multiple CIDRs",
			nodeID:          "worker1=#=#=worker1.domain=#=#=127.0.0.1",
			allowedNetworks: []string{"10.0.0.0/24", "192.168.1.0/24"},
			nodeLabels: map[string]string{
				"csi-isilon.dellemc.com/az-10.0.0.0-24-10.0.0.1":       "true",
				"csi-isilon.dellemc.com/az-192.168.1.0-24-192.168.1.5": "true",
				"csi-isilon.dellemc.com/az-172.16.0.0-16-172.16.0.9":   "true",
			},
			expectedIPs: []string{"10.0.0.1", "192.168.1.5"},
		},
		{
			// Scenario: No matching labels
			name:            "no matching labels",
			nodeID:          "worker1=#=#=worker1.domain=#=#=127.0.0.1",
			allowedNetworks: []string{"10.0.0.0/24"},
			nodeLabels: map[string]string{
				"csi-isilon.dellemc.com/az-172.16.0.0-16-172.16.0.9": "true",
			},
			wantErr:     true,
			errContains: "no IPs in node labels match allowedNetworks",
		},
		{
			// Scenario: Kubernetes API error
			name:            "kubernetes API error",
			nodeID:          "worker1=#=#=worker1.domain=#=#=127.0.0.1",
			allowedNetworks: []string{"10.0.0.0/24"},
			labelsErr:       fmt.Errorf("k8s API unavailable"),
			wantErr:         true,
			errContains:     "k8s API unavailable",
		},
		{
			name:        "invalid node ID",
			nodeID:      "invalid-node-id",
			wantErr:     true,
			errContains: "cannot match the expected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getNodeLabelsWithNameFunc = func(_ *service) func(string) (map[string]string, error) {
				return func(string) (map[string]string, error) {
					return tt.nodeLabels, tt.labelsErr
				}
			}

			s := &service{opts: Opts{allowedNetworks: tt.allowedNetworks}}
			ips, err := s.getIpsFromAllowedNetworks(context.Background(), tt.nodeID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("getIpsFromAllowedNetworks() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("getIpsFromAllowedNetworks() error = %v, expected to contain %q", err, tt.errContains)
				}
				return
			}

			sort.Strings(ips)
			sort.Strings(tt.expectedIPs)
			if !reflect.DeepEqual(ips, tt.expectedIPs) {
				t.Errorf("getIpsFromAllowedNetworks() IPs = %v, expected %v", ips, tt.expectedIPs)
			}
		})
	}
}

// TestControllerPublishVolume_AllowedNetworksModeBranching verifies that
// ControllerPublishVolume only invokes the multi-NIC IP resolution
// (getIpsFromAllowedNetworks) when allowedNetworksMode is "multi",
// and never invokes it in "single" mode, preserving backward
// compatibility.
func TestControllerPublishVolume_AllowedNetworksModeBranching(t *testing.T) {
	originalGetNodeLabelsWithName := getNodeLabelsWithNameFunc
	after := func() {
		getNodeLabelsWithNameFunc = originalGetNodeLabelsWithName
	}
	defer after()

	const nodeID = "worker1=#=#=worker1.domain=#=#=10.0.0.1"
	tests := []struct {
		name              string
		mode              string
		allowedNetworks   []string
		nodeLabels        map[string]string
		wantLabelsFetched bool
		wantErrContains   string
	}{
		{
			// Scenario: Single-mode publish uses existing behavior (AC-002)
			name:            "single mode never resolves multi-NIC IPs",
			mode:            constants.AllowedNetworksModeDefault,
			allowedNetworks: []string{"10.0.0.0/24"},
			nodeLabels: map[string]string{
				"csi-isilon.dellemc.com/az-10.0.0.0-24-10.0.0.5": "true",
			},
			wantLabelsFetched: false,
			wantErrContains:   "volume ID is required",
		},
		{
			// Scenario: Multi-mode publish adds all IPs (AC-003)
			name:            "multi mode resolves multi-NIC IPs from node labels",
			mode:            constants.AllowedNetworksModeMulti,
			allowedNetworks: []string{"10.0.0.0/24"},
			nodeLabels: map[string]string{
				"csi-isilon.dellemc.com/az-10.0.0.0-24-10.0.0.5": "true",
				"csi-isilon.dellemc.com/az-10.0.0.0-24-10.0.0.6": "true",
			},
			wantLabelsFetched: true,
			wantErrContains:   "volume ID is required",
		},
		{
			name:              "multi mode without allowedNetworks does not resolve",
			mode:              constants.AllowedNetworksModeMulti,
			allowedNetworks:   nil,
			nodeLabels:        map[string]string{},
			wantLabelsFetched: false,
			wantErrContains:   "volume ID is required",
		},
		{
			name:            "multi mode with no matching labels returns wrapped error",
			mode:            constants.AllowedNetworksModeMulti,
			allowedNetworks: []string{"10.0.0.0/24"},
			nodeLabels: map[string]string{
				"csi-isilon.dellemc.com/az-172.16.0.0-16-172.16.0.9": "true",
			},
			wantLabelsFetched: true,
			wantErrContains:   "multi-NIC: failed to resolve NFS IPs from node labels",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var callCount int
			getNodeLabelsWithNameFunc = func(_ *service) func(string) (map[string]string, error) {
				return func(string) (map[string]string, error) {
					callCount++
					return tt.nodeLabels, nil
				}
			}

			s := &service{
				opts: Opts{
					allowedNetworksMode: tt.mode,
					allowedNetworks:     tt.allowedNetworks,
				},
			}

			req := &csi.ControllerPublishVolumeRequest{
				VolumeId: "",
				NodeId:   nodeID,
			}

			_, err := s.ControllerPublishVolume(context.Background(), req)
			gotFetched := callCount > 0
			if gotFetched != tt.wantLabelsFetched {
				t.Errorf("node labels fetched = %v, want %v (callCount=%d)", gotFetched, tt.wantLabelsFetched, callCount)
			}
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Errorf("error = %v, expected to contain %q", err, tt.wantErrContains)
			}
		})
	}
}

// TestControllerUnpublishVolume_AllowedNetworksModeBranching verifies that
// ControllerUnpublishVolume only invokes the multi-NIC IP resolution
// (getIpsFromAllowedNetworks) when allowedNetworksMode is "multi",
// mirroring the publish-side behavior, and preserves single-mode backward
// compatibility (AC-002).
func TestControllerUnpublishVolume_AllowedNetworksModeBranching(t *testing.T) {
	originalGetNodeLabelsWithName := getNodeLabelsWithNameFunc
	after := func() {
		getNodeLabelsWithNameFunc = originalGetNodeLabelsWithName
	}
	defer after()

	systemPv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "systempv2",
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
		name              string
		mode              string
		allowedNetworks   []string
		nodeLabels        map[string]string
		wantLabelsFetched bool
	}{
		{
			name:            "single mode never resolves multi-NIC IPs",
			mode:            constants.AllowedNetworksModeDefault,
			allowedNetworks: []string{"10.0.0.0/24"},
			nodeLabels: map[string]string{
				"csi-isilon.dellemc.com/az-10.0.0.0-24-10.0.0.5": "true",
			},
			wantLabelsFetched: false,
		},
		{
			name:            "multi mode resolves multi-NIC IPs from node labels",
			mode:            constants.AllowedNetworksModeMulti,
			allowedNetworks: []string{"10.0.0.0/24"},
			nodeLabels: map[string]string{
				"csi-isilon.dellemc.com/az-10.0.0.0-24-10.0.0.5": "true",
				"csi-isilon.dellemc.com/az-10.0.0.0-24-10.0.0.6": "true",
			},
			wantLabelsFetched: true,
		},
		{
			name:              "multi mode without allowedNetworks does not resolve",
			mode:              constants.AllowedNetworksModeMulti,
			allowedNetworks:   nil,
			nodeLabels:        map[string]string{},
			wantLabelsFetched: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var callCount int
			getNodeLabelsWithNameFunc = func(_ *service) func(string) (map[string]string, error) {
				return func(string) (map[string]string, error) {
					callCount++
					return tt.nodeLabels, nil
				}
			}

			mockClient := &isimocks.Client{}
			// Generic catch-all mocks: since these tests only assert on the
			// mode-branching decision (whether node labels were fetched),
			// the downstream export removal is allowed to no-op/fail safely.
			mockClient.On("Get", anyArgs[0:6]...).Return(nil)
			mockClient.On("Put", anyArgs...).Return(nil)
			ignoreUnresolvableHosts := false
			isiConfig := &IsilonClusterConfig{
				ClusterName:             "system",
				IgnoreUnresolvableHosts: &ignoreUnresolvableHosts,
				isiSvc: &isiService{
					client: &isi.Client{
						API: mockClient,
					},
				},
			}

			s := &service{
				k8sclient:             fake.NewSimpleClientset(systemPv),
				defaultIsiClusterName: "system",
				isiClusters:           &sync.Map{},
				opts: Opts{
					allowedNetworksMode: tt.mode,
					allowedNetworks:     tt.allowedNetworks,
				},
			}
			s.isiClusters.Store("system", isiConfig)
			req := &csi.ControllerUnpublishVolumeRequest{
				VolumeId: "systempv2=_=_=19=_=_=csi0zone",
				NodeId:   identifiers.DummyHostNodeID,
			}

			_, _ = s.ControllerUnpublishVolume(context.Background(), req)
			gotFetched := callCount > 0
			if gotFetched != tt.wantLabelsFetched {
				t.Errorf("node labels fetched = %v, want %v (callCount=%d)", gotFetched, tt.wantLabelsFetched, callCount)
			}
		})
	}
}

// Tests for directory-backed volume provisioning

func TestSharedExportPathValidation(t *testing.T) {
	tests := []struct {
		name             string
		sharedExportPath string
		expectError      bool
		errorContains    string
	}{
		{
			name:             "Valid absolute path",
			sharedExportPath: "/ifs/shared/export",
			expectError:      false,
		},
		{
			name:             "Valid path with multiple segments",
			sharedExportPath: "/ifs/data/shared/volumes",
			expectError:      false,
		},
		{
			name:             "Invalid - relative path",
			sharedExportPath: "relative/path",
			expectError:      true,
			errorContains:    "must be an absolute path",
		},
		{
			name:             "Invalid - path traversal at start",
			sharedExportPath: "../etc/passwd",
			expectError:      true,
			errorContains:    "must be an absolute path",
		},
		{
			name:             "Invalid - path traversal in middle",
			sharedExportPath: "/ifs/../etc/passwd",
			expectError:      true,
			errorContains:    "path traversal sequences",
		},
		{
			name:             "Invalid - path traversal at end",
			sharedExportPath: "/ifs/data/../secrets",
			expectError:      true,
			errorContains:    "path traversal sequences",
		},
		{
			name:             "Invalid - double dot only",
			sharedExportPath: "/..",
			expectError:      true,
			errorContains:    "path traversal sequences",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate using the same logic as CreateVolume
			var err error
			if !strings.HasPrefix(tt.sharedExportPath, "/") {
				err = fmt.Errorf("parameter 'SharedExportPath' must be an absolute path (start with '/'), got '%s'", tt.sharedExportPath)
			} else if strings.Contains(tt.sharedExportPath, "..") {
				err = fmt.Errorf("parameter 'SharedExportPath' contains path traversal sequences ('..'), got '%s'", tt.sharedExportPath)
			}

			if tt.expectError {
				assert.Error(t, err, "Expected error for path: %s", tt.sharedExportPath)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err, "Expected no error for path: %s", tt.sharedExportPath)
			}
		})
	}
}

func TestGetCSIVolume_DirectoryBacked(t *testing.T) {
	s := &service{}
	ctx := context.Background()

	tests := []struct {
		name              string
		directoryBacked   bool
		sharedExportPath  string
		expectedProvMode  string
		expectedDirPath   string
		expectedSharedExp string
	}{
		{
			name:              "Export-backed volume",
			directoryBacked:   false,
			sharedExportPath:  "",
			expectedProvMode:  "export",
			expectedDirPath:   "",
			expectedSharedExp: "",
		},
		{
			name:              "Directory-backed volume",
			directoryBacked:   true,
			sharedExportPath:  "/ifs/shared/export",
			expectedProvMode:  "directory",
			expectedDirPath:   "test-vol",
			expectedSharedExp: "/ifs/shared/export",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volume := s.getCSIVolume(ctx, 123, "test-vol", "/ifs/test/path", "System", 1073741824,
				"192.168.1.1", "true", "", "", "cluster1", "192.168.1.0/24",
				tt.directoryBacked, tt.sharedExportPath, "", "")

			if volume == nil {
				t.Fatal("Expected volume to be non-nil")
			}

			if mode, ok := volume.VolumeContext["ProvisioningMode"]; !ok || mode != tt.expectedProvMode {
				t.Errorf("Expected ProvisioningMode=%s, got %s", tt.expectedProvMode, mode)
			}

			if tt.directoryBacked {
				if dirPath, ok := volume.VolumeContext["DirectoryPath"]; !ok || dirPath != tt.expectedDirPath {
					t.Errorf("Expected DirectoryPath=%s, got %s", tt.expectedDirPath, dirPath)
				}
				if sharedPath, ok := volume.VolumeContext["SharedExportPath"]; !ok || sharedPath != tt.expectedSharedExp {
					t.Errorf("Expected SharedExportPath=%s, got %s", tt.expectedSharedExp, sharedPath)
				}
				if _, ok := volume.VolumeContext["SharedExportID"]; !ok {
					t.Error("Expected SharedExportID to be present for directory-backed volumes")
				}
			} else {
				if _, ok := volume.VolumeContext["DirectoryPath"]; ok {
					t.Error("DirectoryPath should not be present for export-backed volumes")
				}
				if _, ok := volume.VolumeContext["SharedExportPath"]; ok {
					t.Error("SharedExportPath should not be present for export-backed volumes")
				}
			}
		})
	}
}

func TestGetCreateVolumeResponse_DirectoryBacked(t *testing.T) {
	s := &service{}
	ctx := context.Background()

	// Test export-backed mode
	response := s.getCreateVolumeResponse(ctx, 100, "vol1", "/ifs/data/vol1", "System",
		1073741824, "192.168.1.1", "true", "", "", "cluster1", "192.168.1.0/24", false, "", "", "")

	if response == nil || response.Volume == nil {
		t.Fatal("Expected non-nil response and volume")
	}

	if mode := response.Volume.VolumeContext["ProvisioningMode"]; mode != "export" {
		t.Errorf("Expected ProvisioningMode=export, got %s", mode)
	}

	// Test directory-backed mode
	response2 := s.getCreateVolumeResponse(ctx, 100, "vol2", "/ifs/shared/vol2", "System",
		2147483648, "192.168.1.1", "true", "", "", "cluster1", "192.168.1.0/24", true, "/ifs/shared", "", "")

	if response2 == nil || response2.Volume == nil {
		t.Fatal("Expected non-nil response and volume for directory-backed")
	}

	if mode := response2.Volume.VolumeContext["ProvisioningMode"]; mode != "directory" {
		t.Errorf("Expected ProvisioningMode=directory, got %s", mode)
	}

	if dirPath := response2.Volume.VolumeContext["DirectoryPath"]; dirPath != "vol2" {
		t.Errorf("Expected DirectoryPath=vol2, got %s", dirPath)
	}

	if sharedPath := response2.Volume.VolumeContext["SharedExportPath"]; sharedPath != "/ifs/shared" {
		t.Errorf("Expected SharedExportPath=/ifs/shared, got %s", sharedPath)
	}
}

func TestDirectoryBackedVolumeContextFields(t *testing.T) {
	s := &service{}
	ctx := context.Background()

	tests := []struct {
		name             string
		exportID         int
		volName          string
		path             string
		directoryBacked  bool
		sharedExportPath string
		wantMode         string
		wantDirPath      bool
		wantSharedPath   bool
		wantSharedID     bool
	}{
		{
			name:             "Export-backed volume should not have directory fields",
			exportID:         1,
			volName:          "export-vol",
			path:             "/ifs/data/export-vol",
			directoryBacked:  false,
			sharedExportPath: "",
			wantMode:         "export",
			wantDirPath:      false,
			wantSharedPath:   false,
			wantSharedID:     false,
		},
		{
			name:             "Directory-backed volume should have all directory fields",
			exportID:         2,
			volName:          "dir-vol",
			path:             "/ifs/shared/dir-vol",
			directoryBacked:  true,
			sharedExportPath: "/ifs/shared",
			wantMode:         "directory",
			wantDirPath:      true,
			wantSharedPath:   true,
			wantSharedID:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vol := s.getCSIVolume(ctx, tt.exportID, tt.volName, tt.path, "System",
				1073741824, "192.168.1.1", "true", "", "", "cluster1", "192.168.1.0/24",
				tt.directoryBacked, tt.sharedExportPath, "", "")

			if vol == nil {
				t.Fatal("Expected non-nil volume")
			}

			// Check ProvisioningMode
			if mode, ok := vol.VolumeContext["ProvisioningMode"]; !ok {
				t.Error("ProvisioningMode should always be present")
			} else if mode != tt.wantMode {
				t.Errorf("ProvisioningMode = %s, want %s", mode, tt.wantMode)
			}

			// Check DirectoryPath
			if _, ok := vol.VolumeContext["DirectoryPath"]; ok != tt.wantDirPath {
				t.Errorf("DirectoryPath presence = %v, want %v", ok, tt.wantDirPath)
			}

			// Check SharedExportPath
			if _, ok := vol.VolumeContext["SharedExportPath"]; ok != tt.wantSharedPath {
				t.Errorf("SharedExportPath presence = %v, want %v", ok, tt.wantSharedPath)
			}

			// Check SharedExportID
			if _, ok := vol.VolumeContext["SharedExportID"]; ok != tt.wantSharedID {
				t.Errorf("SharedExportID presence = %v, want %v", ok, tt.wantSharedID)
			}

			// Verify field values for directory-backed
			if tt.directoryBacked {
				if dirPath := vol.VolumeContext["DirectoryPath"]; dirPath != tt.volName {
					t.Errorf("DirectoryPath = %s, want %s", dirPath, tt.volName)
				}
				if sharedPath := vol.VolumeContext["SharedExportPath"]; sharedPath != tt.sharedExportPath {
					t.Errorf("SharedExportPath = %s, want %s", sharedPath, tt.sharedExportPath)
				}
			}
		})
	}
}

func TestVolumeContextMetadata(t *testing.T) {
	s := &service{}
	ctx := context.Background()

	// Test that all required base fields are present
	vol := s.getCSIVolume(ctx, 123, "test-vol", "/ifs/test/path", "TestZone",
		5368709120, "10.0.0.1", "false", "snap-123", "vol-456", "test-cluster",
		"10.0.0.0/24", false, "", "", "")

	requiredFields := []string{
		"ID", "Name", "Path", "AccessZone", "AzServiceIP",
		"AzNetwork", "RootClientEnabled", "ClusterName", "ProvisioningMode",
	}

	for _, field := range requiredFields {
		if _, ok := vol.VolumeContext[field]; !ok {
			t.Errorf("Required field %s missing from VolumeContext", field)
		}
	}

	// Verify specific values
	if vol.VolumeContext["ID"] != "123" {
		t.Errorf("ID = %s, want 123", vol.VolumeContext["ID"])
	}
	if vol.VolumeContext["Name"] != "test-vol" {
		t.Errorf("Name = %s, want test-vol", vol.VolumeContext["Name"])
	}
	if vol.VolumeContext["AccessZone"] != "TestZone" {
		t.Errorf("AccessZone = %s, want TestZone", vol.VolumeContext["AccessZone"])
	}
}

func TestGetCreateVolumeResponseWithSourceSnapAndVol(t *testing.T) {
	s := &service{}
	ctx := context.Background()

	// Test with source snapshot and volume IDs
	resp := s.getCreateVolumeResponse(ctx, 999, "vol-with-sources",
		"/ifs/data/vol-with-sources", "System", 10737418240,
		"192.168.100.1", "true", "snapshot-abc", "source-vol-xyz",
		"prod-cluster", "192.168.100.0/24", false, "", "", "")

	if resp == nil || resp.Volume == nil {
		t.Fatal("Expected non-nil response")
	}

	vol := resp.Volume

	// VolumeId is normalized (e.g. "vol-with-sources=_=_=999=_=_=System=_=_=prod-cluster")
	if vol.VolumeId == "" {
		t.Error("VolumeId should not be empty")
	}
	if vol.CapacityBytes != 10737418240 {
		t.Errorf("CapacityBytes = %d, want 10737418240", vol.CapacityBytes)
	}

	// Verify source information in content source (snapshot takes priority over volume)
	if vol.ContentSource == nil {
		t.Error("ContentSource should not be nil when sourceSnapshotID provided")
	}
	if vol.ContentSource.GetSnapshot() == nil {
		t.Error("ContentSource should contain snapshot info")
	}
}

func TestDirectoryBackedWithMultipleExportIDs(t *testing.T) {
	s := &service{}
	ctx := context.Background()

	exportIDs := []int{1, 100, 999, 12345}

	for _, exportID := range exportIDs {
		vol := s.getCSIVolume(ctx, exportID, "test-vol", "/ifs/shared/test-vol",
			"System", 1073741824, "192.168.1.1", "true", "", "",
			"cluster1", "192.168.1.0/24", true, "/ifs/shared", "", "")

		if vol == nil {
			t.Fatalf("Expected non-nil volume for exportID %d", exportID)
		}

		// Verify SharedExportID matches the export ID
		sharedIDStr := vol.VolumeContext["SharedExportID"]
		if sharedIDStr == "" {
			t.Errorf("SharedExportID should not be empty for exportID %d", exportID)
		}
	}
}

func TestGetCSIVolumeFuncIntegration(t *testing.T) {
	s := &service{}
	ctx := context.Background()

	// Save original function
	originalFunc := getCSIVolumeFunc

	// Test with default getCSIVolumeFunc
	vol := getCSIVolumeFunc(s)(ctx, 100, "test-vol", "/ifs/test", "System",
		1073741824, "192.168.1.1", "true", "", "", "cluster1",
		"192.168.1.0/24", false, "", "", "")

	if vol == nil {
		t.Fatal("getCSIVolumeFunc should return non-nil volume")
	}

	// Restore original function
	getCSIVolumeFunc = originalFunc
}

func TestVolumeContextWithEmptySourceIDs(t *testing.T) {
	s := &service{}
	ctx := context.Background()

	// Test with empty source snapshot and volume IDs (normal creation)
	vol := s.getCSIVolume(ctx, 50, "normal-vol", "/ifs/data/normal-vol",
		"System", 2147483648, "192.168.1.1", "false", "", "",
		"cluster1", "192.168.1.0/24", false, "", "", "")

	if vol == nil {
		t.Fatal("Expected non-nil volume")
	}

	// ContentSource should be nil when no source specified
	if vol.ContentSource != nil {
		t.Error("ContentSource should be nil when no source snapshot or volume specified")
	}
}

func TestDirectoryBackedVolumeCapacity(t *testing.T) {
	s := &service{}
	ctx := context.Background()

	capacities := []int64{
		1073741824,    // 1 GiB
		10737418240,   // 10 GiB
		107374182400,  // 100 GiB
		1099511627776, // 1 TiB
	}

	for _, capacity := range capacities {
		vol := s.getCSIVolume(ctx, 1, "capacity-test", "/ifs/shared/capacity-test",
			"System", capacity, "192.168.1.1", "true", "", "",
			"cluster1", "192.168.1.0/24", true, "/ifs/shared", "", "")

		if vol == nil {
			t.Fatalf("Expected non-nil volume for capacity %d", capacity)
		}

		if vol.CapacityBytes != capacity {
			t.Errorf("CapacityBytes = %d, want %d", vol.CapacityBytes, capacity)
		}

		// Verify directory-backed specific fields
		if vol.VolumeContext["ProvisioningMode"] != "directory" {
			t.Error("Expected ProvisioningMode=directory")
		}
	}
}

func strPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func TestValidateMutableParamKeys(t *testing.T) {
	valid := map[string]string{
		"AdvisoryLimit": "10",
		"SoftLimit":     "20",
		"SoftGracePrd":  "30",
	}
	assert.NoError(t, validateMutableParamKeys(valid))

	invalid := map[string]string{
		"AdvisoryLimit": "10",
		"InvalidKey":    "20",
	}
	assert.Error(t, validateMutableParamKeys(invalid))
}

func TestHasOtherDirectoryBackedVolumesOnExport_NoOtherVolumes(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	svc := &service{k8sclient: fakeClient}

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-123"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:           constants.PluginName,
					VolumeHandle:     "pvc-123=_=_=100=_=_=System=_=_=cluster1=_=_=directory",
					VolumeAttributes: map[string]string{"ProvisioningMode": "directory"},
				},
			},
		},
	}
	_, err := fakeClient.CoreV1().PersistentVolumes().Create(ctx, pv, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Create VolumeAttachment for the volume being unpublished (should be excluded)
	pvName := "pvc-123"
	va := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: "va-123"},
		Spec: storagev1.VolumeAttachmentSpec{
			Attacher: constants.PluginName,
			NodeName: "node-1",
			Source:   storagev1.VolumeAttachmentSource{PersistentVolumeName: &pvName},
		},
		Status: storagev1.VolumeAttachmentStatus{Attached: true},
	}
	_, err = fakeClient.StorageV1().VolumeAttachments().Create(ctx, va, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Use proper nodeID format: nodeName=#=#=fqdn=#=#=ip
	nodeID := "node-1=#=#=node-1.example.com=#=#=10.0.0.1"
	hasOther, err := svc.hasOtherDirectoryBackedVolumesOnExport(ctx, nodeID, 100, "System", "pvc-123")
	assert.NoError(t, err)
	assert.False(t, hasOther)
}

func TestHasOtherDirectoryBackedVolumesOnExport_HasOtherVolumes(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	svc := &service{k8sclient: fakeClient}

	pv1 := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-111"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:           constants.PluginName,
					VolumeHandle:     "pvc-111=_=_=100=_=_=System=_=_=cluster1=_=_=directory",
					VolumeAttributes: map[string]string{"ProvisioningMode": "directory"},
				},
			},
		},
		Status: corev1.PersistentVolumeStatus{
			Phase: corev1.VolumeBound,
		},
	}
	pv2 := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-222"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:           constants.PluginName,
					VolumeHandle:     "pvc-222=_=_=100=_=_=System=_=_=cluster1=_=_=directory",
					VolumeAttributes: map[string]string{"ProvisioningMode": "directory"},
				},
			},
		},
		Status: corev1.PersistentVolumeStatus{
			Phase: corev1.VolumeBound,
		},
	}

	_, err := fakeClient.CoreV1().PersistentVolumes().Create(ctx, pv1, metav1.CreateOptions{})
	assert.NoError(t, err)
	_, err = fakeClient.CoreV1().PersistentVolumes().Create(ctx, pv2, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Create VolumeAttachments for both PVs on node-1
	pvName1 := "pvc-111"
	pvName2 := "pvc-222"
	va1 := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: "va-111"},
		Spec: storagev1.VolumeAttachmentSpec{
			Attacher: constants.PluginName,
			NodeName: "node-1",
			Source:   storagev1.VolumeAttachmentSource{PersistentVolumeName: &pvName1},
		},
		Status: storagev1.VolumeAttachmentStatus{Attached: true},
	}
	va2 := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: "va-222"},
		Spec: storagev1.VolumeAttachmentSpec{
			Attacher: constants.PluginName,
			NodeName: "node-1",
			Source:   storagev1.VolumeAttachmentSource{PersistentVolumeName: &pvName2},
		},
		Status: storagev1.VolumeAttachmentStatus{Attached: true},
	}
	_, err = fakeClient.StorageV1().VolumeAttachments().Create(ctx, va1, metav1.CreateOptions{})
	assert.NoError(t, err)
	_, err = fakeClient.StorageV1().VolumeAttachments().Create(ctx, va2, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Use proper nodeID format
	nodeID := "node-1=#=#=node-1.example.com=#=#=10.0.0.1"
	hasOther, err := svc.hasOtherDirectoryBackedVolumesOnExport(ctx, nodeID, 100, "System", "pvc-111")
	assert.NoError(t, err)
	assert.True(t, hasOther)
}

func TestHasOtherDirectoryBackedVolumesOnExport_DifferentExport(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	svc := &service{k8sclient: fakeClient}

	// pv1 on export 100, pv2 on export 200 (different export)
	pv1 := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-aaa"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:           constants.PluginName,
					VolumeHandle:     "pvc-aaa=_=_=100=_=_=System=_=_=cluster1=_=_=directory",
					VolumeAttributes: map[string]string{"ProvisioningMode": "directory"},
				},
			},
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
	}
	pv2 := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-bbb"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:           constants.PluginName,
					VolumeHandle:     "pvc-bbb=_=_=200=_=_=System=_=_=cluster1=_=_=directory",
					VolumeAttributes: map[string]string{"ProvisioningMode": "directory"},
				},
			},
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
	}

	_, err := fakeClient.CoreV1().PersistentVolumes().Create(ctx, pv1, metav1.CreateOptions{})
	assert.NoError(t, err)
	_, err = fakeClient.CoreV1().PersistentVolumes().Create(ctx, pv2, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Create VolumeAttachments for both PVs on node-1
	pvName1 := "pvc-aaa"
	pvName2 := "pvc-bbb"
	va1 := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: "va-aaa"},
		Spec: storagev1.VolumeAttachmentSpec{
			Attacher: constants.PluginName,
			NodeName: "node-1",
			Source:   storagev1.VolumeAttachmentSource{PersistentVolumeName: &pvName1},
		},
		Status: storagev1.VolumeAttachmentStatus{Attached: true},
	}
	va2 := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: "va-bbb"},
		Spec: storagev1.VolumeAttachmentSpec{
			Attacher: constants.PluginName,
			NodeName: "node-1",
			Source:   storagev1.VolumeAttachmentSource{PersistentVolumeName: &pvName2},
		},
		Status: storagev1.VolumeAttachmentStatus{Attached: true},
	}
	_, err = fakeClient.StorageV1().VolumeAttachments().Create(ctx, va1, metav1.CreateOptions{})
	assert.NoError(t, err)
	_, err = fakeClient.StorageV1().VolumeAttachments().Create(ctx, va2, metav1.CreateOptions{})
	assert.NoError(t, err)

	// pvc-bbb is on export 200, not 100, so should return false
	nodeID := "node-1=#=#=node-1.example.com=#=#=10.0.0.1"
	hasOther, err := svc.hasOtherDirectoryBackedVolumesOnExport(ctx, nodeID, 100, "System", "pvc-aaa")
	assert.NoError(t, err)
	assert.False(t, hasOther)
}

func TestHasOtherDirectoryBackedVolumesOnExport_NilCSI(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	svc := &service{k8sclient: fakeClient}

	// Test various PV configurations that should NOT be counted
	pvNilCSI := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-nilcsi"},
		Spec:       corev1.PersistentVolumeSpec{},
		Status:     corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
	}
	pvWrongDriver := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-wrongdrv"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       "other-driver",
					VolumeHandle: "pvc-wrongdrv=_=_=100=_=_=System=_=_=cluster1=_=_=directory",
				},
			},
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
	}
	pvNonDirectory := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-nondir"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:           constants.PluginName,
					VolumeHandle:     "pvc-nondir=_=_=100=_=_=System=_=_=cluster1",
					VolumeAttributes: map[string]string{"ProvisioningMode": "file"},
				},
			},
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
	}
	pvInvalidHandle := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-badhandle"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:           constants.PluginName,
					VolumeHandle:     "invalid-handle-no-separators",
					VolumeAttributes: map[string]string{"ProvisioningMode": "directory"},
				},
			},
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
	}
	pvDiffZone := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-diffzone"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:           constants.PluginName,
					VolumeHandle:     "pvc-diffzone=_=_=100=_=_=OtherZone=_=_=cluster1=_=_=directory",
					VolumeAttributes: map[string]string{"ProvisioningMode": "directory"},
				},
			},
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
	}

	pvs := []*corev1.PersistentVolume{pvNilCSI, pvWrongDriver, pvNonDirectory, pvInvalidHandle, pvDiffZone}
	for _, pv := range pvs {
		_, err := fakeClient.CoreV1().PersistentVolumes().Create(ctx, pv, metav1.CreateOptions{})
		assert.NoError(t, err)
	}

	// Create VolumeAttachments for all PVs on node-1
	for i, pv := range pvs {
		pvName := pv.Name
		va := &storagev1.VolumeAttachment{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("va-%d", i)},
			Spec: storagev1.VolumeAttachmentSpec{
				Attacher: constants.PluginName,
				NodeName: "node-1",
				Source:   storagev1.VolumeAttachmentSource{PersistentVolumeName: &pvName},
			},
			Status: storagev1.VolumeAttachmentStatus{Attached: true},
		}
		_, err := fakeClient.StorageV1().VolumeAttachments().Create(ctx, va, metav1.CreateOptions{})
		assert.NoError(t, err)
	}

	// None of these should match: nil CSI, wrong driver, non-directory mode, invalid handle, different zone
	nodeID := "node-1=#=#=node-1.example.com=#=#=10.0.0.1"
	hasOther, err := svc.hasOtherDirectoryBackedVolumesOnExport(ctx, nodeID, 100, "System", "__excluded__")
	assert.NoError(t, err)
	assert.False(t, hasOther)
}

func TestHasOtherDirectoryBackedVolumesOnExport_ReleasedPVIgnored(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	svc := &service{k8sclient: fakeClient}

	// Create a Released PV (simulating deleted PVC)
	releasedPV := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pvc-released",
		},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       constants.PluginName,
					VolumeHandle: "pvc-released=_=_=100=_=_=System=_=_=cluster1=_=_=directory",
					VolumeAttributes: map[string]string{
						"ProvisioningMode": "directory",
					},
				},
			},
		},
		Status: corev1.PersistentVolumeStatus{
			Phase: corev1.VolumeReleased, // PV is Released, not Bound
		},
	}

	_, err := fakeClient.CoreV1().PersistentVolumes().Create(ctx, releasedPV, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Create VolumeAttachment for the released PV (attachment may still exist briefly)
	pvName := "pvc-released"
	va := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: "va-released"},
		Spec: storagev1.VolumeAttachmentSpec{
			Attacher: constants.PluginName,
			NodeName: "node-1",
			Source:   storagev1.VolumeAttachmentSource{PersistentVolumeName: &pvName},
		},
		Status: storagev1.VolumeAttachmentStatus{Attached: true},
	}
	_, err = fakeClient.StorageV1().VolumeAttachments().Create(ctx, va, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Should return false because Released PVs are ignored (even if VolumeAttachment exists)
	nodeID := "node-1=#=#=node-1.example.com=#=#=10.0.0.1"
	hasOther, err := svc.hasOtherDirectoryBackedVolumesOnExport(ctx, nodeID, 100, "System", "pvc-current")
	assert.NoError(t, err)
	assert.False(t, hasOther, "Released PVs should not be counted as active volumes")
}

func TestHasOtherDirectoryBackedVolumesOnExport_RWXVolumes(t *testing.T) {
	// This test verifies that RWX volumes (which have no NodeAffinity) are correctly
	// detected using VolumeAttachments instead of NodeAffinity
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	svc := &service{k8sclient: fakeClient}

	// Create two RWX directory-backed PVs (no NodeAffinity set - this is the key difference)
	pv1 := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-rwx-1"},
		Spec: corev1.PersistentVolumeSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:           constants.PluginName,
					VolumeHandle:     "pvc-rwx-1=_=_=100=_=_=System=_=_=cluster1=_=_=directory",
					VolumeAttributes: map[string]string{"ProvisioningMode": "directory"},
				},
			},
			// No NodeAffinity - this is typical for RWX volumes
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
	}
	pv2 := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-rwx-2"},
		Spec: corev1.PersistentVolumeSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:           constants.PluginName,
					VolumeHandle:     "pvc-rwx-2=_=_=100=_=_=System=_=_=cluster1=_=_=directory",
					VolumeAttributes: map[string]string{"ProvisioningMode": "directory"},
				},
			},
			// No NodeAffinity - this is typical for RWX volumes
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
	}

	_, err := fakeClient.CoreV1().PersistentVolumes().Create(ctx, pv1, metav1.CreateOptions{})
	assert.NoError(t, err)
	_, err = fakeClient.CoreV1().PersistentVolumes().Create(ctx, pv2, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Create VolumeAttachments for both RWX PVs on node-1
	pvName1 := "pvc-rwx-1"
	pvName2 := "pvc-rwx-2"
	va1 := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: "va-rwx-1"},
		Spec: storagev1.VolumeAttachmentSpec{
			Attacher: constants.PluginName,
			NodeName: "node-1",
			Source:   storagev1.VolumeAttachmentSource{PersistentVolumeName: &pvName1},
		},
		Status: storagev1.VolumeAttachmentStatus{Attached: true},
	}
	va2 := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: "va-rwx-2"},
		Spec: storagev1.VolumeAttachmentSpec{
			Attacher: constants.PluginName,
			NodeName: "node-1",
			Source:   storagev1.VolumeAttachmentSource{PersistentVolumeName: &pvName2},
		},
		Status: storagev1.VolumeAttachmentStatus{Attached: true},
	}
	_, err = fakeClient.StorageV1().VolumeAttachments().Create(ctx, va1, metav1.CreateOptions{})
	assert.NoError(t, err)
	_, err = fakeClient.StorageV1().VolumeAttachments().Create(ctx, va2, metav1.CreateOptions{})
	assert.NoError(t, err)

	// When unpublishing pvc-rwx-1, should detect pvc-rwx-2 as another volume on the same export
	// This would have FAILED with the old NodeAffinity-based implementation
	nodeID := "node-1=#=#=node-1.example.com=#=#=10.0.0.1"
	hasOther, err := svc.hasOtherDirectoryBackedVolumesOnExport(ctx, nodeID, 100, "System", "pvc-rwx-1")
	assert.NoError(t, err)
	assert.True(t, hasOther, "RWX volumes should be detected via VolumeAttachments even without NodeAffinity")
}

func TestControllerUnpublishVolume_DirectoryBacked_HasOtherVolumes(t *testing.T) {
	ctx := context.Background()

	dirpv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "dirpv"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:           constants.PluginName,
					VolumeHandle:     "dirpv=_=_=19=_=_=System=_=_=system=_=_=directory",
					VolumeAttributes: map[string]string{"ProvisioningMode": "directory"},
				},
			},
		},
	}
	dirpv2 := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "dirpv2"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:           constants.PluginName,
					VolumeHandle:     "dirpv2=_=_=19=_=_=System=_=_=system=_=_=directory",
					VolumeAttributes: map[string]string{"ProvisioningMode": "directory"},
				},
			},
			NodeAffinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{MatchExpressions: []corev1.NodeSelectorRequirement{
							{Key: "kubernetes.io/hostname", Operator: corev1.NodeSelectorOpIn, Values: []string{"node-1"}},
						}},
					},
				},
			},
		},
		Status: corev1.PersistentVolumeStatus{
			Phase: corev1.VolumeBound,
		},
	}

	mockClient := &isimocks.Client{}
	isiConfig := &IsilonClusterConfig{
		ClusterName: "system",
		isiSvc: &isiService{
			client: &isi.Client{API: mockClient},
		},
	}
	s := &service{
		k8sclient:             fake.NewSimpleClientset(dirpv, dirpv2),
		defaultIsiClusterName: "system",
		isiClusters:           &sync.Map{},
		directoryExportMu:     sync.Map{},
	}
	s.isiClusters.Store("system", isiConfig)

	req := &csi.ControllerUnpublishVolumeRequest{
		VolumeId: "dirpv=_=_=19=_=_=System",
		NodeId:   "node-1",
	}

	resp, err := s.ControllerUnpublishVolume(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestControllerUnpublishVolume_DirectoryBacked_CheckError(t *testing.T) {
	ctx := context.Background()

	dirpv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "dirpv-err"},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:           constants.PluginName,
					VolumeHandle:     "dirpv-err=_=_=19=_=_=System=_=_=system=_=_=directory",
					VolumeAttributes: map[string]string{"ProvisioningMode": "directory"},
				},
			},
		},
	}

	mockClient := &isimocks.Client{}
	isiConfig := &IsilonClusterConfig{
		ClusterName: "system",
		isiSvc: &isiService{
			client: &isi.Client{API: mockClient},
		},
	}

	fakeClient := fake.NewSimpleClientset(dirpv)
	fakeClient.Fake.PrependReactor("list", "persistentvolumes", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("simulated list error")
	})

	svcWithListErr := &service{
		k8sclient:             fakeClient,
		defaultIsiClusterName: "system",
		isiClusters:           &sync.Map{},
		directoryExportMu:     sync.Map{},
	}
	svcWithListErr.isiClusters.Store("system", isiConfig)

	req := &csi.ControllerUnpublishVolumeRequest{
		VolumeId: "dirpv-err=_=_=19=_=_=System",
		NodeId:   "node-1",
	}

	resp, err := svcWithListErr.ControllerUnpublishVolume(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestLabelDirectoryBackedPV(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		pvName       string
		exportID     int
		accessZone   string
		exportPath   string
		setupClient  func() *fake.Clientset
		expectPatch  bool
		expectLabels map[string]string
		expectAnnots map[string]string
	}{
		{
			name:       "successful labeling of new PV",
			pvName:     "pvc-test-123",
			exportID:   100,
			accessZone: "System",
			exportPath: "/ifs/shared",
			setupClient: func() *fake.Clientset {
				pv := &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pvc-test-123",
					},
				}
				return fake.NewSimpleClientset(pv)
			},
			expectPatch: true,
			expectLabels: map[string]string{
				"powerscale.csi.dell.com/provisioning-mode": "directory",
				"powerscale.csi.dell.com/shared-export-id":  "100",
			},
			expectAnnots: map[string]string{
				"powerscale.csi.dell.com/access-zone":        "System",
				"powerscale.csi.dell.com/shared-export-path": "/ifs/shared",
			},
		},
		{
			name:       "idempotent - PV already labeled",
			pvName:     "pvc-already-labeled",
			exportID:   200,
			accessZone: "zone1",
			exportPath: "/ifs/zone1/shared",
			setupClient: func() *fake.Clientset {
				pv := &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pvc-already-labeled",
						Labels: map[string]string{
							"powerscale.csi.dell.com/provisioning-mode": "directory",
							"powerscale.csi.dell.com/shared-export-id":  "200",
						},
						Annotations: map[string]string{
							"powerscale.csi.dell.com/access-zone":        "zone1",
							"powerscale.csi.dell.com/shared-export-path": "/ifs/zone1/shared",
						},
					},
				}
				return fake.NewSimpleClientset(pv)
			},
			expectPatch: true, // Patch is still called but is idempotent
			expectLabels: map[string]string{
				"powerscale.csi.dell.com/provisioning-mode": "directory",
				"powerscale.csi.dell.com/shared-export-id":  "200",
			},
			expectAnnots: map[string]string{
				"powerscale.csi.dell.com/access-zone":        "zone1",
				"powerscale.csi.dell.com/shared-export-path": "/ifs/zone1/shared",
			},
		},
		{
			name:       "PV not found - logs warning but does not panic",
			pvName:     "pvc-nonexistent",
			exportID:   300,
			accessZone: "System",
			exportPath: "/ifs/shared",
			setupClient: func() *fake.Clientset {
				return fake.NewSimpleClientset() // Empty clientset, no PV
			},
			expectPatch: false,
		},
		{
			name:       "different export IDs",
			pvName:     "pvc-export-999",
			exportID:   999,
			accessZone: "CustomZone",
			exportPath: "/ifs/custom/shared",
			setupClient: func() *fake.Clientset {
				pv := &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pvc-export-999",
					},
				}
				return fake.NewSimpleClientset(pv)
			},
			expectPatch: true,
			expectLabels: map[string]string{
				"powerscale.csi.dell.com/provisioning-mode": "directory",
				"powerscale.csi.dell.com/shared-export-id":  "999",
			},
			expectAnnots: map[string]string{
				"powerscale.csi.dell.com/access-zone":        "CustomZone",
				"powerscale.csi.dell.com/shared-export-path": "/ifs/custom/shared",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := tt.setupClient()

			// Track if patch was called
			patchCalled := false
			fakeClient.Fake.PrependReactor("patch", "persistentvolumes", func(action k8stesting.Action) (bool, runtime.Object, error) {
				patchCalled = true
				patchAction := action.(k8stesting.PatchAction)

				// Verify patch is for the correct PV
				assert.Equal(t, tt.pvName, patchAction.GetName())

				// Return the patched PV
				pv := &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name:        tt.pvName,
						Labels:      tt.expectLabels,
						Annotations: tt.expectAnnots,
					},
				}
				return true, pv, nil
			})

			svc := &service{
				k8sclient: fakeClient,
			}

			// Call the function - should not panic
			svc.labelDirectoryBackedPV(ctx, tt.pvName, tt.exportID, tt.accessZone, tt.exportPath)

			if tt.expectPatch {
				assert.True(t, patchCalled, "Expected patch to be called")
			}
		})
	}
}

func TestLabelDirectoryBackedPV_NilK8sClient(_ *testing.T) {
	ctx := context.Background()

	svc := &service{
		k8sclient: nil, // No k8s client
	}

	// Should not panic when k8sclient is nil
	svc.labelDirectoryBackedPV(ctx, "pvc-test", 100, "System", "/ifs/shared")
}

func TestLabelDirectoryBackedPV_PatchError(_ *testing.T) {
	ctx := context.Background()

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pvc-patch-error",
		},
	}
	fakeClient := fake.NewSimpleClientset(pv)

	// Simulate patch error
	fakeClient.Fake.PrependReactor("patch", "persistentvolumes", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("simulated patch error")
	})

	svc := &service{
		k8sclient: fakeClient,
	}

	// Should not panic on patch error, just log warning
	svc.labelDirectoryBackedPV(ctx, "pvc-patch-error", 100, "System", "/ifs/shared")
}

func TestLabelDirectoryBackedPV_MarshalError(_ *testing.T) {
	ctx := context.Background()

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pvc-marshal-error",
		},
	}
	fakeClient := fake.NewSimpleClientset(pv)

	// Mock jsonMarshalFunc to return an error
	oldJSONMarshalFunc := jsonMarshalFunc
	defer func() { jsonMarshalFunc = oldJSONMarshalFunc }()
	jsonMarshalFunc = func(_ interface{}) ([]byte, error) {
		return nil, errors.New("simulated marshal error")
	}

	svc := &service{
		k8sclient: fakeClient,
	}

	// Should not panic on marshal error, just log warning and return
	svc.labelDirectoryBackedPV(ctx, "pvc-marshal-error", 100, "System", "/ifs/shared")
}

func TestLabelDirectoryBackedPV_VerifyPatchContent(t *testing.T) {
	ctx := context.Background()

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pvc-verify-patch",
		},
	}
	fakeClient := fake.NewSimpleClientset(pv)

	var capturedPatch []byte
	fakeClient.Fake.PrependReactor("patch", "persistentvolumes", func(action k8stesting.Action) (bool, runtime.Object, error) {
		patchAction := action.(k8stesting.PatchAction)
		capturedPatch = patchAction.GetPatch()
		return true, pv, nil
	})

	svc := &service{
		k8sclient: fakeClient,
	}

	svc.labelDirectoryBackedPV(ctx, "pvc-verify-patch", 42, "TestZone", "/ifs/test/shared")

	// Verify patch content
	assert.NotEmpty(t, capturedPatch)

	// Parse the patch JSON
	var patchData map[string]interface{}
	err := json.Unmarshal(capturedPatch, &patchData)
	assert.NoError(t, err)

	// Verify structure
	metadata, ok := patchData["metadata"].(map[string]interface{})
	assert.True(t, ok, "Expected metadata in patch")

	labels, ok := metadata["labels"].(map[string]interface{})
	assert.True(t, ok, "Expected labels in metadata")
	assert.Equal(t, "directory", labels["powerscale.csi.dell.com/provisioning-mode"])
	assert.Equal(t, "42", labels["powerscale.csi.dell.com/shared-export-id"])

	annotations, ok := metadata["annotations"].(map[string]interface{})
	assert.True(t, ok, "Expected annotations in metadata")
	assert.Equal(t, "TestZone", annotations["powerscale.csi.dell.com/access-zone"])
	assert.Equal(t, "/ifs/test/shared", annotations["powerscale.csi.dell.com/shared-export-path"])
}

// TestControllerPublishVolume_DirectoryBacked_SkipsOtherClientsCheck verifies that
// directory-backed volumes do NOT call OtherClientsAlreadyAdded, since shared exports
// are expected to have multiple clients. This test protects against regressions where
// the !isDirectoryBacked guard is accidentally removed.
func TestControllerPublishVolume_DirectoryBacked_SkipsOtherClientsCheck(t *testing.T) {
	fmt.Println("TestControllerPublishVolume_DirectoryBacked_SkipsOtherClientsCheck")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	s := &service{
		nodeID:                identifiers.DummyHostNodeID,
		nodeIP:                "192.168.1.10",
		defaultIsiClusterName: "system",
		opts:                  Opts{},
		isiClusters:           &sync.Map{},
		directoryExportMu:     sync.Map{},
	}

	mockClient := &isimocks.Client{}
	isiConfig := &IsilonClusterConfig{
		ClusterName:             "system",
		IgnoreUnresolvableHosts: new(bool),
		isiSvc: &isiService{
			client: &isi.Client{
				API: mockClient,
			},
		},
	}
	s.isiClusters.Store("system", isiConfig)

	ctx := context.Background()
	// Directory-backed volume ID (note the 5th token "directory")
	req := &csi.ControllerPublishVolumeRequest{
		VolumeId: "test-vol=_=_=100=_=_=System=_=_=system=_=_=directory",
		NodeId:   identifiers.DummyHostNodeID,
		VolumeCapability: &csi.VolumeCapability{
			AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
			AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
		},
		VolumeContext: map[string]string{
			"ProvisioningMode": "directory",
		},
	}

	// Mock GetExportByIDWithZone to return a valid export
	mockClient.On("Get", anyArgs[0:6]...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*v2.ExportList)
		*resp = v2.ExportList{
			&v2.Export{
				ID:      100,
				Paths:   &[]string{"/ifs/data/shared"},
				Clients: &[]string{"192.168.1.5"}, // Another client already exists
			},
		}
	}).Once()

	// Mock GetExportsCountAttachedToNode (v1) — return one export attached to the node IP
	mockClient.On("Get", anyArgs[0:6]...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**v1.GetIsiExportsResp)
		*resp = &v1.GetIsiExportsResp{
			ExportList: []*v1.IsiExport{
				{ID: 100, Clients: []string{"192.168.1.5"}},
			},
		}
	}).Once()

	// Mock IsHostAlreadyAdded for directory-backed check (line 1938) — node not yet authorized
	mockClient.On("Get", anyArgs[0:6]...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*v2.ExportList)
		empty := []string{}
		*resp = v2.ExportList{
			&v2.Export{
				ID:               100,
				Clients:          &[]string{"192.168.1.5"}, // Node IP not in list
				ReadOnlyClients:  &empty,
				ReadWriteClients: &empty,
				RootClients:      &empty,
			},
		}
	}).Once()

	// Mock IsHostAlreadyAdded for SINGLE_NODE_WRITER case (line 2013) — node now authorized,
	// so the normal-client add is skipped and only the final addClientFunc Put is issued.
	mockClient.On("Get", anyArgs[0:6]...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*v2.ExportList)
		empty := []string{}
		*resp = v2.ExportList{
			&v2.Export{
				ID:               100,
				Clients:          &[]string{"192.168.1.5", "localhost"},
				ReadOnlyClients:  &empty,
				ReadWriteClients: &empty,
				RootClients:      &empty,
			},
		}
	}).Once()

	// Mock GetExportByIDWithZone inside AddExportClientByIDWithZone
	mockClient.On("Get", anyArgs[0:6]...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*v2.ExportList)
		empty := []string{}
		*resp = v2.ExportList{
			&v2.Export{
				ID:               100,
				Zone:             "System",
				Clients:          &[]string{"192.168.1.5"},
				ReadOnlyClients:  &empty,
				ReadWriteClients: &empty,
				RootClients:      &empty,
			},
		}
	}).Once()

	// Mock AddExportClientsByIDWithZone (the actual authorization call)
	mockClient.On("Put", anyArgs[0:7]...).Return(nil).Once()

	// The critical assertion: OtherClientsAlreadyAdded should NOT be called for directory-backed.
	// If it were called and returned true, the publish would abort incorrectly.
	// We do NOT mock OtherClientsAlreadyAdded here; if the code calls it, the test will fail
	// because the mock client has no matching expectation.

	resp, err := s.ControllerPublishVolume(ctx, req)
	assert.Nil(t, err, "ControllerPublishVolume should succeed for directory-backed volume with other clients")
	assert.NotNil(t, resp, "Response should not be nil")

	mockClient.AssertExpectations(t)
}

func TestHasOtherDirectoryBackedVolumesOnExport_ListAttachmentsError(t *testing.T) {
	ctx := context.Background()

	nodeID := "test-node=#=#=test-node.example.com=#=#=10.0.0.1"

	// Mock k8sListVolumeAttachmentsFunc to return an error
	oldK8sListVolumeAttachmentsFunc := k8sListVolumeAttachmentsFunc
	defer func() { k8sListVolumeAttachmentsFunc = oldK8sListVolumeAttachmentsFunc }()
	k8sListVolumeAttachmentsFunc = func(_ context.Context, _ kubernetes.Interface, _ metav1.ListOptions) (*storagev1.VolumeAttachmentList, error) {
		return nil, errors.New("simulated list error")
	}

	fakeClient := fake.NewSimpleClientset()

	svc := &service{
		k8sclient: fakeClient,
	}

	// Should return error when listing attachments fails
	_, err := svc.hasOtherDirectoryBackedVolumesOnExport(ctx, nodeID, 100, "System", "pv-exclude")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list VolumeAttachments")
}

func TestHasOtherDirectoryBackedVolumesOnExport_GetPVError(t *testing.T) {
	ctx := context.Background()

	nodeID := "test-node=#=#=test-node.example.com=#=#=10.0.0.1"

	// Create a volume attachment
	attachment := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "va-test",
		},
		Spec: storagev1.VolumeAttachmentSpec{
			NodeName: "test-node",
			Source: storagev1.VolumeAttachmentSource{
				PersistentVolumeName: &[]string{"pv-test"}[0],
			},
		},
		Status: storagev1.VolumeAttachmentStatus{
			Attached: true,
		},
	}

	fakeClient := fake.NewSimpleClientset(attachment)

	// Mock k8sGetPersistentVolumeFunc to return an error
	oldK8sGetPersistentVolumeFunc := k8sGetPersistentVolumeFunc
	defer func() { k8sGetPersistentVolumeFunc = oldK8sGetPersistentVolumeFunc }()
	k8sGetPersistentVolumeFunc = func(_ context.Context, _ kubernetes.Interface, _ string, _ metav1.GetOptions) (*corev1.PersistentVolume, error) {
		return nil, errors.New("simulated get error")
	}

	svc := &service{
		k8sclient: fakeClient,
	}

	// Should handle PV get error gracefully and continue
	hasOther, err := svc.hasOtherDirectoryBackedVolumesOnExport(ctx, nodeID, 100, "System", "pv-exclude")
	assert.NoError(t, err)
	assert.False(t, hasOther)
}

// TestExportHasDummyHostClient tests the exportHasDummyHostClient function
func TestExportHasDummyHostClient(t *testing.T) {
	ctx := context.Background()

	// DummyHostNodeID is "localhost=#=#=localhost=#=#=127.0.0.1"
	// which parses to: name="localhost", fqdn="localhost", ip="127.0.0.1"
	dummyName := "localhost"
	dummyIP := "127.0.0.1"

	tests := []struct {
		name     string
		export   isi.Export
		expected bool
	}{
		{
			name:     "nil export",
			export:   nil,
			expected: false,
		},
		{
			name: "export with no clients",
			export: &v2.Export{
				ID: 1,
			},
			expected: false,
		},
		{
			name: "export with dummy host name in Clients",
			export: &v2.Export{
				ID:      1,
				Clients: &[]string{dummyName, "other-client"},
			},
			expected: true,
		},
		{
			name: "export with dummy host name in ReadOnlyClients",
			export: &v2.Export{
				ID:              1,
				ReadOnlyClients: &[]string{dummyName},
			},
			expected: true,
		},
		{
			name: "export with dummy host name in ReadWriteClients",
			export: &v2.Export{
				ID:               1,
				ReadWriteClients: &[]string{"other", dummyName},
			},
			expected: true,
		},
		{
			name: "export with dummy host name in RootClients",
			export: &v2.Export{
				ID:          1,
				RootClients: &[]string{dummyName},
			},
			expected: true,
		},
		{
			name: "export with dummy host IP in Clients",
			export: &v2.Export{
				ID:      1,
				Clients: &[]string{dummyIP},
			},
			expected: true,
		},
		{
			name: "export with dummy host IP in RootClients",
			export: &v2.Export{
				ID:          1,
				RootClients: &[]string{"other", dummyIP},
			},
			expected: true,
		},
		{
			name: "export without dummy host",
			export: &v2.Export{
				ID:               1,
				Clients:          &[]string{"client1", "client2"},
				ReadOnlyClients:  &[]string{"ro-client"},
				ReadWriteClients: &[]string{"rw-client"},
				RootClients:      &[]string{"root-client"},
			},
			expected: false,
		},
		{
			name: "export with empty client lists",
			export: &v2.Export{
				ID:               1,
				Clients:          &[]string{},
				ReadOnlyClients:  &[]string{},
				ReadWriteClients: &[]string{},
				RootClients:      &[]string{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exportHasDummyHostClient(ctx, tt.export)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsDependencyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "JSON error with 409 status code",
			err:      &isiapi.JSONError{StatusCode: 409, Err: []isiapi.Error{{Code: "conflict", Message: "resource conflict"}}},
			expected: true,
		},
		{
			name:     "JSON error with 404 status code",
			err:      &isiapi.JSONError{StatusCode: 404, Err: []isiapi.Error{{Code: "not_found", Message: "resource not found"}}},
			expected: false,
		},
		{
			name:     "error with has dependent message",
			err:      errors.New("snapshot has dependent volumes"),
			expected: true,
		},
		{
			name:     "error with dependency message",
			err:      errors.New("cannot delete due to dependency"),
			expected: true,
		},
		{
			name:     "error with conflict and snapshot message",
			err:      errors.New("conflict: snapshot is in use"),
			expected: true,
		},
		{
			name:     "error with conflict and writable message",
			err:      errors.New("conflict: writable snapshot exists"),
			expected: true,
		},
		{
			name:     "error with conflict but no snapshot/writable",
			err:      errors.New("conflict: resource busy"),
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("some other error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDependencyError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// newTestServiceWithCluster creates a minimal *service with a single named cluster for
// unit tests that need getIsilonConfig to succeed but do not require full API mocking.
func newTestServiceWithCluster(clusterName string, opts Opts) *service {
	mockClient := &isimocks.Client{}
	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	isiConfig := &IsilonClusterConfig{
		ClusterName: clusterName,
		Endpoint:    "http://testendpoint:8080",
		isiSvc: &isiService{
			client: &isi.Client{
				API: mockClient,
			},
		},
	}
	svc := &service{
		k8sclient:             fake.NewSimpleClientset(),
		defaultIsiClusterName: clusterName,
		isiClusters:           &sync.Map{},
		opts:                  opts,
	}
	svc.isiClusters.Store(clusterName, isiConfig)
	return svc
}

// TestCreateVolume_InvalidMutableParams covers the validateMutableParamKeys error return
// path in CreateVolume (controller.go:292-294) — fired when MutableParameters contains
// an unsupported key.  This path is reached before getIsilonConfig, so no cluster setup
// is needed.
func TestCreateVolume_InvalidMutableParams(t *testing.T) {
	svc := &service{
		defaultIsiClusterName: "system",
		isiClusters:           &sync.Map{},
	}
	req := &csi.CreateVolumeRequest{
		Name:       "test-volume",
		Parameters: map[string]string{},
		MutableParameters: map[string]string{
			"UnsupportedMutableKey": "value",
		},
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
				AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
			},
		},
	}
	_, err := svc.CreateVolume(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported mutable parameter")
}

// TestCreateVolume_EmptyIsiVolumePathPermissions covers the branch in CreateVolume
// (controller.go:379-381) where IsiVolumePathPermissions is present but empty, causing
// the cluster default to be used.  The test is expected to fail at a later stage
// (remote system lookup) before any real isiSvc operation is needed.
func TestCreateVolume_EmptyIsiVolumePathPermissions(t *testing.T) {
	svc := newTestServiceWithCluster("system", Opts{
		CustomTopologyEnabled: true,
		replicationPrefix:     "UT",
	})
	req := &csi.CreateVolumeRequest{
		Name: "test-volume",
		Parameters: map[string]string{
			IsiVolumePathPermissionsParam: "", // empty → uses cluster default (line 380)
			"UT/isReplicationEnabled":     "true",
			"UT/volumeGroupPrefix":        "UT",
			"UT/rpo":                      "Five_Minutes",
			"UT/remoteSystem":             "nonexistent-remote",
		},
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
				AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
			},
		},
		CapacityRange: &csi.CapacityRange{RequiredBytes: 1024 * 1024 * 1024},
	}
	_, err := svc.CreateVolume(context.Background(), req)
	// Fails at remote system lookup, but line 380 is covered before that.
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-remote")
}

// TestCreateVolume_InvalidReplicationFlagWithRelativePath covers two uncovered paths in
// one call: (1) the warning log when the replication flag is an invalid boolean
// (controller.go:392-394), and (2) the non-absolute SharedExportPath validation error
// (controller.go:451-455).
func TestCreateVolume_InvalidReplicationFlagWithRelativePath(t *testing.T) {
	svc := newTestServiceWithCluster("system", Opts{
		CustomTopologyEnabled: true,
		replicationPrefix:     "UT",
	})
	req := &csi.CreateVolumeRequest{
		Name: "test-volume",
		Parameters: map[string]string{
			"UT/isReplicationEnabled":       "notabool", // invalid bool → line 393
			constants.DirectoryBackedParam:  "true",
			constants.SharedExportPathParam: "relative/path", // non-absolute → line 451
		},
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
				AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
			},
		},
		CapacityRange: &csi.CapacityRange{RequiredBytes: 1024 * 1024 * 1024},
	}
	_, err := svc.CreateVolume(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be an absolute path")
}

// TestCreateVolume_EmptyAzServiceIPWithPathTraversal covers two uncovered paths: (1) the
// branch where AzServiceIPParam is present but empty and CustomTopologyEnabled is false,
// so the cluster endpoint is used as azServiceIP (controller.go:406-409), and (2) the
// path-traversal validation error for SharedExportPath (controller.go:456-460).
func TestCreateVolume_EmptyAzServiceIPWithPathTraversal(t *testing.T) {
	svc := newTestServiceWithCluster("system", Opts{
		CustomTopologyEnabled: false, // must be false so AzServiceIPParam branch is entered
		replicationPrefix:     "UT",
	})
	req := &csi.CreateVolumeRequest{
		Name: "test-volume",
		Parameters: map[string]string{
			AzServiceIPParam:                "", // empty → line 408
			constants.DirectoryBackedParam:  "true",
			constants.SharedExportPathParam: "/ifs/../etc/passwd", // traversal → line 456
		},
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
				AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
			},
		},
		CapacityRange: &csi.CapacityRange{RequiredBytes: 1024 * 1024 * 1024},
	}
	_, err := svc.CreateVolume(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal sequences")
}
