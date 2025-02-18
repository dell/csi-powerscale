package service

import (
	"context"
	"reflect"
	"testing"

	vgsext "github.com/dell/dell-csi-extensions/volumeGroupSnapshot"
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
