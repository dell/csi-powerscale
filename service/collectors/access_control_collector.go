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

	"github.com/Ecosystems/container-storage-modules/src/csm-metrics-common/pkg/naming"
	"github.com/prometheus/client_golang/prometheus"
)

// ZoneInfo holds basic information about a PowerScale access zone.
type ZoneInfo struct {
	Name string
}

// AccessControlClient is the minimal interface for fetching zone and export data.
type AccessControlClient interface {
	GetZones(ctx context.Context) ([]ZoneInfo, error)
	GetExportCountByZone(ctx context.Context, zone string) (int, error)
}

// AccessControlCollector collects PowerScale access zone and NFS export metrics.
type AccessControlCollector struct {
	client                AccessControlClient
	clusterName           string
	zoneTotal             *prometheus.GaugeVec
	exportTotal           *prometheus.GaugeVec
	authFailureTotal      *prometheus.CounterVec
	permissionDenialTotal *prometheus.CounterVec
}

// NewAccessControlCollector creates a new AccessControlCollector and registers its metrics.
func NewAccessControlCollector(client AccessControlClient, reg prometheus.Registerer, clusterName string) *AccessControlCollector {
	c := &AccessControlCollector{
		client:      client,
		clusterName: clusterName,
		zoneTotal: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_powerscale_access_zone_total",
			Help: "Total number of active PowerScale access zones.",
		}, []string{naming.LabelClusterName})),
		exportTotal: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_powerscale_nfs_export_total",
			Help: "Total number of NFS exports per PowerScale access zone.",
		}, []string{naming.LabelClusterName, "access_zone"})),
		authFailureTotal: RegisterOrGetCounterVec(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dell_powerscale_nfs_mount_auth_failures_total",
			Help: "Total number of NFS mount authentication failures.",
		}, []string{naming.LabelClusterName})),
		permissionDenialTotal: RegisterOrGetCounterVec(reg, prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dell_powerscale_permission_denials_total",
			Help: "Total number of file access permission denials.",
		}, []string{naming.LabelClusterName})),
	}
	// Initialize counters with 0 value so they appear in metrics output
	c.authFailureTotal.WithLabelValues(clusterName).Add(0)
	c.permissionDenialTotal.WithLabelValues(clusterName).Add(0)
	return c
}

// Collect fetches zone and export counts and updates the Prometheus gauges.
func (c *AccessControlCollector) Collect(ctx context.Context) error {
	zones, err := c.client.GetZones(ctx)
	if err != nil {
		return fmt.Errorf("AccessControlCollector: failed to get zones: %w", err)
	}

	c.zoneTotal.WithLabelValues(c.clusterName).Set(float64(len(zones)))

	for _, z := range zones {
		count, err := c.client.GetExportCountByZone(ctx, z.Name)
		if err != nil {
			return fmt.Errorf("AccessControlCollector: failed to get exports for zone %q: %w", z.Name, err)
		}
		c.exportTotal.WithLabelValues(c.clusterName, z.Name).Set(float64(count))
	}

	return nil
}

// Name returns the collector name.
func (c *AccessControlCollector) Name() string { return "AccessControlCollector" }

// Cleanup removes all access-control metrics for this cluster.
func (c *AccessControlCollector) Cleanup() {
	labels := prometheus.Labels{naming.LabelClusterName: c.clusterName}
	c.zoneTotal.DeletePartialMatch(labels)
	c.exportTotal.DeletePartialMatch(labels)
	c.authFailureTotal.DeletePartialMatch(labels)
	c.permissionDenialTotal.DeletePartialMatch(labels)
}

// RecordAuthFailure records an NFS mount authentication failure.
func (c *AccessControlCollector) RecordAuthFailure() {
	if c.authFailureTotal != nil {
		c.authFailureTotal.WithLabelValues(c.clusterName).Inc()
	}
}

// RecordPermissionDenial records a file access permission denial.
func (c *AccessControlCollector) RecordPermissionDenial() {
	if c.permissionDenialTotal != nil {
		c.permissionDenialTotal.WithLabelValues(c.clusterName).Inc()
	}
}
