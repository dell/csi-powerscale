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

	"github.com/Ecosystems/container-storage-modules/src/csm-metrics-common/pkg/naming"
	"github.com/prometheus/client_golang/prometheus"
)

// NFSBasicStats represents the basic NFS statistics from OneFS
type NFSBasicStats struct {
	Basic struct {
		SvcCounters struct {
			BytesRead    float64 `json:"bytes_read"`
			BytesWritten float64 `json:"bytes_written"`
		} `json:"svc_counters"`
	} `json:"basic"`
	Optime struct {
		V3TimeBuckets []float64 `json:"v3_time_buckets"`
		V4TimeBuckets []float64 `json:"v4_time_buckets"`
	} `json:"optime"`
}

// NFSStatsClient is the minimal interface for fetching NFS statistics
type NFSStatsClient interface {
	GetNFSStats(ctx context.Context) (*NFSBasicStats, error)
}

// NFSPerformanceCollector collects PowerScale NFS performance metrics
type NFSPerformanceCollector struct {
	client      NFSStatsClient
	clusterName string
	bytesRead   *prometheus.GaugeVec
	bytesWrite  *prometheus.GaugeVec
	latencyV3   *prometheus.HistogramVec
	latencyV4   *prometheus.HistogramVec
}

// NewNFSPerformanceCollector creates a new NFSPerformanceCollector and registers its metrics
func NewNFSPerformanceCollector(client NFSStatsClient, reg prometheus.Registerer, clusterName string) *NFSPerformanceCollector {
	c := &NFSPerformanceCollector{
		client:      client,
		clusterName: clusterName,
		bytesRead: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_powerscale_nfs_read_throughput_bytes",
			Help: "PowerScale cluster NFS read throughput in bytes per second.",
		}, []string{naming.LabelClusterName})),
		bytesWrite: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_powerscale_nfs_write_throughput_bytes",
			Help: "PowerScale cluster NFS write throughput in bytes per second.",
		}, []string{naming.LabelClusterName})),
		latencyV3: registerOrGetHistogramVec(reg, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "dell_powerscale_nfs_v3_latency_seconds",
			Help:    "PowerScale cluster NFSv3 operation latency distribution in seconds.",
			Buckets: naming.HistogramBuckets,
		}, []string{naming.LabelClusterName})),
		latencyV4: registerOrGetHistogramVec(reg, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "dell_powerscale_nfs_v4_latency_seconds",
			Help:    "PowerScale cluster NFSv4 operation latency distribution in seconds.",
			Buckets: naming.HistogramBuckets,
		}, []string{naming.LabelClusterName})),
	}
	return c
}

// Collect fetches NFS performance statistics and updates the Prometheus gauges
func (c *NFSPerformanceCollector) Collect(ctx context.Context) error {
	if c.client == nil {
		return fmt.Errorf("NFSPerformanceCollector: client is nil")
	}

	stats, err := c.client.GetNFSStats(ctx)
	if err != nil {
		return fmt.Errorf("NFSPerformanceCollector: failed to get NFS stats: %w", err)
	}

	if stats != nil {
		c.bytesRead.WithLabelValues(c.clusterName).Set(stats.Basic.SvcCounters.BytesRead)
		c.bytesWrite.WithLabelValues(c.clusterName).Set(stats.Basic.SvcCounters.BytesWritten)
	}

	// Process latency buckets if available
	if stats != nil && len(stats.Optime.V3TimeBuckets) > 0 {
		// Convert bucket counts to histogram observations
		// The buckets are in microseconds, convert to seconds
		bucketBounds := naming.HistogramBuckets
		hasNonZero := false
		for i, count := range stats.Optime.V3TimeBuckets {
			if count > 0 && i < len(bucketBounds) {
				hasNonZero = true
				// Observe the bucket bound value for each count in the bucket
				// Note: This is an approximation - we observe the bucket bound value
				// since the actual latency values aren't provided in the API response
				for j := 0; j < int(count); j++ {
					c.latencyV3.WithLabelValues(c.clusterName).Observe(bucketBounds[i])
				}
			}
		}
		// If all buckets are zero, observe a zero value to ensure metric appears in Prometheus
		if !hasNonZero {
			c.latencyV3.WithLabelValues(c.clusterName).Observe(0)
		}
	}

	if stats != nil && len(stats.Optime.V4TimeBuckets) > 0 {
		// Convert bucket counts to histogram observations
		// The buckets are in microseconds, convert to seconds
		bucketBounds := naming.HistogramBuckets
		hasNonZero := false
		for i, count := range stats.Optime.V4TimeBuckets {
			if count > 0 && i < len(bucketBounds) {
				hasNonZero = true
				// Observe the bucket bound value for each count in the bucket
				// Note: This is an approximation - we observe the bucket bound value
				// since the actual latency values aren't provided in the API response
				for j := 0; j < int(count); j++ {
					c.latencyV4.WithLabelValues(c.clusterName).Observe(bucketBounds[i])
				}
			}
		}
		// If all buckets are zero, observe a zero value to ensure metric appears in Prometheus
		if !hasNonZero {
			c.latencyV4.WithLabelValues(c.clusterName).Observe(0)
		}
	}

	return nil
}

// Name returns the collector name
func (c *NFSPerformanceCollector) Name() string { return "NFSPerformanceCollector" }

// Cleanup removes all NFS performance metrics for this cluster
func (c *NFSPerformanceCollector) Cleanup() {
	labels := prometheus.Labels{naming.LabelClusterName: c.clusterName}
	c.bytesRead.DeletePartialMatch(labels)
	c.bytesWrite.DeletePartialMatch(labels)
	c.latencyV3.DeletePartialMatch(labels)
	c.latencyV4.DeletePartialMatch(labels)
}
