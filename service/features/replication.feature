Feature: Isilon CSI interface
    As a consumer of the CSI interface
    I want to test service methods
    So that they are known to work

@createRemoteVolume
@v1.0.0
    Scenario: Create remote volume good scenario
          Given a Isilon service
          When I call Probe
          And I call CreateRemoteVolume
          Then a valid CreateRemoteVolumeResponse is returned

    Scenario Outline: Create remote volume with parameters
          Given a Isilon service
          When I call Probe
          And I call WithParamsCreateRemoteVolume <volhand> <keyreplremsys>
          Then the error contains <errormsg>

     Examples:
     |volhand                                   |keyreplremsys                |errormsg                                                             |
     |""                                        |"KeyReplicationRemoteSystem" |"volume ID is required"                                              |
     |"volume1=_=_=19=_=_=System"               | ""                          |"replication enabled but no remote system specified in storage class"|
     |"volume1=_=_=43=_=_=System=_=_=cluster2"  |"KeyReplicationRemoteSystem" |"failed to get cluster config details for clusterName: 'cluster2'"   |


    Scenario Outline: Create remote volume with induced errors and quota enabled
       Given a Isilon service
       And I enable quota
       When I call Probe
       And I induce error <induced>
       And I call CreateRemoteVolume
       Then the error contains <errormsg>

      Examples:
      | induced                             | errormsg                                           |
      | "InstancesError"                    | "none"                                             |
      | "CreateQuotaError"                  | "EOF"                                              |
      | "CreateExportError"                 | "EOF"                                              |
      | "GetExportInternalError"            | "EOF"                                              |
      | "none"                              | "none"                                             |

    Scenario Outline: Create remote volume with different volume and export status and induce server errors
       Given a Isilon service
       And I enable quota
       When I call Probe
       And I induce error <getVolumeError>
       And I induce error <getExportError>
       And I induce error <serverError1>
       And I induce error <serverError2>
       And I call CreateRemoteVolume
       Then the error contains <errormsg>

     Examples:
      | getVolumeError           | getExportError            | serverError1         | serverError2           | errormsg                                 |
      | "VolumeExists"           | "ExportExists"            | "none"               | "none"                 | "none"                                   |
      | "VolumeNotExistError"    | "ExportNotFoundError"     | "none"               | "none"                 | "none"                                   |
      | "VolumeExists"           | "ExportNotFoundError"     | "none"               | "none"                 | "none"                                   |
      | "VolumeNotExistError"    | "ExportExists"            | "none"               | "none"                 | "none"                                   |
      | "VolumeNotExistError"    | "ExportExists"            | "UnexportError"      | "none"                 | "none"                                   |



@createStorageProtectionGroup
@v1.0.0
    Scenario: Create storage protection group
            Given a Isilon service
            When I call CreateStorageProtectionGroup
            Then a valid CreateStorageProtectionGroupResponse is returned

    Scenario Outline: Create storage protection group and induce errors
            Given a Isilon service
            And I enable quota
            When I call Probe
            And I induce error <induced>
            And I call CreateStorageProtectionGroup
            Then the error contains <errormsg>

        Examples:
        | induced                             | errormsg                                           |
        | "GetExportByIDNotFoundError"        | "Export id 9999999 does not exist"                 |
        | "GetExportInternalError"            | "EOF"                                              |

  Scenario Outline: Create storage protection group with parameters
    Given a Isilon service
    When I call Probe
    And I call WithParamsCreateStorageProtectionGroup <volhand> <keyreplremsys>
    Then the error contains <errormsg>

    Examples:
      |volhand                                   |keyreplremsys                |errormsg                                                             |
      |""                                        |"remoteSystem"               |"volume ID is required"                                              |
      |"volume1=_=_=19=_=_=System"               | ""                          |"replication enabled but no remote system specified in storage class"|
      |"volume1=_=_=43=_=_=System=_=_=cluster2"  |"remoteSystem"               |"failed to get cluster config details for clusterName: 'cluster2'"   |

@deleteStorageProtectionGroup
@v1.0.0
    Scenario: Delete storage protection group
            Given a Isilon service
            When I call DeleteStorageProtectionGroup
            Then a valid DeleteStorageProtectionGroupResponse is returned