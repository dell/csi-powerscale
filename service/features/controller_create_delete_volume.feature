Feature: Isilon CSI interface
    As a consumer of the CSI interface
    I want to test service methods
    So that they are known to work

@createVolume
@v1.0.0
    Scenario: Create volume good scenario
      Given a Isilon service
      When I call Probe
      And I call CreateVolume "volume1"
      Then a valid CreateVolumeResponse is returned

    Scenario: Create volume good scenario with persistent metadata
      Given a Isilon service
      When I call Probe
      And I call CreateVolume with persistent metadata "volume1"
      Then a valid CreateVolumeResponse is returned

    Scenario: Create volume good scenario with quota enabled
      Given a Isilon service
      And I enable quota
      When I call CreateVolume "volume1"
      Then a valid CreateVolumeResponse is returned 

   Scenario Outline: Create volume with induced errors and quota enabled
      Given a Isilon service
      And I enable quota
      When I call Probe
      And I induce error <induced>
      And I call CreateVolume "volume1"
      Then the error contains <errormsg>

     Examples:
     | induced                             | errormsg                                           |
     | "InstancesError"                    | "Error retrieving Volume"                          |
     | "CreateQuotaError"                  | "error creating quota"                             |
     | "CreateExportError"                 | "EOF"                                              |
     | "GetExportInternalError"            | "EOF"                                              |
     | "none"                              | "none"                                             |

   Scenario Outline: Create volume with parameters
      Given a Isilon service
      When I call Probe
      And I call CreateVolume with params <volumeName> <rangeInGiB> <accessZone> <isiPath> <AzServiceIP> <clusterName>
      Then the error contains <errormsg>

     Examples:
     | volumeName    | rangeInGiB    | accessZone      | isiPath                   | AzServiceIP         | clusterName    | errormsg               |
     | "volume1"     | 8             | ""              | "/ifs/data/csi-isilon"    | "127.0.0.1"         | "none"         | "none"                 |
     | "volume1"     | 8             | "System"        | ""                        | "127.0.0.1"         | "none"         | "none"                 |
     | "volume1"     | 8             | "none"          | "/ifs/data/csi-isilon"    | "127.0.0.1"         | "none"         | "none"                 |
     | "volume1"     | 8             | "System"        | "none"                    | "127.0.0.1"         | "none"         | "none"                 |
     | "volume1"     | 8             | "System"        | "/ifs/data/csi-isilon"    | "none"              | "none"         | "none"                 |
     | "volume1"     | -1            | "System"        | "/ifs/data/csi-isilon"    | "none"              | "none"         | "must not be negative" |
     | ""            | 0             | "System"        | "/ifs/data/csi-isilon"    | "none"              | "none"         | "name cannot be empty" |
     | "volume1"     | 8             | "none"          | "/ifs/data/csi-isilon"    | "none"              | "none"         | "none"                 |
     | "volume1"     | 8             | ""              | "/ifs/data/csi-isilon"    | "127.0.0.1"         | "cluster1"     | "none"                 |
     | "volume1"     | 8             | "System"        | ""                        | "127.0.0.1"         | "cluster1"     | "none"                 |
     | "volume1"     | 8             | "none"          | "/ifs/data/csi-isilon"    | "127.0.0.1"         | "cluster1"     | "none"                 |
     | "volume1"     | 8             | "System"        | "none"                    | "127.0.0.1"         | "cluster1"     | "none"                 |
     | "volume1"     | 8             | "System"        | "/ifs/data/csi-isilon"    | "none"              | "cluster1"     | "none"                 |
     | "volume1"     | -1            | "System"        | "/ifs/data/csi-isilon"    | "none"              | "cluster1"     | "must not be negative" |
     | ""            | 0             | "System"        | "/ifs/data/csi-isilon"    | "none"              | "cluster1"     | "name cannot be empty" |
     | "volume1"     | 8             | "none"          | "/ifs/data/csi-isilon"    | "none"              | "cluster1"     | "none"                 |
     | "volume1"     | 8             | "none"          | "/ifs/data/csi-isilon"    | "127.0.0.1"         | "cluster2"     | "failed to get cluster config details for clusterName: 'cluster2'" |

    Scenario Outline: Create volume with different volume and export status and induce server errors
      Given a Isilon service
      And I enable quota
      When I call Probe
      And I induce error <getVolumeError>
      And I induce error <getExportError>
      And I induce error <serverError1>
      And I induce error <serverError2>
      And I call CreateVolume "volume1"
      Then the error contains <errormsg>

    Examples:
     | getVolumeError           | getExportError            | serverError1         | serverError2           | errormsg                                 |
     | "VolumeExists"           | "ExportExists"            | "none"               | "none"                 | "none"                                   |
     | "VolumeNotExistError"    | "ExportNotFoundError"     | "none"               | "none"                 | "none"                                   |
     | "VolumeExists"           | "ExportNotFoundError"     | "none"               | "none"                 | "the export may not be ready yet"        |
     | "VolumeNotExistError"    | "ExportExists"            | "none"               | "none"                 | "none"                                   |
     | "VolumeNotExistError"    | "ExportExists"            | "UnexportError"      | "none"                 | "failed to unexport volume directory"    |
     | "VolumeNotExistError"    | "ExportNotFoundError"     | "CreateQuotaError"   | "DeleteVolumeError"    | "failed to delete volume"                |
     | "VolumeNotExistError"    | "ExportNotFoundError"     | "CreateExportError"  | "DeleteQuotaError"     | "EOF"                                    |
     | "VolumeNotExistError"    | "ExportNotFoundError"     | "CreateExportError"  | "DeleteVolumeError"    | "EOF"                                    |


    Scenario: Create volume from volume good scenario
      Given a Isilon service
      When I call CreateVolumeFromVolume "volume2=_=_=19=_=_=System" "volume1"
      Then a valid CreateVolumeResponse is returned

    Scenario Outline: Create volume from volume with negative or idempotent arguments
      Given a Isilon service
      When I call CreateVolumeFromVolume <srcVolumeID> <volumeName>
      Then the error contains <errormsg>

      Examples:
      | srcVolumeID                   | volumeName      | errormsg                     |
      | "volume1=_=_=10=_=_=System"   | "volume1"       | "failed to get volume"       |
      | "volume2=_=_=20=_=_=System"   | "volume2"       | "none"                       |

    Scenario Outline: Create Volume with Replication Enabled and with invalid arguments
      Given a Isilon service
      When I call CreateVolumeRequestWithReplicationParams <vgPrefix> <rpo> <remoteSystemName>
      Then the error contains <errormsg>
      Examples:
      |  vgPrefix           | rpo               | remoteSystemName | errormsg                                                   |
      | ""                  | "Five_Minutes"    | "cluster1"       | "replication enabled but no volume group prefix specified" |
      | "volumeGroupPrefix" | ""                | "cluster1"       | "replication enabled but no RPO specified"                 |
      | "volumeGroupPrefix" | "Fifty_Minutes"   | "cluster1"       | "invalid rpo value"                                        |
      | "volumeGroupPrefix" | "Thirty_Minutes"  | ""               | "replication enabled but no remote system specified"       |

    Scenario Outline: Create Volume with Replication Enabled and induced errors
      Given a Isilon service
      And I induce error <induced>
      When I call CreateVolumeRequestWithReplicationParams "volumeGroupPrefix" <rpo> "cluster1"
      Then the error contains <errormsg>
      Examples:
      | induced                  | rpo               | errormsg                                   |
      | "GetPolicyInternalError" | "Fifteen_Minutes" | "can't ensure protection policy exists"    |
      | "GetPolicyNotFoundError" | "Six_Hours"       | "policy job couldn't reach FINISHED state" |

@deleteVolume
@v1.0.0
    Scenario: Delete volume good scenario with quota enabled
      Given a Isilon service
      And I enable quota
      When I call DeleteVolume "volume1=_=_=43=_=_=System"
      Then a valid DeleteVolumeResponse is returned

    Scenario Outline: Delete volume with invalid volume id
      Given a Isilon service
      And I enable quota
      When I call DeleteVolume <volumeID>
      Then the error contains <errormsg>

     Examples:
     | volumeID                                 | errormsg                                                           |
     | "volume1=_=_=43=_=_=System"              | "none"                                                             |
     | "volume1=_=_=43=_=_=System=_=_=cluster1" | "none"                                                             |
     | "volume1=_=_=43"                         | "failed to parse volume ID"                                        |
     | ""                                       | "no volume id is provided by the DeleteVolumeRequest instance"     |
     | "volume1=_=_=43=_=_=System=_=_=cluster2" | "failed to get cluster config details for clusterName: 'cluster2'" |

    Scenario Outline: Delete volume with induced errors
      Given a Isilon service
      And I enable quota
      And I induce error <induced>
      When I call DeleteVolume "volume1=_=_=557=_=_=System"
      Then the error contains <errormsg>

     Examples:
     | induced                          | errormsg                                                            |
     | "GetExportInternalError"         | "EOF"                                                               |
     | "GetExportByIDNotFoundError"     | "none"                                                              |
     | "VolumeNotExistError"            | "none"                                                              |
     | "DeleteQuotaError"               | "EOF"                                                               |
     | "GetExportInternalError"         | "EOF"                                                               |
     | "QuotaNotFoundError"             | "Failed to fetch quota domain record: No such file or directory"    |
