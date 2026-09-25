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
	apiv2 "github.com/Ecosystems/container-storage-modules/src/gopowerscale/api/v2"
	isimocks "github.com/Ecosystems/container-storage-modules/src/gopowerscale/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var anyArgs = []interface{}{mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything}

func newAccessControlAdapterWithMock(mockAPI *isimocks.Client) *AccessControlAdapter {
	client := &isi.Client{API: mockAPI}
	return NewAccessControlAdapter(client)
}

func TestNewAccessControlAdapter(t *testing.T) {
	client := &isi.Client{}
	adapter := NewAccessControlAdapter(client)
	assert.NotNil(t, adapter)
}

func TestAccessControlAdapter_GetZones_Success(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv1.GetIsiZonesResp)
		*resp = apiv1.GetIsiZonesResp{
			Zones: []*apiv1.IsiZone{{Name: "System"}, {Name: "zone1"}},
		}
	}).Once()

	adapter := newAccessControlAdapterWithMock(mockAPI)
	zones, err := adapter.GetZones(context.Background())
	assert.NoError(t, err)
	assert.Len(t, zones, 2)
	assert.Equal(t, "System", zones[0].Name)
}

func TestAccessControlAdapter_GetZones_Error(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(errors.New("api error")).Once()

	adapter := newAccessControlAdapterWithMock(mockAPI)
	zones, err := adapter.GetZones(context.Background())
	assert.Error(t, err)
	assert.Nil(t, zones)
}

func TestAccessControlAdapter_GetZones_NilZones(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv1.GetIsiZonesResp)
		*resp = apiv1.GetIsiZonesResp{Zones: nil}
	}).Once()

	adapter := newAccessControlAdapterWithMock(mockAPI)
	zones, err := adapter.GetZones(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, zones)
}

func TestAccessControlAdapter_GetZones_NilClient(t *testing.T) {
	adapter := NewAccessControlAdapter(nil)
	zones, err := adapter.GetZones(context.Background())
	assert.Error(t, err)
	assert.Nil(t, zones)
	assert.Contains(t, err.Error(), "PowerScale client not yet initialized")
}

func TestAccessControlAdapter_GetExportCountByZone_NilExports(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv2.Exports)
		*resp = apiv2.Exports{Total: 0}
	}).Once()

	adapter := newAccessControlAdapterWithMock(mockAPI)
	count, err := adapter.GetExportCountByZone(context.Background(), "System")
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestAccessControlAdapter_GetExportCountByZone_EmptyZone(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv2.Exports)
		*resp = apiv2.Exports{Total: 1, Exports: []*apiv2.Export{{ID: 1}}}
	}).Once()

	adapter := newAccessControlAdapterWithMock(mockAPI)
	count, err := adapter.GetExportCountByZone(context.Background(), "")
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestAccessControlAdapter_GetExportCountByZone_System(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv2.Exports)
		*resp = apiv2.Exports{Total: 2, Exports: []*apiv2.Export{{ID: 1}, {ID: 2}}}
	}).Once()

	adapter := newAccessControlAdapterWithMock(mockAPI)
	count, err := adapter.GetExportCountByZone(context.Background(), "System")
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestAccessControlAdapter_GetExportCountByZone_SystemError(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(errors.New("export error")).Once()

	adapter := newAccessControlAdapterWithMock(mockAPI)
	count, err := adapter.GetExportCountByZone(context.Background(), "System")
	assert.Error(t, err)
	assert.Equal(t, 0, count)
}

func TestAccessControlAdapter_GetExportCountByZone_OtherZone(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv2.Exports)
		*resp = apiv2.Exports{Total: 0}
	}).Once()

	adapter := newAccessControlAdapterWithMock(mockAPI)
	count, err := adapter.GetExportCountByZone(context.Background(), "customzone")
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestAccessControlAdapter_GetExportCountByZone_NilClient(t *testing.T) {
	adapter := NewAccessControlAdapter(nil)
	count, err := adapter.GetExportCountByZone(context.Background(), "System")
	assert.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "PowerScale client not yet initialized")
}

func TestAccessControlAdapter_GetExportCountByZone_WithRuntime_Success(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv2.Exports)
		*resp = apiv2.Exports{Total: 3, Exports: []*apiv2.Export{{ID: 1}, {ID: 2}, {ID: 3}}}
	}).Once()

	rt := NewMetricsRuntime("cluster1", RuntimeConfig{
		Timeout: time.Second, CacheTTL: time.Minute,
		RateLimit: 100, CBThreshold: 10, CBResetTimeout: time.Minute,
	})
	adapter := NewAccessControlAdapterWithRuntime(&isi.Client{API: mockAPI}, rt)

	count, err := adapter.GetExportCountByZone(context.Background(), "System")
	assert.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestNewAccessControlAdapterWithRuntime(t *testing.T) {
	rt := NewMetricsRuntime("cluster1", RuntimeConfig{
		Timeout: time.Second, CacheTTL: time.Minute,
		RateLimit: 100, CBThreshold: 10, CBResetTimeout: time.Minute,
	})
	adapter := NewAccessControlAdapterWithRuntime(&isi.Client{}, rt)
	assert.NotNil(t, adapter)
	assert.NotNil(t, adapter.runtime)
}

func TestAccessControlAdapterWithRuntime_GetZones_SuccessPath(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv1.GetIsiZonesResp)
		*resp = apiv1.GetIsiZonesResp{
			Zones: []*apiv1.IsiZone{{Name: "System"}},
		}
	}).Once()

	rt := NewMetricsRuntime("cluster1", RuntimeConfig{
		Timeout: time.Second, CacheTTL: time.Minute,
		RateLimit: 100, CBThreshold: 10, CBResetTimeout: time.Minute,
	})
	adapter := NewAccessControlAdapterWithRuntime(&isi.Client{API: mockAPI}, rt)
	zones, err := adapter.GetZones(context.Background())
	require.NoError(t, err)
	assert.Len(t, zones, 1)
	assert.Equal(t, "System", zones[0].Name)
}

func TestAccessControlAdapterWithRuntime_GetZones_StaleOnCacheHit(t *testing.T) {
	staleCalled := false
	rt := NewMetricsRuntime("cluster1", RuntimeConfig{
		Timeout: time.Second, CacheTTL: time.Minute,
		RateLimit: 100, CBThreshold: 10, CBResetTimeout: time.Minute,
		StaleReporter: func(_ string, s bool) { staleCalled = s },
	})

	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv1.GetIsiZonesResp)
		*resp = apiv1.GetIsiZonesResp{Zones: []*apiv1.IsiZone{{Name: "System"}}}
	}).Once()
	mockAPI.On("Get", anyArgs...).Return(errors.New("api down")).Once()

	adapter := NewAccessControlAdapterWithRuntime(&isi.Client{API: mockAPI}, rt)
	_, err := adapter.GetZones(context.Background())
	require.NoError(t, err)

	zones, err := adapter.GetZones(context.Background())
	require.NoError(t, err, "cache hit must not return error")
	assert.Len(t, zones, 1)
	assert.True(t, staleCalled, "stale reporter must be called on cache fallback")
}

func TestAccessControlAdapterWithRuntime_GetExportCountByZone_StaleOnCacheHit(t *testing.T) {
	staleCalled := false
	rt := NewMetricsRuntime("cluster1", RuntimeConfig{
		Timeout: time.Second, CacheTTL: time.Minute,
		RateLimit: 100, CBThreshold: 10, CBResetTimeout: time.Minute,
		StaleReporter: func(_ string, s bool) { staleCalled = s },
	})

	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv2.Exports)
		*resp = apiv2.Exports{Total: 1, Exports: []*apiv2.Export{{ID: 1}}}
	}).Once()
	mockAPI.On("Get", anyArgs...).Return(errors.New("api down")).Once()

	adapter := NewAccessControlAdapterWithRuntime(&isi.Client{API: mockAPI}, rt)
	_, err := adapter.GetExportCountByZone(context.Background(), "System")
	require.NoError(t, err)

	count, err := adapter.GetExportCountByZone(context.Background(), "System")
	require.NoError(t, err, "cache hit must not return error")
	assert.Equal(t, 1, count)
	assert.True(t, staleCalled, "stale reporter must be called on cache fallback")
}

func TestAccessControlAdapter_GetZones_EmptyZones(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv1.GetIsiZonesResp)
		*resp = apiv1.GetIsiZonesResp{Zones: []*apiv1.IsiZone{}}
	}).Once()

	adapter := NewAccessControlAdapter(&isi.Client{API: mockAPI})
	zones, err := adapter.GetZones(context.Background())
	require.NoError(t, err)
	assert.Len(t, zones, 0)
}

func TestAccessControlAdapter_GetExportCountByZone_EmptyExports(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv2.Exports)
		*resp = apiv2.Exports{Total: 0}
	}).Once()

	adapter := NewAccessControlAdapter(&isi.Client{API: mockAPI})
	count, err := adapter.GetExportCountByZone(context.Background(), "System")
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestAccessControlAdapter_GetExportCountByZone_MultipleExports(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv2.Exports)
		*resp = apiv2.Exports{
			Total: 3,
			Exports: []*apiv2.Export{
				{ID: 1},
				{ID: 2},
				{ID: 3},
			},
		}
	}).Once()

	adapter := NewAccessControlAdapter(&isi.Client{API: mockAPI})
	count, err := adapter.GetExportCountByZone(context.Background(), "System")
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestAccessControlAdapter_GetExportCountByZone_Error(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(errors.New("api error")).Once()

	adapter := NewAccessControlAdapter(&isi.Client{API: mockAPI})
	count, err := adapter.GetExportCountByZone(context.Background(), "System")
	assert.Error(t, err)
	assert.Equal(t, 0, count)
}

func TestAccessControlAdapter_GetZones_WithRuntime(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv1.GetIsiZonesResp)
		*resp = apiv1.GetIsiZonesResp{
			Zones: []*apiv1.IsiZone{{Name: "System"}, {Name: "zone1"}},
		}
	}).Maybe()

	rt := NewMetricsRuntime("cluster1", RuntimeConfig{
		Timeout: time.Second, CacheTTL: time.Minute,
		RateLimit: 100, CBThreshold: 10, CBResetTimeout: time.Minute,
	})
	adapter := NewAccessControlAdapterWithRuntime(&isi.Client{API: mockAPI}, rt)
	zones, err := adapter.GetZones(context.Background())
	require.NoError(t, err)
	assert.Len(t, zones, 2)
}

// U-ACCESS-CONTROL-FETCH-EXPORT-NIL-EXPORTS: fetchExportCountByZone with nil exports
func TestAccessControlAdapter_fetchExportCountByZone_NilExports(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		resp := args.Get(5).(*apiv2.Exports)
		*resp = apiv2.Exports{Exports: nil}
	}).Once()

	adapter := NewAccessControlAdapter(&isi.Client{API: mockAPI})
	count, err := adapter.fetchExportCountByZone(context.Background(), "System")

	assert.NoError(t, err)
	assert.Equal(t, 0, count)
	mockAPI.AssertExpectations(t)
}

// U-ACCESS-CONTROL-FETCH-EXPORT-NIL-CLIENT: fetchExportCountByZone with nil client
func TestAccessControlAdapter_fetchExportCountByZone_NilClient(t *testing.T) {
	// Create adapter with nil client
	adapter := NewAccessControlAdapter(nil)

	count, err := adapter.fetchExportCountByZone(context.Background(), "System")

	assert.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "PowerScale client not yet initialized")
}

// U-ACCESS-CONTROL-FETCH-ZONES-NIL-CLIENT: fetchZones with nil client
func TestAccessControlAdapter_fetchZones_NilClient(t *testing.T) {
	// Create adapter with nil client
	adapter := NewAccessControlAdapter(nil)

	zones, err := adapter.fetchZones(context.Background())

	assert.Error(t, err)
	assert.Nil(t, zones)
	assert.Contains(t, err.Error(), "PowerScale client not yet initialized")
}
