Feature: Isilon CSI interface
    As a consumer of the CSI interface
    I want to test service methods
    So that they are known to work

    Scenario: Identity GetPluginInfo good call
      Given a Isilon service
      When I call GetPluginInfo
      Then a valid GetPlugInfoResponse is returned

    Scenario: Identity GetPluginCapabilitiles good call
      Given a Isilon service
      When I call GetPluginCapabilities
      Then a valid GetPluginCapabilitiesResponse is returned

    Scenario: Identity Probe good call
      Given a Isilon service
      When I call Probe
      Then a valid ProbeResponse is returned

    Scenario: Identity Probe bad call with invalid OneFS REST connection
      Given a Isilon service
      When I render Isilon service unreachable
      And I call Probe
      Then an invalid ProbeResponse is returned  

    Scenario: Identity Probe bad call with empty password for OneFS REST connection
      Given a Isilon service
      When I set empty password for Isilon service
      And I call Probe
      Then an invalid ProbeResponse is returned  

    Scenario: Call ControllerGetCapabilities
      Given a Isilon service
      When I call ControllerGetCapabilities
      Then a valid ControllerGetCapabilitiesResponse is returned

    Scenario Outline: Calls to validate volume capabilities
      Given a Isilon service
      When I call Probe
      And I call CreateVolume "volume1"
      And a valid CreateVolumeResponse is returned
      And I call ValidateVolumeCapabilities with voltype <voltype> access <access>
      Then the error contains <errormsg>

      Examples:
      | voltype    | access                     | errormsg                                                   |
      | "mount"    | "single-writer"            | "none"                                                     |
      | "mount"    | "single-reader"            | "Single node only reader access mode is not supported"     |
      | "mount"    | "multi-reader"             | "none"                                                     |
      | "mount"    | "multi-writer"             | "none"                                                     |
      | "mount"    | "multi-node-single-writer" | "Multi node single writer access mode is not supported"    |
      | "mount"    | "unknown"                  | "unknown or unsupported access mode"                       |
      | "mount"    | ""                         | "unknown or unsupported access mode"                       |
      | "block"    | "single-writer"            | "unknown access type is not Mount"                         |

    Scenario: Call GetCapacity
      Given a Isilon service
      When I call GetCapacity
      Then a valid GetCapacityResponse is returned

    Scenario: Call GetCapacity with invalid capabilities
      Given a Isilon service
      When I call Probe
      And I call GetCapacity with Invalid access mode
      Then the error contains "unknown or unsupported access mode"

    Scenario Outline: Call GetCapacity with induced errors
      Given a Isilon service
      When I call Probe
      And I induce error <induced>
      And I call GetCapacity
      Then the error contains <errormsg>

     Examples:
     | induced               | errormsg                                                           |
     | "StatsError"          | "Could not retrieve capacity. Data returned error 'Error'"         |
     | "InstancesError"      | "Could not retrieve capacity. Error 'Error retrieving Statistics'" |
     | "none"                | "none"                                                             |
    
    Scenario: Call NodeGetInfo
      Given a Isilon service
      When I call NodeGetInfo
      Then a valid NodeGetInfoResponse is returned

    Scenario: Call NodeGetCapabilities
      Given a Isilon service
      When I call NodeGetCapabilities
      Then a valid NodeGetCapabilitiesResponse is returned

    Scenario: ControllerPublishVolume bad scenario with empty access type
      Given a Isilon service
      When I call Probe
      And I induce error "OmitAccessMode"
      And I call ControllerPublishVolume with "" to "vpi7125" 
      Then the error contains "access mode is required"

    Scenario: ControllerPublishVolume good scenario
      Given a Isilon service
      When I call Probe
      And I call ControllerPublishVolume with name "volume2=_=_=43=_=_=System" and access type "multiple-writer" to "vpi7125"
      Then a valid ControllerPublishVolumeResponse is returned

    Scenario Outline: ControllerPublishVolume with different volume id and access type
      Given a Isilon service
      When I call Probe
      And I call ControllerPublishVolume with name <volumeID> and access type <accessType> to "vpi7125"
      Then the error contains <errormsg>

      Examples:
      | volumeID                       | accessType                | errormsg                            |
      | "volume2=_=_=43=_=_=System"    | "single-writer"           | "none"                              |
      | "volume2=_=_=43"               | "single-writer"           | "failed to parse volume ID"         |
      | "volume2=_=_=0=_=_=System"     | "single-writer"           | "invalid export ID"                 |
      | "volume2=_=_=43=_=_=System"    | "multiple-reader"         | "none"                              |
      | "volume2=_=_=43=_=_=System"    | "multiple-writer"         | "none"                              |
      | "volume2=_=_=43=_=_=System"    | "single-reader"           | "unsupported access mode"           |
      | "volume2=_=_=43=_=_=System"    | "unknown"                 | "unknown or unsupported access mode"|

    Scenario: ControllerUnpublishVolume good scenario
      Given a Isilon service
      When I call Probe
      And I call ControllerUnpublishVolume with name "volume2=_=_=43=_=_=System" and access type "single-writer" to "vpi7125"
      Then valid ControllerUnpublishVolumeResponse is returned
    
    Scenario Outline: ControllerUnpublishVolume bad calls
      Given a Isilon service
      When I call Probe
      And I call ControllerUnpublishVolume with name <volumeID> and access type "single-writer" to "vpi7125"
      Then the error contains <errormsg>

      Examples:
      | volumeID               | errormsg                                                |
      | ""                     | "ControllerUnpublishVolumeRequest.VolumeId is empty"    |
      | "volume2=_=_=43"       | "failed to parse volume ID"                             |

    Scenario Outline: Calls to ListVolumes
      Given a Isilon service
      When I call ListVolumes with max entries <entry> starting token <token>
      Then the error contains <errormsg>

      Examples:
      | entry    | token                     | errormsg                                                   |
      | -1       | ""                        | "Invalid max entries"                                      |
      | 0        | ""                        | "none"                                                     |
      | 2        | ""                        | "none"                                                     |
      | 2        | "1-1-MAAA1"               | "none"                                                     |
      | 2        | "invalid"                 | "The starting token is not valid"                          |

    Scenario: Create volume from snapshot good scenario
      Given a Isilon service
      When I call Probe
      And I call CreateVolumeFromSnapshot "2" "volume1"
      Then a valid CreateVolumeResponse is returned

    Scenario Outline: Create volume from snapshot with negative or idempotent arguments
      Given a Isilon service
      When I call CreateVolumeFromSnapshot <snapshotID> <volumeName>
      Then the error contains <errormsg>

      Examples:
      | snapshotID             | volumeName                          | errormsg                        |
      | "1"                    | "volume1"                           | "failed to get snapshot"        |
      | "2"                    | "volume2"                           | "none"                          |

    Scenario Outline: Controller publish volume with different access mode and node id
      Given a Isilon service
      When I call ControllerPublishVolume with <accessMode> to <nodeID>
      Then the error contains <errormsg>

      Examples:
      | accessMode            | nodeID          | errormsg                                 |
      | "single-writer"       | "vpi7125"       | "none"                                   |
      | "multiple-reader"     | "vpi7125"       | "none"                                   |
      | "multiple-writer"     | "vpi7125"       | "none"                                   |
      | "unknown"             | "vpi7125"       | "unknown or unsupported access mode"     |
      | "single-writer"       | ""              | "node ID is required"                    |

    Scenario: Identify initialize real isilon service bad call
      When I call initialize real isilon service
      Then the error contains "node ID is required"
    
    Scenario Outline: Calling isilon service with parameters and induced error
      Given I induce error <serviceErr>
      And a Isilon service with params <user> <mode>
      And I induce error <connectionErr>
      When I call Probe
      Then the error contains <errormsg>

      Examples:
      | user        | mode           | serviceErr              | connectionErr                        | errormsg                              |
      | "blah"      | "controller"   | "none"                  | "none"                               | "none"                                |
      | "blah"      | "controller"   | "none"                  | "ControllerHasNoConnectionError"     | "controller probe failed"             |
      | ""          | "controller"   | "none"                  | "none"                               | "controller probe failed"             |
      | "blah"      | "node"         | "none"                  | "none"                               | "none"                                |
      | "blah"      | "node"         | "none"                  | "NodeHasNoConnectionError"           | "node probe failed"                   |
      | "blah"      | "unknown"      | "none"                  | "none"                               | "Service mode not set"                |
      | "blah"      | "controller"   | "noIsiService"          | "none"                               | "s.isiSvc (type isiService) is nil"   |
      | "blah"      | "node"         | "noIsiService"          | "none"                               | "s.isiSvc (type isiService) is nil"   |

    Scenario Outline: Calling logStatistics different times
      Given a Isilon service
      When I call logStatistics <times> times
      Then the error contains <errormsg>
      
    Examples:
      | times    | errormsg     |
      | 100      | "none"       |

    Scenario: Calling BeforeServe
      Given a Isilon service
      When I call BeforeServe
      Then the error contains "Service mode not set"
      
    Scenario: Calling unimplemented functions
      Given a Isilon service
      When I call unimplemented functions
      Then the error contains "Unimplemented"
    
    Scenario: Calling autoProbe with not enabled
      Given I induce error "noIsiService"
      And I induce error "autoProbeNotEnabled"
      And a Isilon service with params "blah" "controller"
      When I call autoProbe
      Then the error contains "auto probe is not enabled"

    Scenario: Calling autoProbe with no isiService
      Given I induce error "noIsiService"
      And a Isilon service with params "blah" "controller"
      When I call autoProbe
      Then the error contains "s.isiSvc (type isiService) is nil, probe failed"
    
    Scenario: Calling functions with autoProbe failed
      Given a Isilon service
      And I induce error "autoProbeFailed"
      When I call CreateVolume "volume1"
      And I call ControllerPublishVolume with "single-writer" to "vpi7125"
      And I call DeleteVolume "volume2=_=_=43=_=_=System"
      And I call ValidateVolumeCapabilities with voltype "mount" access "single-writer"
      And I call GetCapacity
      And I call CreateSnapshot "volume2=_=_=19=_=_=System" "existent_comp_snapshot_name" "/ifs/data/csi-isilon"
      And I call DeleteSnapshot "34"
      And I call NodePublishVolume
      Then the error contains "auto probe is not enabled"

    Scenario: Initialize Service object
      When I call init Service object
      Then the error contains "none"
