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

package service

import (
	"context"
	"fmt"

	csmserver "github.com/Ecosystems/container-storage-modules/src/csm-metrics-common/pkg/server"
	csmlog "github.com/Ecosystems/container-storage-modules/src/csmlog"
)

// startMetricsServer builds and starts the Prometheus metrics HTTP/HTTPS server
// using the parsed opts. TLS is enabled only when both cert and key files are
// present. If both are provided but invalid, startup returns an error.
func (s *service) startMetricsServer(ctx context.Context) error {
	logEntry := csmlog.WithContext(ctx)

	certFile := s.opts.MetricsTLSCertFile
	keyFile := s.opts.MetricsTLSKeyFile

	// Validate TLS configuration using the shared library helper.
	// Returns nil when both files are empty (HTTP mode).
	// Returns error when only one file is set or the pair is invalid.
	if _, err := csmserver.TLSConfig(certFile, keyFile); err != nil {
		return fmt.Errorf("invalid TLS configuration: %w", err)
	}

	cfg := csmserver.Config{
		Port:            s.opts.MetricsPort,
		Registry:        s.metricsRegistry,
		StaleMetricName: "dell_powerscale_metrics_stale",
		StaleLabels:     []string{"cluster_name"},
		CertFile:        certFile,
		KeyFile:         keyFile,
	}

	srv := csmserver.NewMetricsServer(cfg)

	s.metricsStaleReporter = func(clusterName string, stale bool) {
		srv.SetStale([]string{clusterName}, stale)
	}

	// Initialize stale metric for all clusters to 0 (fresh data)
	if s.isiClusters != nil {
		s.isiClusters.Range(func(_, value interface{}) bool {
			if cfg, ok := value.(*IsilonClusterConfig); ok && cfg != nil {
				s.metricsStaleReporter(cfg.ClusterName, false)
			}
			return true
		})
	}

	proto := "HTTP"
	if csmserver.IsTLSEnabled(certFile, keyFile) {
		proto = "HTTPS"
	}
	logEntry.Infof("Starting %s metrics server on %s", proto, s.opts.MetricsPort)

	go func() {
		if err := srv.Start(ctx); err != nil {
			logEntry.Warnf("metrics server on %s stopped: %v", s.opts.MetricsPort, err)
		}
	}()
	return nil
}
