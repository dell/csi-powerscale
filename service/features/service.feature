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

    Scenario: Identity Probe good call with session-based auth
      Given a Isilon service with IsiAuthType as session based
      And I call Probe
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

    Scenario: Call ControllerGetCapabilities with health monitor feature enabled
      Given a Isilon service
      When I call ControllerGetCapabilities "true"
      Then a valid ControllerGetCapabilitiesResponse is returned

    Scenario: Call ControllerGetCapabilities with health monitor feature disabled
      Given a Isilon service
      When I call ControllerGetCapabilities "false"
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
      | "mount"    | "single-node-single-writer"   | "none"                                                     |
      | "mount"    | "single-node-multiple-writer" | "none"                                                     |
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

    Scenario: Call GetCapacity with cluster name
      Given a Isilon service
      When I call GetCapacity with params "cluster1"
      Then a valid GetCapacityResponse is returned

    Scenario: Call GetCapacity with non-existing cluster config
      Given a Isilon service
      When I call Probe
      And I call GetCapacity with params "cluster2"
      Then the error contains "failed to get cluster config details for clusterName: 'cluster2'"

    Scenario: Call GetCapacity with invalid capabilities
      Given a Isilon service
      When I call Probe
      And I call GetCapacity with Invalid access mode
      Then a valid GetCapacityResponse is returned

    Scenario Outline: Call GetCapacity with induced errors
      Given a Isilon service
      When I call Probe
      And I induce error <induced>
      And I call GetCapacity
      Then the error contains <errormsg>

     Examples:
     | induced               | errormsg                                                           |
     | "StatsError"          | "runid=1 Could not retrieve capacity. Data returned error"         |
     | "InstancesError"      | "runid=1 Could not retrieve capacity. Error retrieving Statistics" |
     | "none"                | "none"                                                             |

    Scenario: Call NodeGetInfo
      Given a Isilon service
      When I call NodeGetInfo
      Then a valid NodeGetInfoResponse is returned

    Scenario: Call NodeGetInfo with volume limit
      Given a Isilon service
      When I call set attribute MaxVolumesPerNode "1234"
      And I call NodeGetInfo
      Then a valid NodeGetInfoResponse is returned with volume limit "1234"

    Scenario: Call NodeGetInfo with invalid volume limit
      Given a Isilon service
      When I call NodeGetInfo with invalid volume limit "-1"
      Then the error contains "maxIsilonVolumesPerNode MUST NOT be set to negative value"

    Scenario: Call NodeGetInfo with max-isilon-volumes-per-node node label set
      Given a Isilon service
      When I call apply node label "max-isilon-volumes-per-node=5"
      And I call NodeGetInfo
      Then a valid NodeGetInfoResponse is returned with volume limit "5"
      And I call remove node labels

    Scenario: Call NodeGetInfo with both max-isilon-volumes-per-node node label and maxIsilonVolumesPerNode attribute set
      Given a Isilon service
      When I call set attribute MaxVolumesPerNode "1"
      And I call apply node label "max-isilon-volumes-per-node=2"
      And I call NodeGetInfo
      Then a valid NodeGetInfoResponse is returned with volume limit "2"
      And I call remove node labels

    Scenario: Call NodeGetInfo When reverse DNS is absent
      Given a Isilon service
      When I call iCallNodeGetInfoWithNoFQDN
      Then a valid NodeGetInfoResponse is returned

    Scenario: Call NodeGetCapabilities with health monitor feature enabled
      Given a Isilon service
      When I call NodeGetCapabilities "true"
      Then a valid NodeGetCapabilitiesResponse is returned

    Scenario: Call NodeGetCapabilities with health monitor feature disabled
      Given a Isilon service
      When I call NodeGetCapabilities "false"
      Then a valid NodeGetCapabilitiesResponse is returned

    Scenario: ControllerPublishVolume bad scenario with empty access type
      Given a Isilon service
      When I call Probe
      And I induce error "OmitAccessMode"
      And I call ControllerPublishVolume with name "volume2=_=_=43=_=_=System" and access type "" to "vpi7125=#=#=vpi7125.a.b.com=#=#=1.1.1.1"
      Then the error contains "access mode is required"

    Scenario: ControllerPublishVolume good scenario
      Given a Isilon service
      When I call Probe
      And I call ControllerPublishVolume with name "volume2=_=_=43=_=_=System" and access type "multiple-writer" to "vpi7125=#=#=vpi7125.a.b.com=#=#=1.1.1.1"
      Then a valid ControllerPublishVolumeResponse is returned

    Scenario: Calling ControllerPublishVolume function with autoProbe failed
      Given a Isilon service
      And I induce error "autoProbeFailed"
      And I call ControllerPublishVolume with name "volume2=_=_=43=_=_=System" and access type "multiple-writer" to "vpi7125=#=#=vpi7125.a.b.com=#=#=1.1.1.1"
      Then the error contains "auto probe is not enabled"

    Scenario Outline: ControllerPublishVolume with different volume id and access type
      Given a Isilon service
      When I call Probe
      And I call ControllerPublishVolume with name <volumeID> and access type <accessType> to "vpi7125=#=#=vpi7125.a.b.com=#=#=1.1.1.1"
      Then the error contains <errormsg>

      Examples:
      | volumeID                       | accessType                | errormsg                            |
      | "volume2=_=_=43=_=_=System"    | "single-writer"           | "none"                              |
      | "volume2=_=_=43=_=_=System=_=_=cluster1" | "single-writer" | "none"                              |
      | "volume2=_=_=43=_=_=System=_=_=cluster2" | "single-writer" | "failed to get cluster config details for clusterName: 'cluster2'" |
      | "volume2=_=_=43"               | "single-writer"           | "failed to parse volume ID"         |
      | "volume2=_=_=0=_=_=System"     | "single-writer"           | "invalid export ID"                 |
      | "volume2=_=_=43=_=_=System"    | "multiple-reader"         | "none"                              |
      | "volume2=_=_=43=_=_=System"    | "multiple-writer"         | "none"                              |
      | "volume2=_=_=43=_=_=System"    | "single-reader"           | "unsupported access mode"           |
      | "volume2=_=_=43=_=_=System"    | "unknown"                 | "unknown or unsupported access mode"|

    Scenario: ControllerPublishVolume With RootClient Enabled good scenario
      Given a Isilon service
      When I call Probe
      And I set RootClientEnabled to "true"
      And I call CreateVolume "volume2"
      And I call ControllerPublishVolume with name "volume2=_=_=43=_=_=System" and access type "multiple-writer" to "vpi7125=#=#=vpi7125.a.b.com=#=#=1.1.1.1"
      And I set RootClientEnabled to "false"
      Then a valid ControllerPublishVolumeResponse is returned
      Then a valid CreateVolumeResponse is returned

    Scenario: ControllerUnpublishVolume good scenario
      Given a Isilon service
      When I call Probe
      And I call ControllerUnpublishVolume with name "volume2=_=_=43=_=_=System" and access type "single-writer" to "vpi7125=#=#=vpi7125.a.b.com=#=#=1.1.1.1"
      Then a valid ControllerUnpublishVolumeResponse is returned

    Scenario Outline: ControllerUnpublishVolume bad calls
      Given a Isilon service
      When I call Probe
      And I call ControllerUnpublishVolume with name <volumeID> and access type "single-writer" to "vpi7125=#=#=vpi7125.a.b.com=#=#=1.1.1.1"
      Then the error contains <errormsg>

      Examples:
      | volumeID               | errormsg                                                |
      | ""                     | "ControllerUnpublishVolumeRequest.VolumeId is empty"    |
      | "volume2=_=_=43"       | "failed to parse volume ID"                             |

    @todo
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

    Scenario: Create volume from snapshot with different isi path good scenario
      Given a Isilon service
      When I call Probe
      And I call CreateVolumeFromSnapshot "4" "volume1"
      Then a valid CreateVolumeResponse is returned

    Scenario Outline: Create volume from snapshot with negative or idempotent arguments
      Given a Isilon service
      When I call CreateVolumeFromSnapshot <snapshotID> <volumeName>
      Then the error contains <errormsg>

      Examples:
      | snapshotID             | volumeName                          | errormsg                        |
      | "1"                    | "volume1"                           | "Error retrieving Snapshot"     |
      | "2"                    | "volume2"                           | "none"                          |

@todo
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

@todo
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
      | "blah"      | "controller"   | "none"                  | "ControllerHasNoConnectionError"     | "probe of all isilon clusters failed" |
      | ""          | "controller"   | "none"                  | "none"                               | "probe of all isilon clusters failed" |
      | "blah"      | "node"         | "none"                  | "none"                               | "none"                                |
      | "blah"      | "node"         | "none"                  | "NodeHasNoConnectionError"           | "probe of all isilon clusters failed" |
      | "blah"      | "unknown"      | "none"                  | "none"                               | "probe of all isilon clusters failed" |
      | "blah"      | "controller"   | "noIsiService"          | "none"                               | "none"                                |
      | "blah"      | "node"         | "noIsiService"          | "none"                               | "none"                                |
      | "blah"      | ""             | "none"                  | "none"                               | "none" |

    Scenario Outline: Calling logStatistics different times
      Given a Isilon service
      When I call logStatistics <times> times
      Then the error contains <errormsg>

    Examples:
      | times    | errormsg     |
      | 100      | "none"       |

    Scenario: Calling BeforeServe function with noProbeOnStart set to true
      Given a Isilon service
      When I set noProbeOnStart to "true"
      When I call BeforeServe
      Then the error contains "none"

   Scenario: Calling probe function with noProbeOnStart set to true
      Given a Isilon service
      When I set noProbeOnStart to "true"
      When I call Probe
      Then the error contains "none"

    Scenario: Calling NodeGetInfo function with noProbeOnStart set to true
      Given a Isilon service
      When I set noProbeOnStart to "true"
      When I call NodeGetInfo
      Then the error contains "none"

    Scenario: Calling BeforeServe
      Given a Isilon service
      When I set noProbeOnStart to "false"
      When I call BeforeServe
      Then the error contains "probe of all isilon clusters failed"

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
      Then the error contains "none"

    Scenario: Calling functions with autoProbe failed
      Given a Isilon service
      And I induce error "autoProbeFailed"
      When I call CreateVolume "volume1"
      And I call ControllerPublishVolume with "single-writer" to "vpi7125=#=#=vpi7125.a.b.com=#=#=1.1.1.1"
      And I call DeleteVolume "volume2=_=_=43=_=_=System"
      And I call ValidateVolumeCapabilities with voltype "mount" access "single-writer"
      And I call GetCapacity
      And I call CreateSnapshot "volume2=_=_=19=_=_=System" "existent_comp_snapshot_name"
      And I call DeleteSnapshot "34"
      And I call NodePublishVolume
      And I call NodeUnpublishVolume
      Then the error contains "auto probe is not enabled"

    Scenario: Initialize Service object
      When I call init Service object
      Then the error contains "none"

    Scenario: Verify Custom Networks
      Given a Isilon service
      When I call set allowed networks "127.0.0.0/8"
      And I call NodeGetInfo
      Then a valid NodeGetInfoResponse is returned

    Scenario: Verify Invalid Custom Networks
      Given a Isilon service
      When I call set allowed networks "1.2.3.4/33"
      And I call NodeGetInfo with invalid networks
      Then the error contains "no valid IP address found matching against allowedNetworks"

    Scenario: Verify Multiple Custom Networks
      Given a Isilon service
      When I call set allowed networks with multiple networks "1.2.3.4/33" "127.0.0.0/8"
      And I call NodeGetInfo
      Then a valid NodeGetInfoResponse is returned

    Scenario: ControllerGetVolume good scenario
      Given a Isilon service
      When I call Probe
      And I call ControllerGetVolume with name "volume2=_=_=43=_=_=System=_=_=cluster1"
      Then a valid ControllerGetVolumeResponse is returned

    Scenario: ControllerGetVolume volume does not exist scenario
      Given a Isilon service
      When I call Probe
      And I induce error "VolumeNotExistError"
      And I call ControllerGetVolume with name ""
      Then the error contains "no VolumeID found in request"

    Scenario: ControllerGetVolume with Invalid clustername
      Given a Isilon service
      When I call Probe
      And I call ControllerGetVolume with name "volume2=_=_=43=_=_=System=_=_=cluster2"
      Then the error contains "failed to get cluster config details for clusterName: 'cluster2'"

    Scenario: Calling functions with autoProbe failed
      Given a Isilon service
      And I induce error "autoProbeFailed"
      And I call ControllerGetVolume with name "volume2=_=_=43=_=_=System=_=_=cluster1"
      Then the error contains "auto probe is not enabled"

    Scenario: ControllerGetVolume volume does not exist scenario
      Given a Isilon service
      And I call ControllerGetVolume with name "volume2=_=_=43"
      Then the error contains "cannot be split into tokens"

    Scenario: ControllerGetVolume volume does not exist scenario
      Given a Isilon service
      When I call Probe
      And I call ControllerGetVolume with name "volume3=_=_=43=_=_=System=_=_=cluster1"
      Then a valid ControllerGetVolumeResponse is returned

    Scenario: Calling functions with autoProbe failed
      Given a Isilon service
      And I induce error "autoProbeFailed"
      And I call NodeGetVolumeStats with name "volume2=_=_=43=_=_=System=_=_=cluster1"
      Then the error contains "auto probe is not enabled"

    Scenario: NodeGetVolumeStats volume does not exist scenario
      Given a Isilon service
      When I call Probe
      And I call NodeGetVolumeStats with name ""
      Then the error contains "no VolumeID found in request"

    Scenario: NodeGetVolumeStats volume path not found scenario
      Given a Isilon service
      When I call Probe
      And I induce error "volumePathNotFound"
      And I call NodeGetVolumeStats with name "volume2=_=_=43=_=_=System=_=_=cluster1"
      Then the error contains "no Volume Path found in request"

    Scenario: NodeGetVolumeStats volume does not mount scenario
      Given a Isilon service
      When I call Probe
      And I call NodeGetVolumeStats with name "volume2=_=_=43=_=_=System=_=_=cluster1"
      Then the error contains "no volume is mounted at path"

    Scenario: NodeGetVolumeStats volume does not exist at path scenario
      Given a Isilon service
      When I call Probe
      And I call NodeGetVolumeStats with name "volume3=_=_=43=_=_=System=_=_=cluster1"
      Then the error contains "volume3 does not exist at path"

    Scenario: NodeGetVolumeStats cluster does not exist scenario
      Given a Isilon service
      When I call Probe
      And I call NodeGetVolumeStats with name "volume2=_=_=43=_=_=System=_=_=cluster2"
      Then the error contains "failed to get cluster config details for clusterName: 'cluster2'"

    Scenario: NodeGetVolumeStats correct scenario
      Given a Isilon service
      When I call Probe
      And I call ControllerPublishVolume with name "volume2=_=_=43=_=_=System" and access type "multiple-writer" to "vpi7125=#=#=vpi7125.a.b.com=#=#=1.1.1.1"
      And a capability with voltype "mount" access "single-writer"
      And get Node Publish Volume Request with Volume Name "volume2=_=_=43=_=_=System=_=_=cluster1"
      When I call Probe
      And I call NodePublishVolume
      And I call NodeGetVolumeStats with name "volume2=_=_=43=_=_=System=_=_=cluster1"
      Then a NodeGetVolumeResponse is returned

    Scenario: Identity GetReplicationCapabilities call
      Given a Isilon service
      When I call GetReplicationCapabilities
      Then a valid GetReplicationCapabilitiesResponse is returned

    Scenario: Call Isilon Service with custom topology
      Given a Isilon service with custom topology "blah" "controller"
      When I call Probe
      Then the error contains "probe of all isilon clusters failed"

    Scenario: Call ProbeController
      Given a Isilon service
      When I call Probe
      And I call ProbeController
      Then the error contains "none"

    Scenario: Create Volume with Replication Enabled
      Given a Isilon service
      When I call CreateVolumeRequest
      Then the error contains "json: cannot unmarshal number into Go value of type api.JSONError"

    Scenario Outline: Create RO Volume from snapshot
      Given a Isilon service
      When I call CreateVolumeFromSnapshotMultiReader <exportID> <snapVolName>
      Then a valid CreateVolumeResponse is returned
      Examples:
        | exportID  | snapVolName |
        | "43"      | "snapVol1"  |
        | "44"      | "snapVol2"  |

    Scenario: Call DeleteVolume From Snapshot
      Given a Isilon service
      When I call DeleteVolume "snapVol3=_=_=47=_=_=System=_=_=cluster1"
      Then a valid DeleteVolumeResponse is returned

    Scenario: Call DeleteSnapshot
      Given a Isilon service
      When I call DeleteSnapshot "48=_=_=cluster1=_=_=System"
      Then the error contains "failed to unexport volume directory '0' in access zone '' : 'EOF'"

  Scenario: Create RO volume from snapshot With RootClient Enabled
    Given a Isilon service
    And I set RootClientEnabled to "true"
    And I call CreateVolumeFromSnapshotMultiReader "43" "snapVol1"
    And I call ControllerPublishVolume on Snapshot with name "snapVol1=_=_=43=_=_=System" and access type "multiple-reader" to "vpi7125=#=#=vpi7125.a.b.com=#=#=1.1.1.1" and path "/ifs/.snapshot/snapshot-snappath"
    Then a valid ControllerPublishVolumeResponse is returned
    Then a valid CreateVolumeResponse is returned

  Scenario: Call ProbeController with invalid input
    Given a Isilon service with params "" "controller"
    And I call ProbeController
    Then the error contains "probe of all isilon clusters failed"

  Scenario: ControllerGetVolume export does not exist scenario
    Given a Isilon service
    When I call Probe
    And I induce error "GetExportInternalError"
    And I call ControllerGetVolume with name "volume2=_=_=43=_=_=System=_=_=cluster1"
    Then the error contains "none"

  Scenario: Create RO volume from snapshot with NodePublishVolume
    Given a Isilon service
    When I call Probe
    And I call CreateVolumeFromSnapshotMultiReader "43" "snapVol1"
    And I call ControllerPublishVolume on Snapshot with name "snapVol1=_=_=43=_=_=System" and access type "multiple-reader" to "vpi7125=#=#=vpi7125.a.b.com=#=#=1.1.1.1" and path "/ifs/.snapshot/snapshot-snappath"
    And a capability with voltype "mount" access "multiple-reader"
    And get Node Publish Volume Request with Volume Name "volume2=_=_=43=_=_=System=_=_=cluster1" and path "/ifs/.snapshot/snapshot-snappath"
    And I call NodePublishVolume
    And get Node Unpublish Volume Request for RO Snapshot "volume2=_=_=43=_=_=System=_=_=cluster1" and path "/ifs/.snapshot/snapshot-snappath"
    And I call NodeUnpublishVolume
    Then a valid CreateVolumeResponse is returned

  Scenario: Create RO volume from snapshot with no volume name in NodePublishVolume
    Given a Isilon service
    When I call Probe
    And I call CreateVolumeFromSnapshotMultiReader "43" "snapVol1"
    And I call ControllerPublishVolume on Snapshot with name "snapVol1=_=_=43=_=_=System" and access type "multiple-reader" to "vpi7125=#=#=vpi7125.a.b.com=#=#=1.1.1.1" and path "/ifs/.snapshot/snapshot-snappath"
    And a capability with voltype "mount" access "multiple-reader"
    And get Node Publish Volume Request with Volume Name "volume2=_=_=43=_=_=System=_=_=cluster1" and path "/ifs/.snapshot/snapshot-snappath"
    And I call NodePublishVolume
    And get Node Unpublish Volume Request for RO Snapshot "" and path "/ifs/.snapshot/snapshot-snappath"
    And I call NodeUnpublishVolume
    Then the error contains "no VolumeID found in request"
