// Copyright © 2025-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collectors

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Ecosystems/container-storage-modules/src/csm-metrics-common/pkg/naming"
	"github.com/prometheus/client_golang/prometheus"
)

// NodePoolInfo holds information about a PowerScale node pool.
type NodePoolInfo struct {
	ID               int32
	Name             string
	Total            int64
	Used             int64
	Avail            int64
	Usable           int64
	ProtectionPolicy string
	Tier             string
}

// NodePoolClient is the minimal interface for fetching node pool data.
type NodePoolClient interface {
	GetNodePools(ctx context.Context) ([]NodePoolInfo, error)
}

// NodePoolCollector collects PowerScale node pool metrics.
type NodePoolCollector struct {
	client              NodePoolClient
	clusterName         string
	nodePoolCapacity    *prometheus.GaugeVec
	nodePoolUtilization *prometheus.GaugeVec
	nodePoolProtection  *prometheus.GaugeVec
	nodePoolTier        *prometheus.GaugeVec
}

// NewNodePoolCollector creates a new NodePoolCollector and registers its metrics.
func NewNodePoolCollector(client NodePoolClient, reg prometheus.Registerer, clusterName string) *NodePoolCollector {
	c := &NodePoolCollector{
		client:      client,
		clusterName: clusterName,
		nodePoolCapacity: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_powerscale_nodepool_capacity_bytes",
			Help: "PowerScale node pool storage capacity in bytes (total/used/avail/usable).",
		}, []string{naming.LabelClusterName, "nodepool_id", "nodepool_name", "type"})),
		nodePoolUtilization: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_powerscale_nodepool_utilization_percent",
			Help: "PowerScale node pool utilization percentage (used/total × 100).",
		}, []string{naming.LabelClusterName, "nodepool_id", "nodepool_name"})),
		nodePoolProtection: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_powerscale_nodepool_protection_policy",
			Help: "PowerScale node pool protection policy (info metric).",
		}, []string{naming.LabelClusterName, "nodepool_id", "nodepool_name", "policy"})),
		nodePoolTier: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_powerscale_nodepool_tier_info",
			Help: "PowerScale node pool tier information (info metric).",
		}, []string{naming.LabelClusterName, "nodepool_id", "nodepool_name", "tier"})),
	}
	return c
}

// Collect fetches node pool metrics and updates the Prometheus gauges.
func (c *NodePoolCollector) Collect(ctx context.Context) error {
	if c.client == nil {
		return fmt.Errorf("NodePoolCollector: client is nil")
	}

	nodePools, err := c.client.GetNodePools(ctx)
	if err != nil {
		return fmt.Errorf("NodePoolCollector: failed to get node pools: %w", err)
	}

	for _, np := range nodePools {
		npID := strconv.Itoa(int(np.ID))

		// Capacity metrics
		c.nodePoolCapacity.WithLabelValues(c.clusterName, npID, np.Name, "total").Set(float64(np.Total))
		c.nodePoolCapacity.WithLabelValues(c.clusterName, npID, np.Name, "used").Set(float64(np.Used))
		c.nodePoolCapacity.WithLabelValues(c.clusterName, npID, np.Name, "avail").Set(float64(np.Avail))
		c.nodePoolCapacity.WithLabelValues(c.clusterName, npID, np.Name, "usable").Set(float64(np.Usable))

		// Utilization percentage
		if np.Total > 0 {
			utilization := float64(np.Used) / float64(np.Total) * 100
			c.nodePoolUtilization.WithLabelValues(c.clusterName, npID, np.Name).Set(utilization)
		} else {
			c.nodePoolUtilization.WithLabelValues(c.clusterName, npID, np.Name).Set(0.0)
		}

		// Info metrics (always set to 1)
		c.nodePoolProtection.WithLabelValues(c.clusterName, npID, np.Name, np.ProtectionPolicy).Set(1.0)
		c.nodePoolTier.WithLabelValues(c.clusterName, npID, np.Name, np.Tier).Set(1.0)
	}

	return nil
}

// Name returns the collector name.
func (c *NodePoolCollector) Name() string { return "NodePoolCollector" }

// Cleanup removes all node pool metrics for this cluster.
func (c *NodePoolCollector) Cleanup() {
	labels := prometheus.Labels{naming.LabelClusterName: c.clusterName}
	c.nodePoolCapacity.DeletePartialMatch(labels)
	c.nodePoolUtilization.DeletePartialMatch(labels)
	c.nodePoolProtection.DeletePartialMatch(labels)
	c.nodePoolTier.DeletePartialMatch(labels)
}
