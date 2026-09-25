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

// Package collectors implements PowerScale-specific Prometheus metric collectors.
// Lifecycle management delegates to csm-metrics-common/pkg/collector so that
// each collector runs in its own goroutine with panic recovery and proper Stop().
package collectors

import (
	"context"
	"fmt"
	"time"

	"github.com/Ecosystems/container-storage-modules/src/csm-metrics-common/pkg/collector"
	"github.com/prometheus/client_golang/prometheus"
)

// Collector is the driver-local two-method interface implemented by every
// per-metric collector.  It intentionally omits Register so that constructors
// can call reg.MustRegister themselves and keep registration atomic with
// construction.
type Collector interface {
	Collect(ctx context.Context) error
	Name() string
}

// CollectorAdapter bridges a Collector to collector.MetricsCollector.
// The Register method is a no-op because the constructor already registered.
type CollectorAdapter struct {
	Collector
}

func (a *CollectorAdapter) Register(_ prometheus.Registerer) error { return nil }

// Ensure CollectorAdapter implements collector.MetricsCollector
var _ collector.MetricsCollector = (*CollectorAdapter)(nil)

type cleanupCollector interface {
	Cleanup()
}

// CollectorManager holds a set of Collectors and manages their lifecycle.
// It exposes CollectAll for synchronous use in tests and Start/Stop for
// production use via csm-metrics-common's per-goroutine runner.
type CollectorManager struct {
	collectors []Collector
	csmMgr     *collector.Manager
}

// NewCollectorManager creates an empty CollectorManager.
func NewCollectorManager() *CollectorManager {
	return &CollectorManager{}
}

// Register adds a Collector to the manager.
func (m *CollectorManager) Register(c Collector) {
	m.collectors = append(m.collectors, c)
}

// Collectors returns the list of registered collectors (read-only copy).
func (m *CollectorManager) Collectors() []Collector {
	out := make([]Collector, len(m.collectors))
	copy(out, m.collectors)
	return out
}

// CollectAll calls Collect on every registered collector sequentially.
// Errors from individual collectors are accumulated and returned together.
// This method is used in unit tests; production code should call Start.
func (m *CollectorManager) CollectAll(ctx context.Context) error {
	var errs []string
	for _, c := range m.collectors {
		if err := c.Collect(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", c.Name(), err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("CollectorManager errors: %v", errs)
	}
	return nil
}

// Start launches each collector in its own goroutine via csm-metrics-common's
// Manager, which provides panic recovery and a proper Stop signal.
// interval controls how often each collector's Collect method is called.
func (m *CollectorManager) Start(ctx context.Context, interval time.Duration) {
	adapters := make([]collector.MetricsCollector, len(m.collectors))
	for i, c := range m.collectors {
		adapters[i] = &CollectorAdapter{c}
	}
	m.csmMgr = collector.NewManager(adapters, interval)
	m.csmMgr.Start(ctx)
}

// Stop signals all collector goroutines to exit and waits for them to finish.
func (m *CollectorManager) Stop() {
	if m.csmMgr != nil {
		m.csmMgr.Stop()
	}

	for _, c := range m.collectors {
		if cleaner, ok := c.(cleanupCollector); ok {
			cleaner.Cleanup()
		}
	}
}
