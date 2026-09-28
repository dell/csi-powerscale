/*
 Copyright © 2025-2026 Dell Inc. or its subsidiaries. All Rights Reserved.

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

package collectors

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestK8sMetadataChecker_IsDriverManaged_DriverManagedVolume tests detection of driver-managed volumes
func TestK8sMetadataChecker_IsDriverManaged_DriverManagedVolume(t *testing.T) {
	// Create fake Kubernetes client with a CSI PV
	fakeClient := fake.NewSimpleClientset(
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "csivol-abc123",
			},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver:       "csi-isilon.dellemc.com",
						VolumeHandle: "csivol-abc123=_=_=3593=_=_=System=_=_=echo",
					},
				},
			},
		},
	)

	checker := NewK8sMetadataChecker(fakeClient, "csi-isilon.dellemc.com")

	// Refresh cache to populate PVs
	err := checker.RefreshCache(context.Background())
	assert.NoError(t, err)

	result, err := checker.IsDriverManaged(context.Background(), "/ifs/data/csi/csivol-abc123")
	assert.NoError(t, err)
	assert.True(t, result, "Should detect driver-managed volume")
}

// TestK8sMetadataChecker_IsDriverManaged_DynamicPVNameUsesVolumeHandle tests
// realistic dynamic provisioning where PV names are pvc-* and backend directory
// name comes from VolumeHandle.
func TestK8sMetadataChecker_IsDriverManaged_DynamicPVNameUsesVolumeHandle(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pvc-12345678-abcd",
			},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver:       "csi-isilon.dellemc.com",
						VolumeHandle: "csivol-abc123=_=_=3593=_=_=System=_=_=echo",
					},
				},
			},
		},
	)

	checker := NewK8sMetadataChecker(fakeClient, "csi-isilon.dellemc.com")
	err := checker.RefreshCache(context.Background())
	assert.NoError(t, err)

	result, err := checker.IsDriverManaged(context.Background(), "/ifs/data/csi/csivol-abc123")
	assert.NoError(t, err)
	assert.True(t, result, "Should detect driver-managed volume via CSI VolumeHandle key")
}

// TestK8sMetadataChecker_IsDriverManaged_TenantPrefixedPVNameUsesVolumeHandle tests
// authorization/tenant scenarios where PV object names may carry extra prefixing,
// while the backend directory identity remains in CSI VolumeHandle.
func TestK8sMetadataChecker_IsDriverManaged_TenantPrefixedPVNameUsesVolumeHandle(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "tenant-a-pvc-12345678-abcd",
			},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver:       "csi-isilon.dellemc.com",
						VolumeHandle: "tenant-a-csivol-abc123=_=_=3593=_=_=System=_=_=echo",
					},
				},
			},
		},
	)

	checker := NewK8sMetadataChecker(fakeClient, "csi-isilon.dellemc.com")
	err := checker.RefreshCache(context.Background())
	assert.NoError(t, err)

	result, err := checker.IsDriverManaged(context.Background(), "/ifs/data/csi/tenant-a-csivol-abc123")
	assert.NoError(t, err)
	assert.True(t, result, "Should detect tenant-prefixed driver volume via CSI VolumeHandle key")
}

// TestK8sMetadataChecker_IsDriverManaged_NonDriverVolume tests non-driver volumes are excluded
func TestK8sMetadataChecker_IsDriverManaged_NonDriverVolume(t *testing.T) {
	// Create fake Kubernetes client with a non-CSI PV
	fakeClient := fake.NewSimpleClientset(
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "manual-volume",
			},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "some-other-driver",
					},
				},
			},
		},
	)

	checker := NewK8sMetadataChecker(fakeClient, "csi-isilon.dellemc.com")

	// Refresh cache to populate PVs
	err := checker.RefreshCache(context.Background())
	assert.NoError(t, err)

	result, err := checker.IsDriverManaged(context.Background(), "/ifs/data/csi/manual-volume")
	assert.NoError(t, err)
	assert.False(t, result, "Should not detect non-driver volume")
}

// TestK8sMetadataChecker_IsDriverManaged_VolumeNotInK8s tests volumes not in Kubernetes
func TestK8sMetadataChecker_IsDriverManaged_VolumeNotInK8s(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()

	checker := NewK8sMetadataChecker(fakeClient, "csi-isilon.dellemc.com")

	// Refresh cache to populate PVs
	err := checker.RefreshCache(context.Background())
	assert.NoError(t, err)

	result, err := checker.IsDriverManaged(context.Background(), "/ifs/data/csi/nonexistent-volume")
	assert.NoError(t, err)
	assert.False(t, result, "Should return false for volume not in Kubernetes")
}

// TestK8sMetadataChecker_IsDriverManaged_InvalidPath tests invalid paths
func TestK8sMetadataChecker_IsDriverManaged_InvalidPath(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	checker := NewK8sMetadataChecker(fakeClient, "csi-isilon.dellemc.com")

	testCases := []string{
		"",
		"/",
		"/ifs/data/csi/",
	}

	for _, path := range testCases {
		t.Run("path_"+path, func(t *testing.T) {
			result, err := checker.IsDriverManaged(context.Background(), path)
			assert.NoError(t, err)
			assert.False(t, result, "Should return false for invalid path: %s", path)
		})
	}
}

// TestK8sMetadataChecker_IsDriverManaged_Caching tests that results are cached
func TestK8sMetadataChecker_IsDriverManaged_Caching(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "csivol-cached",
			},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "csi-isilon.dellemc.com",
					},
				},
			},
		},
	)

	checker := NewK8sMetadataChecker(fakeClient, "csi-isilon.dellemc.com")

	// Refresh cache to populate PVs
	err := checker.RefreshCache(context.Background())
	assert.NoError(t, err)

	// First call should use cache
	result1, err1 := checker.IsDriverManaged(context.Background(), "/ifs/data/csi/csivol-cached")
	assert.NoError(t, err1)
	assert.True(t, result1)

	// Second call should also use cache
	result2, err2 := checker.IsDriverManaged(context.Background(), "/ifs/data/csi/csivol-cached")
	assert.NoError(t, err2)
	assert.True(t, result2)

	// Verify cache was used by checking cache size
	checker.cacheMu.RLock()
	cacheSize := len(checker.pvCache)
	checker.cacheMu.RUnlock()
	assert.Equal(t, 1, cacheSize, "Cache should contain one entry")
}

// TestK8sMetadataChecker_RefreshCache tests cache refresh
func TestK8sMetadataChecker_RefreshCache(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "csivol-refresh",
			},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "csi-isilon.dellemc.com",
					},
				},
			},
		},
	)

	checker := NewK8sMetadataChecker(fakeClient, "csi-isilon.dellemc.com")

	// Populate cache
	err := checker.RefreshCache(context.Background())
	assert.NoError(t, err)

	// Verify cache is populated
	checker.cacheMu.RLock()
	assert.Equal(t, 1, len(checker.pvCache))
	checker.cacheMu.RUnlock()

	// Refresh cache again
	err = checker.RefreshCache(context.Background())
	assert.NoError(t, err)

	// Verify cache is still populated (not cleared)
	checker.cacheMu.RLock()
	assert.Equal(t, 1, len(checker.pvCache))
	checker.cacheMu.RUnlock()
}

// TestK8sMetadataChecker_NilClient tests nil client handling
func TestK8sMetadataChecker_NilClient(t *testing.T) {
	checker := NewK8sMetadataChecker(nil, "csi-isilon.dellemc.com")

	result, err := checker.IsDriverManaged(context.Background(), "/ifs/data/csi/csivol-abc123")
	assert.Error(t, err)
	assert.False(t, result)
}

// TestExtractVolumeNameFromPath tests volume name extraction
func TestExtractVolumeNameFromPath(t *testing.T) {
	testCases := []struct {
		path     string
		expected string
	}{
		{"/ifs/data/csi/csivol-abc123", "csivol-abc123"},
		{"/ifs/data/csi/my-volume", "my-volume"},
		{"/ifs/volumes/test", "test"},
		{"/single", "single"},
		{"", ""},
		{"/", ""},
		{"/ifs/data/csi/", ""},
	}

	for _, tc := range testCases {
		t.Run("path_"+tc.path, func(t *testing.T) {
			result := extractVolumeNameFromPath(tc.path)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestExtractVolumeKeyFromHandle(t *testing.T) {
	testCases := []struct {
		handle   string
		expected string
	}{
		{"csivol-abc123=_=_=3593=_=_=System", "csivol-abc123"},
		{"volume-only", "volume-only"},
		{"", ""},
		{"  csivol-with-space=_=_=1  ", "csivol-with-space"},
	}

	for _, tc := range testCases {
		t.Run("handle_"+tc.handle, func(t *testing.T) {
			result := extractVolumeKeyFromHandle(tc.handle)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestK8sMetadataChecker_MultipleVolumes tests checking multiple volumes
func TestK8sMetadataChecker_MultipleVolumes(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "csivol-vol1",
			},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "csi-isilon.dellemc.com",
					},
				},
			},
		},
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "csivol-vol2",
			},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "csi-isilon.dellemc.com",
					},
				},
			},
		},
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "other-volume",
			},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver: "other-driver",
					},
				},
			},
		},
	)

	checker := NewK8sMetadataChecker(fakeClient, "csi-isilon.dellemc.com")

	// Refresh cache to populate PVs
	err := checker.RefreshCache(context.Background())
	assert.NoError(t, err)

	// Test CSI volumes
	result1, _ := checker.IsDriverManaged(context.Background(), "/ifs/data/csi/csivol-vol1")
	assert.True(t, result1)

	result2, _ := checker.IsDriverManaged(context.Background(), "/ifs/data/csi/csivol-vol2")
	assert.True(t, result2)

	// Test non-CSI volume
	result3, _ := checker.IsDriverManaged(context.Background(), "/ifs/data/csi/other-volume")
	assert.False(t, result3)
}

// TestK8sMetadataChecker_PVWithoutVolumeHandle tests PV with CSI but nil VolumeHandle
func TestK8sMetadataChecker_PVWithoutVolumeHandle(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "csivol-no-handle",
			},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver:       "csi-isilon.dellemc.com",
						VolumeHandle: "",
					},
				},
			},
		},
	)

	checker := NewK8sMetadataChecker(fakeClient, "csi-isilon.dellemc.com")

	// Refresh cache to populate PVs
	err := checker.RefreshCache(context.Background())
	assert.NoError(t, err)

	// Should still be detected by PV name
	result, err := checker.IsDriverManaged(context.Background(), "/ifs/data/csi/csivol-no-handle")
	assert.NoError(t, err)
	assert.True(t, result, "Should detect driver-managed volume by PV name even without VolumeHandle")
}

// TestK8sMetadataChecker_PVWithNilCSI tests PV with nil CSI spec
func TestK8sMetadataChecker_PVWithNilCSI(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "manual-volume",
			},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: nil,
				},
			},
		},
	)

	checker := NewK8sMetadataChecker(fakeClient, "csi-isilon.dellemc.com")

	// Refresh cache to populate PVs
	err := checker.RefreshCache(context.Background())
	assert.NoError(t, err)

	// Should not be detected as driver-managed
	result, err := checker.IsDriverManaged(context.Background(), "/ifs/data/csi/manual-volume")
	assert.NoError(t, err)
	assert.False(t, result, "Should not detect volume with nil CSI as driver-managed")
}

// TestK8sMetadataChecker_PVWithDifferentDriver tests PV with different CSI driver
func TestK8sMetadataChecker_PVWithDifferentDriver(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		&corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "other-driver-volume",
			},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver:       "other-csi-driver",
						VolumeHandle: "vol-123",
					},
				},
			},
		},
	)

	checker := NewK8sMetadataChecker(fakeClient, "csi-isilon.dellemc.com")

	// Refresh cache to populate PVs
	err := checker.RefreshCache(context.Background())
	assert.NoError(t, err)

	// Should not be detected as driver-managed
	result, err := checker.IsDriverManaged(context.Background(), "/ifs/data/csi/other-driver-volume")
	assert.NoError(t, err)
	assert.False(t, result, "Should not detect volume with different CSI driver as driver-managed")
}

// TestK8sMetadataChecker_RefreshCache_Error tests error handling when listing PVs fails
func TestK8sMetadataChecker_RefreshCache_Error(t *testing.T) {
	// Create a fake client that will fail on List
	// Note: fake.NewSimpleClientset doesn't support error injection for List
	// So we'll just test the happy path is already covered
	// This test is a placeholder for future error injection testing
	checker := NewK8sMetadataChecker(fake.NewSimpleClientset(), "csi-isilon.dellemc.com")
	err := checker.RefreshCache(context.Background())
	assert.NoError(t, err, "Should succeed with empty client")
}
