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
	"encoding/json"
	"fmt"

	"github.com/Ecosystems/container-storage-modules/src/csmlog"
	isi "github.com/Ecosystems/container-storage-modules/src/gopowerscale"
)

// NFSAdapter fetches NFS performance statistics from OneFS
type NFSAdapter struct {
	client  *isi.Client
	runtime *MetricsRuntime
}

// NewNFSAdapter creates a new NFSAdapter
func NewNFSAdapter(client *isi.Client) *NFSAdapter {
	return &NFSAdapter{client: client}
}

// NewNFSAdapterWithRuntime creates a new NFSAdapter that routes collection
// calls through the provided MetricsRuntime for timeout, rate limiting,
// circuit breaking, and caching.
func NewNFSAdapterWithRuntime(client *isi.Client, rt *MetricsRuntime) *NFSAdapter {
	return &NFSAdapter{client: client, runtime: rt}
}

// SetRuntime sets the MetricsRuntime on the adapter for timeout, rate limiting,
// circuit breaking, and caching.
func (a *NFSAdapter) SetRuntime(rt *MetricsRuntime) {
	a.runtime = rt
}

// GetNFSStats fetches NFS basic stats and optime stats from OneFS using gopowerscale GetComplexStatistics
func (a *NFSAdapter) GetNFSStats(ctx context.Context) (*NFSBasicStats, error) {
	stats, err := a.client.GetComplexStatistics(ctx, []string{"node.nfs.basic_stats", "node.nfs.optime_stats"})
	if err != nil {
		csmlog.Errorf("GetNFSStats: failed to get NFS stats: %v", err)
		return nil, fmt.Errorf("failed to get NFS stats: %w", err)
	}

	if stats == nil || len(stats.StatsList) == 0 {
		return nil, nil // No stats available
	}

	nfsStats := &NFSBasicStats{}

	// Parse the stats - the API returns complex JSON
	for _, stat := range stats.StatsList {
		if stat == nil {
			continue
		}

		if stat.Key == "node.nfs.basic_stats" {
			// The value is a JSON object that needs to be marshaled then unmarshaled
			valueBytes, err := json.Marshal(stat.Value)
			if err != nil {
				csmlog.Errorf("GetNFSStats: failed to marshal NFS basic stats value: %v", err)
				return nil, fmt.Errorf("failed to marshal NFS basic stats value: %w", err)
			}

			// Try to unmarshal into a map first to handle dynamic structure
			var basicStats map[string]interface{}
			if err := json.Unmarshal(valueBytes, &basicStats); err != nil {
				csmlog.Errorf("GetNFSStats: failed to unmarshal NFS basic stats: %v", err)
				return nil, fmt.Errorf("failed to unmarshal NFS basic stats: %w", err)
			}

			// Extract bytes_read and bytes_written from the nested structure
			if basic, ok := basicStats["basic"].(map[string]interface{}); ok {
				if svcCounters, ok := basic["svc_counters"].(map[string]interface{}); ok {
					if bytesRead, ok := svcCounters["bytes_read"].(float64); ok {
						nfsStats.Basic.SvcCounters.BytesRead = bytesRead
					}
					if bytesWritten, ok := svcCounters["bytes_written"].(float64); ok {
						nfsStats.Basic.SvcCounters.BytesWritten = bytesWritten
					}
				}
			}
		}
		if stat.Key == "node.nfs.optime_stats" {
			// The value is a JSON object that needs to be marshaled then unmarshaled
			valueBytes, err := json.Marshal(stat.Value)
			if err != nil {
				csmlog.Errorf("GetNFSStats: failed to marshal NFS optime stats value: %v", err)
				return nil, fmt.Errorf("failed to marshal NFS optime stats value: %w", err)
			}

			// Try to unmarshal into a map first to handle dynamic structure
			var optimeStats map[string]interface{}
			if err := json.Unmarshal(valueBytes, &optimeStats); err != nil {
				csmlog.Errorf("GetNFSStats: failed to unmarshal NFS optime stats: %v", err)
				return nil, fmt.Errorf("failed to unmarshal NFS optime stats: %w", err)
			}

			// Extract v3_time_buckets and v4_time_buckets from the nested structure
			if optime, ok := optimeStats["optime"].(map[string]interface{}); ok {
				if v3TimeBuckets, ok := optime["v3_time_buckets"].([]interface{}); ok {
					for _, bucket := range v3TimeBuckets {
						if bucketVal, ok := bucket.(float64); ok {
							nfsStats.Optime.V3TimeBuckets = append(nfsStats.Optime.V3TimeBuckets, bucketVal)
						}
					}
				}
				if v4TimeBuckets, ok := optime["v4_time_buckets"].([]interface{}); ok {
					for _, bucket := range v4TimeBuckets {
						if bucketVal, ok := bucket.(float64); ok {
							nfsStats.Optime.V4TimeBuckets = append(nfsStats.Optime.V4TimeBuckets, bucketVal)
						}
					}
				}
			}
		}
	}

	return nfsStats, nil
}
