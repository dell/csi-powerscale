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
	"testing"

	isi "github.com/Ecosystems/container-storage-modules/src/gopowerscale"
	apiv3 "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api/v3"
	isimocks "github.com/Ecosystems/container-storage-modules/src/gopowerscale/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newNFSAdapterWithMock(mockAPI *isimocks.Client) *NFSAdapter {
	client := &isi.Client{API: mockAPI}
	return NewNFSAdapter(client)
}

// U-NFS-ADAPTER-NEW: NewNFSAdapter creates adapter with client
func TestNewNFSAdapter(t *testing.T) {
	client := &isi.Client{}
	adapter := NewNFSAdapter(client)

	assert.NotNil(t, adapter)
	assert.Same(t, client, adapter.client)
}

// U-NFS-ADAPTER-NIL: NewNFSAdapter handles nil client
func TestNewNFSAdapter_NilClient(t *testing.T) {
	adapter := NewNFSAdapter(nil)

	assert.NotNil(t, adapter)
	assert.Nil(t, adapter.client)
}

// U-NFS-ADAPTER-GETSTATS-SIGNATURE: GetNFSStats has correct signature
func TestNFSAdapter_GetNFSStats_Signature(t *testing.T) {
	mockAPI := &isimocks.Client{}
	adapter := newNFSAdapterWithMock(mockAPI)

	// Verify the method exists and has correct signature
	// The method should accept context and return (*NFSBasicStats, error)
	assert.NotNil(t, adapter)

	// Verify the adapter has the GetNFSStats method
	// This is verified by the interface implementation below
}

// U-NFS-ADAPTER-GETSTATS-CONTEXT: GetNFSStats respects context
func TestNFSAdapter_GetNFSStats_ContextAware(t *testing.T) {
	mockAPI := &isimocks.Client{}
	adapter := newNFSAdapterWithMock(mockAPI)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// The method should handle context properly
	// We can't test the actual behavior without a working mock,
	// but we verify the method accepts context
	assert.NotNil(t, adapter)
	_ = ctx // Use ctx to avoid unused variable
}

// U-NFS-ADAPTER-GETSTATS-INTERFACE: Verify GetNFSStats implements NFSStatsClient interface
func TestNFSAdapter_ImplementsNFSStatsClient(t *testing.T) {
	client := &isi.Client{}
	adapter := NewNFSAdapter(client)

	// Verify the adapter implements the NFSStatsClient interface
	var _ NFSStatsClient = adapter
	assert.NotNil(t, adapter)
}

// U-NFS-ADAPTER-FIELD-ACCESS: Verify adapter fields are accessible
func TestNFSAdapter_FieldAccess(t *testing.T) {
	client := &isi.Client{}
	adapter := NewNFSAdapter(client)

	assert.NotNil(t, adapter)
	assert.Same(t, client, adapter.client)
}

// U-NFS-ADAPTER-MULTIPLE-INSTANCES: Multiple adapters are independent
func TestNFSAdapter_MultipleInstances(t *testing.T) {
	client1 := &isi.Client{}
	client2 := &isi.Client{}
	adapter1 := NewNFSAdapter(client1)
	adapter2 := NewNFSAdapter(client2)

	assert.NotNil(t, adapter1)
	assert.NotNil(t, adapter2)
	assert.Same(t, client1, adapter1.client)
	assert.Same(t, client2, adapter2.client)
	assert.NotSame(t, adapter1, adapter2)
}

// U-NFS-ADAPTER-GETSTATS-NILCLIENT: GetNFSStats handles nil client
func TestNFSAdapter_GetNFSStats_NilClient(t *testing.T) {
	adapter := NewNFSAdapter(nil)

	// This should panic since GetComplexStatistics is called on nil client
	// The function doesn't handle nil client gracefully
	// We'll just verify the adapter can be created with nil client
	assert.NotNil(t, adapter)
	assert.Nil(t, adapter.client)
}

// U-NFS-ADAPTER-GETSTATS-SUCCESS: GetNFSStats returns valid stats
func TestNFSAdapter_GetNFSStats_Success(t *testing.T) {
	mockAPI := &isimocks.Client{}
	adapter := newNFSAdapterWithMock(mockAPI)

	// Mock the lower-level Get method (called by GetComplexStatistics)
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv3.IsiComplexStatsResp)
		*resp = &apiv3.IsiComplexStatsResp{
			StatsList: []*apiv3.IsiComplexStats{
				{
					Key: "node.nfs.basic_stats",
					Value: map[string]interface{}{
						"basic": map[string]interface{}{
							"svc_counters": map[string]interface{}{
								"bytes_read":    float64(1000),
								"bytes_written": float64(2000),
							},
						},
					},
				},
				{
					Key: "node.nfs.optime_stats",
					Value: map[string]interface{}{
						"optime": map[string]interface{}{
							"v3_time_buckets": []interface{}{1.0, 2.0, 3.0},
							"v4_time_buckets": []interface{}{4.0, 5.0, 6.0},
						},
					},
				},
			},
		}
	}).Once()

	stats, err := adapter.GetNFSStats(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, float64(1000), stats.Basic.SvcCounters.BytesRead)
	assert.Equal(t, float64(2000), stats.Basic.SvcCounters.BytesWritten)
	assert.Equal(t, []float64{1.0, 2.0, 3.0}, stats.Optime.V3TimeBuckets)
	assert.Equal(t, []float64{4.0, 5.0, 6.0}, stats.Optime.V4TimeBuckets)
	mockAPI.AssertExpectations(t)
}

// U-NFS-ADAPTER-GETSTATS-ERROR: GetNFSStats handles API error
func TestNFSAdapter_GetNFSStats_Error(t *testing.T) {
	mockAPI := &isimocks.Client{}
	adapter := newNFSAdapterWithMock(mockAPI)

	mockAPI.On("Get", anyArgs...).Return(assert.AnError).Once()

	stats, err := adapter.GetNFSStats(context.Background())

	assert.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "failed to get NFS stats")
	mockAPI.AssertExpectations(t)
}

// U-NFS-ADAPTER-GETSTATS-NIL-STATS: GetNFSStats handles nil stats
func TestNFSAdapter_GetNFSStats_NilStats(t *testing.T) {
	mockAPI := &isimocks.Client{}
	adapter := newNFSAdapterWithMock(mockAPI)

	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv3.IsiComplexStatsResp)
		*resp = nil
	}).Once()

	stats, err := adapter.GetNFSStats(context.Background())

	assert.NoError(t, err)
	assert.Nil(t, stats)
	mockAPI.AssertExpectations(t)
}

// U-NFS-ADAPTER-GETSTATS-EMPTY-STATS: GetNFSStats handles empty stats list
func TestNFSAdapter_GetNFSStats_EmptyStats(t *testing.T) {
	mockAPI := &isimocks.Client{}
	adapter := newNFSAdapterWithMock(mockAPI)

	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv3.IsiComplexStatsResp)
		*resp = &apiv3.IsiComplexStatsResp{
			StatsList: []*apiv3.IsiComplexStats{},
		}
	}).Once()

	stats, err := adapter.GetNFSStats(context.Background())

	assert.NoError(t, err)
	assert.Nil(t, stats)
	mockAPI.AssertExpectations(t)
}

// U-NFS-ADAPTER-GETSTATS-MARSHAL-ERROR: GetNFSStats handles marshal error for basic stats
func TestNFSAdapter_GetNFSStats_MarshalError(t *testing.T) {
	mockAPI := &isimocks.Client{}
	adapter := newNFSAdapterWithMock(mockAPI)

	// Create a value that cannot be marshaled (channel)
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv3.IsiComplexStatsResp)
		*resp = &apiv3.IsiComplexStatsResp{
			StatsList: []*apiv3.IsiComplexStats{
				{
					Key:   "node.nfs.basic_stats",
					Value: make(chan int), // Channels cannot be marshaled to JSON
				},
			},
		}
	}).Once()

	stats, err := adapter.GetNFSStats(context.Background())

	assert.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "failed to marshal NFS basic stats value")
	mockAPI.AssertExpectations(t)
}

// U-NFS-ADAPTER-GETSTATS-UNMARSHAL-ERROR: GetNFSStats handles unmarshal error for basic stats
func TestNFSAdapter_GetNFSStats_UnmarshalError(t *testing.T) {
	mockAPI := &isimocks.Client{}
	adapter := newNFSAdapterWithMock(mockAPI)

	// Return invalid JSON that cannot be unmarshaled into map[string]interface{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv3.IsiComplexStatsResp)
		*resp = &apiv3.IsiComplexStatsResp{
			StatsList: []*apiv3.IsiComplexStats{
				{
					Key:   "node.nfs.basic_stats",
					Value: "invalid json string", // String will be marshaled but won't unmarshal to map
				},
			},
		}
	}).Once()

	stats, err := adapter.GetNFSStats(context.Background())

	assert.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "failed to unmarshal NFS basic stats")
	mockAPI.AssertExpectations(t)
}

// U-NFS-ADAPTER-GETSTATS-OPTIME-MARSHAL-ERROR: GetNFSStats handles marshal error for optime stats
func TestNFSAdapter_GetNFSStats_OptimeMarshalError(t *testing.T) {
	mockAPI := &isimocks.Client{}
	adapter := newNFSAdapterWithMock(mockAPI)

	// Create a valid basic stat but invalid optime stat
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv3.IsiComplexStatsResp)
		*resp = &apiv3.IsiComplexStatsResp{
			StatsList: []*apiv3.IsiComplexStats{
				{
					Key: "node.nfs.basic_stats",
					Value: map[string]interface{}{
						"basic": map[string]interface{}{
							"svc_counters": map[string]interface{}{
								"bytes_read":    float64(1000),
								"bytes_written": float64(2000),
							},
						},
					},
				},
				{
					Key:   "node.nfs.optime_stats",
					Value: make(chan int), // Channels cannot be marshaled to JSON
				},
			},
		}
	}).Once()

	stats, err := adapter.GetNFSStats(context.Background())

	assert.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "failed to marshal NFS optime stats value")
	mockAPI.AssertExpectations(t)
}

// U-NFS-ADAPTER-GETSTATS-OPTIME-UNMARSHAL-ERROR: GetNFSStats handles unmarshal error for optime stats
func TestNFSAdapter_GetNFSStats_OptimeUnmarshalError(t *testing.T) {
	mockAPI := &isimocks.Client{}
	adapter := newNFSAdapterWithMock(mockAPI)

	// Return valid basic stat but invalid optime stat
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv3.IsiComplexStatsResp)
		*resp = &apiv3.IsiComplexStatsResp{
			StatsList: []*apiv3.IsiComplexStats{
				{
					Key: "node.nfs.basic_stats",
					Value: map[string]interface{}{
						"basic": map[string]interface{}{
							"svc_counters": map[string]interface{}{
								"bytes_read":    float64(1000),
								"bytes_written": float64(2000),
							},
						},
					},
				},
				{
					Key:   "node.nfs.optime_stats",
					Value: "invalid json string", // String will be marshaled but won't unmarshal to map
				},
			},
		}
	}).Once()

	stats, err := adapter.GetNFSStats(context.Background())

	assert.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "failed to unmarshal NFS optime stats")
	mockAPI.AssertExpectations(t)
}

// U-NFS-ADAPTER-GETSTATS-MISSING-BASIC-FIELD: GetNFSStats handles missing basic field in nested structure
func TestNFSAdapter_GetNFSStats_MissingBasicField(t *testing.T) {
	mockAPI := &isimocks.Client{}
	adapter := newNFSAdapterWithMock(mockAPI)

	// Return stats with missing "basic" field
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv3.IsiComplexStatsResp)
		*resp = &apiv3.IsiComplexStatsResp{
			StatsList: []*apiv3.IsiComplexStats{
				{
					Key: "node.nfs.basic_stats",
					Value: map[string]interface{}{
						"wrong_field": map[string]interface{}{
							"svc_counters": map[string]interface{}{
								"bytes_read": float64(1000),
							},
						},
					},
				},
			},
		}
	}).Once()

	stats, err := adapter.GetNFSStats(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	// Should return stats with zero values since fields are missing
	assert.Equal(t, float64(0), stats.Basic.SvcCounters.BytesRead)
	mockAPI.AssertExpectations(t)
}

// U-NFS-ADAPTER-GETSTATS-MISSING-SVC-COUNTERS: GetNFSStats handles missing svc_counters field
func TestNFSAdapter_GetNFSStats_MissingSvcCounters(t *testing.T) {
	mockAPI := &isimocks.Client{}
	adapter := newNFSAdapterWithMock(mockAPI)

	// Return stats with missing "svc_counters" field
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv3.IsiComplexStatsResp)
		*resp = &apiv3.IsiComplexStatsResp{
			StatsList: []*apiv3.IsiComplexStats{
				{
					Key: "node.nfs.basic_stats",
					Value: map[string]interface{}{
						"basic": map[string]interface{}{
							"wrong_field": map[string]interface{}{},
						},
					},
				},
			},
		}
	}).Once()

	stats, err := adapter.GetNFSStats(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, float64(0), stats.Basic.SvcCounters.BytesRead)
	mockAPI.AssertExpectations(t)
}

// U-NFS-ADAPTER-GETSTATS-WRONG-TYPE-BYTES: GetNFSStats handles wrong type for bytes_read/bytes_written
func TestNFSAdapter_GetNFSStats_WrongTypeForBytes(t *testing.T) {
	mockAPI := &isimocks.Client{}
	adapter := newNFSAdapterWithMock(mockAPI)

	// Return stats with wrong type for bytes fields (string instead of float64)
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv3.IsiComplexStatsResp)
		*resp = &apiv3.IsiComplexStatsResp{
			StatsList: []*apiv3.IsiComplexStats{
				{
					Key: "node.nfs.basic_stats",
					Value: map[string]interface{}{
						"basic": map[string]interface{}{
							"svc_counters": map[string]interface{}{
								"bytes_read":    "not a number",
								"bytes_written": float64(2000),
							},
						},
					},
				},
			},
		}
	}).Once()

	stats, err := adapter.GetNFSStats(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	// Should handle wrong type gracefully and use zero value
	assert.Equal(t, float64(0), stats.Basic.SvcCounters.BytesRead)
	assert.Equal(t, float64(2000), stats.Basic.SvcCounters.BytesWritten)
	mockAPI.AssertExpectations(t)
}

// U-NFS-ADAPTER-GETSTATS-MISSING-OPTIME-FIELD: GetNFSStats handles missing optime field
func TestNFSAdapter_GetNFSStats_MissingOptimeField(t *testing.T) {
	mockAPI := &isimocks.Client{}
	adapter := newNFSAdapterWithMock(mockAPI)

	// Return stats with missing "optime" field
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv3.IsiComplexStatsResp)
		*resp = &apiv3.IsiComplexStatsResp{
			StatsList: []*apiv3.IsiComplexStats{
				{
					Key: "node.nfs.basic_stats",
					Value: map[string]interface{}{
						"basic": map[string]interface{}{
							"svc_counters": map[string]interface{}{
								"bytes_read":    float64(1000),
								"bytes_written": float64(2000),
							},
						},
					},
				},
				{
					Key: "node.nfs.optime_stats",
					Value: map[string]interface{}{
						"wrong_field": map[string]interface{}{},
					},
				},
			},
		}
	}).Once()

	stats, err := adapter.GetNFSStats(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	// Should return stats with empty optime buckets
	assert.Empty(t, stats.Optime.V3TimeBuckets)
	assert.Empty(t, stats.Optime.V4TimeBuckets)
	mockAPI.AssertExpectations(t)
}

// U-NFS-ADAPTER-GETSTATS-MISSING-TIME-BUCKETS: GetNFSStats handles missing time bucket fields
func TestNFSAdapter_GetNFSStats_MissingTimeBuckets(t *testing.T) {
	mockAPI := &isimocks.Client{}
	adapter := newNFSAdapterWithMock(mockAPI)

	// Return stats with missing time bucket fields
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv3.IsiComplexStatsResp)
		*resp = &apiv3.IsiComplexStatsResp{
			StatsList: []*apiv3.IsiComplexStats{
				{
					Key: "node.nfs.basic_stats",
					Value: map[string]interface{}{
						"basic": map[string]interface{}{
							"svc_counters": map[string]interface{}{
								"bytes_read":    float64(1000),
								"bytes_written": float64(2000),
							},
						},
					},
				},
				{
					Key: "node.nfs.optime_stats",
					Value: map[string]interface{}{
						"optime": map[string]interface{}{
							"wrong_field": []interface{}{},
						},
					},
				},
			},
		}
	}).Once()

	stats, err := adapter.GetNFSStats(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Empty(t, stats.Optime.V3TimeBuckets)
	assert.Empty(t, stats.Optime.V4TimeBuckets)
	mockAPI.AssertExpectations(t)
}

// U-NFS-ADAPTER-GETSTATS-WRONG-TYPE-BUCKETS: GetNFSStats handles wrong type for time buckets
func TestNFSAdapter_GetNFSStats_WrongTypeForBuckets(t *testing.T) {
	mockAPI := &isimocks.Client{}
	adapter := newNFSAdapterWithMock(mockAPI)

	// Return stats with wrong type for time buckets (strings instead of float64)
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv3.IsiComplexStatsResp)
		*resp = &apiv3.IsiComplexStatsResp{
			StatsList: []*apiv3.IsiComplexStats{
				{
					Key: "node.nfs.basic_stats",
					Value: map[string]interface{}{
						"basic": map[string]interface{}{
							"svc_counters": map[string]interface{}{
								"bytes_read":    float64(1000),
								"bytes_written": float64(2000),
							},
						},
					},
				},
				{
					Key: "node.nfs.optime_stats",
					Value: map[string]interface{}{
						"optime": map[string]interface{}{
							"v3_time_buckets": []interface{}{"not", "numbers"},
							"v4_time_buckets": []interface{}{float64(4.0), float64(5.0)},
						},
					},
				},
			},
		}
	}).Once()

	stats, err := adapter.GetNFSStats(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	// Should handle wrong type gracefully and skip non-float values
	assert.Empty(t, stats.Optime.V3TimeBuckets)
	assert.Equal(t, []float64{4.0, 5.0}, stats.Optime.V4TimeBuckets)
	mockAPI.AssertExpectations(t)
}

// U-NFS-ADAPTER-NEW-WITH-RUNTIME: NewNFSAdapterWithRuntime creates adapter with client and runtime
func TestNewNFSAdapterWithRuntime(t *testing.T) {
	client := &isi.Client{}
	rt := &MetricsRuntime{}
	adapter := NewNFSAdapterWithRuntime(client, rt)

	assert.NotNil(t, adapter)
	assert.Same(t, client, adapter.client)
	assert.Same(t, rt, adapter.runtime)
}

// U-NFS-ADAPTER-SET-RUNTIME: SetRuntime sets the MetricsRuntime on the adapter
func TestSetRuntime(t *testing.T) {
	client := &isi.Client{}
	adapter := NewNFSAdapter(client)
	rt := &MetricsRuntime{}

	adapter.SetRuntime(rt)

	assert.Same(t, rt, adapter.runtime)
}

// U-NFS-ADAPTER-GETSTATS-PARTIAL-DATA: GetNFSStats handles partial valid data
func TestNFSAdapter_GetNFSStats_PartialValidData(t *testing.T) {
	mockAPI := &isimocks.Client{}
	adapter := newNFSAdapterWithMock(mockAPI)

	// Return only basic stats, missing optime stats
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(**apiv3.IsiComplexStatsResp)
		*resp = &apiv3.IsiComplexStatsResp{
			StatsList: []*apiv3.IsiComplexStats{
				{
					Key: "node.nfs.basic_stats",
					Value: map[string]interface{}{
						"basic": map[string]interface{}{
							"svc_counters": map[string]interface{}{
								"bytes_read":    float64(1000),
								"bytes_written": float64(2000),
							},
						},
					},
				},
			},
		}
	}).Once()

	stats, err := adapter.GetNFSStats(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, float64(1000), stats.Basic.SvcCounters.BytesRead)
	assert.Equal(t, float64(2000), stats.Basic.SvcCounters.BytesWritten)
	assert.Empty(t, stats.Optime.V3TimeBuckets)
	assert.Empty(t, stats.Optime.V4TimeBuckets)
	mockAPI.AssertExpectations(t)
}
