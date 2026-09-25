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
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockNFSStatsClient is a mock implementation of NFSStatsClient
type mockNFSStatsClient struct {
	stats *NFSBasicStats
	err   error
}

func (m *mockNFSStatsClient) GetNFSStats(_ context.Context) (*NFSBasicStats, error) {
	return m.stats, m.err
}

// U-NFS-NEW: NewNFSPerformanceCollector creates collector with metrics
func TestNewNFSPerformanceCollector(t *testing.T) {
	reg := prometheus.NewRegistry()
	mockClient := &mockNFSStatsClient{}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	assert.NotNil(t, c)
	assert.Equal(t, "test-cluster", c.clusterName)
	assert.NotNil(t, c.bytesRead)
	assert.NotNil(t, c.bytesWrite)
	assert.NotNil(t, c.latencyV3)
	assert.NotNil(t, c.latencyV4)
}

// U-NFS-NAME: Name returns correct collector name
func TestNFSPerformanceCollector_Name(t *testing.T) {
	reg := prometheus.NewRegistry()
	mockClient := &mockNFSStatsClient{}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	assert.Equal(t, "NFSPerformanceCollector", c.Name())
}

// U-NFS-COLLECT-SUCCESS: Collect successfully updates metrics
func TestNFSPerformanceCollector_Collect_Success(t *testing.T) {
	reg := prometheus.NewRegistry()
	stats := &NFSBasicStats{
		Basic: struct {
			SvcCounters struct {
				BytesRead    float64 `json:"bytes_read"`
				BytesWritten float64 `json:"bytes_written"`
			} `json:"svc_counters"`
		}{
			SvcCounters: struct {
				BytesRead    float64 `json:"bytes_read"`
				BytesWritten float64 `json:"bytes_written"`
			}{
				BytesRead:    1024.0,
				BytesWritten: 2048.0,
			},
		},
		Optime: struct {
			V3TimeBuckets []float64 `json:"v3_time_buckets"`
			V4TimeBuckets []float64 `json:"v4_time_buckets"`
		}{
			V3TimeBuckets: []float64{10, 20, 30},
			V4TimeBuckets: []float64{5, 15, 25},
		},
	}
	mockClient := &mockNFSStatsClient{stats: stats}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	err := c.Collect(context.Background())
	require.NoError(t, err)
}

// U-NFS-COLLECT-NIL: Collect handles nil stats gracefully
func TestNFSPerformanceCollector_Collect_NilStats(t *testing.T) {
	reg := prometheus.NewRegistry()
	mockClient := &mockNFSStatsClient{stats: nil}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	err := c.Collect(context.Background())
	require.NoError(t, err)
}

// U-NFS-COLLECT-ERROR: Collect propagates client errors
func TestNFSPerformanceCollector_Collect_Error(t *testing.T) {
	reg := prometheus.NewRegistry()
	mockClient := &mockNFSStatsClient{err: errors.New("API error")}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	err := c.Collect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get NFS stats")
}

// U-NFS-COLLECT-ZERO-BUCKETS: Collect handles zero bucket values
func TestNFSPerformanceCollector_Collect_ZeroBuckets(t *testing.T) {
	reg := prometheus.NewRegistry()
	stats := &NFSBasicStats{
		Basic: struct {
			SvcCounters struct {
				BytesRead    float64 `json:"bytes_read"`
				BytesWritten float64 `json:"bytes_written"`
			} `json:"svc_counters"`
		}{
			SvcCounters: struct {
				BytesRead    float64 `json:"bytes_read"`
				BytesWritten float64 `json:"bytes_written"`
			}{
				BytesRead:    0,
				BytesWritten: 0,
			},
		},
		Optime: struct {
			V3TimeBuckets []float64 `json:"v3_time_buckets"`
			V4TimeBuckets []float64 `json:"v4_time_buckets"`
		}{
			V3TimeBuckets: []float64{0, 0, 0},
			V4TimeBuckets: []float64{0, 0, 0},
		},
	}
	mockClient := &mockNFSStatsClient{stats: stats}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	err := c.Collect(context.Background())
	require.NoError(t, err)
}

// U-NFS-COLLECT-EMPTY-BUCKETS: Collect handles empty bucket arrays
func TestNFSPerformanceCollector_Collect_EmptyBuckets(t *testing.T) {
	reg := prometheus.NewRegistry()
	stats := &NFSBasicStats{
		Basic: struct {
			SvcCounters struct {
				BytesRead    float64 `json:"bytes_read"`
				BytesWritten float64 `json:"bytes_written"`
			} `json:"svc_counters"`
		}{
			SvcCounters: struct {
				BytesRead    float64 `json:"bytes_read"`
				BytesWritten float64 `json:"bytes_written"`
			}{
				BytesRead:    512.0,
				BytesWritten: 1024.0,
			},
		},
		Optime: struct {
			V3TimeBuckets []float64 `json:"v3_time_buckets"`
			V4TimeBuckets []float64 `json:"v4_time_buckets"`
		}{
			V3TimeBuckets: []float64{},
			V4TimeBuckets: []float64{},
		},
	}
	mockClient := &mockNFSStatsClient{stats: stats}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	err := c.Collect(context.Background())
	require.NoError(t, err)
}

// U-NFS-CLEANUP: Cleanup removes metrics
func TestNFSPerformanceCollector_Cleanup(t *testing.T) {
	reg := prometheus.NewRegistry()
	mockClient := &mockNFSStatsClient{}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	// Should not panic
	assert.NotPanics(t, func() {
		c.Cleanup()
	})
}

// U-NFS-COLLECT-PARTIAL: Collect with partial data (only basic stats)
func TestNFSPerformanceCollector_Collect_PartialData(t *testing.T) {
	reg := prometheus.NewRegistry()
	stats := &NFSBasicStats{
		Basic: struct {
			SvcCounters struct {
				BytesRead    float64 `json:"bytes_read"`
				BytesWritten float64 `json:"bytes_written"`
			} `json:"svc_counters"`
		}{
			SvcCounters: struct {
				BytesRead    float64 `json:"bytes_read"`
				BytesWritten float64 `json:"bytes_written"`
			}{
				BytesRead:    2048.0,
				BytesWritten: 4096.0,
			},
		},
		Optime: struct {
			V3TimeBuckets []float64 `json:"v3_time_buckets"`
			V4TimeBuckets []float64 `json:"v4_time_buckets"`
		}{
			V3TimeBuckets: nil,
			V4TimeBuckets: nil,
		},
	}
	mockClient := &mockNFSStatsClient{stats: stats}
	c := NewNFSPerformanceCollector(mockClient, reg, "test-cluster")

	err := c.Collect(context.Background())
	require.NoError(t, err)
}
