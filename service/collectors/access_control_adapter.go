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

	gopowerscale "github.com/Ecosystems/container-storage-modules/src/gopowerscale"
	"github.com/Ecosystems/container-storage-modules/src/gopowerscale/api"
)

// AccessControlAdapter implements AccessControlClient interface using gopowerscale.
type AccessControlAdapter struct {
	client  *gopowerscale.Client
	runtime *MetricsRuntime
}

// NewAccessControlAdapter creates a new AccessControlAdapter.
func NewAccessControlAdapter(client *gopowerscale.Client) *AccessControlAdapter {
	return &AccessControlAdapter{client: client}
}

// NewAccessControlAdapterWithRuntime creates an AccessControlAdapter that routes
// collection calls through the provided MetricsRuntime.
func NewAccessControlAdapterWithRuntime(client *gopowerscale.Client, rt *MetricsRuntime) *AccessControlAdapter {
	return &AccessControlAdapter{client: client, runtime: rt}
}

// GetZones implements AccessControlClient.GetZones.
func (a *AccessControlAdapter) GetZones(ctx context.Context) ([]ZoneInfo, error) {
	if a.runtime == nil {
		return a.fetchZones(ctx)
	}
	v, err := a.runtime.Do(ctx, "zones", "all_zones", func(callCtx context.Context) (any, error) {
		return a.fetchZones(callCtx)
	})
	if err != nil {
		return nil, err
	}
	result, _ := v.([]ZoneInfo)
	return result, nil
}

func (a *AccessControlAdapter) fetchZones(ctx context.Context) ([]ZoneInfo, error) {
	if a.client == nil {
		return nil, fmt.Errorf("PowerScale client not yet initialized")
	}
	zones, err := a.client.GetIsiZoneList(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get zones from PowerScale: %w", err)
	}

	if zones == nil || zones.Zones == nil {
		return []ZoneInfo{}, nil
	}

	zoneInfos := make([]ZoneInfo, len(zones.Zones))
	for i, zone := range zones.Zones {
		zoneInfos[i] = ZoneInfo{Name: zone.Name}
	}

	return zoneInfos, nil
}

// GetExportCountByZone implements AccessControlClient.GetExportCountByZone.
func (a *AccessControlAdapter) GetExportCountByZone(ctx context.Context, zone string) (int, error) {
	if a.runtime == nil {
		return a.fetchExportCountByZone(ctx, zone)
	}
	v, err := a.runtime.Do(ctx, "exports", "exports/"+zone, func(callCtx context.Context) (any, error) {
		return a.fetchExportCountByZone(callCtx, zone)
	})
	if err != nil {
		return 0, err
	}
	result, _ := v.(int)
	return result, nil
}

func (a *AccessControlAdapter) fetchExportCountByZone(ctx context.Context, zone string) (int, error) {
	if a.client == nil {
		return 0, fmt.Errorf("PowerScale client not yet initialized")
	}

	// Use GetExportsWithParams with zone parameter to filter exports by zone
	params := api.OrderedValues{
		[][]byte{[]byte("zone"), []byte(zone)},
	}

	exports, err := a.client.GetExportsWithParams(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("failed to get exports for zone %q: %w", zone, err)
	}

	if exports == nil {
		return 0, nil
	}

	return exports.Total, nil
}
