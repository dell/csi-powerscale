Feature: Isilon CSI interface
    As a consumer of the CSI interface
    I want to run a system test
    So that I know the service functions correctly

  @v1.0
    Scenario: Create and delete basic volume with/without Quota enabled
      Given a Isilon service
      And a basic volume request "integration0" "8"
      When I call CreateVolume 
      Then there is a directory "integration0"
      Then there is an export "integration0"
      Then verify "integration0" size
      When I call DeleteVolume
      And there is not an export "integration0"
      And there is not a quota "integration0"
      Then there are no errors
  
  @v1.0
    Scenario: Idempotent create and delete basic volume with/without Quota enabled
      Given a Isilon service
      And a basic volume request "integration0" "8"
      When I call CreateVolume 
      When I call CreateVolume 
      Then there is a directory "integration0"
      Then there is an export "integration0"
      Then verify "integration0" size
      When I call DeleteVolume 
      When I call DeleteVolume
      Then there is not a directory "integration0"
      Then there is not an export "integration0"

  @v1.0
    Scenario: Create volume, create snapshot, create volume from snapshot, delete original volume, delete new volume
      Given a Isilon service
      And a basic volume request "integration0" "8"
      When I call CreateVolume
      Then there is a directory "integration0"
      Then there is an export "integration0"
      Then verify "integration0" size
      When I call CreateSnapshot "snapshot0" "integration0"
      Then there is a snapshot "snapshot0"
      When I call CreateVolumeFromSnapshot "volFromSnap0"
      Then there is a directory "volFromSnap0"
      And there is an export "volFromSnap0"
      And verify "volFromSnap0" size
      When I call DeleteSnapshot 
      Then there is not a snapshot "snapshot0"
      When I call DeleteVolume
      Then there is not a directory "volFromSnap0"
      And there is not an export "volFromSnap0"
      And there is not a quota "volFromSnap0"
      When I call DeleteAllVolumes
      Then there are no errors

  @v1.0
    Scenario Outline: Create, ControllerPublish, NodeStage, NodePublish, NodeUnpublish, NodeUnstage, ControllerUnpublish, delete basic volume 
      Given a Isilon service
      And a capability with access <access>
      And a volume request "integration0" "8"
      When I call CreateVolume
      When I call ControllerPublishVolume "X_CSI_NODE_NAME"
      Then check Isilon client exists "X_CSI_NODE_NAME"
      When I call NodeStageVolume
      Then there are no errors
      When I call NodePublishVolume "datadir0"
      Then verify published volume with access <access> "datadir0"
      When I call NodeUnpublishVolume "datadir0"
      Then verify not published volume with access <access> "datadir0"
      When I call NodeUnstageVolume
      Then there are no errors
      When I call ControllerUnpublishVolume "X_CSI_NODE_NAME"
      Then check Isilon client not exists "X_CSI_NODE_IP"
      When I call DeleteVolume
      Then there is not a directory "integration0"
      Then there is not an export "integration0" 

      Examples:
      | access          |
      | "single-writer" |
      | "multi-reader"  |
      | "multi-writer"  |
