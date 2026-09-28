# Copyright © 2025-2026 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

Feature: Directory-Backed Volume Authorization Optimization
  CSI driver optimizes NFS export authorization for directory-backed volumes
  with per-export mutex, deduplication, conditional deauth, and stale IP cleanup

  Background:
    Given a CSI service
    And I enable quota
    And I have a cluster "cluster1"
    And I induce error "none"
    And I call Probe
    And a shared NFS export exists at "/ifs/k8s/shared" with ID 100

  Scenario: Per-export mutex prevents concurrent authorization race
    Given two directory-backed volumes "pvc-1" and "pvc-2" on shared export 100
    When I call ControllerPublishVolume for both volumes concurrently to node "node-1"
    Then both publish operations succeed
    And node "node-1" IP is added to export 100 client list exactly once
    And no authorization conflicts occur

  Scenario: Authorization deduplication skips re-authorization for same node
    Given a directory-backed volume "pvc-alpha" on shared export 100
    And node "node-1" is already authorized to export 100
    When I call ControllerPublishVolume for volume "pvc-alpha" to node "node-1"
    Then the operation succeeds
    And the driver logs "already authorized"
    And no duplicate IP entries exist in export 100 client list

  Scenario: Second volume on same node reuses existing authorization
    Given a directory-backed volume "pvc-beta" on shared export 100 published to node "node-2"
    And node "node-2" is authorized to export 100
    When I publish a second volume "pvc-gamma" on export 100 to node "node-2"
    Then the operation succeeds
    And the driver skips IP addition
    And export 100 client list contains node "node-2" IP once

  Scenario: Conditional deauth retains IP when other volumes exist
    Given three directory-backed volumes on shared export 100 published to node "node-3"
    And the volumes are "pvc-1", "pvc-2", "pvc-3"
    When I call ControllerUnpublishVolume for "pvc-1" from node "node-3"
    Then the operation succeeds
    And node "node-3" IP remains in export 100 client list
    And the driver logs "other volumes exist"

  Scenario: Conditional deauth removes IP when last volume is unpublished
    Given one directory-backed volume "pvc-solo" on shared export 100 published to node "node-4"
    When I call ControllerUnpublishVolume for "pvc-solo" from node "node-4"
    Then the operation succeeds
    And node "node-4" IP is removed from export 100 client list
    And the driver logs "last volume on this export"

  Scenario: Mixed volumes - directory-backed and export-backed on same node
    Given a directory-backed volume "pvc-dir" on shared export 100 published to node "node-5"
    And an export-backed volume "vol-export" on export 200 published to node "node-5"
    When I call ControllerUnpublishVolume for "pvc-dir" from node "node-5"
    Then the operation succeeds
    And node "node-5" IP is removed from export 100 client list
    And node "node-5" IP remains in export 200 client list

  Scenario: Node deletion triggers stale IP cleanup
    Given two directory-backed volumes on shared export 100 published to node "node-6"
    And node "node-6" has IP "10.0.0.100"
    When Kubernetes node "node-6" is deleted
    Then the driver detects the deletion event
    And IP "10.0.0.100" is removed from all shared export client lists
    And the driver logs "cleaned up stale IPs"

  Scenario: Stale IP cleanup handles multiple shared exports
    Given directory-backed volumes on three shared exports (100, 200, 300)
    And all volumes are published to node "node-7" with IP "10.0.0.200"
    When Kubernetes node "node-7" is deleted
    Then IP "10.0.0.200" is removed from export 100 client list
    And IP "10.0.0.200" is removed from export 200 client list
    And IP "10.0.0.200" is removed from export 300 client list
    And cleanup completes successfully

  Scenario: Authorization survives driver restart
    Given a directory-backed volume "pvc-persist" on shared export 100 published to node "node-8"
    And node "node-8" IP is in export 100 client list
    When the CSI driver restarts
    And I publish a second volume "pvc-persist-2" on export 100 to node "node-8"
    Then the driver detects existing authorization
    And the driver skips IP re-addition
    And no duplicate entries are created

  Scenario: IP reuse detection after node replacement
    Given a directory-backed volume "pvc-old" on shared export 100 published to node "old-node"
    And node "old-node" has IP "10.0.0.50"
    And node "old-node" is deleted
    When a new node "new-node" is created with IP "10.0.0.50"
    And I publish volume "pvc-new" on export 100 to node "new-node"
    Then the driver detects IP reuse
    And the driver refreshes authorization
    And the driver logs "IP reuse detected"

  Scenario: Multiple nodes with different access zones
    Given a directory-backed volume "pvc-zone1" on shared export 100 in access zone "System"
    And a directory-backed volume "pvc-zone2" on shared export 200 in access zone "Custom"
    When I publish "pvc-zone1" to node "node-a"
    And I publish "pvc-zone2" to node "node-b"
    Then node "node-a" is authorized to export 100 in zone "System"
    And node "node-b" is authorized to export 200 in zone "Custom"
    And authorization is isolated per access zone
