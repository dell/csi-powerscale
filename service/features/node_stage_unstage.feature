Feature: Isilon CSI interface
	As a consumer of the CSI interface
	I want to test list service methods
	So that they are known to work


  @nodeStage
  Scenario Outline: Node stage various use cases from examples
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype <voltype> access <access>
    When I call NodeStageVolume
    Then the error contains <errormsg>

    Examples:
    | voltype      | access                         |  errormsg                                    |
    | "mount"      | "single-writer"                | "none"                                       |
    | "mount"      | "multiple-writer"              | "none"                                       |
    | "mount"      | "single-node-single-writer"    | "none"                                       |
    | "mount"      | "single-node-multiple-writer"     | "none"                                       |

  @nodeStage
  Scenario Outline: Node stage mount volumes various induced error use cases from examples
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype "mount" access "single-writer"
    And get Node Stage Volume Request
    And I induce error <errora>
    When I call Probe
    When I call NodeStageVolume
    Then the error contains <errormsg>

    Examples:
    | errora                                  | errormsg                                                                  |
    | "GOFSMockDevMountsError"                | "none"                                                                    |
    | "GOFSMockMountError"                    | "mode conflicts with existing mounts@@mount induced error"                |
    | "GOFSMockGetMountsError"                | "could not reliably determine existing mount status"                      |
    # may be different for Windows vs. Linux
    | "TargetNotCreatedForNodeStage"          | "none"                                                                    |
    | "NodeStageNoStagingTargetPath"          | "StagingTargetPath is required"                                           |
    | "NodeStageNoVolumeCapability"           | "Volume Capability is required"                                           |
    | "NodeStageNoAccessMode"                 | "Volume Access Mode is required"                                          |
    | "NodeStageNoAccessType"                 | "Invalid access type"                                                     |
    | "NodeStageFileTargetNotDir"             | "existing path is not a directory"                                        |
    | "VolInstanceError"                      | "Error retrieving Volume"                                                 |

  @nodeStage
  Scenario Outline: Node stage mount volumes various induced error use cases from examples
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype "mount" access "single-node-single-writer"
    And get Node Stage Volume Request
    And I induce error <errora>
    When I call Probe
    When I call NodeStageVolume
    Then the error contains <errormsg>

    Examples:
    | errora                                  | errormsg                                                                  |
    | "GOFSMockDevMountsError"                | "none"                                                                    |
    | "GOFSMockMountError"                    | "mode conflicts with existing mounts@@mount induced error"                |
    | "GOFSMockGetMountsError"                | "could not reliably determine existing mount status"                      |
    # may be different for Windows vs. Linux
    | "TargetNotCreatedForNodeStage"          | "none"                                                                    |
    | "NodeStageNoStagingTargetPath"          | "StagingTargetPath is required"                                           |
    | "NodeStageNoVolumeCapability"           | "Volume Capability is required"                                           |
    | "NodeStageNoAccessMode"                 | "Volume Access Mode is required"                                          |
    | "NodeStageNoAccessType"                 | "Invalid access type"                                                     |
    | "NodeStageFileTargetNotDir"             | "existing path is not a directory"                                        |
    | "VolInstanceError"                      | "Error retrieving Volume"                                                 |

  @nodeStage
  Scenario Outline: Node stage various use cases from examples when volume already published
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype <voltype> access <access>
    When I call Probe
    And I call NodeStageVolume
    And I change the staging target path
    And I call NodeStageVolume
    Then the error contains <errormsg>

    Examples:
    | voltype      | access                         | errormsg                                             |
    | "mount"      | "multiple-reader"              | "none"                                               |
    | "mount"      | "single-writer"                | "mount point already in use for same device"         |
    | "mount"      | "single-node-single-writer"    | "mount point already in use for same device"         |
    | "mount"      | "multiple-writer"              | "none"                                               |
    | "mount"      | "single-node-multiple-writer"  | "none"                                               |
    | "block"      | "single-writer"                | "Invalid access type"                                |
    | "block"      | "multiple-writer"              | "Invalid access type"                                |
    | "block"      | "multiple-reader"              | "Invalid access type"                                |
    | "block"      | "single-node-single-writer"    | "Invalid access type"                                |
    | "block"      | "single-node-multiple-writer"  | "Invalid access type"                                |

  @nodeStage
  Scenario Outline: Node stage various use cases from examples when read-only mount volume already published (T1!=T2, P1==P2)
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype <voltype> access <access>
    And get Node Stage Volume Request
    When I call Probe
    And I call NodeStageVolume
    And I change the staging target path
    And I call NodeStageVolume
    Then the error contains <errormsg>

    Examples:
    | voltype      | access                         | errormsg                                     |
    | "mount"      | "single-reader"                | "mount point already in use for same device" |
    | "mount"      | "multiple-reader"              | "none"                                       |
    | "mount"      | "single-writer"                | "mount point already in use for same device" |
    | "mount"      | "multiple-writer"              | "none"                                       |
    | "block"      | "multiple-reader"              | "Invalid access type"                        |

  @nodeStage
  Scenario Outline: Node stage various use cases from examples when already published volume changed to read-only (T1!=T2, P1!=P2)
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype <voltype> access <access>
    And get Node Stage Volume Request
    When I call Probe
    And I call NodeStageVolume
    And I change the staging target path
    And I call NodeStageVolume
    Then the error contains <errormsg>

    Examples:
    | voltype      | access                         | errormsg                                     |
    | "mount"      | "single-reader"                | "mount point already in use for same device" |
    | "mount"      | "multiple-reader"              | "none"                                       |
    | "mount"      | "single-writer"                | "mount point already in use for same device" |
    | "mount"      | "single-node-single-writer"    | "mount point already in use for same device" |
    | "mount"      | "multiple-writer"              | "none"                                       |
    | "mount"      | "single-node-multiple-writer"  | "none"                                       |
    | "block"      | "multiple-reader"              | "Invalid access type"                        |

  @nodeStage
  Scenario Outline: Node stage various use cases from examples when already published volume changed with same target(T1==T2, P1==P2)
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype <voltype> access <access>
    And get Node Stage Volume Request
    When I call Probe
    And I call NodeStageVolume
    And I call NodeStageVolume
    Then the error contains <errormsg>

    Examples:
    | voltype      | access                         | errormsg                                     |
    | "mount"      | "single-reader"                | "none"                                       |
    | "mount"      | "multiple-reader"              | "none"                                       |
    | "mount"      | "single-writer"                | "none" |
    | "mount"      | "single-node-single-writer"                | "none" |
    | "mount"      | "single-node-multiple-writer"              | "none" |
    | "mount"      | "multiple-writer"              | "none"                                       |
    | "block"      | "multiple-reader"              | "Invalid access type"                        |

  @nodeStage
  Scenario Outline: Node Stage Volume with no volume context in request
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype <voltype> access <access>
    And get Node Stage Volume Request with no volume context
    When I call Probe
    And I call NodeStageVolume
    Then the error contains <errormsg>

    Examples:
    | voltype      | access                         | errormsg                                     |
    | "mount"      | "single-reader"                | "VolumeContext is nil"                       |

  @nodeStage
  Scenario Outline: Node Stage Volume with missing inputs in volume context request
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype <voltype> access <access>
    And get Node Stage Volume Request with Volume Name <volname> and path <path>
    When I call Probe
    And I call NodeStageVolume
    Then the error contains <errormsg>

    Examples:
    | voltype      | access              | volname        | path                      | errormsg                                             |
    | "mount"      | "single-reader"     | "volume1"      | ""                        | "no entry keyed by 'Path' found in VolumeContext"    |
    | "mount"      | "single-reader"     | ""             | "/ifs/.snapshot/snappath" | "no entry keyed by 'Name' found in VolumeContext "   |