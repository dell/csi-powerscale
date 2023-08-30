Feature: Isilon CSI interface
  As a consumer of the CSI interface
  I want to test list service methods
  So that they are known to work

@createSnapshot
@v1.0.0
  Scenario: Create snapshot good scenario
    Given a Isilon service
    When I call Probe
    And I call CreateSnapshot "volume2=_=_=19=_=_=System" "create_snapshot_name" "/ifs/data/csi-isilon"
    Then a valid CreateSnapshotResponse is returned

  Scenario: Create snapshot with cluster name in volume id good scenario
    Given a Isilon service
    When I call Probe
    And I call CreateSnapshot "volume2=_=_=19=_=_=System=_=_=cluster1" "create_snapshot_name" "/ifs/data/csi-isilon"
    Then a valid CreateSnapshotResponse is returned

  Scenario: Create snapshot with cluster name in volume id whose config doesn't exists
    Given a Isilon service
    When I call Probe
    And I call CreateSnapshot "volume2=_=_=19=_=_=System=_=_=cluster2" "create_snapshot_name" "/ifs/data/csi-isilon"
    Then the error contains "failed to get cluster config details for clusterName: 'cluster2'"

  Scenario: Create snapshot with internal server error
    Given a Isilon service
    When I call Probe
    And I induce error "CreateSnapshotError"
    And I call CreateSnapshot "volume2=_=_=19=_=_=System" "create_snapshot_name" "/ifs/data/csi-isilon"
    Then the error contains "EOF"

  Scenario Outline: Create snapshot with negative or idempotent arguments
    Given a Isilon service
    When I call CreateSnapshot <volumeID> <snapshotName> <isiPath>
    Then the error contains <errormsg>

    Examples:
    | volumeID                       | snapshotName                                  | isiPath                    | errormsg                               |
    | "volume1=_=_=10=_=_=System"    | "create_snapshot_name"                        | "/ifs/data/csi-isilon"     | "source volume id is invalid"          |
    | "volume2=_=_=19=_=_=System"    | "existent_snapshot_name"                      | "/ifs/data/csi-isilon"     | "already exists but is incompatible"   |
    | "volume2=_=_=19=_=_=System"    | "existent_comp_snapshot_name"                 | "/ifs/data/csi-isilon"     | "none"                                 |
    | "volume2=_=_=19=_=_=System"    | "existent_comp_snapshot_name"                 | "/ifs/data/csi-isilon"     | "none"                                 |
    | "volume2=_=_=19=_=_=System"    | "existent_comp_snapshot_name_longer_than_max" | "/ifs/data/csi-isilon"     | "already exists but is incompatible"   |
    | "volume2=_=_=19"               | "existent_comp_snapshot_name"                 | "/ifs/data/csi-isilon"     | "cannot be split into tokens"            |
    | "volume2=_=_=19=_=_=System"    | "create_snapshot_name"                        | ""                         | "none"                                 |
    | "volume2=_=_=19=_=_=System"    | "create_snapshot_name"                        | "none"                     | "none"                                 |
    | "volume2=_=_=19=_=_=System"    | ""                                            | "/ifs/data/csi-isilon"     | "name cannot be empty"                 |

@todo
@createROVolumeFromSnapshot
  Scenario: Create RO volume from snapshot good scenario
    Given a Isilon service
    When I call Probe
    And I call CreateROVolumeFromSnapshot "2" "volume1"
    Then a valid CreateVolumeResponse is returned

@deleteSnapshot
@v1.0.0
@todo
  Scenario Outline: Delete snapshot with various induced error use cases from examples
    Given a Isilon service
    When I call Probe
    And I induce error <induced>
    And I call DeleteSnapshot "34"
    Then the error contains <errormsg>

    Examples:
    | induced                 | errormsg                                        |
    | "GetSnapshotError"      | "cannot check the existence of the snapshot"    |
    | "DeleteSnapshotError"   | "error deleting snapshot"                      |
  

  Scenario Outline: Controller delete snapshot various use cases from examples
    Given a Isilon service
    When I call Probe
    And I call DeleteSnapshot <snapshotId>
    Then the error contains <errormsg>

    Examples:
    | snapshotId   | errormsg                                 |
    | "34=_=_=cluster2=_=_=System" | "failed to get cluster config details for clusterName: 'cluster2'"                                   |
    | ""           | "snapshot id to be deleted is required"  |
    | "404"        | "none"                                   |
    | "str"        | "cannot convert snapshot to integer"     |

  Scenario: Calling Snapshot create and delete functionality
    Given a Isilon service
    When I call CreateVolume "volume2"
    And I call ControllerPublishVolume with "single-writer" to "vpi7125=#=#=vpi7125.a.b.com=#=#=1.1.1.1"
    And I call DeleteVolume "volume2=_=_=43=_=_=System"
    And I call ValidateVolumeCapabilities with voltype "mount" access "single-writer"
    And I call GetCapacity
    And I call CreateSnapshot "volume2=_=_=43=_=_=System" "existent_comp_snapshot_name" "/ifs/data/csi-isilon"
    And I call DeleteSnapshot "34"
    And I call NodePublishVolume
    And I call NodeUnpublishVolume
    Then the error contains "none"
