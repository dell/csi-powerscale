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
      Then there is not a directory "integration0"
      And there is not an export "integration0"
      And there is not a quota "integration0"
      Then there are no errors
 
  @v1.0
    Scenario: Create volume with different sizes 1EB
      Given a Isilon service
      And a basic volume request "integration0" "1099511627776"
      When I call CreateVolume
      Then there is a directory "integration0"
      Then there is an export "integration0"
      Then verify "integration0" size
      When I call DeleteVolume
      Then there is not a directory "integration0"
      And there is not an export "integration0"
      And there is not a quota "integration0"
      Then there are no errors

  @v1.0
    Scenario: Create and delete basic volume with/without Quota enabled and verify the size while checking the idempotence of CreateVolume and DeleteVolume
      Given a Isilon service
      And a basic volume request "integration0" "8"
      When I call CreateVolume
      And I call CreateVolume
      Then there is a directory "integration0"
      Then there is an export "integration0"
      Then verify "integration0" size
      When I call DeleteVolume
      And I call DeleteVolume
      Then there is not a directory "integration0"
      Then there is not an export "integration0"
  
  @todo
    Scenario Outline: ListVolumes with different max entries and starting token
      Given a Isilon service
      When I call ListVolumes with max entries <entry> starting token <token>
      Then the error contains <errormsg>
      
      Examples:
      | entry | token                                                                                                       | errormsg                          |
      | -1    | ""                                                                                                          | "Invalid max entries"             |
      | 2     | ""                                                                                                          | "none"                            |
      | 2     | "1-1-MAAA1-MAAA1-NwAA1-MgAA1-MgAA0-1-MgAA2-aWQA1-NwAA32-MjhkOGM0YTE4NDRmNzk1NDRhZjdkMzQ0YzdkNGM2M2YA1-MQAA" | "none"                            |
      | 2     | "invalid"                                                                                                   | "The starting token is not valid" |

  @v1.0
    Scenario: Create and delete basic volume while checking the idempotence of NodePublishVolume and NodeUnpublishVolume
      Given a Isilon service
      And a basic volume request "integration0" "8"
      When I call CreateVolume
      Then there is a directory "integration0"
      Then there is an export "integration0"
      When I call ControllerPublishVolume "X_CSI_NODE_NAME"
      Then check Isilon client exists "X_CSI_NODE_NAME"
      When I call NodePublishVolume "datadir0"
      And I call NodePublishVolume "datadir0"
      Then there are no errors
      When I call NodeUnpublishVolume "datadir0"
      And I call NodeUnpublishVolume "datadir0"
      When I call ControllerUnpublishVolume "X_CSI_NODE_NAME"
      Then check Isilon client not exists "X_CSI_NODE_IP"
      Then there are no errors
      When I call DeleteVolume
      Then there is not a directory "integration0"
      And there is not an export "integration0"
  
  @v1.0
    Scenario: Ephemeral Inline Volume basic and idempotency tests
      Given a Isilon service
      When I call EphemeralNodePublishVolume "datadir9"
      And I call EphemeralNodePublishVolume "datadir9"
      Then there are no errors
      When I call EphemeralNodeUnpublishVolume "datadir9"
      And I call EphemeralNodeUnpublishVolume "datadir9"
      Then there are no errors
 
  @v1.0
    Scenario: Create volume, create snapshot, delete volume, delete snapshot and check
      Given a Isilon service
      And a basic volume request "integration0" "8"
      When I call CreateVolume
      And I call CreateSnapshot "snapshot0" "integration0"
      Then there is a snapshot "snapshot0"
      When I call DeleteVolume
      Then there is not a directory "integration0"
      Then there is a snapshot "snapshot0"
      When I call DeleteSnapshot
      Then there is not a snapshot "snapshot0" 
  
  @v1.0
    Scenario: Create and delete basic volume while creating and deleting snapshots and checking the idempotence in those steps
      Given a Isilon service
      And a basic volume request "integration0" "8"
      When I call CreateVolume
      Then there is a directory "integration0"
      When I call CreateSnapshot "snapshot0" "integration0"
      And I call CreateSnapshot "snapshot0" "integration0"
      Then there are no errors
      When I call DeleteSnapshot
      And I call DeleteSnapshot
      Then there is not a snapshot "snapshot0"
      Then there are no errors
      When I call DeleteVolume
      Then there is not a directory "integration0"
      And there is not an export "integration0"
 
  @v1.0
    Scenario: Create volume, create snapshot with different source volume id
      Given a Isilon service
      And a basic volume request "integration0" "8"
      Then I call CreateVolume 
      And a basic volume request "integration1" "8"
      Then I call CreateVolume 
      Then I call CreateSnapshot "snapshot0" "integration0"
      Then there is a snapshot "snapshot0"
      When I call CreateSnapshot "snapshot0" "integration1"
      Then the error contains "incompatible"
      Then I call DeleteAllVolumes
      Then there is not a directory "integration0"
      And there is not an export "integration0"
      Then there is not a directory "integration1"
      And there is not an export "integration1"
      Then I call DeleteAllSnapshots
      Then there is not a snapshot "snapshot0"

  @v1.0
    Scenario: Create volume, create snapshot, delete old volume, create volume from snapshot
      Given a Isilon service
      And a basic volume request "integration0" "8"
      When I call CreateVolume
      Then there is a directory "integration0"
      Then there is an export "integration0"
      Then verify "integration0" size
      When I call CreateSnapshot "snapshot0" "integration0"
      Then there is a snapshot "snapshot0"
      When I call DeleteVolume
      Then there is not a directory "integration0"
      And there is not an export "integration0"
      And there is not a quota "integration0"
      When I call CreateVolumeFromSnapshot "volFromSnap0"
      Then there is a directory "volFromSnap0"
      And there is an export "volFromSnap0"
      When I call DeleteAllVolumes
      When I call DeleteSnapshot
      Then there is not a snapshot "snapshot0"
      Then there is not a directory "volFromSnap0"

  @v1.0
    Scenario: Create volume, create snapshot, delete old volume, create RO volume from snapshot, delete RO volume, delete snapshot
      Given a Isilon service
      And a basic volume request "integration0" "8"
      When I call CreateVolume
      Then there is a directory "integration0"
      Then there is an export "integration0"
      Then verify "integration0" size
      When I call CreateSnapshot "snapshot0" "integration0"
      Then there is a snapshot "snapshot0"
      When I call DeleteVolume
      Then there is not a directory "integration0"
      And there is not an export "integration0"
      And there is not a quota "integration0"
      When I call CreateROVolumeFromSnapshot "volFromSnap0"
      Then there is no directory "volFromSnap0"
      Then there is an export for snapshot dir "snapshot0"
      When I call DeleteAllVolumes
      Then there is a snapshot "snapshot0"
      When I call DeleteSnapshot
      Then there is not a snapshot "snapshot0"

  @v1.0
    Scenario: Create volume, create snapshot, delete old volume, create RO volume from snapshot, delete snapshot, delete RO volume
      Given a Isilon service
      And a basic volume request "integration0" "8"
      When I call CreateVolume
      Then there is a directory "integration0"
      Then there is an export "integration0"
      Then verify "integration0" size
      When I call CreateSnapshot "snapshot0" "integration0"
      Then there is a snapshot "snapshot0"
      When I call DeleteVolume
      Then there is not a directory "integration0"
      And there is not an export "integration0"
      And there is not a quota "integration0"
      When I call CreateROVolumeFromSnapshot "volFromSnap0"
      Then there is no directory "volFromSnap0"
      Then there is an export for snapshot dir "snapshot0"
      When I call DeleteSnapshot
      Then there is a snapshot "snapshot0"
      Then there is an export for snapshot dir "snapshot0"
      When I call DeleteAllVolumes
      Then there is not a snapshot "snapshot0"

  @ROvolumefailure
    Scenario: create RO volume from snapshot, ControllerPublishVolume, NodePublishVolume, NodeUnpublishVolume, ControllerUnpublishVolume, delete snapshot, delete RO volume
      Given a Isilon service
      And a basic volume request "integration0" "8"
      When I call CreateVolume
      Then there is a directory "integration0"
      Then there is an export "integration0"
      When I call CreateSnapshot "snapshot0" "integration0"
      Then there is a snapshot "snapshot0"
      When I call DeleteVolume
      And there is not an export "integration0"
      When I call CreateROVolumeFromSnapshot "volFromSnap0"
      Then there is no directory "volFromSnap0"
      Then there is an export for snapshot dir "snapshot0"
      When I call ControllerPublishVolume "X_CSI_NODE_NAME"
      Then there are no errors
      Then check Isilon client exists "X_CSI_NODE_NAME"
      When I call NodePublishVolume "datadir0"
      Then verify published volume with access "multi-reader" "datadir0"
      When I call NodeUnpublishVolume "datadir0"
      Then verify not published volume with access "multi-reader" "datadir0"
      When I call ControllerUnpublishVolume "X_CSI_NODE_NAME"
      Then check Isilon client not exists "X_CSI_NODE_NAME"
      When I call DeleteSnapshot
      Then there is a snapshot "snapshot0"
      Then there is an export for snapshot dir "snapshot0"
      When I call DeleteAllVolumes
      Then there is not a snapshot "snapshot0"

  @ROvolumefailure
    Scenario: idempotency check- create RO volume from snapshot, ControllerPublishVolume, NodePublishVolume, NodeUnpublishVolume, ControllerUnpublishVolume, delete snapshot, delete RO volume
      Given a Isilon service
      And a basic volume request "integration0" "8"
      When I call CreateVolume
      Then there is a directory "integration0"
      Then there is an export "integration0"
      When I call CreateSnapshot "snapshot0" "integration0"
      Then there is a snapshot "snapshot0"
      When I call DeleteVolume
      And there is not an export "integration0"
      When I call CreateROVolumeFromSnapshot "volFromSnap0"
      Then there is no directory "volFromSnap0"
      Then there is an export for snapshot dir "snapshot0"
      When I call CreateROVolumeFromSnapshot "volFromSnap0"
      Then there are no errors
      Then there is no directory "volFromSnap0"
      Then there is an export for snapshot dir "snapshot0"
      When I call ControllerPublishVolume "X_CSI_NODE_NAME"
      Then there are no errors
      Then check Isilon client exists "X_CSI_NODE_NAME"
      When I call ControllerPublishVolume "X_CSI_NODE_NAME"
      Then there are no errors
      Then check Isilon client exists "X_CSI_NODE_NAME"
      When I call NodePublishVolume "datadir0"
      Then verify published volume with access "multi-reader" "datadir0"
      When I call NodePublishVolume "datadir0"
      Then there are no errors
      Then verify published volume with access "multi-reader" "datadir0"
      When I call NodeUnpublishVolume "datadir0"
      Then verify not published volume with access "multi-reader" "datadir0"
      Then there are no errors
      When I call NodeUnpublishVolume "datadir0"
      Then verify not published volume with access "multi-reader" "datadir0"
      Then there are no errors
      When I call ControllerUnpublishVolume "X_CSI_NODE_NAME"
      Then check Isilon client not exists "X_CSI_NODE_NAME"
      When I call ControllerUnpublishVolume "X_CSI_NODE_NAME"
      Then check Isilon client not exists "X_CSI_NODE_NAME"
      Then there are no errors
      When I call DeleteSnapshot
      Then there is a snapshot "snapshot0"
      Then there is an export for snapshot dir "snapshot0"
      When I call DeleteAllVolumes
      Then there is not a snapshot "snapshot0"

  @v1.0
    Scenario: Create, ControllerPublish, ControllerUnpublish, delete basic volume
      Given a Isilon service
      And a basic volume request "integration0" "8"
      When I call CreateVolume
      When I call ControllerPublishVolume "X_CSI_NODE_NAME"
      Then there are no errors
      When I call ControllerUnpublishVolume "X_CSI_NODE_NAME"
      Then there are no errors
      When I call DeleteVolume
      Then there is not a directory "integration0"
      Then there is not an export "integration0"
  
  @v1.0
    Scenario Outline: Create, ControllerPublish, NodeStage, NodeUnstage, ControllerUnpublish, delete basic volume
      Given a Isilon service
      And a capability with access <access>
      And a volume request "integration0" "8"
      When I call CreateVolume
      When I call ControllerPublishVolume "X_CSI_NODE_NAME"
      Then check Isilon client exists "X_CSI_NODE_NAME"
      When I call NodeStageVolume
      Then there are no errors
      When I call NodeUnstageVolume
      Then there are no errors
      When I call ControllerUnpublishVolume "X_CSI_NODE_NAME"
      Then check Isilon client not exists "X_CSI_NODE_NAME"
      When I call DeleteVolume
      Then there is not a directory "integration0"
      Then there is not an export "integration0" 

      Examples:
      | access          |
      | "single-writer" |
      | "multi-reader"  |
      | "multi-writer"  |

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

  @v1.0
    Scenario Outline: Create, ControllerPublish, idempotent NodeStage, NodePublish, NodeUnpublish, NodeUnstage, ControllerUnpublish, delete basic volume
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

  @v1.0
    Scenario Outline: Create, ControllerPublish, NodeStage, idempotent NodePublish, NodeUnpublish, NodeUnstage, ControllerUnpublish, delete basic volume 
      Given a Isilon service
      And a capability with access <access>
      And a volume request "integration0" "8"
      When I call CreateVolume
      When I call ControllerPublishVolume "X_CSI_NODE_NAME"
      Then check Isilon client exists "X_CSI_NODE_NAME"
      When I call NodeStageVolume
      Then there are no errors
      When I call NodePublishVolume "datadir0"
      When I call NodePublishVolume "datadir0"
      Then verify published volume with access <access> "datadir0"
      When I call NodeUnpublishVolume "datadir0"
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
  
    #Scenario Outline: Nodepublish the same volume to the same node on the different path
    #  Given a Isilon service
    #  And a capability with access <access>
    #  And a volume request "integration0" "8"
    #  When I call CreateVolume
    #  When I call ControllerPublishVolume "X_CSI_NODE_NAME"
    #  Then there are no errors
    #  When I call NodeStageVolume
    #  Then check Isilon client exists
    #  When I call NodePublishVolume "datadir0"
    #  Then verify published volume with access <access> "datadir0"
    #  When I call NodeUnpublishVolume "datadir0"
    #  Then verify not published volume with access <access> "datadir0"
    #  When I call NodeUnstageVolume
    #  Then check Isilon client not exists
    #  When I call ControllerUnpublishVolume "X_CSI_NODE_NAME"
    #  Then the error contains "Unimplemented"
    #  When I call DeleteVolume
    #  Then there is not a directory "integration0"
    #  Then there is not an export "integration0" 

    #  Examples:
    #  | access          |
    #  | "multi-reader"  |
    #  | "multi-writer"  |

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
    Scenario: Create volume from a nonexistent snapshot
      Given a Isilon service
      And a basic volume request "integration0" "8"
      When I call CreateVolume
      Then there is a directory "integration0"
      Then there is an export "integration0"
      Then verify "integration0" size
      When I call CreateSnapshot "snapshot0" "integration0"
      Then there is a snapshot "snapshot0"
      When I call DeleteSnapshot
      Then there is not a snapshot "snapshot0"
      #Needs to be addressed as part of CSIISILON-307
      #Then I call CreateVolumeFromSnapshot "volFromSnap0"
      #Then the error contains "failed to get snapshot"
      Then I call DeleteAllVolumes

  @v1.0
    Scenario: Volume Clone- Create source volume, create new volume from source volume, delete source volume, delete new volume
      Given a Isilon service
      And a basic volume request "integration0" "8"
      When I call CreateVolume
      Then there is a directory "integration0"
      Then there is an export "integration0"
      Then verify "integration0" size
      When I call CreateVolumeFromVolume "integration1" "integration0" "8"
      Then there is a directory "integration1"
      Then there is an export "integration1"
      Then verify "integration1" size
      When I call DeleteAllVolumes
      Then there is not a directory "integration0"
      And there is not an export "integration0"
      And there is not a quota "integration0"
      And there is not a directory "integration1"
      And there is not an export "integration1"
      And there is not a quota "integration1"

  @v1.0
    Scenario: Volume Clone idempotency check- Create source volume, create new volume from source volume, delete source volume, delete new volume
      Given a Isilon service
      And a basic volume request "integration0" "8"
      When I call CreateVolume
      Then there is a directory "integration0"
      Then there is an export "integration0"
      Then verify "integration0" size
      When I call CreateVolumeFromVolume "integration1" "integration0" "8"
      Then there is a directory "integration1"
      Then there is an export "integration1"
      Then verify "integration1" size
      When I call CreateVolumeFromVolume "integration1" "integration0" "8"
      Then there are no errors
      Then there is a directory "integration1"
      Then there is an export "integration1"
      Then verify "integration1" size
      When I call DeleteAllVolumes
      Then there is not a directory "integration0"
      And there is not an export "integration0"
      And there is not a quota "integration0"
      And there is not a directory "integration1"
      And there is not an export "integration1"
      And there is not a quota "integration1"

  @v1.0
    Scenario: Volume Clone- Create volume from a nonexistent volume
      Given a Isilon service
      And a basic volume request "integration0" "8"
      When I call CreateVolume
      Then there is a directory "integration0"
      Then there is an export "integration0"
      When I call DeleteVolume
      Then there is not a directory "integration0"
      And there is not an export "integration0"
      When I call CreateVolumeFromVolume "integration1" "integration0" "7"
      Then the error contains "failed to get volume name 'integration0'"
      Then there is no directory "integration1"
      Then there is not an export "integration1"

  @v1.0
    Scenario Outline: Scalability test to create volume, ControllerPublish, NodeStage, NodePublish, NodeUnpublish, NodeUnstage, ControllerUnpublish, delete basic volume in parallel
      Given a Isilon service
      When I create <numberOfVolumes> volumes in parallel
      Then there are <numberOfVolumes> directories
      And there are <numberOfVolumes> exports
      When I controllerPublish <numberOfVolumes> volumes in parallel
      Then check <numberOfVolumes> Isilon clients exist
      When I nodeStage <numberOfVolumes> volumes in parallel
      Then there are no errors
      When I nodePublish <numberOfVolumes> volumes in parallel
      Then verify published volumes <numberOfVolumes>
      When I nodeUnpublish <numberOfVolumes> volumes in parallel
      Then verify not published volumes <numberOfVolumes>
      When I nodeUnstage <numberOfVolumes> volumes in parallel
      Then there are no errors
      When I controllerUnpublish <numberOfVolumes> volumes in parallel
      Then check <numberOfVolumes> Isilon clients not exist
      When I delete <numberOfVolumes> volumes in parallel
      Then there are not <numberOfVolumes> directories 
      Then there are not <numberOfVolumes> exports
      Then there are not <numberOfVolumes> quotas

    Examples:
    | numberOfVolumes |
    | 2               |
    | 4               |

  @v1.0
  Scenario: Create, ControllerPublish, ControllerUnpublish, delete basic volume , run with nodeID which has IP isntead of FQDN
    Given a Isilon service
    And a basic volume request "integration0" "8"
    When I call CreateVolume
    When I call ControllerPublishVolume "X_CSI_NODE_NAME_NO_FQDN"
    Then there are no errors
    When I call ControllerUnpublishVolume "X_CSI_NODE_NAME_NO_FQDN"
    Then there are no errors
    When I call DeleteVolume
    Then there is not a directory "integration0"
    Then there is not an export "integration0"


  @v1.0
    Scenario: Cleanup all the files created
      Given a Isilon service
      When I call DeleteAllVolumes
      Then there is not a directory "integration0"
      And there is not a quota "integration0"
      Then there is not a directory "integration1"
      And there is not a quota "integration1"
      Then there is not a directory "volFromSnap0"
      And there is not a quota "volFromSnap0"
      When I call DeleteAllSnapshots
      Then there is not a snapshot "snapshot0"
      Then there are no errors

