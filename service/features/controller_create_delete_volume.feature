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
      And I call CreateVolume with params <volumeName> <rangeInGiB> <accessZone> <isiPath> <AzServiceIP>
      Then the error contains <errormsg>

     Examples:
     | volumeName    | rangeInGiB    | accessZone      | isiPath                   | AzServiceIP         | errormsg                 |
     | "volume1"     | 8             | ""              | "/ifs/data/csi-isilon"    | "127.0.0.1"         | "none"                   |
     | "volume1"     | 8             | "System"        | ""                        | "127.0.0.1"         | "none"                   |
     | "volume1"     | 8             | "none"          | "/ifs/data/csi-isilon"    | "127.0.0.1"         | "none"                   |
     | "volume1"     | 8             | "System"        | "none"                    | "127.0.0.1"         | "none"                   |
     | "volume1"     | 8             | "System"        | "/ifs/data/csi-isilon"    | "none"              | "none"                   |
     | "volume1"     | -1            | "System"        | "/ifs/data/csi-isilon"    | "none"              | "must not be negative"   |
     | ""            | 0             | "System"        | "/ifs/data/csi-isilon"    | "none"              | "name cannot be empty"   |
     | "volume1"     | 8             | "none"          | "/ifs/data/csi-isilon"    | ""                  | "none"                   |

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
     | volumeID                         | errormsg                                                          |
     | "volume1=_=_=43=_=_=System"      | "none"                                                            |
     | "volume1=_=_=43"                 | "failed to parse volume ID"                                       |
     | ""                               | "no volume id is provided by the DeleteVolumeRequest instance"    |

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