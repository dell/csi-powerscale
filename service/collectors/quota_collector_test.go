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

type mockQuotaClient struct {
	quotas []collectors.QuotaInfo
	err    error
}

func (m *mockQuotaClient) GetAllQuotas(_ context.Context) ([]collectors.QuotaInfo, error) {
	return m.quotas, m.err
}

func gatherPSCCollMetric(t *testing.T, reg prometheus.Gatherer, name string) *dto.MetricFamily {
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

func gaugePSC(mf *dto.MetricFamily, labels map[string]string) (float64, bool) {
	if mf == nil {
		return 0, false
	}
	for _, m := range mf.GetMetric() {
		got := make(map[string]string)
		for _, lp := range m.GetLabel() {
			got[lp.GetName()] = lp.GetValue()
		}
		match := true
		for k, v := range labels {
			if got[k] != v {
				match = false
				break
			}
		}
		if match {
			return m.GetGauge().GetValue(), true
		}
	}
	return 0, false
}

// U-PSC-03: 5 quotas; 2 with HardExceeded — breach total = 2
func TestQuotaCollector_Collect_BreachCount(t *testing.T) {
	client := &mockQuotaClient{
		quotas: []collectors.QuotaInfo{
			{VolumeID: "v1", AccessZone: "zone-a", UsedBytes: 900, HardBytes: 1000, HardExceeded: true},
			{VolumeID: "v2", AccessZone: "zone-a", UsedBytes: 500, HardBytes: 1000},
			{VolumeID: "v3", AccessZone: "zone-b", UsedBytes: 1001, HardBytes: 1000, HardExceeded: true},
			{VolumeID: "v4", AccessZone: "zone-b", UsedBytes: 100, HardBytes: 1000},
			{VolumeID: "v5", AccessZone: "zone-a", UsedBytes: 300, HardBytes: 1000},
		},
	}

	reg := prometheus.NewRegistry()
	c := collectors.NewQuotaCollector(client, reg, "cluster-1")

	err := c.Collect(context.Background())
	require.NoError(t, err)

	mf := gatherPSCCollMetric(t, reg, "dell_powerscale_quota_hard_breach_total")
	require.NotNil(t, mf, "dell_powerscale_quota_hard_breach_total should be emitted")

	v, ok := gaugePSC(mf, map[string]string{"cluster_name": "cluster-1"})
	require.True(t, ok)
	assert.Equal(t, 2.0, v, "breach total should be 2 (volumes v1 and v3)")
}

// U-PSC-04: HardBytes == 0 — utilization_ratio set to 0.0, not NaN
func TestQuotaCollector_Collect_ZeroHardBytes_UtilizationIsZero(t *testing.T) {
	client := &mockQuotaClient{
		quotas: []collectors.QuotaInfo{
			{VolumeID: "v1", AccessZone: "zone-a", UsedBytes: 0, HardBytes: 0},
		},
	}

	reg := prometheus.NewRegistry()
	c := collectors.NewQuotaCollector(client, reg, "cluster-1")
	_ = c.Collect(context.Background())

	mf := gatherPSCCollMetric(t, reg, "dell_powerscale_quota_utilization_ratio")
	require.NotNil(t, mf)
	v, ok := gaugePSC(mf, map[string]string{
		"cluster_name": "cluster-1", "volume_id": "v1", "access_zone": "zone-a",
	})
	require.True(t, ok)
	assert.Equal(t, 0.0, v, "utilization_ratio should be 0.0 when HardBytes == 0, not NaN")
}

// U-PSC-05: Empty quota list — breach total = 0, no per-volume metrics
func TestQuotaCollector_Collect_EmptyList(t *testing.T) {
	client := &mockQuotaClient{quotas: []collectors.QuotaInfo{}}

	reg := prometheus.NewRegistry()
	c := collectors.NewQuotaCollector(client, reg, "cluster-1")
	err := c.Collect(context.Background())
	assert.NoError(t, err)

	mf := gatherPSCCollMetric(t, reg, "dell_powerscale_quota_hard_breach_total")
	if mf != nil {
		v, ok := gaugePSC(mf, map[string]string{"cluster_name": "cluster-1"})
		if ok {
			assert.Equal(t, 0.0, v)
		}
	}

	usedMF := gatherPSCCollMetric(t, reg, "dell_powerscale_quota_used_bytes")
	if usedMF != nil {
		assert.Empty(t, usedMF.GetMetric())
	}
}

// U-PSC-QE: Collect propagates GetAllQuotas error
func TestQuotaCollector_Collect_Error(t *testing.T) {
	client := &mockQuotaClient{err: errors.New("api error")}
	reg := prometheus.NewRegistry()
	c := collectors.NewQuotaCollector(client, reg, "cluster-1")
	err := c.Collect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "QuotaCollector")
}

// U-PSC-QN: Name returns expected string
func TestQuotaCollector_Name(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := collectors.NewQuotaCollector(&mockQuotaClient{}, reg, "cluster-1")
	assert.Equal(t, "QuotaCollector", c.Name())
}

// U-PSC-VT: volume total matches number of quotas
func TestQuotaCollector_Collect_VolumeTotal(t *testing.T) {
	client := &mockQuotaClient{
		quotas: []collectors.QuotaInfo{
			{VolumeID: "v1", AccessZone: "zone-a", UsedBytes: 100, HardBytes: 1000},
			{VolumeID: "v2", AccessZone: "zone-a", UsedBytes: 200, HardBytes: 2000},
			{VolumeID: "v3", AccessZone: "zone-b", UsedBytes: 300, HardBytes: 3000},
		},
	}

	reg := prometheus.NewRegistry()
	c := collectors.NewQuotaCollector(client, reg, "cluster-1")
	err := c.Collect(context.Background())
	require.NoError(t, err)

	mf := gatherPSCCollMetric(t, reg, "dell_csi_volume_total")
	require.NotNil(t, mf, "dell_csi_volume_total must be emitted")
	v, ok := gaugePSC(mf, map[string]string{"cluster_name": "cluster-1"})
	require.True(t, ok)
	assert.Equal(t, 3.0, v, "volume_total should equal the number of quotas returned")
}

// U-PSC-06: Only directory-type quotas included (filtered by collector)
func TestQuotaCollector_Collect_QuotaUtilizationCalculation(t *testing.T) {
	client := &mockQuotaClient{
		quotas: []collectors.QuotaInfo{
			{VolumeID: "v1", AccessZone: "zone-a", UsedBytes: 500, HardBytes: 1000},
		},
	}

	reg := prometheus.NewRegistry()
	c := collectors.NewQuotaCollector(client, reg, "cluster-1")
	_ = c.Collect(context.Background())

	mf := gatherPSCCollMetric(t, reg, "dell_powerscale_quota_utilization_ratio")
	require.NotNil(t, mf)
	v, ok := gaugePSC(mf, map[string]string{"cluster_name": "cluster-1", "volume_id": "v1"})
	require.True(t, ok)
	assert.InDelta(t, 0.5, v, 0.001, "utilization = used/hard = 500/1000 = 0.5")
}

func TestQuotaCollector_Cleanup(t *testing.T) {
	client := &mockQuotaClient{}
	reg := prometheus.NewRegistry()
	c := collectors.NewQuotaCollector(client, reg, "cluster-1")

	// Should not panic
	assert.NotPanics(t, func() {
		c.Cleanup()
	})
}

// U-PSC-08: Cleanup of deleted volumes
func TestQuotaCollector_Collect_CleanupDeletedVolumes(t *testing.T) {
	client := &mockQuotaClient{
		quotas: []collectors.QuotaInfo{
			{VolumeID: "v1", AccessZone: "zone-a", UsedBytes: 100, HardBytes: 1000},
		},
	}

	reg := prometheus.NewRegistry()
	c := collectors.NewQuotaCollector(client, reg, "cluster-1")

	// First collection - v1 exists
	_ = c.Collect(context.Background())

	// Change quotas to remove v1 and add v2
	client.quotas = []collectors.QuotaInfo{
		{VolumeID: "v2", AccessZone: "zone-b", UsedBytes: 200, HardBytes: 2000},
	}

	// Second collection - v1 should be cleaned up
	_ = c.Collect(context.Background())

	// Verify v1 metrics are cleaned up
	mf := gatherPSCCollMetric(t, reg, "dell_powerscale_quota_used_bytes")
	require.NotNil(t, mf)

	// v1 should not exist anymore
	_, ok := gaugePSC(mf, map[string]string{"cluster_name": "cluster-1", "volume_id": "v1", "access_zone": "zone-a"})
	require.False(t, ok, "v1 metrics should be cleaned up")

	// v2 should exist
	_, ok = gaugePSC(mf, map[string]string{"cluster_name": "cluster-1", "volume_id": "v2", "access_zone": "zone-b"})
	require.True(t, ok, "v2 metrics should exist")
}

// U-PSC-07: Multiple quotas with different zones
func TestQuotaCollector_Collect_MultipleZones(t *testing.T) {
	client := &mockQuotaClient{
		quotas: []collectors.QuotaInfo{
			{VolumeID: "v1", AccessZone: "zone-a", UsedBytes: 100, HardBytes: 1000},
			{VolumeID: "v2", AccessZone: "zone-b", UsedBytes: 200, HardBytes: 2000},
			{VolumeID: "v3", AccessZone: "zone-a", UsedBytes: 300, HardBytes: 3000},
		},
	}

	reg := prometheus.NewRegistry()
	c := collectors.NewQuotaCollector(client, reg, "cluster-1")
	err := c.Collect(context.Background())
	require.NoError(t, err)

	mf := gatherPSCCollMetric(t, reg, "dell_csi_volume_total")
	require.NotNil(t, mf)
	v, ok := gaugePSC(mf, map[string]string{"cluster_name": "cluster-1"})
	require.True(t, ok)
	assert.Equal(t, 3.0, v, "volume_total should be 3")
}

// U-PSC-08: Quota remaining bytes calculation
func TestQuotaCollector_Collect_RemainingBytes(t *testing.T) {
	client := &mockQuotaClient{
		quotas: []collectors.QuotaInfo{
			{VolumeID: "v1", AccessZone: "zone-a", UsedBytes: 300, HardBytes: 1000},
		},
	}

	reg := prometheus.NewRegistry()
	c := collectors.NewQuotaCollector(client, reg, "cluster-1")
	_ = c.Collect(context.Background())

	mf := gatherPSCCollMetric(t, reg, "dell_powerscale_quota_remaining_bytes")
	require.NotNil(t, mf)
	v, ok := gaugePSC(mf, map[string]string{"cluster_name": "cluster-1", "volume_id": "v1"})
	require.True(t, ok)
	assert.Equal(t, 700.0, v, "remaining = hard - used = 1000 - 300 = 700")
}
