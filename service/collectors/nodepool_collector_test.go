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
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

type mockNodePoolClient struct {
	pools []NodePoolInfo
	err   error
}

func (m *mockNodePoolClient) GetNodePools(_ context.Context) ([]NodePoolInfo, error) {
	return m.pools, m.err
}

func TestNodePoolCollector(t *testing.T) {
	// Mock node pools
	mockPools := []NodePoolInfo{
		{
			ID:               1,
			Name:             "pool1",
			Total:            1000000000,
			Used:             400000000,
			Avail:            600000000,
			Usable:           800000000,
			ProtectionPolicy: "+2d:1n",
			Tier:             "performance",
		},
		{
			ID:               2,
			Name:             "pool2",
			Total:            2000000000,
			Used:             1000000000,
			Avail:            1000000000,
			Usable:           1800000000,
			ProtectionPolicy: "+3d:2n",
			Tier:             "capacity",
		},
	}

	client := &mockNodePoolClient{pools: mockPools}
	reg := prometheus.NewRegistry()
	collector := NewNodePoolCollector(client, reg, "test-cluster")

	// Test collection
	err := collector.Collect(context.Background())
	assert.NoError(t, err)

	// Test node pool capacity metrics
	capacity, err := reg.Gather()
	assert.NoError(t, err)

	// Find dell_powerscale_nodepool_capacity_bytes metric
	found := false
	for _, family := range capacity {
		if family.GetName() == "dell_powerscale_nodepool_capacity_bytes" {
			found = true
			metrics := family.GetMetric()
			assert.Len(t, metrics, 8) // 2 pools × 4 types (total/used/avail/usable)

			// Check pool1 total capacity
			for _, m := range metrics {
				labels := m.GetLabel()
				poolID := ""
				capacityType := ""
				for _, label := range labels {
					if label.GetName() == "nodepool_id" {
						poolID = label.GetValue()
					}
					if label.GetName() == "type" {
						capacityType = label.GetValue()
					}
				}
				if poolID == "1" && capacityType == "total" {
					assert.Equal(t, 1000000000.0, m.GetGauge().GetValue())
				}
			}
			break
		}
	}
	assert.True(t, found, "dell_powerscale_nodepool_capacity_bytes metric not found")

	// Test node pool utilization metric
	expected := fmt.Sprintf(`
# HELP dell_powerscale_nodepool_utilization_percent PowerScale node pool utilization percentage (used/total × 100).
# TYPE dell_powerscale_nodepool_utilization_percent gauge
dell_powerscale_nodepool_utilization_percent{cluster_name="test-cluster",nodepool_id="1",nodepool_name="pool1"} 40
dell_powerscale_nodepool_utilization_percent{cluster_name="test-cluster",nodepool_id="2",nodepool_name="pool2"} 50
`)
	err = testutil.CollectAndCompare(collector.nodePoolUtilization, strings.NewReader(expected))
	assert.NoError(t, err)

	// Test info metrics (protection and tier)
	expectedProtection := fmt.Sprintf(`
# HELP dell_powerscale_nodepool_protection_policy PowerScale node pool protection policy (info metric).
# TYPE dell_powerscale_nodepool_protection_policy gauge
dell_powerscale_nodepool_protection_policy{cluster_name="test-cluster",nodepool_id="1",nodepool_name="pool1",policy="+2d:1n"} 1
dell_powerscale_nodepool_protection_policy{cluster_name="test-cluster",nodepool_id="2",nodepool_name="pool2",policy="+3d:2n"} 1
`)
	err = testutil.CollectAndCompare(collector.nodePoolProtection, strings.NewReader(expectedProtection))
	assert.NoError(t, err)

	expectedTier := fmt.Sprintf(`
# HELP dell_powerscale_nodepool_tier_info PowerScale node pool tier information (info metric).
# TYPE dell_powerscale_nodepool_tier_info gauge
dell_powerscale_nodepool_tier_info{cluster_name="test-cluster",nodepool_id="1",nodepool_name="pool1",tier="performance"} 1
dell_powerscale_nodepool_tier_info{cluster_name="test-cluster",nodepool_id="2",nodepool_name="pool2",tier="capacity"} 1
`)
	err = testutil.CollectAndCompare(collector.nodePoolTier, strings.NewReader(expectedTier))
	assert.NoError(t, err)
}

func TestNodePoolCollectorError(t *testing.T) {
	client := &mockNodePoolClient{err: errors.New("API error")}
	reg := prometheus.NewRegistry()
	collector := NewNodePoolCollector(client, reg, "test-cluster")

	err := collector.Collect(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NodePoolCollector: failed to get node pools")
}

func TestNodePoolCollectorName(t *testing.T) {
	client := &mockNodePoolClient{}
	reg := prometheus.NewRegistry()
	collector := NewNodePoolCollector(client, reg, "test-cluster")

	assert.Equal(t, "NodePoolCollector", collector.Name())
}

func TestNodePoolCollector_ZeroTotal(t *testing.T) {
	mockPools := []NodePoolInfo{
		{
			ID:               1,
			Name:             "pool1",
			Total:            0,
			Used:             0,
			Avail:            0,
			Usable:           0,
			ProtectionPolicy: "+2d:1n",
			Tier:             "performance",
		},
	}

	client := &mockNodePoolClient{pools: mockPools}
	reg := prometheus.NewRegistry()
	collector := NewNodePoolCollector(client, reg, "test-cluster")

	err := collector.Collect(context.Background())
	assert.NoError(t, err)
}

func TestNodePoolCollector_EmptyPools(t *testing.T) {
	client := &mockNodePoolClient{pools: []NodePoolInfo{}}
	reg := prometheus.NewRegistry()
	collector := NewNodePoolCollector(client, reg, "test-cluster")

	err := collector.Collect(context.Background())
	assert.NoError(t, err)
}

func TestNodePoolCollectorZeroCapacity(t *testing.T) {
	// Test edge case where total capacity is 0
	mockPools := []NodePoolInfo{
		{
			ID:               1,
			Name:             "empty-pool",
			Total:            0,
			Used:             0,
			Avail:            0,
			Usable:           0,
			ProtectionPolicy: "none",
			Tier:             "empty",
		},
	}

	client := &mockNodePoolClient{pools: mockPools}
	reg := prometheus.NewRegistry()
	collector := NewNodePoolCollector(client, reg, "test-cluster")

	err := collector.Collect(context.Background())
	assert.NoError(t, err)

	// Check utilization is 0% for empty pool
	expected := fmt.Sprintf(`
# HELP dell_powerscale_nodepool_utilization_percent PowerScale node pool utilization percentage (used/total × 100).
# TYPE dell_powerscale_nodepool_utilization_percent gauge
dell_powerscale_nodepool_utilization_percent{cluster_name="test-cluster",nodepool_id="1",nodepool_name="empty-pool"} 0
`)
	err = testutil.CollectAndCompare(collector.nodePoolUtilization, strings.NewReader(expected))
	assert.NoError(t, err)
}

func TestNodePoolCollector_Cleanup(t *testing.T) {
	client := &mockNodePoolClient{}
	reg := prometheus.NewRegistry()
	collector := NewNodePoolCollector(client, reg, "test-cluster")

	// Should not panic
	assert.NotPanics(t, func() {
		collector.Cleanup()
	})
}
