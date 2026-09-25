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

func newQuotaAdapterWithMock(mockAPI *isimocks.Client) *QuotaAdapter {
	client := &isi.Client{API: mockAPI}
	return NewQuotaAdapter(client, "/ifs/data/csi")
}

func TestNewQuotaAdapter(t *testing.T) {
	client := &isi.Client{}
	adapter := NewQuotaAdapter(client, "/ifs/data/csi")
	assert.NotNil(t, adapter)
}

func TestQuotaAdapter_SetRuntime(t *testing.T) {
	client := &isi.Client{}
	adapter := NewQuotaAdapter(client, "/ifs/data/csi")
	rt := NewMetricsRuntime("cluster1", RuntimeConfig{
		Timeout: time.Second, CacheTTL: time.Minute,
		RateLimit: 100, CBThreshold: 10, CBResetTimeout: time.Minute,
	})

	// Set runtime
	adapter.SetRuntime(rt)
	assert.NotNil(t, adapter.runtime)
}

func TestQuotaAdapter_populateZoneCache_Error(t *testing.T) {
	mockAPI := &isimocks.Client{}

	// Mock zone API call returning error
	mockAPI.On("Get", anyArgs...).Return(errors.New("zone API error")).Once()

	adapter := newQuotaAdapterWithMock(mockAPI)
	// This should handle zone API error gracefully
	_, err := adapter.GetAllQuotas(context.Background())
	assert.Error(t, err)
}

func TestQuotaAdapter_populateZoneCache_DuplicateZones(t *testing.T) {
	mockAPI := &isimocks.Client{}

	// Mock zone API call returning duplicate zones
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		path := args.Get(1).(string)
		if path == "platform/1/zones" {
			pp := args.Get(5).(*apiv1.GetIsiZonesResp)
			*pp = apiv1.GetIsiZonesResp{
				Zones: []*apiv1.IsiZone{
					{Name: "System", Path: "/ifs"},
					{Name: "System", Path: "/ifs"}, // Duplicate
				},
			}
		} else {
			// Quota API call
			pp := args.Get(5).(**apiv1.IsiQuotaListRespResume)
			*pp = &apiv1.IsiQuotaListRespResume{Quotas: []*apiv1.IsiQuota{}}
		}
	}).Maybe()

	adapter := newQuotaAdapterWithMock(mockAPI)
	quotas, err := adapter.GetAllQuotas(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, quotas)
}

func TestQuotaAdapter_populateZoneCache_RuntimeCacheHit(t *testing.T) {
	mockAPI := &isimocks.Client{}

	// Mock zone API call - should be called twice (once for zone, once for quota per call)
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		path := args.Get(1).(string)
		if path == "platform/1/zones" {
			pp := args.Get(5).(*apiv1.GetIsiZonesResp)
			*pp = apiv1.GetIsiZonesResp{
				Zones: []*apiv1.IsiZone{
					{Name: "System", Path: "/ifs"},
				},
			}
		} else {
			pp := args.Get(5).(**apiv1.IsiQuotaListRespResume)
			*pp = &apiv1.IsiQuotaListRespResume{Quotas: []*apiv1.IsiQuota{}}
		}
	}).Maybe()

	rt := NewMetricsRuntime("cluster1", RuntimeConfig{
		Timeout: time.Second, CacheTTL: time.Minute,
		RateLimit: 100, CBThreshold: 10, CBResetTimeout: time.Minute,
	})
	adapter := newQuotaAdapterWithMock(mockAPI)
	adapter.SetRuntime(rt)

	// First call - should hit API
	_, err := adapter.GetAllQuotas(context.Background())
	assert.NoError(t, err)

	// Second call - should use cache
	_, err = adapter.GetAllQuotas(context.Background())
	assert.NoError(t, err)
}

func TestQuotaAdapter_populateZoneCache_RuntimeCacheMiss(t *testing.T) {
	mockAPI := &isimocks.Client{}

	// Mock zone API call
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		path := args.Get(1).(string)
		if path == "platform/1/zones" {
			pp := args.Get(5).(*apiv1.GetIsiZonesResp)
			*pp = apiv1.GetIsiZonesResp{
				Zones: []*apiv1.IsiZone{
					{Name: "System", Path: "/ifs"},
				},
			}
		} else {
			pp := args.Get(5).(**apiv1.IsiQuotaListRespResume)
			*pp = &apiv1.IsiQuotaListRespResume{Quotas: []*apiv1.IsiQuota{}}
		}
	})

	rt := NewMetricsRuntime("cluster1", RuntimeConfig{
		Timeout: time.Second, CacheTTL: time.Nanosecond, // Very short TTL
		RateLimit: 100, CBThreshold: 10, CBResetTimeout: time.Minute,
	})
	adapter := newQuotaAdapterWithMock(mockAPI)
	adapter.SetRuntime(rt)

	// First call
	_, err := adapter.GetAllQuotas(context.Background())
	assert.NoError(t, err)

	// Wait for cache to expire
	time.Sleep(10 * time.Millisecond)

	// Second call - should work even if cache expired
	_, err = adapter.GetAllQuotas(context.Background())
	assert.NoError(t, err)
}

func TestQuotaAdapter_GetAllQuotas_Success(t *testing.T) {
	mockAPI := &isimocks.Client{}

	// Mock zone API call
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		// Check if this is a zone API call or quota API call
		path := args.Get(1).(string)
		if path == "platform/1/zones" {
			// GetIsiZoneList calls client.Get(..., &zonesResp) where zonesResp is *GetIsiZonesResp
			pp := args.Get(5).(interface{})
			if zonesResp, ok := pp.(*apiv1.GetIsiZonesResp); ok {
				*zonesResp = apiv1.GetIsiZonesResp{
					Zones: []*apiv1.IsiZone{
						{
							Name: "System",
							Path: "/ifs",
						},
					},
				}
			}
		} else {
			// GetAllIsiQuota calls client.Get(..., &quotaResp) where quotaResp is *IsiQuotaListRespResume
			pp := args.Get(5).(**apiv1.IsiQuotaListRespResume)
			*pp = &apiv1.IsiQuotaListRespResume{
				Quotas: []*apiv1.IsiQuota{
					{
						ID:   "q1",
						Path: "/ifs/data/csi/vol1",
					},
				},
			}
		}
	}).Maybe()

	adapter := newQuotaAdapterWithMock(mockAPI)
	quotas, err := adapter.GetAllQuotas(context.Background())
	assert.NoError(t, err)
	assert.Len(t, quotas, 1)
	assert.Equal(t, "/ifs/data/csi/vol1", quotas[0].VolumeID)
}

func TestQuotaAdapter_GetAllQuotas_Error(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(errors.New("quota API error")).Maybe()

	adapter := newQuotaAdapterWithMock(mockAPI)
	quotas, err := adapter.GetAllQuotas(context.Background())
	assert.Error(t, err)
	assert.Nil(t, quotas)
}

func TestQuotaAdapter_GetAllQuotas_Empty(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		path := args.Get(1).(string)
		if path == "platform/1/zones" {
			pp := args.Get(5).(interface{})
			if zonesResp, ok := pp.(*apiv1.GetIsiZonesResp); ok {
				*zonesResp = apiv1.GetIsiZonesResp{
					Zones: []*apiv1.IsiZone{
						{Name: "System", Path: "/ifs"},
					},
				}
			}
		} else {
			pp := args.Get(5).(**apiv1.IsiQuotaListRespResume)
			*pp = &apiv1.IsiQuotaListRespResume{Quotas: []*apiv1.IsiQuota{}}
		}
	}).Maybe()

	adapter := newQuotaAdapterWithMock(mockAPI)
	quotas, err := adapter.GetAllQuotas(context.Background())
	assert.NoError(t, err)
	assert.Empty(t, quotas)
}

func TestQuotaAdapter_GetAllQuotas_WithNilEntries(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		path := args.Get(1).(string)
		if path == "platform/1/zones" {
			pp := args.Get(5).(interface{})
			if zonesResp, ok := pp.(*apiv1.GetIsiZonesResp); ok {
				*zonesResp = apiv1.GetIsiZonesResp{
					Zones: []*apiv1.IsiZone{
						{Name: "System", Path: "/ifs"},
					},
				}
			}
		} else {
			pp := args.Get(5).(**apiv1.IsiQuotaListRespResume)
			*pp = &apiv1.IsiQuotaListRespResume{
				Quotas: []*apiv1.IsiQuota{
					{ID: "q1", Path: "/ifs/data/csi/vol1"},
					nil,
					{ID: "q2", Path: "/ifs/data/csi/vol2"},
				},
			}
		}
	}).Maybe()

	adapter := newQuotaAdapterWithMock(mockAPI)
	quotas, err := adapter.GetAllQuotas(context.Background())
	assert.NoError(t, err)
	assert.Len(t, quotas, 2)
	assert.Equal(t, "/ifs/data/csi/vol1", quotas[0].VolumeID)
	assert.Equal(t, "/ifs/data/csi/vol2", quotas[1].VolumeID)
}

// Mock implementation of VolumeValidator for testing
type mockVolumeValidator struct {
	csiManagedPaths map[string]bool
	err             error
	refreshCacheErr error
}

func (m *mockVolumeValidator) IsDriverManaged(_ context.Context, path string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.csiManagedPaths[path], nil
}

func (m *mockVolumeValidator) RefreshCache(_ context.Context) error {
	return m.refreshCacheErr
}

func TestQuotaAdapter_GetAllQuotas_WithValidator_Success(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		path := args.Get(1).(string)
		if path == "platform/1/zones" {
			pp := args.Get(5).(interface{})
			if zonesResp, ok := pp.(*apiv1.GetIsiZonesResp); ok {
				*zonesResp = apiv1.GetIsiZonesResp{
					Zones: []*apiv1.IsiZone{
						{Name: "System", Path: "/ifs"},
					},
				}
			}
		} else {
			pp := args.Get(5).(**apiv1.IsiQuotaListRespResume)
			*pp = &apiv1.IsiQuotaListRespResume{
				Quotas: []*apiv1.IsiQuota{
					{ID: "q1", Path: "/ifs/data/csi/vol1"},
					{ID: "q2", Path: "/ifs/data/csi/vol2"},
				},
			}
		}
	}).Maybe()

	client := &isi.Client{API: mockAPI}
	validator := &mockVolumeValidator{
		csiManagedPaths: map[string]bool{
			"/ifs/data/csi/vol1": true,
			"/ifs/data/csi/vol2": false,
		},
	}

	adapter := NewQuotaAdapterWithValidator(client, validator)
	quotas, err := adapter.GetAllQuotas(context.Background())
	assert.NoError(t, err)
	assert.Len(t, quotas, 1)
	assert.Equal(t, "/ifs/data/csi/vol1", quotas[0].VolumeID)
}

func TestQuotaAdapter_GetAllQuotas_WithValidator_ValidationError(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		pp := args.Get(5).(**apiv1.IsiQuotaListRespResume)
		*pp = &apiv1.IsiQuotaListRespResume{
			Quotas: []*apiv1.IsiQuota{
				{ID: "q1", Path: "/ifs/data/csi/vol1"},
			},
		}
	}).Once()

	client := &isi.Client{API: mockAPI}
	validator := &mockVolumeValidator{
		err: errors.New("validation failed"),
	}

	adapter := NewQuotaAdapterWithValidator(client, validator)
	quotas, err := adapter.GetAllQuotas(context.Background())
	assert.NoError(t, err)
	assert.Len(t, quotas, 0)
}

// U-QUOTA-REFRESH-CACHE-ERROR: Test fetchAllQuotas with RefreshCache error
func TestQuotaAdapter_GetAllQuotas_WithValidator_RefreshCacheError(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		path := args.Get(1).(string)
		if path == "platform/1/zones" {
			pp := args.Get(5).(interface{})
			if zonesResp, ok := pp.(*apiv1.GetIsiZonesResp); ok {
				*zonesResp = apiv1.GetIsiZonesResp{
					Zones: []*apiv1.IsiZone{
						{Name: "System", Path: "/ifs"},
					},
				}
			}
		} else {
			pp := args.Get(5).(**apiv1.IsiQuotaListRespResume)
			*pp = &apiv1.IsiQuotaListRespResume{
				Quotas: []*apiv1.IsiQuota{
					{ID: "q1", Path: "/ifs/data/csi/vol1"},
				},
			}
		}
	}).Maybe()

	client := &isi.Client{API: mockAPI}
	validator := &mockVolumeValidator{
		csiManagedPaths: map[string]bool{
			"/ifs/data/csi/vol1": true,
		},
		refreshCacheErr: errors.New("refresh cache failed"),
	}

	adapter := NewQuotaAdapterWithValidator(client, validator)
	// Should continue with path-based validation even if RefreshCache fails
	quotas, err := adapter.GetAllQuotas(context.Background())
	assert.NoError(t, err)
	assert.Len(t, quotas, 1)
}

func TestQuotaAdapter_GetAllQuotas_FiltersNonDriverPaths(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		path := args.Get(1).(string)
		if path == "platform/1/zones" {
			pp := args.Get(5).(interface{})
			if zonesResp, ok := pp.(*apiv1.GetIsiZonesResp); ok {
				*zonesResp = apiv1.GetIsiZonesResp{
					Zones: []*apiv1.IsiZone{
						{Name: "System", Path: "/ifs"},
					},
				}
			}
		} else {
			pp := args.Get(5).(**apiv1.IsiQuotaListRespResume)
			*pp = &apiv1.IsiQuotaListRespResume{
				Quotas: []*apiv1.IsiQuota{
					{ID: "q1", Path: "/ifs/data/csi/vol1"},
					{ID: "q2", Path: "/ifs/data/other/vol2"},
					{ID: "q3", Path: "/ifs/data/csi/vol3"},
				},
			}
		}
	}).Maybe()

	adapter := newQuotaAdapterWithMock(mockAPI)
	quotas, err := adapter.GetAllQuotas(context.Background())
	assert.NoError(t, err)
	assert.Len(t, quotas, 2)
	assert.Equal(t, "/ifs/data/csi/vol1", quotas[0].VolumeID)
	assert.Equal(t, "/ifs/data/csi/vol3", quotas[1].VolumeID)
}

func testRuntime() *MetricsRuntime {
	return NewMetricsRuntime("cluster1", RuntimeConfig{
		Timeout: time.Second, CacheTTL: time.Minute,
		RateLimit: 100, CBThreshold: 10, CBResetTimeout: time.Minute,
	})
}

// WithRuntime constructor
func TestNewQuotaAdapterWithRuntime(t *testing.T) {
	rt := testRuntime()
	adapter := NewQuotaAdapterWithRuntime(&isi.Client{}, rt)
	assert.NotNil(t, adapter)
	assert.NotNil(t, adapter.runtime)
}

// WithRuntime: success path routes through runtime.Do and returns result.
func TestQuotaAdapterWithRuntime_GetAllQuotas_SuccessPath(t *testing.T) {
	mockAPI := &isimocks.Client{}
	// Mock both zone list and quota list calls - distinguish by checking the response type
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		// Check if this is a zone list call or quota list call by examining the response parameter type
		respPtr := args.Get(5)
		// Try to cast to zone response first
		if zonesResp, ok := respPtr.(**apiv1.GetIsiZonesResp); ok {
			*zonesResp = &apiv1.GetIsiZonesResp{
				Zones: []*apiv1.IsiZone{{Path: "/ifs", Name: "System"}},
			}
			return
		}
		// Otherwise, assume it's a quota response
		if quotaResp, ok := respPtr.(**apiv1.IsiQuotaListRespResume); ok {
			*quotaResp = &apiv1.IsiQuotaListRespResume{
				Quotas: []*apiv1.IsiQuota{{ID: "q1", Path: "/ifs/data/vol1"}},
			}
			return
		}
	}).Times(2)

	adapter := NewQuotaAdapterWithRuntime(&isi.Client{API: mockAPI}, testRuntime())
	quotas, err := adapter.GetAllQuotas(context.Background())
	require.NoError(t, err)
	assert.Len(t, quotas, 1)
	assert.Equal(t, "/ifs/data/vol1", quotas[0].VolumeID)
}

// WithRuntime: on downstream failure with warm cache, stale result is returned and reporter fires.
func TestQuotaAdapterWithRuntime_GetAllQuotas_StaleOnCacheHit(t *testing.T) {
	staleCalled := false
	rt := NewMetricsRuntime("cluster1", RuntimeConfig{
		Timeout: time.Second, CacheTTL: time.Minute,
		RateLimit: 100, CBThreshold: 10, CBResetTimeout: time.Minute,
		StaleReporter: func(_ string, s bool) { staleCalled = s },
	})

	mockAPI := &isimocks.Client{}
	// First call succeeds — warms the cache (zone list + quota list)
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		respPtr := args.Get(5)
		if zonesResp, ok := respPtr.(**apiv1.GetIsiZonesResp); ok {
			*zonesResp = &apiv1.GetIsiZonesResp{
				Zones: []*apiv1.IsiZone{{Path: "/ifs", Name: "System"}},
			}
			return
		}
		if quotaResp, ok := respPtr.(**apiv1.IsiQuotaListRespResume); ok {
			*quotaResp = &apiv1.IsiQuotaListRespResume{
				Quotas: []*apiv1.IsiQuota{{ID: "q1", Path: "/ifs/data/vol1"}},
			}
			return
		}
	}).Times(2)
	// Second call fails.
	mockAPI.On("Get", anyArgs...).Return(errors.New("api down")).Once()

	adapter := NewQuotaAdapterWithRuntime(&isi.Client{API: mockAPI}, rt)

	_, err := adapter.GetAllQuotas(context.Background())
	require.NoError(t, err)

	quotas, err := adapter.GetAllQuotas(context.Background())
	require.NoError(t, err, "cache hit must not return error")
	assert.Len(t, quotas, 1, "cached result must be returned")
	assert.True(t, staleCalled, "stale reporter must have been called")
}

// Test populateZoneCache with nil zones
func TestQuotaAdapter_populateZoneCache_NilZones(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		pp := args.Get(5).(*apiv1.GetIsiZonesResp)
		*pp = apiv1.GetIsiZonesResp{Zones: nil}
	}).Once()

	adapter := newQuotaAdapterWithMock(mockAPI)
	err := adapter.populateZoneCache(context.Background())
	assert.NoError(t, err)
}

// Test getZoneFromPath with populated cache
func TestQuotaAdapter_getZoneFromPath_WithCache(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(nil).Run(func(args mock.Arguments) {
		pp := args.Get(5).(*apiv1.GetIsiZonesResp)
		*pp = apiv1.GetIsiZonesResp{
			Zones: []*apiv1.IsiZone{
				{Name: "System", Path: "/ifs"},
			},
		}
	}).Once()

	adapter := newQuotaAdapterWithMock(mockAPI)
	zone := adapter.getZoneFromPath(context.Background(), "/ifs/data/csi/vol1")
	assert.Equal(t, "System", zone)
}

// Test getZoneFromPath with cache miss
func TestQuotaAdapter_getZoneFromPath_CacheMiss(t *testing.T) {
	mockAPI := &isimocks.Client{}
	mockAPI.On("Get", anyArgs...).Return(errors.New("zone api error")).Once()

	adapter := newQuotaAdapterWithMock(mockAPI)
	zone := adapter.getZoneFromPath(context.Background(), "/ifs/data/csi/vol1")
	assert.Equal(t, "", zone)
}

// Test SetRuntime with adapter
func TestQuotaAdapter_SetRuntime_WithAdapter(t *testing.T) {
	mockAPI := &isimocks.Client{}
	adapter := newQuotaAdapterWithMock(mockAPI)

	rt := NewMetricsRuntime("cluster1", RuntimeConfig{
		Timeout: time.Second, CacheTTL: time.Minute,
		RateLimit: 100, CBThreshold: 10, CBResetTimeout: time.Minute,
	})

	adapter.SetRuntime(rt)
	assert.NotNil(t, adapter.runtime)
}
