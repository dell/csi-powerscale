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

  Scenario Outline: Create remote volume bad scenario
    Given a Isilon service
    When I call Probe
    And I call BadCreateRemoteVolume
    Then the error contains <errormsg>
    Examples:
      | errormsg                                                 |
      | "can't find cluster with name cluster2 in driver config" |


  Scenario Outline: Create remote volume with parameters
    Given a Isilon service
    When I call Probe
    And I call WithParamsCreateRemoteVolume <volhand> <keyreplremsys>
    Then the error contains <errormsg>
    Examples:
      | volhand                                  | keyreplremsys                | errormsg                                                              |
      | ""                                       | "KeyReplicationRemoteSystem" | "volume ID is required"                                               |
      | "volume1=_=_=19=_=_=System"              | ""                           | "replication enabled but no remote system specified in storage class" |
      | "volume1=_=_=43=_=_=System=_=_=cluster2" | "KeyReplicationRemoteSystem" | "failed to get cluster config details for clusterName: 'cluster2'"    |
      | "volume1=_=_=43"                         | "KeyReplicationRemoteSystem" | "cannot be split into tokens"                                         |


  Scenario Outline: Create remote volume with induced errors and quota enabled
    Given a Isilon service
    And I enable quota
    When I call Probe
    And I induce error <induced>
    And I call CreateRemoteVolume
    Then the error contains <errormsg>
    Examples:
      | induced                  | errormsg                    |
      | "InstancesError"         | "none"                      |
      | "QuotaNotFoundError"     | "Failed to fetch quota"     |
      | "InvalidQuotaError"      | "none"                      |
      | "CreateExportError"      | "EOF"                       |
      | "GetExportInternalError" | "EOF"                       |
      | "none"                   | "none"                      |
      | "GetPolicyInternalError" | "failed to sync data "      |
      | "GetExportPolicyError"   | "NotFound"                  |
      | "autoProbeFailed"        | "auto probe is not enabled" |

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
      | getVolumeError        | getExportError        | serverError1    | serverError2 | errormsg |
      | "VolumeExists"        | "ExportExists"        | "none"          | "none"       | "none"   |
      | "VolumeNotExistError" | "ExportNotFoundError" | "none"          | "none"       | "none"   |
      | "VolumeExists"        | "ExportNotFoundError" | "none"          | "none"       | "none"   |
      | "VolumeNotExistError" | "ExportExists"        | "none"          | "none"       | "none"   |
      | "VolumeNotExistError" | "ExportExists"        | "UnexportError" | "none"       | "none"   |


  @createStorageProtectionGroup
  @v1.0.0
  Scenario: Create storage protection group
    Given a Isilon service
    When I call CreateStorageProtectionGroup
    Then a valid CreateStorageProtectionGroupResponse is returned

  Scenario Outline: Create storage protection group bad scenario
    Given a Isilon service
    When I call BadCreateStorageProtectionGroup
    Then the error contains <errormsg>
    Examples:
      | errormsg                                                 |
      | "can't find cluster with name cluster2 in driver config" |

  Scenario Outline: Create storage protection group and induce errors
    Given a Isilon service
    And I enable quota
    When I call Probe
    And I induce error <induced>
    And I call CreateStorageProtectionGroup
    Then the error contains <errormsg>
    Examples:
      | induced                      | errormsg                           |
      | "GetExportByIDNotFoundError" | "Export id 9999999 does not exist" |
      | "GetExportInternalError"     | "EOF"                              |
      | "autoProbeFailed"            | "auto probe is not enabled"        |

  Scenario Outline: Create storage protection group with parameters
    Given a Isilon service
    When I call Probe
    And I call WithParamsCreateStorageProtectionGroup <volhand> <keyreplremsys>
    Then the error contains <errormsg>
    Examples:
      | volhand                                  | keyreplremsys  | errormsg                                                              |
      | ""                                       | "remoteSystem" | "volume ID is required"                                               |
      | "volume1=_=_=19=_=_=System"              | ""             | "replication enabled but no remote system specified in storage class" |
      | "volume1=_=_=43=_=_=System=_=_=cluster2" | "remoteSystem" | "failed to get cluster config details for clusterName: 'cluster2'"    |
      | "volume1=_=_=43"                         | "remoteSystem" | "cannot be split into tokens"                                         |

  @deleteStorageProtectionGroup
  @v1.0.0
  Scenario Outline: Delete storage protection group
    Given a Isilon service
    When I call StorageProtectionGroupDelete <volume> and <systemname> and <clustername> and <vgname>
    Then the error contains <errormsg>
    Examples:
      | volume                                                | systemname        | clustername | vgname   | errormsg                                                           |
      | "cluster1::/ifs/data/csi-isilon/volume1"              | "systemName"      | "cluster1"  | ""       | "can't find `VolumeGroupName` parameter from PG params"            |
      | "cluster1::/ifs/data/csi-isilon/volume1"              | "wrongSystemName" | "cluster5"  | "vgname" | "Can't get systemName from PG params"                              |
      | "cluster1::/ifs/data/csi-isilon/volume1"              | "systemName"      | "cluster5"  | "vgname" | "failed to get cluster config details for clusterName: 'cluster5'" |
      | ""                                                    | "systemName"      | "cluster1"  | "vgname" | "Error: Can't obtain valid isiPath from PG"                        |
      | "cluster1::/ifs/badData/csi-isilon/volumeNonexistent" | "systemName"      | "cluster1"  | "vgname" | "none"                                                             |

  Scenario Outline: Delete storage protection group with induced errors
    Given a Isilon service
    And I induce error <induced>
    When I call StorageProtectionGroupDelete "cluster1::/ifs/data/csi-isilon/volume1" and "systemName" and "cluster1" and "vgname"
    Then the error contains <errormsg>
    Examples:
      | induced                   | errormsg                              |
      | "GetPolicyInternalError"  | "Unknown error while retrieving PP"   |
      | "DeletePolicyError"       | "Unknown error while deleting PP"     |
      | "GetJobsInternalError"    | "none"                                |

  Scenario Outline: Get storage protection group status with parameters
    Given a Isilon service
    When I call Probe
    And I call WithParamsGetStorageProtectionGroupStatus <id> <localSystemName> <remoteSystemName> <vgname> <clustername1> <clustername2>
    Then the error contains <errormsg>
    Examples:
      | id         | localSystemName   | remoteSystemName   | vgname   | clustername1 | clustername2 | errormsg                                                                                                                   |
      | "cluster2" | "wrongSystemName" | "wrongSystemName"  | "vgname" | "cluster1"   | "cluster1"   | "can't find `systemName` in replication group"                                                                             |
      | "cluster2" | "systemName"      | "wrongSystemName"  | "vgname" | "cluster1"   | "cluster1"   | "can't find `remoteSystemName` parameter in replication group"                                                             |
      | "cluster2" | "systemName"      | "remoteSystemName" | "vgname" | "cluster1"   | "cluster1"   | "can't find `VolumeGroupName` parameter in replication group"                                                              |
      | "cluster2" | "systemName"      | "remoteSystemName" | "vgname" | "cluster1"   | "cluster1"   | "can't find `VolumeGroupName` parameter in replication group"                                                              |
      | "cluster2" | "systemName"      | "remoteSystemName" | "vgname" | "cluster1"   | "cluster1"   | "can't find `VolumeGroupName` parameter in replication group"                                                              |
      | "cluster2" | "systemName"      | "remoteSystemName" | "vgname" | "cluster2"   | "cluster1"   | "can't find cluster with name cluster2 in driver config: failed to get cluster config details for clusterName: 'cluster2'" |
      | "cluster2" | "systemName"      | "remoteSystemName" | "vgname" | "cluster1"   | "cluster2"   | "can't find cluster with name cluster2 in driver config: failed to get cluster config details for clusterName: 'cluster2'" |

  Scenario Outline: Get storage protection group status and induce errors
    Given a Isilon service
    And I induce error <induced>
    And I call GetStorageProtectionGroupStatus
    Then the error contains <errormsg>
    Examples:
      | induced                        | errormsg                                          |
      | "GetJobsInternalError"         | "querying active jobs for local or remote policy" |
      | "GetPolicyInternalError"       | "error while getting link state"                  |
      | "GetTargetPolicyInternalError" | "error while getting link state"                  |
      | "FailedStatus"                 | "none"                                            |
      | "UnknownStatus"                | "error while getting link state"                  |
      | "Jobs"                         | "querying active jobs for local or remote policy" |
      | "RunningJob"                   | "none"                                            |
      | "GetSpgErrors"                 | "error while getting link state"                  |
      | "GetSpgTPErrors"               | "error while getting link state"                  |

  Scenario Outline: Delete local volume with parameters
    Given a Isilon service
    When I call Probe
    And I call WithParamsDeleteLocalVolume <volhandle>
    Then the error contains <errormsg>
    Examples:
      | volhandle                                | errormsg                                                           |
      | "volume1=_=_=43"                         | "cannot be split into tokens"                                      |
      | "volume1=_=_=xx=_=_=System=_=_=cluster1" | "failed to parse volume ID"                                        |
      | "volume1=_=_=43=_=_=System=_=_=cluster2" | "failed to get cluster config details for clusterName: 'cluster2'" |

  Scenario Outline: Delete local volume with induced errors
    Given a Isilon service
    And I enable quota
    When I call Probe
    And I induce error <induced>
    And I call DeleteLocalVolume
    Then the error contains <errormsg>
    Examples:
      | induced                      | errormsg                                                                      |
      | "GetExportByIDNotFoundError" | "none"                                                                        |
      | "GetExportInternalError"     | "EOF"                                                                         |
      | "none"                       | "has other clients in AccessZone System. It is not safe to delete the export" |

  @executeAction
  @v1.0.0
  Scenario Outline: Execute action
    Given a Isilon service
    When I call ExecuteAction to <systemName> to <clusterNameOne> to <clusterNameTwo> to <remoteSystemName> to <vgname> to <ppname>
    Then the error contains <errormsg>
    Examples:
      | systemName        | clusterNameOne | clusterNameTwo | remoteSystemName   | vgname            | ppname                                               | errormsg                                                                                                                   |
      | "systemName"      | "cluster1"     | "cluster1"     | "remoteSystemName" | "VolumeGroupName" | "csi-prov-test-19743d82-192-168-111-25-Five_Minutes" | "error while getting link state"                                                                                           |
      | "wrongSystemName" | "cluster1"     | "cluster1"     | "remoteSystemName" | "VolumeGroupName" | "csi-prov-test-19743d82-192-168-111-25-Five_Minutes" | "can't find `systemName` parameter in replication group"                                                                   |
      | "systemName"      | "cluster2"     | "cluster1"     | "remoteSystemName" | "VolumeGroupName" | "csi-prov-test-19743d82-192-168-111-25-Five_Minutes" | "can't find cluster with name cluster2 in driver config: failed to get cluster config details for clusterName: 'cluster2'" |
      | "systemName"      | "cluster1"     | "cluster2"     | "remoteSystemName" | "VolumeGroupName" | "csi-prov-test-19743d82-192-168-111-25-Five_Minutes" | "can't find cluster with name cluster2 in driver config: failed to get cluster config details for clusterName: 'cluster2'" |
      | "systemName"      | "cluster1"     | "cluster1"     | "wrongSystemName"  | "VolumeGroupName" | "csi-prov-test-19743d82-192-168-111-25-Five_Minutes" | "can't find `remoteSystemName` parameter in replication group"                                                             |
      | "systemName"      | "cluster1"     | "cluster1"     | "remoteSystemName" | ""                | "csi-prov-test-19743d82-192-168-111-25-Five_Minutes" | "can't find `VolumeGroupName` parameter in replication group"                                                              |

  Scenario Outline: Execute action suspend
    Given a Isilon service
    And I induce error <induced>
    When I call SuspendExecuteAction
    Then the error contains <errormsg>
    Examples:
      | induced             | errormsg                                            |
      | "UpdatePolicyError" | "suspend: can't disable local policy"               |
      | "autoProbeFailed"   | "auto probe is not enabled"                         |
      | "GetSpgErrors"      | "suspend: policy couldn't reach disabled condition" |

  Scenario Outline: Execute action sync
    Given a Isilon service
    And I induce error <induced>
    When I call SyncExecuteAction
    Then the error contains <errormsg>
    Examples:
      | induced                  | errormsg                                                  |
      | "GetPolicyError"         | "policy sync failed EOF"                                  |
      | "GetJobsInternalError"   | "policy sync failed"                                      |
      | "QuotaScanError"         | "error while retrieving reports for failed sync job"      |
      | "JobReportErrorNotFound" | "found no retryable error in reports for failed sync job" |
      | "UpdatePolicyError"      | "policy sync failed"                                      |

  Scenario Outline: Execute action failover
    Given a Isilon service
    When I induce error <induced>
    And I call FailoverExecuteAction
    Then the error contains <errormsg>
    Examples:
      | induced                  | errormsg                                                 |
      | "GetPolicyInternalError" | "failover: encountered error when trying to sync policy" |
      | "GetJobsInternalError"   | "failover: encountered error when trying to sync policy" |
      | "GetSpgErrors"           | "failover: can't disable local policy"                   |

  Scenario Outline: Execute action unplanned failover
    Given a Isilon service
    When I induce error <induced>
    And I call FailoverUnplannedExecuteAction
    Then the error contains <errormsg>
    Examples:
      | induced                        | errormsg                                                 |
      | "GetTargetPolicyInternalError" | "unplanned failover: allow writes on target site failed" |

  Scenario Outline: Execute action failback discard local
    Given a Isilon service
    When I induce error <induced>
    And I call FailbackExecuteAction
    Then the error contains <errormsg>
    Examples:
      | induced                        | errormsg                                                                            |
      | "GetPolicyInternalError"       | "failback (discard local): can't disable local policy"                              |
      | "GetTargetPolicyInternalError" | "failback (discard local): error waiting for condition on the remote target policy" |
      | "UpdatePolicyError"            | "failback (discard local): can't disable local policy"                              |
      | "ModifyPolicyError"            | "failback (discard local): can't set local policy to manual"                        |
      | "GetJobsInternalError"         | "failback (discard local): can't run resync-prep on local policy"                   |

  Scenario Outline: Execute action failback discard remote
    Given a Isilon service
    When I induce error <induced>
    And I call FailbackDiscardExecuteAction
    Then the error contains <errormsg>
    Examples:
      | induced                        | errormsg                                                           |
      | "GetPolicyInternalError"       | "failback (discard remote): can't disable local policy"            |
      | "GetTargetPolicyInternalError" | "failback (discard remote): disallow writes on target site failed" |
      | "UpdatePolicyError"            | "failback (discard remote): can't disable local policy"            |
      | "ModifyPolicyError"            | "failback (discard remote): can't set local policy to manual"      |

  Scenario Outline: Execute action reprotect
    Given a Isilon service
    When I induce error <induced>
    And I call ReprotectExecuteAction
    Then the error contains <errormsg>
    Examples:
      | induced                        | errormsg                                                                                   |
      | "GetTargetPolicyInternalError" | "reprotect: can't find target policy on the local site, perform reprotect on another side" |
      | "GetPolicyInternalError"       | "reprotect: can't find remote replication policy"                                          |
      | "DeletePolicyError"            | "reprotect: delete policy on remote site failed"                                           |
      | "CreatePolicyError"            | "reprotect: create protection policy on the local site failed"                             |

  Scenario Outline: Execute action
    Given a Isilon service
    When I induce error <induced>
    And I call ExecuteAction to <systemName> to <clusterNameOne> to <clusterNameTwo> to <remoteSystemName> to <vgname> to <ppname>
    Then the error contains <errormsg>
    Examples:
      | induced                  | systemName   | clusterNameOne | clusterNameTwo | remoteSystemName   | vgname            | ppname                                               | errormsg                                          |
      | "GetPolicyInternalError" | "systemName" | "cluster1"     | "cluster1"     | "remoteSystemName" | "VolumeGroupName" | "csi-prov-test-19743d82-192-168-111-25-Five_Minutes" | "resume: can't enable local policy"               |
      | "GetSpgErrors"           | "systemName" | "cluster1"     | "cluster1"     | "remoteSystemName" | "VolumeGroupName" | "csi-prov-test-19743d82-192-168-111-25-Five_Minutes" | "resume: policy couldn't reach enabled condition" |

  @executeAction
  Scenario Outline: Execute action failback with bad params
    Given a Isilon service
    And I call ExecuteActionFailBackWithParams to <systemName> to <clusterNameOne> to <clusterNameTwo> to <remoteSystemName> to <vgname> to <ppname>
    Then the error contains <errormsg>
    Examples:
      | systemName   | clusterNameOne | clusterNameTwo | remoteSystemName   | vgname            | ppname                                            | errormsg                      |
      | "systemName" | "cluster1"     | "cluster1"     | "remoteSystemName" | "VolumeGroupName" | "csi-prov-test-19743d82-192-168-111-25-Fifty_Min" | "unable to parse RPO seconds" |

  @executeAction
  Scenario Outline: Execute action failback discard with bad params
    Given a Isilon service
    And I call ExecuteActionFailBackDiscardWithParams to <systemName> to <clusterNameOne> to <clusterNameTwo> to <remoteSystemName> to <vgname> to <ppname>
    Then the error contains <errormsg>
    Examples:
      | systemName   | clusterNameOne | clusterNameTwo | remoteSystemName   | vgname            | ppname                                            | errormsg                      |
      | "systemName" | "cluster1"     | "cluster1"     | "remoteSystemName" | "VolumeGroupName" | "csi-prov-test-19743d82-192-168-111-25-Fifty_Min" | "unable to parse RPO seconds" |

  Scenario Outline: Execute bad action
    Given a Isilon service
    When I call BadExecuteAction
    Then the error contains <errormsg>
    Examples:
      | errormsg                                                 |
      | "requested action does not match with supported actions" |
