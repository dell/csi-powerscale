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

	"github.com/Ecosystems/container-storage-modules/src/csmlog"
	gopowerscale "github.com/Ecosystems/container-storage-modules/src/gopowerscale"
)

// populateZoneCache fetches all access zones from PowerScale and populates the zone cache.

func (a *QuotaAdapter) populateZoneCache(ctx context.Context) error {
	zones, err := a.client.GetIsiZoneList(ctx)
	if err != nil {
		return fmt.Errorf("failed to get zone list: %w", err)
	}

	for _, zone := range zones.Zones {
		if zone != nil {
			a.zoneCache[zone.Path] = zone.Name
		}
	}
	return nil
}

// getZoneFromPath determines the access zone for a volume path by matching against zone paths.
func (a *QuotaAdapter) getZoneFromPath(ctx context.Context, path string) string {
	// Populate cache if empty
	if len(a.zoneCache) == 0 {
		if err := a.populateZoneCache(ctx); err != nil {
			// Return empty string if zone API fails
			return ""
		}
	}

	// Find the longest matching zone path
	var matchedZone string
	var longestMatch string
	for zonePath, zoneName := range a.zoneCache {
		if strings.HasPrefix(path, zonePath) && len(zonePath) > len(longestMatch) {
			longestMatch = zonePath
			matchedZone = zoneName
		}
	}

	if matchedZone != "" {
		return matchedZone
	}

	// Return empty string if no match found
	return ""
}

// QuotaAdapter implements QuotaClient using a gopowerscale Client.
type QuotaAdapter struct {
	client    *gopowerscale.Client
	validator VolumeValidator
	zoneCache map[string]string // zone path -> zone name cache
	runtime   *MetricsRuntime
}

// NewQuotaAdapter creates a QuotaAdapter wrapping the provided gopowerscale client.
// Deprecated: Use NewQuotaAdapterWithPath instead to explicitly specify the isiPath.
func NewQuotaAdapter(client *gopowerscale.Client, isiPath string) *QuotaAdapter {
	return NewQuotaAdapterWithPath(client, isiPath)
}

// NewQuotaAdapterWithPath creates a QuotaAdapter with path-based filtering for driver-managed volumes.
func NewQuotaAdapterWithPath(client *gopowerscale.Client, isiPath string) *QuotaAdapter {
	return &QuotaAdapter{
		client:    client,
		validator: NewHybridVolumeValidator(isiPath),
		zoneCache: make(map[string]string),
	}
}

// NewQuotaAdapterWithValidator creates a QuotaAdapter with a custom volume validator.
func NewQuotaAdapterWithValidator(client *gopowerscale.Client, validator VolumeValidator) *QuotaAdapter {
	return &QuotaAdapter{
		client:    client,
		validator: validator,
		zoneCache: make(map[string]string),
	}
}

// NewQuotaAdapterWithRuntime creates a QuotaAdapter that routes collection calls
// through the provided MetricsRuntime for timeout, rate limiting, circuit
// breaking, and caching.
func NewQuotaAdapterWithRuntime(client *gopowerscale.Client, rt *MetricsRuntime) *QuotaAdapter {
	return &QuotaAdapter{
		client:    client,
		validator: NewHybridVolumeValidator(""),
		zoneCache: make(map[string]string),
		runtime:   rt,
	}
}

// SetRuntime sets the MetricsRuntime on the adapter for timeout, rate limiting,
// circuit breaking, and caching.
func (a *QuotaAdapter) SetRuntime(rt *MetricsRuntime) {
	a.runtime = rt
}

// GetAllQuotas fetches all directory quotas from OneFS and maps them to QuotaInfo.
// Uses hybrid validation: fast path-based filtering with optional Kubernetes validation.
func (a *QuotaAdapter) GetAllQuotas(ctx context.Context) ([]QuotaInfo, error) {
	if a.runtime == nil {
		return a.fetchAllQuotas(ctx)
	}
	v, err := a.runtime.Do(ctx, "quotas", "all_quotas", func(callCtx context.Context) (any, error) {
		return a.fetchAllQuotas(callCtx)
	})
	if err != nil {
		return nil, err
	}
	result, _ := v.([]QuotaInfo)
	return result, nil
}

func (a *QuotaAdapter) fetchAllQuotas(ctx context.Context) ([]QuotaInfo, error) {
	// Refresh validator cache upfront (fetches all K8s PVs at once)
	if err := a.validator.RefreshCache(ctx); err != nil {
		// Log error but continue with path-based validation only
		csmlog.Warnf("QuotaAdapter: RefreshCache failed, will use path-based validation: %v", err)
	}

	quotas, err := a.client.GetAllQuotas(ctx)
	if err != nil {
		return nil, fmt.Errorf("QuotaAdapter: failed to get quotas: %w", err)
	}

	csmlog.Debugf("QuotaAdapter: fetched %d quotas from PowerScale", len(quotas))

	result := make([]QuotaInfo, 0, len(quotas))
	for _, q := range quotas {
		if q == nil {
			continue
		}

		// Use hybrid validator: path-based filtering + optional K8s validation
		isManaged, _ := a.validator.IsDriverManaged(ctx, q.Path)
		if !isManaged {
			csmlog.Debugf("QuotaAdapter: quota path %s is not driver-managed, skipping", q.Path)
			continue
		}
		csmlog.Debugf("QuotaAdapter: quota path %s is driver-managed, including in results", q.Path)

		// Determine access zone from actual PowerScale zone configuration
		accessZone := a.getZoneFromPath(ctx, q.Path)

		result = append(result, QuotaInfo{
			VolumeID:     q.Path,
			AccessZone:   accessZone,
			UsedBytes:    q.Usage.Physical,
			HardBytes:    q.Thresholds.Hard,
			HardExceeded: q.Thresholds.HardExceeded,
		})
	}
	return result, nil
}
