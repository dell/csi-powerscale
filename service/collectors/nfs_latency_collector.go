/*
 Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.

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
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// nfsLatencySnapshot holds the most recently observed per-bucket counts and
// approximate sum for a single cluster, as emitted by one collection cycle.
type nfsLatencySnapshot struct {
	// bucketCounts holds the exclusive (non-cumulative) observation count
	// for each bucket in nfsLatencyCollector.bounds, in the same order.
	bucketCounts []uint64
	sum          float64
}

// nfsLatencyCollector is a prometheus.Collector that reports the NFS latency
// distribution for one or more clusters as a snapshot, const histogram
// instead of replaying every underlying OneFS operation count through
// HistogramVec.Observe on every collection cycle.
//
// Using prometheus.NewConstHistogram makes each Prometheus scrape cost
// O(len(bounds)) regardless of how many NFS operations OneFS reports, and it
// reports the current snapshot rather than permanently accumulating it into
// an ever-growing client-side histogram (a prometheus Histogram never resets
// its internal counters, so repeatedly Observe()-ing the same OneFS counts
// every collection interval would make _count/_sum grow without bound).
type nfsLatencyCollector struct {
	desc   *prometheus.Desc
	bounds []float64

	mu   sync.RWMutex
	data map[string]nfsLatencySnapshot
}

// newNFSLatencyCollector creates an nfsLatencyCollector for the given metric
// name/help/bounds. bounds are the histogram's upper bucket bounds, matching
// the semantics of prometheus.HistogramOpts.Buckets.
func newNFSLatencyCollector(name, help string, bounds []float64, labelNames []string) *nfsLatencyCollector {
	return &nfsLatencyCollector{
		desc:   prometheus.NewDesc(name, help, labelNames, nil),
		bounds: bounds,
		data:   make(map[string]nfsLatencySnapshot),
	}
}

// Describe implements prometheus.Collector.
func (c *nfsLatencyCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

// Collect implements prometheus.Collector. It emits one const histogram per
// cluster currently tracked, built from the latest snapshot set via set().
func (c *nfsLatencyCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	snapshots := make(map[string]nfsLatencySnapshot, len(c.data))
	for clusterName, snap := range c.data {
		snapshots[clusterName] = snap
	}
	c.mu.RUnlock()

	for clusterName, snap := range snapshots {
		cumulative := make(map[float64]uint64, len(c.bounds))
		var total uint64
		for i, bound := range c.bounds {
			if i < len(snap.bucketCounts) {
				total += snap.bucketCounts[i]
			}
			cumulative[bound] = total
		}

		metric, err := prometheus.NewConstHistogram(c.desc, total, snap.sum, cumulative, clusterName)
		if err != nil {
			// Skip a malformed snapshot rather than panicking the scrape for
			// every other cluster sharing this collector.
			continue
		}
		ch <- metric
	}
}

// set stores the latest snapshot for clusterName, replacing any previous
// value. bucketCounts must be exclusive (non-cumulative) per-bucket counts
// aligned with c.bounds.
func (c *nfsLatencyCollector) set(clusterName string, bucketCounts []uint64, sum float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[clusterName] = nfsLatencySnapshot{bucketCounts: bucketCounts, sum: sum}
}

// delete removes any snapshot tracked for clusterName.
func (c *nfsLatencyCollector) delete(clusterName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, clusterName)
}
