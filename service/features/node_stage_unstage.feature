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

Feature: Node Stage and Unstage Volume
  CSI driver implements NodeStageVolume and NodeUnstageVolume for directory-backed volumes
  with direct subdirectory mounting and fsGroup ownership application

  Background:
    Given a Isilon service
    And I enable quota
    And I induce error "none"
    And I call Probe

  Scenario: NodeStageVolume mounts directory-backed volume successfully
    Given a directory-backed volume with ID "pvc-123" on shared export "/ifs/k8s/shared"
    And the volume has directory path "pvc-123"
    And the staging path is "/var/lib/kubelet/plugins/staging/pvc-123"
    When I call NodeStageVolume
    Then the error contains "none"
    And the volume is mounted at staging path
    And the mount source is "/ifs/k8s/shared/pvc-123"

  Scenario: NodeStageVolume applies fsGroup ownership when present
    Given a directory-backed volume with ID "pvc-456" on shared export "/ifs/k8s/shared"
    And the volume has directory path "pvc-456"
    And the staging path is "/var/lib/kubelet/plugins/staging/pvc-456"
    And the pod security context has fsGroup "1000"
    When I call NodeStageVolume
    Then the error contains "none"
    And the volume is mounted at staging path
    And the directory ownership is "root:1000"

  Scenario: NodeStageVolume without fsGroup keeps root:root ownership
    Given a directory-backed volume with ID "pvc-789" on shared export "/ifs/k8s/shared"
    And the volume has directory path "pvc-789"
    And the staging path is "/var/lib/kubelet/plugins/staging/pvc-789"
    And the pod security context has no fsGroup
    When I call NodeStageVolume
    Then the error contains "none"
    And the volume is mounted at staging path
    And the directory ownership is "root:root"

  Scenario: NodeStageVolume fails when directoryPath missing
    Given a directory-backed volume with ID "pvc-invalid"
    And the volume context does not contain "DirectoryPath"
    And the staging path is "/var/lib/kubelet/plugins/staging/pvc-invalid"
    When I call NodeStageVolume
    Then the error contains "DirectoryPath not found"

  Scenario: NodeStageVolume fails when SharedExportPath missing
    Given a directory-backed volume with ID "pvc-invalid2"
    And the volume context does not contain "SharedExportPath"
    And the staging path is "/var/lib/kubelet/plugins/staging/pvc-invalid2"
    When I call NodeStageVolume
    Then the error contains "SharedExportPath not found"

  Scenario: NodeUnstageVolume unmounts staging path successfully
    Given a directory-backed volume with ID "pvc-999" is staged
    And the staging path is "/var/lib/kubelet/plugins/staging/pvc-999"
    When I call NodeUnstageVolume
    Then the error contains "none"
    And the staging path is unmounted

  Scenario: NodeUnstageVolume is idempotent when already unmounted
    Given a directory-backed volume with ID "pvc-888"
    And the staging path is "/var/lib/kubelet/plugins/staging/pvc-888"
    And the staging path is not mounted
    When I call NodeUnstageVolume
    Then the error contains "none"

  Scenario: NodePublishVolume performs bind-mount from staging path
    Given a directory-backed volume with ID "pvc-777" is staged at "/var/lib/kubelet/plugins/staging/pvc-777"
    And the target path is "/var/lib/kubelet/pods/pod-123/volumes/pvc-777"
    When I call NodePublishVolume with staging path
    Then the error contains "none"
    And the target is bind-mounted from staging path

  Scenario: NodePublishVolume falls back to direct NFS mount without staging
    Given an export-backed volume with ID "vol-legacy"
    And the target path is "/var/lib/kubelet/pods/pod-456/volumes/vol-legacy"
    When I call NodePublishVolume without staging path
    Then the error contains "none"
    And the target is NFS-mounted directly

  Scenario: Export-backed volume works without staging (backward compatibility)
    Given an export-backed volume with ID "vol-export-1"
    And the target path is "/var/lib/kubelet/pods/pod-789/volumes/vol-export-1"
    When I call NodePublishVolume without staging
    Then the error contains "none"
    And the volume is accessible to pods
