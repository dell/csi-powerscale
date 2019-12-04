Feature: Isilon CSI interface
  As a consumer of the CSI interface
  I want to run a system test
  So that I know the service functions correctly

@first_run    
  Scenario: Create, ControllerPublish, NodeStage, NodeUnstage, ControllerUnpublish, delete basic volume with accessMode multi-reader
    Given a Isilon service
    And a capability with access "multi-reader"
    And a volume request "integration0" "8"
    When I call CreateVolume
    When I call ControllerPublishVolume "X_CSI_NODE_NAME"
    Then there are no errors
    When I call NodeStageVolume
    Then check Isilon client exists "X_CSI_NODE_IP"

@first_run
  Scenario: Create, ControllerPublish, NodeStage, NodeUnstage, ControllerUnpublish, delete basic volume with accessMode multi-writer
    Given a Isilon service
    And a capability with access "multi-writer"
    And a volume request "integration1" "8"
    When I call CreateVolume
    When I call ControllerPublishVolume "X_CSI_NODE_NAME"
    Then there are no errors
    When I call NodeStageVolume
    Then check Isilon client exists "X_CSI_NODE_IP"

@first_run
  Scenario: Create, ControllerPublish, NodeStage, NodeUnstage, ControllerUnpublish, delete basic volume with accessMode single-writer
    Given a Isilon service
    And a capability with access "single-writer"
    And a volume request "integration2" "8"
    When I call CreateVolume
    When I call ControllerPublishVolume "X_CSI_NODE_NAME"
    Then there are no errors
    When I call NodeStageVolume
    Then check Isilon client exists "X_CSI_NODE_IP"

@second_run
  Scenario: Create, ControllerPublish, NodeStage, NodeUnstage, ControllerUnpublish, delete basic volume with accessMode multi-reader
    Given a Isilon service
    And a capability with access "multi-reader"
    And a volume request "integration0" "8"
    When I call CreateVolume
    When I call ControllerPublishVolume "X_CSI_NODE_NAME"
    Then there are no errors
    When I call NodeStageVolume
    Then check Isilon client exists "X_CSI_NODE_IP"
    When I call NodeUnstageVolume
    Then check Isilon client not exists "X_CSI_NODE_IP"
    When I call ControllerUnpublishVolume "X_CSI_NODE_NAME"
    Then the error contains "Unimplemented"
    When I call DeleteVolume
    Then there is not a directory "integration0"
    Then there is not an export "integration0" 

@second_run
  Scenario: Create, ControllerPublish, NodeStage, NodeUnstage, ControllerUnpublish, delete basic volume with accessMode multi-writer
    Given a Isilon service
    And a capability with access "multi-writer"
    And a volume request "integration1" "8"
    When I call CreateVolume
    When I call ControllerPublishVolume "X_CSI_NODE_NAME"
    Then there are no errors
    When I call NodeStageVolume
    Then check Isilon client exists "X_CSI_NODE_IP"
    When I call NodeUnstageVolume
    Then check Isilon client not exists "X_CSI_NODE_IP"
    When I call ControllerUnpublishVolume "X_CSI_NODE_NAME"
    Then the error contains "Unimplemented"
    When I call DeleteVolume
    Then there is not a directory "integration1"
    Then there is not an export "integration1" 

@second_run
  Scenario: Create, ControllerPublish, NodeStage, NodeUnstage, ControllerUnpublish, delete basic volume with accessMode single-writer
    Given a Isilon service
    And a capability with access "single-writer"
    And a volume request "integration2" "8"
    When I call CreateVolume
    When I call ControllerPublishVolume "X_CSI_NODE_NAME"
    Then there are no errors
    When I call NodeStageVolume
    Then the error contains "already has other clients added to it, and the access mode is SINGLE_NODE_WRITER"
    When I call NodeUnstageVolume
    Then check Isilon client not exists "X_CSI_NODE_IP"
    When I call ControllerUnpublishVolume "X_CSI_NODE_NAME"
    Then the error contains "Unimplemented"
    When I call DeleteVolume
    Then there is not a directory "integration0"
    Then there is not an export "integration0" 