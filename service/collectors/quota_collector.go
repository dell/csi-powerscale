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
	"github.com/Ecosystems/container-storage-modules/src/csmlog"
	"github.com/prometheus/client_golang/prometheus"
)

// QuotaInfo holds OneFS quota data for a directory quota.

type QuotaInfo struct {
	VolumeID     string
	AccessZone   string
	UsedBytes    int64
	HardBytes    int64
	HardExceeded bool
}

// QuotaClient is the minimal interface for OneFS quota API calls.
type QuotaClient interface {
	GetAllQuotas(ctx context.Context) ([]QuotaInfo, error)
}

// QuotaCollector collects PowerScale quota metrics.
type QuotaCollector struct {
	client            QuotaClient
	clusterName       string
	volumeTotal       *prometheus.GaugeVec
	volumeSizeBytes   *prometheus.GaugeVec
	quotaUsedBytes    *prometheus.GaugeVec
	quotaHardBytes    *prometheus.GaugeVec
	quotaRemaining    *prometheus.GaugeVec
	quotaUtilization  *prometheus.GaugeVec
	quotaBreachTotal  *prometheus.GaugeVec
	previousVolumeIDs map[string]string // Track volumes from previous collection (volumeID -> accessZone)
}

// NewQuotaCollector creates a new QuotaCollector.
func NewQuotaCollector(client QuotaClient, reg prometheus.Registerer, clusterName string) *QuotaCollector {
	c := &QuotaCollector{
		client:            client,
		clusterName:       clusterName,
		previousVolumeIDs: make(map[string]string),
		volumeTotal: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: naming.MetricCSIVolumeTotal,
			Help: "Total number of NFS exports (volumes) managed by the driver.",
		}, []string{naming.LabelClusterName})),
		volumeSizeBytes: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_powerscale_volume_size_bytes",
			Help: "PowerScale volume size (hard quota) in bytes.",
		}, []string{naming.LabelClusterName, "volume_id", "access_zone"})),
		quotaUsedBytes: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_powerscale_quota_used_bytes",
			Help: "PowerScale quota used bytes.",
		}, []string{naming.LabelClusterName, "volume_id", "access_zone"})),
		quotaHardBytes: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_powerscale_quota_hard_bytes",
			Help: "PowerScale quota hard limit in bytes.",
		}, []string{naming.LabelClusterName, "volume_id", "access_zone"})),
		quotaRemaining: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_powerscale_quota_remaining_bytes",
			Help: "PowerScale quota remaining bytes.",
		}, []string{naming.LabelClusterName, "volume_id", "access_zone"})),
		quotaUtilization: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_powerscale_quota_utilization_ratio",
			Help: "PowerScale quota utilization ratio.",
		}, []string{naming.LabelClusterName, "volume_id", "access_zone"})),
		quotaBreachTotal: registerOrGetGaugeVec(reg, prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dell_powerscale_quota_hard_breach_total",
			Help: "Number of quotas with hard limit exceeded.",
		}, []string{naming.LabelClusterName})),
	}
	return c
}

// Collect fetches quota metrics.
func (c *QuotaCollector) Collect(ctx context.Context) error {
	if c.client == nil {
		return fmt.Errorf("QuotaCollector: client is nil")
	}

	quotas, err := c.client.GetAllQuotas(ctx)
	if err != nil {
		return fmt.Errorf("QuotaCollector: failed to get quotas: %w", err)
	}

	csmlog.Infof("QuotaCollector: collected %d driver-managed quotas for cluster %s", len(quotas), c.clusterName)
	c.volumeTotal.WithLabelValues(c.clusterName).Set(float64(len(quotas)))

	// Build set of current volume IDs with their access zones
	currentVolumeIDs := make(map[string]string)
	for _, q := range quotas {
		currentVolumeIDs[q.VolumeID] = q.AccessZone
	}

	// Clean up metrics for deleted volumes (volumes that existed before but don't now)
	for volumeID, accessZone := range c.previousVolumeIDs {
		if _, exists := currentVolumeIDs[volumeID]; !exists {
			// Volume was deleted, remove its metrics
			c.volumeSizeBytes.DeleteLabelValues(c.clusterName, volumeID, accessZone)
			c.quotaUsedBytes.DeleteLabelValues(c.clusterName, volumeID, accessZone)
			c.quotaHardBytes.DeleteLabelValues(c.clusterName, volumeID, accessZone)
			c.quotaRemaining.DeleteLabelValues(c.clusterName, volumeID, accessZone)
			c.quotaUtilization.DeleteLabelValues(c.clusterName, volumeID, accessZone)
		}
	}

	// Set metrics for current volumes
	var breachCount float64
	for _, q := range quotas {
		c.volumeSizeBytes.WithLabelValues(c.clusterName, q.VolumeID, q.AccessZone).Set(float64(q.HardBytes))
		c.quotaUsedBytes.WithLabelValues(c.clusterName, q.VolumeID, q.AccessZone).Set(float64(q.UsedBytes))
		c.quotaHardBytes.WithLabelValues(c.clusterName, q.VolumeID, q.AccessZone).Set(float64(q.HardBytes))
		c.quotaRemaining.WithLabelValues(c.clusterName, q.VolumeID, q.AccessZone).Set(float64(q.HardBytes - q.UsedBytes))

		if q.HardBytes > 0 {
			c.quotaUtilization.WithLabelValues(c.clusterName, q.VolumeID, q.AccessZone).
				Set(float64(q.UsedBytes) / float64(q.HardBytes))
		} else {
			c.quotaUtilization.WithLabelValues(c.clusterName, q.VolumeID, q.AccessZone).Set(0.0)
		}
		if q.HardExceeded {
			breachCount++
		}
	}
	c.quotaBreachTotal.WithLabelValues(c.clusterName).Set(breachCount)

	// Update previousVolumeIDs for next collection cycle
	c.previousVolumeIDs = currentVolumeIDs

	return nil
}

// Name returns the collector name.
func (c *QuotaCollector) Name() string { return "QuotaCollector" }

// Cleanup removes all quota metrics for this cluster.
func (c *QuotaCollector) Cleanup() {
	labels := prometheus.Labels{naming.LabelClusterName: c.clusterName}
	c.volumeTotal.DeletePartialMatch(labels)
	c.volumeSizeBytes.DeletePartialMatch(labels)
	c.quotaUsedBytes.DeletePartialMatch(labels)
	c.quotaHardBytes.DeletePartialMatch(labels)
	c.quotaRemaining.DeletePartialMatch(labels)
	c.quotaUtilization.DeletePartialMatch(labels)
	c.quotaBreachTotal.DeletePartialMatch(labels)
}
