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
    | "volume2=_=_=19"               | "existent_comp_snapshot_name"                 | "/ifs/data/csi-isilon"     | "cannot match the expected"            |
    | "volume2=_=_=19=_=_=System"    | "create_snapshot_name"                        | ""                         | "none"                                 |
    | "volume2=_=_=19=_=_=System"    | "create_snapshot_name"                        | "none"                     | "none"                                 |
    | "volume2=_=_=19=_=_=System"    | ""                                            | "/ifs/data/csi-isilon"     | "name cannot be empty"                 |

@deleteSnapshot
@v1.0.0
  Scenario Outline: Delete snapshot with various induced error use cases from examples
    Given a Isilon service
    When I call Probe
    And I induce error <induced>
    And I call DeleteSnapshot "34"
    Then the error contains <errormsg>

    Examples:
    | induced                 | errormsg                                        |
    | "GetSnapshotError"      | "cannot check the existence of the snapshot"    |
    | "DeleteSnapshotError"   | "error deleteing snapshot"                      |
    
  Scenario Outline: Controller delete snapshot various use cases from examples
    Given a Isilon service
    When I call Probe
    And I call DeleteSnapshot <snapshotId>
    Then the error contains <errormsg>

    Examples:
    | snapshotId   | errormsg                                 |
    | "34"         | "none"                                   |
    | ""           | "snapshot id to be deleted is required"  |
    | "404"        | "none"                                   |
    | "str"        | "cannot convert snapshot to integer"     |