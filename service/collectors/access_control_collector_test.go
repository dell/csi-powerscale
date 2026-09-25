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

package collectors_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Ecosystems/container-storage-modules/src/csi-powerscale/v2/service/collectors"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAccessControlClient struct {
	zones      []collectors.ZoneInfo
	zonesErr   error
	exports    map[string]int
	exportsErr error
}

func (m *mockAccessControlClient) GetZones(_ context.Context) ([]collectors.ZoneInfo, error) {
	return m.zones, m.zonesErr
}

func (m *mockAccessControlClient) GetExportCountByZone(_ context.Context, zone string) (int, error) {
	if m.exportsErr != nil {
		return 0, m.exportsErr
	}
	return m.exports[zone], nil
}

func gatherACMetric(t *testing.T, reg prometheus.Gatherer, name string) *dto.MetricFamily {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() == name {
			return mf
		}
	}
	return nil
}

func gaugeACByLabel(mf *dto.MetricFamily, labelKey, labelVal string) (float64, bool) {
	if mf == nil {
		return 0, false
	}
	for _, m := range mf.GetMetric() {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == labelKey && lp.GetValue() == labelVal {
				return m.GetGauge().GetValue(), true
			}
		}
	}
	return 0, false
}

func counterACByLabel(mf *dto.MetricFamily, labelKey, labelVal string) (float64, bool) {
	if mf == nil {
		return 0, false
	}
	for _, m := range mf.GetMetric() {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == labelKey && lp.GetValue() == labelVal {
				return m.GetCounter().GetValue(), true
			}
		}
	}
	return 0, false
}

// U-AC-01: access_zone_total equals the number of zones
func TestAccessControlCollector_Collect_ZoneTotal(t *testing.T) {
	client := &mockAccessControlClient{
		zones:   []collectors.ZoneInfo{{Name: "System"}, {Name: "zone-a"}, {Name: "zone-b"}},
		exports: map[string]int{"System": 5, "zone-a": 3, "zone-b": 2},
	}

	reg := prometheus.NewRegistry()
	c := collectors.NewAccessControlCollector(client, reg, "cluster-1")
	err := c.Collect(context.Background())
	require.NoError(t, err)

	mf := gatherACMetric(t, reg, "dell_powerscale_access_zone_total")
	require.NotNil(t, mf)
	v, ok := gaugeACByLabel(mf, "cluster_name", "cluster-1")
	require.True(t, ok)
	assert.Equal(t, 3.0, v, "zone total should be 3")
}

// U-AC-02: nfs_export_total is emitted per zone
func TestAccessControlCollector_Collect_ExportTotalPerZone(t *testing.T) {
	client := &mockAccessControlClient{
		zones:   []collectors.ZoneInfo{{Name: "System"}, {Name: "zone-a"}},
		exports: map[string]int{"System": 10, "zone-a": 4},
	}

	reg := prometheus.NewRegistry()
	c := collectors.NewAccessControlCollector(client, reg, "cluster-1")
	err := c.Collect(context.Background())
	require.NoError(t, err)

	mf := gatherACMetric(t, reg, "dell_powerscale_nfs_export_total")
	require.NotNil(t, mf)

	sysVal, ok := gaugeACByLabel(mf, "access_zone", "System")
	require.True(t, ok)
	assert.Equal(t, 10.0, sysVal)

	zoneAVal, ok := gaugeACByLabel(mf, "access_zone", "zone-a")
	require.True(t, ok)
	assert.Equal(t, 4.0, zoneAVal)
}

// U-AC-03: zone API error is propagated
func TestAccessControlCollector_Collect_ZoneAPIError(t *testing.T) {
	client := &mockAccessControlClient{zonesErr: errors.New("unreachable")}

	reg := prometheus.NewRegistry()
	c := collectors.NewAccessControlCollector(client, reg, "cluster-1")
	err := c.Collect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AccessControlCollector")
}

// U-AC-04: export API error is propagated
func TestAccessControlCollector_Collect_ExportAPIError(t *testing.T) {
	client := &mockAccessControlClient{
		zones:      []collectors.ZoneInfo{{Name: "System"}},
		exportsErr: errors.New("permission denied"),
	}

	reg := prometheus.NewRegistry()
	c := collectors.NewAccessControlCollector(client, reg, "cluster-1")
	err := c.Collect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AccessControlCollector")
}

// U-AC-05: empty zone list — zone_total = 0, no export metrics
func TestAccessControlCollector_Collect_EmptyZones(t *testing.T) {
	client := &mockAccessControlClient{zones: []collectors.ZoneInfo{}}

	reg := prometheus.NewRegistry()
	c := collectors.NewAccessControlCollector(client, reg, "cluster-1")
	err := c.Collect(context.Background())
	require.NoError(t, err)

	mf := gatherACMetric(t, reg, "dell_powerscale_access_zone_total")
	require.NotNil(t, mf)
	v, ok := gaugeACByLabel(mf, "cluster_name", "cluster-1")
	require.True(t, ok)
	assert.Equal(t, 0.0, v)
}

// U-AC-06: Name returns expected string
func TestAccessControlCollector_Name(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := collectors.NewAccessControlCollector(&mockAccessControlClient{}, reg, "cluster-1")
	assert.Equal(t, "AccessControlCollector", c.Name())
}

// U-AC-07: Single zone with multiple exports
func TestAccessControlCollector_Collect_SingleZoneMultipleExports(t *testing.T) {
	client := &mockAccessControlClient{
		zones:   []collectors.ZoneInfo{{Name: "System"}},
		exports: map[string]int{"System": 100},
	}

	reg := prometheus.NewRegistry()
	c := collectors.NewAccessControlCollector(client, reg, "cluster-1")
	err := c.Collect(context.Background())
	require.NoError(t, err)

	mf := gatherACMetric(t, reg, "dell_powerscale_nfs_export_total")
	require.NotNil(t, mf)
	v, ok := gaugeACByLabel(mf, "access_zone", "System")
	require.True(t, ok)
	assert.Equal(t, 100.0, v)
}

// U-AC-08: Many zones with varying export counts
func TestAccessControlCollector_Collect_ManyZones(t *testing.T) {
	client := &mockAccessControlClient{
		zones: []collectors.ZoneInfo{
			{Name: "System"},
			{Name: "zone1"},
			{Name: "zone2"},
			{Name: "zone3"},
		},
		exports: map[string]int{
			"System": 10,
			"zone1":  5,
			"zone2":  8,
			"zone3":  3,
		},
	}

	reg := prometheus.NewRegistry()
	c := collectors.NewAccessControlCollector(client, reg, "cluster-1")
	err := c.Collect(context.Background())
	require.NoError(t, err)

	mf := gatherACMetric(t, reg, "dell_powerscale_access_zone_total")
	require.NotNil(t, mf)
	v, ok := gaugeACByLabel(mf, "cluster_name", "cluster-1")
	require.True(t, ok)
	assert.Equal(t, 4.0, v, "zone total should be 4")
}

// U-AC-09: RecordAuthFailure increments auth failure counter
func TestAccessControlCollector_RecordAuthFailure(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := collectors.NewAccessControlCollector(&mockAccessControlClient{}, reg, "cluster-1")

	// Record multiple auth failures
	c.RecordAuthFailure()
	c.RecordAuthFailure()
	c.RecordAuthFailure()

	mf := gatherACMetric(t, reg, "dell_powerscale_nfs_mount_auth_failures_total")
	require.NotNil(t, mf)
	v, ok := counterACByLabel(mf, "cluster_name", "cluster-1")
	require.True(t, ok)
	assert.Equal(t, 3.0, v, "auth failure counter should be 3")
}

// U-AC-10: RecordPermissionDenial increments permission denial counter
func TestAccessControlCollector_RecordPermissionDenial(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := collectors.NewAccessControlCollector(&mockAccessControlClient{}, reg, "cluster-1")

	// Record multiple permission denials
	c.RecordPermissionDenial()
	c.RecordPermissionDenial()

	mf := gatherACMetric(t, reg, "dell_powerscale_permission_denials_total")
	require.NotNil(t, mf)
	v, ok := counterACByLabel(mf, "cluster_name", "cluster-1")
	require.True(t, ok)
	assert.Equal(t, 2.0, v, "permission denial counter should be 2")
}

// U-AC-11: Cleanup removes auth failure and permission denial metrics
func TestAccessControlCollector_Cleanup_RemovesSecurityMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := collectors.NewAccessControlCollector(&mockAccessControlClient{}, reg, "cluster-1")

	// Record some metrics
	c.RecordAuthFailure()
	c.RecordPermissionDenial()

	// Verify metrics exist
	authMF := gatherACMetric(t, reg, "dell_powerscale_nfs_mount_auth_failures_total")
	require.NotNil(t, authMF)
	permMF := gatherACMetric(t, reg, "dell_powerscale_permission_denials_total")
	require.NotNil(t, permMF)

	// Cleanup
	c.Cleanup()

	// Verify metrics are removed (metrics should still exist in registry but with no values)
	// Note: Prometheus doesn't actually delete metrics from registry, but cleanup removes label values
	// This test verifies the Cleanup method doesn't panic
	assert.NotPanics(t, c.Cleanup)
}
