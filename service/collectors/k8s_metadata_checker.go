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
	"fmt"
	"strings"
	"sync"

	id "github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/common/utils/identifiers"
	"github.com/Ecosystems/container-storage-modules/src/csmlog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// K8sMetadataChecker checks if a volume is managed by the CSI driver
// by querying Kubernetes PVs and PVCs
type K8sMetadataChecker struct {
	k8sClient  kubernetes.Interface
	driverName string
	// Cache of all PVs with their driver information
	pvCache map[string]bool // volume name -> is driver managed
	cacheMu sync.RWMutex
}

// NewK8sMetadataChecker creates a new K8sMetadataChecker
func NewK8sMetadataChecker(k8sClient kubernetes.Interface, driverName string) *K8sMetadataChecker {
	return &K8sMetadataChecker{
		k8sClient:  k8sClient,
		driverName: driverName,
		pvCache:    make(map[string]bool),
	}
}

// IsDriverManaged checks if a volume is managed by the CSI driver (implements VolumeValidator interface)
// by looking up the corresponding PV in Kubernetes
func (c *K8sMetadataChecker) IsDriverManaged(_ context.Context, path string) (bool, error) {
	if c.k8sClient == nil {
		return false, fmt.Errorf("K8sMetadataChecker: kubernetes client not initialized")
	}

	// Extract volume name from path (e.g., /ifs/data/csi/csivol-abc123 -> csivol-abc123)
	volumeName := extractVolumeNameFromPath(path)
	if volumeName == "" {
		return false, nil
	}

	// Check cache (should be populated by RefreshCache call before quota processing)
	c.cacheMu.RLock()
	cached, exists := c.pvCache[volumeName]
	c.cacheMu.RUnlock()

	if exists {
		return cached, nil
	}

	// If not in cache, it's not managed by our driver
	return false, nil
}

// fetchAndCachePVs fetches all PVs and populates the cache with driver-managed volumes
func (c *K8sMetadataChecker) fetchAndCachePVs(ctx context.Context) error {
	// Fetch all PVs (field selector for spec.csi.driver is not supported in Kubernetes)
	pvList, err := c.k8sClient.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("K8sMetadataChecker: failed to list PVs: %w", err)
	}

	csmlog.Debugf("K8sMetadataChecker: fetched %d PVs from Kubernetes", len(pvList.Items))

	// Build new cache with only driver-managed PVs
	newCache := make(map[string]bool)
	for _, pv := range pvList.Items {
		if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == c.driverName {
			// Keep PV name for backwards compatibility with existing tests and
			// static/manual workflows where path tail may equal PV name.
			newCache[pv.Name] = true

			// Also cache volume key derived from VolumeHandle because dynamic
			// provisioned PV names are usually pvc-* while PowerScale paths end
			// with the backing directory name (for example csivol-*).
			if volumeKey := extractVolumeKeyFromHandle(pv.Spec.CSI.VolumeHandle); volumeKey != "" {
				newCache[volumeKey] = true
			}
		}
	}

	csmlog.Debugf("K8sMetadataChecker: cached %d driver-managed PVs", len(newCache))

	// Update cache atomically
	c.cacheMu.Lock()
	c.pvCache = newCache
	c.cacheMu.Unlock()

	return nil
}

// RefreshCache fetches and caches all PVs with field selector (implements VolumeValidator interface)
func (c *K8sMetadataChecker) RefreshCache(ctx context.Context) error {
	return c.fetchAndCachePVs(ctx)
}

// extractVolumeNameFromPath extracts the volume name from a PowerScale path
// Example: /ifs/data/csi/csivol-abc123 -> csivol-abc123
func extractVolumeNameFromPath(path string) string {
	if path == "" {
		return ""
	}

	// Find the last component of the path
	lastSlash := -1
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			lastSlash = i
			break
		}
	}

	if lastSlash == -1 || lastSlash == len(path)-1 {
		return ""
	}

	return path[lastSlash+1:]
}

func extractVolumeKeyFromHandle(volumeHandle string) string {
	if volumeHandle == "" {
		return ""
	}

	parts := strings.SplitN(volumeHandle, id.VolumeIDSeparator, 2)
	return strings.TrimSpace(parts[0])
}
