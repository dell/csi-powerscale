# Copyright (c) 2026 Dell Inc. or its subsidiaries. All Rights Reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# 	http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

Feature: Isilon CSI writable snapshots
    As a consumer of the CSI interface
    I want to provision writable volumes from snapshots
    So that I can restore and modify snapshot data

@writableSnapshots

    Scenario: Create writable volume from snapshot with parameter set to true
      Given a Isilon service
      When I call Probe
      And I call CreateVolumeFromWritableSnapshot "existent_comp_snapshot_name" "volume1"
      Then a valid CreateVolumeResponse is returned

    Scenario: Create volume from snapshot with writable parameter absent (backward compatibility)
      Given a Isilon service
      When I call Probe
      And I call CreateVolumeFromSnapshot "existent_comp_snapshot_name" "volume1"
      Then a valid CreateVolumeResponse is returned

    Scenario: Create writable volume from snapshot with snapshot not found
      Given a Isilon service
      When I call Probe
      And I induce error "GetSnapshotError"
      And I call CreateVolumeFromWritableSnapshot "non_existent_snapshot" "volume1"
      Then the error contains "failed to get snapshot"

    Scenario: Create writable volume from snapshot with size too small
      Given a Isilon service
      When I call Probe
      And I call CreateVolumeFromWritableSnapshotSmallSize "existent_comp_snapshot_name" "volume1"
      Then the error contains "smaller than source snapshot size"

    Scenario: Create writable volume from snapshot with writable snapshot API error
      Given a Isilon service
      When I call Probe
      And I induce error "CreateWritableSnapshotError"
      And I call CreateVolumeFromWritableSnapshot "existent_comp_snapshot_name" "volume1"
      Then the error contains "failed to create writable snapshot"

    Scenario: Writable from snapshot parameter with volume clone is rejected
      Given a Isilon service
      When I call Probe
      And I call CreateVolumeFromVolumeWithWritableParam "volume2=_=_=557=_=_=System" "volume1"
      Then the error contains "writable-from-snapshot parameter is not supported with PVC clone"

    Scenario: Writable snapshot create skips quota creation
      Given a Isilon service
      And I enable quota
      When I call Probe
      And I induce error "CreateQuotaError"
      And I call CreateVolumeFromWritableSnapshot "existent_comp_snapshot_name" "volume1"
      Then a valid CreateVolumeResponse is returned

    Scenario: Writable snapshot cleanup on export creation failure
      Given a Isilon service
      When I call Probe
      And I induce error "CreateExportError"
      And I call CreateVolumeFromWritableSnapshot "existent_comp_snapshot_name" "volume1"
      Then the error contains "EOF"

    Scenario: Idempotent create writable volume when writable snapshot already exists
      Given a Isilon service
      When I call Probe
      And I induce error "WritableSnapshotExists"
      And I call CreateVolumeFromWritableSnapshot "existent_comp_snapshot_name" "volume1"
      Then a valid CreateVolumeResponse is returned

    Scenario: Delete snapshot with dependency error returns FailedPrecondition
      Given a Isilon service
      When I call Probe
      And I induce error "SnapshotDependencyError"
      And I call DeleteSnapshot "48=_=_=cluster1=_=_=System"
      Then the error contains "has dependent writable volumes"
