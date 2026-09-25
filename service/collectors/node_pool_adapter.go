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
	"strconv"

	gopowerscale "github.com/Ecosystems/container-storage-modules/src/gopowerscale"
)

const (
	unknownTier = "unknown"
)

// NodePoolAdapter implements NodePoolClient interface using gopowerscale.
type NodePoolAdapter struct {
	client  *gopowerscale.Client
	runtime *MetricsRuntime
}

// NewNodePoolAdapter creates a new NodePoolAdapter.
func NewNodePoolAdapter(client *gopowerscale.Client) *NodePoolAdapter {
	return &NodePoolAdapter{client: client}
}

// NewNodePoolAdapterWithRuntime creates a NodePoolAdapter that routes collection
// calls through the provided MetricsRuntime.
func NewNodePoolAdapterWithRuntime(client *gopowerscale.Client, rt *MetricsRuntime) *NodePoolAdapter {
	return &NodePoolAdapter{client: client, runtime: rt}
}

// GetNodePools implements NodePoolClient.GetNodePools.
func (a *NodePoolAdapter) GetNodePools(ctx context.Context) ([]NodePoolInfo, error) {
	if a.runtime == nil {
		return a.fetchNodePools(ctx)
	}
	v, err := a.runtime.Do(ctx, "node_pools", "all_node_pools", func(callCtx context.Context) (any, error) {
		return a.fetchNodePools(callCtx)
	})
	if err != nil {
		return nil, err
	}
	result, _ := v.([]NodePoolInfo)
	return result, nil
}

func (a *NodePoolAdapter) fetchNodePools(ctx context.Context) ([]NodePoolInfo, error) {
	nodePools, err := a.client.GetNodePools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get node pools from PowerScale: %w", err)
	}

	if nodePools == nil || nodePools.NodePools == nil {
		return []NodePoolInfo{}, nil
	}

	pools := make([]NodePoolInfo, 0, len(nodePools.NodePools))
	for _, np := range nodePools.NodePools {
		// Skip node pools without usage data
		if np.Usage == nil {
			continue
		}

		// Convert string values to int64
		total, _ := strconv.ParseInt(np.Usage.TotalBytes, 10, 64)
		used, _ := strconv.ParseInt(np.Usage.UsedBytes, 10, 64)
		avail, _ := strconv.ParseInt(np.Usage.AvailBytes, 10, 64)
		usable, _ := strconv.ParseInt(np.Usage.UsableBytes, 10, 64)

		// Handle null tier
		tier := np.Tier
		if tier == "" {
			tier = unknownTier
		}

		pools = append(pools, NodePoolInfo{
			ID:               np.ID,
			Name:             np.Name,
			Total:            total,
			Used:             used,
			Avail:            avail,
			Usable:           usable,
			ProtectionPolicy: np.ProtectionPolicy,
			Tier:             tier,
		})
	}

	return pools, nil
}
