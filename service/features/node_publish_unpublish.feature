Feature: Isilon CSI interface
	As a consumer of the CSI interface
	I want to test list service methods
	So that they are known to work


@nodePublish
@v1.0.0
  Scenario Outline: Node publish various use cases from examples
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype <voltype> access <access>
    When I call NodePublishVolume
    Then the error contains <errormsg>

    Examples:
    | voltype      | access                         |  errormsg                                    |
    | "mount"      | "single-writer"                | "none"                                       |
    | "mount"      | "multiple-writer"              | "none"                                       |
    | "mount"      | "single-node-single-writer"    | "none"                                       |
    | "mount"      | "single-node-multiple-writer"     | "none"                                       |

  Scenario Outline: Node publish mount volumes various induced error use cases from examples
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype "mount" access "single-writer"
    And get Node Publish Volume Request
    And I induce error <errora>
    When I call Probe
    When I call NodePublishVolume
    Then the error contains <errormsg>

    Examples:
    | errora                                  | errormsg                                                                  |
    | "GOFSMockDevMountsError"                | "none"                                                                    |
    | "GOFSMockMountError"                    | "mode conflicts with existing mounts@@mount induced error"                |
    | "GOFSMockGetMountsError"                | "could not reliably determine existing mount status"                      |
    # may be different for Windows vs. Linux
    | "TargetNotCreatedForNodePublish"        | "none"                                                                    |
    | "NodePublishNoTargetPath"               | "Target Path is required"                                                 |
    | "NodePublishNoVolumeCapability"         | "Volume Capability is required"                                           |
    | "NodePublishNoAccessMode"               | "Volume Access Mode is required"                                          |
    | "NodePublishNoAccessType"               | "Invalid access type"                                                     |
    | "NodePublishFileTargetNotDir"           | "existing path is not a directory"                                        |
    | "VolInstanceError"                      | "Error retrieving Volume"                                                 |

  Scenario Outline: Node publish mount volumes various induced error use cases from examples
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype "mount" access "single-node-single-writer"
    And get Node Publish Volume Request
    And I induce error <errora>
    When I call Probe
    When I call NodePublishVolume
    Then the error contains <errormsg>

    Examples:
    | errora                                  | errormsg                                                                  |
    | "GOFSMockDevMountsError"                | "none"                                                                    |
    | "GOFSMockMountError"                    | "mode conflicts with existing mounts@@mount induced error"                |
    | "GOFSMockGetMountsError"                | "could not reliably determine existing mount status"                      |
    # may be different for Windows vs. Linux
    | "TargetNotCreatedForNodePublish"        | "none"                                                                    |
    | "NodePublishNoTargetPath"               | "Target Path is required"                                                 |
    | "NodePublishNoVolumeCapability"         | "Volume Capability is required"                                           |
    | "NodePublishNoAccessMode"               | "Volume Access Mode is required"                                          |
    | "NodePublishNoAccessType"               | "Invalid access type"                                                     |
    | "NodePublishFileTargetNotDir"           | "existing path is not a directory"                                        |
    | "VolInstanceError"                      | "Error retrieving Volume"                                                 |

  Scenario Outline: Node publish various use cases from examples when volume already published
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype <voltype> access <access>
    When I call Probe
    And I call NodePublishVolume
    And I change the target path
    And I call NodePublishVolume
    Then the error contains <errormsg>

    Examples:
    | voltype      | access                         | errormsg                                             |
    | "mount"      | "multiple-reader"              | "none"                                               |
    | "mount"      | "single-writer"                | "Mount point already in use for same device"         |
    | "mount"      | "single-node-single-writer"    | "Mount point already in use for same device"         |
    | "mount"      | "multiple-writer"              | "none"                                               |
    | "mount"      | "single-node-multiple-writer"  | "none"                                               |
    | "block"      | "single-writer"                | "Invalid access type"                                |
    | "block"      | "multiple-writer"              | "Invalid access type"                                |
    | "block"      | "multiple-reader"              | "Invalid access type"                                |
    | "block"      | "single-node-single-writer"    | "Invalid access type"                                |
    | "block"      | "single-node-multiple-writer"  | "Invalid access type"                                |

  Scenario Outline: Node publish various use cases from examples when read-only mount volume already published (T1!=T2, P1==P2)
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype <voltype> access <access>
    And get Node Publish Volume Request
    And I mark request read only
    When I call Probe
    And I call NodePublishVolume
    And I change the target path
    And I call NodePublishVolume
    Then the error contains <errormsg>

    Examples:
    | voltype      | access                         | errormsg                                     |
    | "mount"      | "single-reader"                | "Mount point already in use for same device" |
    | "mount"      | "multiple-reader"              | "none"                                       |
    | "mount"      | "single-writer"                | "Mount point already in use for same device" |
    | "mount"      | "multiple-writer"              | "none"                                       |
    | "block"      | "multiple-reader"              | "Invalid access type"                        |

  Scenario Outline: Node publish various use cases from examples when already published volume changed to read-only (T1!=T2, P1!=P2)
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype <voltype> access <access>
    And get Node Publish Volume Request
    When I call Probe
    And I call NodePublishVolume
    And I change the target path
    And I mark request read only
    And I call NodePublishVolume
    Then the error contains <errormsg>

    Examples:
    | voltype      | access                         | errormsg                                     |
    | "mount"      | "single-reader"                | "Mount point already in use for same device" |
    | "mount"      | "multiple-reader"              | "none"                                       |
    | "mount"      | "single-writer"                | "Mount point already in use for same device" |
    | "mount"      | "single-node-single-writer"    | "Mount point already in use for same device" |
    | "mount"      | "multiple-writer"              | "none"                                       |
    | "mount"      | "single-node-multiple-writer"  | "none"                                       |
    | "block"      | "multiple-reader"              | "Invalid access type"                        |

  Scenario Outline: Node publish various use cases from examples when already published volume changed to read-only with same target(T1==T2, P1!=P2)
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype <voltype> access <access>
    And get Node Publish Volume Request
    When I call Probe
    And I call NodePublishVolume
    And I mark request read only
    And I call NodePublishVolume
    Then the error contains <errormsg>

    Examples:
    | voltype      | access                         | errormsg                                                                                             |
    | "mount"      | "single-reader"                | "Mount point already in use by device with different options"                                        |
    | "mount"      | "multiple-reader"              | "Mount point already in use by device with different options"                                        |
    | "mount"      | "single-writer"                | "Mount point already in use by device with different options"                                        |
    | "mount"      | "single-node-single-writer"    | "Mount point already in use by device with different options"                                        |
    | "mount"      | "single-node-multiple-writer"  | "Mount point already in use by device with different options"                                        |
    | "mount"      | "multiple-writer"              | "Mount point already in use by device with different options"                                        |
    | "block"      | "multiple-reader"              | "Invalid access type"                                                                                |

  Scenario Outline: Node publish various use cases from examples when already published volume changed with same target(T1==T2, P1==P2)
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype <voltype> access <access>
    And get Node Publish Volume Request
    When I call Probe
    And I call NodePublishVolume
    And I call NodePublishVolume
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

@nodeUnpublish
@v1.0.0
  Scenario: Identity node unpublish good call
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And I call NodePublishVolume
    When I call NodeUnpublishVolume
    Then a valid NodeUnpublishVolumeResponse is returned

  Scenario: Node unpublish when volume already unpublished
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype "mount" access "single-writer"
    And I call NodePublishVolume
    And I call NodeUnpublishVolume
    And I call NodeUnpublishVolume
    Then the error contains "none"

  Scenario Outline: Node unpublish mount volumes various induced error use cases from examples
    Given a Isilon service
    And I have a Node "node1" with AccessZone
    And a controller published volume
    And a capability with voltype "mount" access "single-writer"
    And I call NodePublishVolume
    And I induce error <errora>
    When I call NodeUnpublishVolume
    Then the error contains <errormsg>

    Examples:
    | errora                                  | errormsg                                                                  |
    | "GOFSMockGetMountsError"                | "could not reliably determine existing mount status"                      |
    | "NodeUnpublishNoTargetPath"             | "Target Path is required"                                                 |
    | "TargetNotCreatedForNodeUnpublish"      | "none"                                                                    |
    | "GOFSMockUnmountError"                  | "error unmounting target"                                                 |

@v1.0.0
  Scenario: Ephemeral NodePublish test cases
    Given a Isilon service
    And I call EphemeralNodePublishVolume
    Then the error contains "none"

@v1.0.0
  Scenario: Ephemeral NodeUnpublish test cases
  Given a Isilon service
  And I call EphemeralNodeUnpublishVolume
  Then the error contains "none"

  #This test is failing and when it is working it is not doing its job correctly
  @todo
  Scenario: Ephemeral NodePublish NodeUnpublish test cases
    Given a Isilon service
    And I call EphemeralNodePublishVolume
    And I call EphemeralNodeUnpublishVolume
    Then the error contains "none"

@todo
  Scenario Outline: Ephemeral NodePublish negative scenario
    Given a Isilon service
    And get Node Publish Volume Request
    And I induce error <errora>
    And I call EphemeralNodePublishVolume
    Then the error contains <errormsg>

    Examples:
    | errora                                  | errormsg                                                                  |
    |"GOFSMockGetMountsError"                 | "could not reliably determine existing mount status"                      |

