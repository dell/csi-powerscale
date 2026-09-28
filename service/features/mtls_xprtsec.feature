# Copyright © 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

Feature: NFS Export Transport Security (xprtsec) for mTLS

  Background:
    Given a PowerScale cluster with OneFS 9.16.0 or later
    And the cluster has NFS TLS configured

  Scenario: Create export with mTLS-only xprtsec
    Given the cluster nfs_tls_mode is "none:tls:mtls"
    And I have a StorageClass with NFSTransportSecurity "mtls"
    When I create a volume
    Then the export should be created successfully
    And the export should have xprtsec "mtls"
    And plaintext mount attempts should fail
    And mTLS mount attempts should succeed

  Scenario: Create export with TLS-only xprtsec
    Given the cluster nfs_tls_mode is "none:tls:mtls"
    And I have a StorageClass with NFSTransportSecurity "tls"
    When I create a volume
    Then the export should be created successfully
    And the export should have xprtsec "tls"
    And plaintext mount attempts should fail
    And TLS mount attempts should succeed
    And mTLS mount attempts should fail

  Scenario: Create export with default xprtsec (backward compatibility)
    Given the cluster nfs_tls_mode is "none:tls:mtls"
    And I have a StorageClass without NFSTransportSecurity parameter
    When I create a volume
    Then the export should be created successfully
    And the export should have default xprtsec
    And all mount types should succeed based on cluster configuration

  Scenario: Create export with plaintext-only xprtsec
    Given the cluster nfs_tls_mode is "none:tls:mtls"
    And I have a StorageClass with NFSTransportSecurity "none"
    When I create a volume
    Then the export should be created successfully
    And the export should have xprtsec "none"
    And plaintext mount attempts should succeed
    And TLS mount attempts should fail
    And mTLS mount attempts should fail

  Scenario: Cluster TLS mode validation - mTLS not supported (SECURITY CRITICAL)
    Given the cluster nfs_tls_mode is "none:tls"
    And I have a StorageClass with NFSTransportSecurity "mtls"
    When I create a volume
    Then the driver should fail with error "cluster nfs_tls_mode 'none:tls' does not support 'mtls'"
    And the driver should log "Failing operation to prevent plaintext fallback"
    And no export should be created
    # SECURITY: This prevents plaintext fallback when mTLS was explicitly requested

  Scenario: Cluster TLS mode validation - TLS not supported (SECURITY CRITICAL)
    Given the cluster nfs_tls_mode is "none"
    And I have a StorageClass with NFSTransportSecurity "tls"
    When I create a volume
    Then the driver should fail with error "cluster nfs_tls_mode 'none' does not support 'tls'"
    And the driver should log "Failing operation to prevent plaintext fallback"
    And no export should be created
    # SECURITY: This prevents plaintext fallback when TLS was explicitly requested

  Scenario: Array doesn't support NFS over TLS - mTLS requested (SECURITY CRITICAL)
    Given a PowerScale cluster with OneFS 9.15.0
    And I have a StorageClass with NFSTransportSecurity "mtls"
    When I create a volume
    Then the driver should fail with error "cluster does not support NFS over TLS (OneFS 9.16.0+ required for 'mtls' mode)"
    And the driver should log "Failing operation to prevent plaintext fallback when TLS/mTLS was explicitly requested"
    And no export should be created
    # SECURITY: OneFS < 9.16.0 doesn't support NFS over TLS, so we MUST fail when mTLS is requested

  Scenario: Array doesn't support NFS over TLS - TLS requested (SECURITY CRITICAL)
    Given a PowerScale cluster with OneFS 9.15.0
    And I have a StorageClass with NFSTransportSecurity "tls"
    When I create a volume
    Then the driver should fail with error "cluster does not support NFS over TLS (OneFS 9.16.0+ required for 'tls' mode)"
    And the driver should log "Failing operation to prevent plaintext fallback when TLS/mTLS was explicitly requested"
    And no export should be created
    # SECURITY: OneFS < 9.16.0 doesn't support NFS over TLS, so we MUST fail when TLS is requested

  Scenario: Backward compatibility - plaintext on older arrays
    Given a PowerScale cluster with OneFS 9.15.0
    And I have a StorageClass without NFSTransportSecurity parameter
    When I create a volume
    Then the export should be created successfully
    And the export should have cluster default xprtsec
    # BACKWARD COMPATIBILITY: Plaintext/default mode works on all array versions

  Scenario: NFS TLS not configured on array - mTLS requested (SECURITY CRITICAL)
    Given a PowerScale cluster with OneFS 9.16.0
    And the cluster nfs_tls_mode is not configured
    And I have a StorageClass with NFSTransportSecurity "mtls"
    When I create a volume
    Then the driver should fail with error "cluster nfs_tls_mode is not configured"
    And the driver should log "Failing operation to prevent plaintext fallback"
    And no export should be created
    # SECURITY: nfs_tls_mode not configured means TLS is not enabled, so we MUST fail

  Scenario: NFS TLS not configured on array - plaintext works
    Given a PowerScale cluster with OneFS 9.16.0
    And the cluster nfs_tls_mode is not configured
    And I have a StorageClass without NFSTransportSecurity parameter
    When I create a volume
    Then the export should be created successfully
    # BACKWARD COMPATIBILITY: Plaintext works even when TLS is not configured

  Scenario: Defense-in-depth security validation
    Given the cluster nfs_tls_mode is "none:tls:mtls"
    And I have a StorageClass with NFSTransportSecurity "mtls"
    When I create a volume
    Then the export should have xprtsec "mtls" at PowerScale level
    And the mount should use xprtsec=mtls at client level
    And both layers should enforce mTLS

  Scenario: Multiple volumes with different transport security
    Given the cluster nfs_tls_mode is "none:tls:mtls"
    And I have a StorageClass "sc-mtls" with NFSTransportSecurity "mtls"
    And I have a StorageClass "sc-tls" with NFSTransportSecurity "tls"
    And I have a StorageClass "sc-none" with NFSTransportSecurity "none"
    When I create a volume with "sc-mtls"
    And I create a volume with "sc-tls"
    And I create a volume with "sc-none"
    Then all exports should be created successfully
    And each export should have its respective xprtsec value
    And each mount should enforce its respective transport security

  Scenario: Volume deletion with xprtsec
    Given the cluster nfs_tls_mode is "none:tls:mtls"
    And I have a StorageClass with NFSTransportSecurity "mtls"
    And I have created a volume
    When I delete the volume
    Then the export should be deleted successfully
    And no xprtsec-related errors should occur

  Scenario: Read-only volume from snapshot with xprtsec
    Given the cluster nfs_tls_mode is "none:tls:mtls"
    And I have a StorageClass with NFSTransportSecurity "mtls"
    And I have created a volume with a snapshot
    When I create a read-only volume from the snapshot
    Then the export should be created successfully
    And the export should have xprtsec "mtls"
    And the read-only mount should use mTLS

  Scenario: Replication with xprtsec
    Given a source cluster with nfs_tls_mode "none:tls:mtls"
    And a target cluster with nfs_tls_mode "none:tls:mtls"
    And I have a StorageClass with NFSTransportSecurity "mtls"
    And replication is enabled
    When I create a replicated volume
    Then the source export should have xprtsec "mtls"
    And the target export should have cluster default xprtsec
    And replication should work correctly

  Scenario: Cluster TLS settings query
    Given the cluster nfs_tls_mode is "tls:mtls"
    And the cluster nfs_tls_min_version is "1.3"
    When the driver queries cluster TLS settings
    Then the driver should receive the correct TLS configuration
    And the driver should validate transport security compatibility

  Scenario: Export default settings query
    Given the cluster has export default xprtsec "tls:mtls"
    When the driver queries export default settings
    Then the driver should receive the default xprtsec value
    And new exports without explicit xprtsec should use this default
