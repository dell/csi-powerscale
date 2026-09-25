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
	"time"

	csmcache "github.com/Ecosystems/container-storage-modules/src/csm-metrics-common/pkg/cache"
	"github.com/Ecosystems/container-storage-modules/src/csm-metrics-common/pkg/middleware"
)

// RuntimeConfig holds the tuning parameters for OneFS metrics calls.
// Zero values are replaced with safe defaults in NewMetricsRuntime.
type RuntimeConfig struct {
	Timeout        time.Duration
	CacheTTL       time.Duration
	RateLimit      int
	CBThreshold    int
	CBResetTimeout time.Duration
	StaleReporter  func(clusterName string, stale bool)
}

// MetricsRuntime wraps OneFS metrics calls with timeout, rate limiting,
// circuit breaking, and response caching. It is scoped per cluster so that
// failures on one cluster do not affect others.
type MetricsRuntime struct {
	clusterName    string
	timeout        time.Duration
	cache          *csmcache.ResponseCache
	rateLimiter    *middleware.RateLimiter
	circuitBreaker *middleware.CircuitBreaker
	staleReporter  func(clusterName string, stale bool)
}

// NewMetricsRuntime creates a MetricsRuntime for the given cluster using
// the provided configuration. Zero or negative config values are replaced
// with safe defaults so callers never need to guard against invalid opts.
func NewMetricsRuntime(clusterName string, cfg RuntimeConfig) *MetricsRuntime {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = 25 * time.Second
	}
	if cfg.RateLimit <= 0 {
		cfg.RateLimit = 100
	}
	if cfg.CBThreshold <= 0 {
		cfg.CBThreshold = 3
	}
	if cfg.CBResetTimeout <= 0 {
		cfg.CBResetTimeout = 30 * time.Second
	}
	return &MetricsRuntime{
		clusterName:    clusterName,
		timeout:        cfg.Timeout,
		cache:          csmcache.NewResponseCache(cfg.CacheTTL),
		rateLimiter:    middleware.NewRateLimiter(cfg.RateLimit),
		circuitBreaker: middleware.NewCircuitBreaker(clusterName, cfg.CBThreshold, cfg.CBResetTimeout),
		staleReporter:  cfg.StaleReporter,
	}
}

// Do executes fn through rate limiting, timeout, and circuit breaking, then
// caches the result. On failure, a cached result is served when available and
// the stale indicator is set. On success the stale indicator is cleared.
//
// endpoint is a logical name used as the rate-limiter bucket (e.g. "quotas").
// cacheKey must be unique per endpoint+parameter combination within a cluster.
func (r *MetricsRuntime) Do(ctx context.Context, endpoint, cacheKey string, fn func(context.Context) (any, error)) (any, error) {
	// 1. Rate-limit before acquiring timeout budget.
	if err := r.rateLimiter.Wait(ctx, endpoint); err != nil {
		return r.serveStaleOrError(cacheKey, fmt.Errorf("rate limiter cancelled for %s/%s: %w", r.clusterName, endpoint, err))
	}

	// 2. Derive a timeout-bound context for the actual OneFS call.
	callCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// 3. Execute through the circuit breaker.
	var result any
	cbErr := r.circuitBreaker.Call(func() error {
		v, err := fn(callCtx)
		if err != nil {
			return err
		}
		result = v
		return nil
	})

	if cbErr != nil {
		return r.serveStaleOrError(cacheKey, fmt.Errorf("%s/%s: %w", r.clusterName, endpoint, cbErr))
	}

	// 4. Success: cache result and clear stale flag.
	r.cache.Set(cacheKey, result)
	r.setStale(false)
	return result, nil
}

// serveStaleOrError marks the cluster metrics as stale (because the collection
// failed) and returns a cached result when one is available, or propagates err
// when the cache is empty.
//
// The stale flag is set unconditionally on any failure — not only when cached
// data exists — because the collection interval (default 30 s) is intentionally
// longer than the cache TTL (default 25 s) to avoid serving perpetually-stale
// data.  This means the cache may have already expired by the time the next
// collection attempt fails (e.g. due to a NetworkPolicy blocking the OneFS API).
// Without setting stale eagerly, dell_powerscale_metrics_stale would never
// reach 1 and the PowerScaleMetricsStale alert would never fire.
func (r *MetricsRuntime) serveStaleOrError(cacheKey string, err error) (any, error) {
	r.setStale(true)
	if cached, ok := r.cache.Get(cacheKey); ok {
		return cached, nil
	}
	return nil, err
}

func (r *MetricsRuntime) setStale(stale bool) {
	if r.staleReporter != nil {
		r.staleReporter(r.clusterName, stale)
	}
}
