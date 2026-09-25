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
	"time"

	isi "github.com/Ecosystems/container-storage-modules/src/gopowerscale"
	apiv1 "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api/v1"
	isimocks "github.com/Ecosystems/container-storage-modules/src/gopowerscale/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newNodePoolAdapterWithMock(mockAPI *isimocks.Client) *NodePoolAdapter {
	client := &isi.Client{API: mockAPI}
	return NewNodePoolAdapter(client)
}

func TestNewNodePoolAdapter(t *testing.T) {
	client := &isi.Client{}
	adapter := NewNodePoolAdapter(client)
	assert.NotNil(t, adapter)
}

func TestNodePoolAdapter_GetNodePools_Success(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv1.IsiNodePoolsResp)
		*resp = apiv1.IsiNodePoolsResp{
			NodePools: []*apiv1.IsiNodePool{
				{
					ID:               1,
					Name:             "pool1",
					ProtectionPolicy: "+2d:1n",
					Tier:             "ssd",
					Usage: &apiv1.IsiNodePoolUsage{
						TotalBytes:  "1000",
						UsedBytes:   "500",
						AvailBytes:  "500",
						UsableBytes: "450",
					},
				},
			},
		}
	}).Once()

	adapter := newNodePoolAdapterWithMock(mockAPI)
	pools, err := adapter.GetNodePools(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, pools)
	assert.Len(t, pools, 1)
	assert.Equal(t, int64(1000), pools[0].Total)
	assert.Equal(t, int64(500), pools[0].Used)
}

func TestNodePoolAdapter_GetNodePools_Error(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("api error")).Once()

	adapter := newNodePoolAdapterWithMock(mockAPI)
	pools, err := adapter.GetNodePools(context.Background())
	assert.Error(t, err)
	assert.Nil(t, pools)
}

func TestNodePoolAdapter_GetNodePools_NilResponse(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()

	adapter := newNodePoolAdapterWithMock(mockAPI)
	pools, err := adapter.GetNodePools(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, pools)
}

func TestNodePoolAdapter_GetNodePools_NilNodePools(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv1.IsiNodePoolsResp)
		*resp = apiv1.IsiNodePoolsResp{NodePools: nil}
	}).Once()

	adapter := newNodePoolAdapterWithMock(mockAPI)
	pools, err := adapter.GetNodePools(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, pools)
}

func TestNewNodePoolAdapterWithRuntime(t *testing.T) {
	rt := NewMetricsRuntime("cluster1", RuntimeConfig{
		Timeout: time.Second, CacheTTL: time.Minute,
		RateLimit: 100, CBThreshold: 10, CBResetTimeout: time.Minute,
	})
	adapter := NewNodePoolAdapterWithRuntime(&isi.Client{}, rt)
	assert.NotNil(t, adapter)
	assert.NotNil(t, adapter.runtime)
}

func TestNodePoolAdapterWithRuntime_GetNodePools_SuccessPath(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv1.IsiNodePoolsResp)
		*resp = apiv1.IsiNodePoolsResp{
			NodePools: []*apiv1.IsiNodePool{
				{
					ID:               1,
					Name:             "pool1",
					ProtectionPolicy: "+2d:1n",
					Tier:             "ssd",
					Usage: &apiv1.IsiNodePoolUsage{
						TotalBytes:  "1000",
						UsedBytes:   "500",
						AvailBytes:  "500",
						UsableBytes: "450",
					},
				},
			},
		}
	}).Once()

	rt := NewMetricsRuntime("cluster1", RuntimeConfig{
		Timeout: time.Second, CacheTTL: time.Minute,
		RateLimit: 100, CBThreshold: 10, CBResetTimeout: time.Minute,
	})
	adapter := NewNodePoolAdapterWithRuntime(&isi.Client{API: mockAPI}, rt)
	pools, err := adapter.GetNodePools(context.Background())
	require.NoError(t, err)
	assert.Len(t, pools, 1)
	assert.Equal(t, "pool1", pools[0].Name)
}

func TestNodePoolAdapterWithRuntime_GetNodePools_StaleOnCacheHit(t *testing.T) {
	staleCalled := false
	rt := NewMetricsRuntime("cluster1", RuntimeConfig{
		Timeout: time.Second, CacheTTL: time.Minute,
		RateLimit: 100, CBThreshold: 10, CBResetTimeout: time.Minute,
		StaleReporter: func(_ string, s bool) { staleCalled = s },
	})

	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv1.IsiNodePoolsResp)
		*resp = apiv1.IsiNodePoolsResp{
			NodePools: []*apiv1.IsiNodePool{
				{
					ID:               1,
					Name:             "pool1",
					ProtectionPolicy: "+2d:1n",
					Tier:             "ssd",
					Usage: &apiv1.IsiNodePoolUsage{
						TotalBytes:  "1000",
						UsedBytes:   "500",
						AvailBytes:  "500",
						UsableBytes: "450",
					},
				},
			},
		}
	}).Once()
	mockAPI.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("api down")).Once()

	adapter := NewNodePoolAdapterWithRuntime(&isi.Client{API: mockAPI}, rt)
	_, err := adapter.GetNodePools(context.Background())
	require.NoError(t, err)

	pools, err := adapter.GetNodePools(context.Background())
	require.NoError(t, err, "cache hit must not return error")
	assert.Len(t, pools, 1)
	assert.True(t, staleCalled, "stale reporter must be called on cache fallback")
}

func TestNodePoolAdapter_GetNodePools_NilUsage(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv1.IsiNodePoolsResp)
		*resp = apiv1.IsiNodePoolsResp{
			NodePools: []*apiv1.IsiNodePool{
				{
					ID:               1,
					Name:             "pool1",
					ProtectionPolicy: "+2d:1n",
					Tier:             "ssd",
					Usage:            nil, // Nil usage should be skipped
				},
			},
		}
	}).Once()

	adapter := newNodePoolAdapterWithMock(mockAPI)
	pools, err := adapter.GetNodePools(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, pools, "node pools with nil usage should be skipped")
}

func TestNodePoolAdapter_GetNodePools_NullTier(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv1.IsiNodePoolsResp)
		*resp = apiv1.IsiNodePoolsResp{
			NodePools: []*apiv1.IsiNodePool{
				{
					ID:               1,
					Name:             "pool1",
					ProtectionPolicy: "+2d:1n",
					Tier:             "", // Empty tier should become "unknown"
					Usage: &apiv1.IsiNodePoolUsage{
						TotalBytes:  "1000",
						UsedBytes:   "500",
						AvailBytes:  "500",
						UsableBytes: "450",
					},
				},
			},
		}
	}).Once()

	adapter := newNodePoolAdapterWithMock(mockAPI)
	pools, err := adapter.GetNodePools(context.Background())
	assert.NoError(t, err)
	assert.Len(t, pools, 1)
	assert.Equal(t, "unknown", pools[0].Tier)
}

func TestNodePoolAdapter_GetNodePools_EmptyNodePools(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv1.IsiNodePoolsResp)
		*resp = apiv1.IsiNodePoolsResp{
			NodePools: []*apiv1.IsiNodePool{},
		}
	}).Once()

	adapter := newNodePoolAdapterWithMock(mockAPI)
	pools, err := adapter.GetNodePools(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, pools)
}

func TestNodePoolAdapter_GetNodePools_MultiplePools(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv1.IsiNodePoolsResp)
		*resp = apiv1.IsiNodePoolsResp{
			NodePools: []*apiv1.IsiNodePool{
				{
					ID:               1,
					Name:             "pool1",
					ProtectionPolicy: "+2d:1n",
					Tier:             "ssd",
					Usage: &apiv1.IsiNodePoolUsage{
						TotalBytes:  "1000",
						UsedBytes:   "500",
						AvailBytes:  "500",
						UsableBytes: "450",
					},
				},
				{
					ID:               2,
					Name:             "pool2",
					ProtectionPolicy: "+1d:1n",
					Tier:             "hdd",
					Usage: &apiv1.IsiNodePoolUsage{
						TotalBytes:  "2000",
						UsedBytes:   "1000",
						AvailBytes:  "1000",
						UsableBytes: "900",
					},
				},
			},
		}
	}).Once()

	adapter := newNodePoolAdapterWithMock(mockAPI)
	pools, err := adapter.GetNodePools(context.Background())
	assert.NoError(t, err)
	assert.Len(t, pools, 2)
}
