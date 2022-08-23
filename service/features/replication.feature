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
      | induced                  | errormsg                     |
      | "InstancesError"         | "none"                       |
      | "CreateQuotaError"       | "EOF"                        |
      | "CreateExportError"      | "EOF"                        |
      | "GetExportInternalError" | "EOF"                        |
      | "none"                   | "none"                       |
      | "GetPolicyInternalError" | "failed to sync data "       |
      | "GetExportPolicyError"   | "NotFound"                   |
      | "autoProbeFailed"        | "auto probe is not enabled"  |

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
    When I call StorageProtectionGroupDelete <volume> and <systemname> and <clustername>
    Then the error contains <errormsg>
    Examples:
      | volume                                   | systemname        | clustername | errormsg                                                           |
      | "cluster1::/ifs/data/csi-isilon/volume1" | "systemName"      | "cluster1"  | "Unable to get Volume Group '/ifs/data/csi-isilon/volume1'"        |
      | "cluster1::/ifs/data/csi-isilon/volume1" | "wrongSystemName" | "cluster5"  | "Can't get systemName from PG params"                              |
      | "cluster1::/ifs/data/csi-isilon/volume1" | "systemName"      | "cluster5"  | "failed to get cluster config details for clusterName: 'cluster5'" |
      | ""                                       | "systemName"      | "cluster1"  | "Unable to get Volume Group ''"                                    |
      | ""                                       | ""                | "cluster1"  | "Can't get systemName from PG params"                              |


  @getStorageProtectionGroupStatus
  @v1.0.0
  Scenario: Get storage protection group status
    Given a Isilon service
    When I call GetStorageProtectionGroupStatus
    Then a valid GetStorageProtectionGroupStatusResponse is returned

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
      | induced                        | errormsg                                   |
      | "GetJobsInternalError"         | "can't find active jobs for local policy"  |
      | "GetPolicyInternalError"       | "can't find local replication policy"      |
      | "GetTargetPolicyInternalError" | "can't find local replication policy"      |
      | "GetExportInternalError"       | "none"                                     |
      | "UnexportError"                | "none"                                     |
      | "FailedStatus"                 | "none"                                     |
      | "UnknownStatus"                | "none"                                     |
      | "Jobs"                         | "can't find active jobs for remote policy" |
      | "GetSpgErrors"                 | "can't find remote replication policy"     |
      | "GetSpgTPErrors"               | "can't find remote replication policy"     |

  @executeAction
    @v1.0.0
  Scenario Outline: Execute action
    Given a Isilon service
    When I call ExecuteAction to <systemName> to <clusterNameOne> to <clusterNameTwo> to <remoteSystemName> to <vgname> to <ppname>
    Then the error contains <errormsg>
    Examples:
      | systemName        | clusterNameOne | clusterNameTwo | remoteSystemName   | vgname            | ppname                                               | errormsg                                                                                                                   |
      | "systemName"      | "cluster1"     | "cluster1"     | "remoteSystemName" | "VolumeGroupName" | "csi-prov-test-19743d82-192-168-111-25-Five_Minutes" | "none"                                                                                                                     |
      | "wrongSystemName" | "cluster1"     | "cluster1"     | "remoteSystemName" | "VolumeGroupName" | "csi-prov-test-19743d82-192-168-111-25-Five_Minutes" | "can't find `systemName` parameter in replication group"                                                                   |
      | "systemName"      | "cluster2"     | "cluster1"     | "remoteSystemName" | "VolumeGroupName" | "csi-prov-test-19743d82-192-168-111-25-Five_Minutes" | "can't find cluster with name cluster2 in driver config: failed to get cluster config details for clusterName: 'cluster2'" |
      | "systemName"      | "cluster1"     | "cluster2"     | "remoteSystemName" | "VolumeGroupName" | "csi-prov-test-19743d82-192-168-111-25-Five_Minutes" | "can't find cluster with name cluster2 in driver config: failed to get cluster config details for clusterName: 'cluster2'" |
      | "systemName"      | "cluster1"     | "cluster1"     | "wrongSystemName"  | "VolumeGroupName" | "csi-prov-test-19743d82-192-168-111-25-Five_Minutes" | "can't find `remoteSystemName` parameter in replication group"                                                             |
      | "systemName"      | "cluster1"     | "cluster1"     | "remoteSystemName" | ""                | "csi-prov-test-19743d82-192-168-111-25-Five_Minutes" | "can't find `VolumeGroupName` parameter in replication group"                                                              |

  Scenario Outline: Execute action suspend
    Given a Isilon service
    And I enable quota
    When I call SuspendExecuteAction
    Then the error contains <errormsg>

    Examples:
      | errormsg                                                                                                  |
      | "suspend: can't create suspend volume in csi-prov-test-19743d82-192-168-111-25-Five_Minutes volume group" |

  Scenario Outline: Execute action suspend
    Given a Isilon service
    And I induce error <induced>
    When I call SuspendExecuteAction
    Then the error contains <errormsg>

    Examples:
      | induced                  | errormsg                              |
      | "UpdatePolicyError"      | "suspend: can't disable local policy" |
      | "autoProbeFailed"        | "auto probe is not enabled" |
    
  Scenario: Execute action sync
    Given a Isilon service
    And I enable quota
    When I call SyncExecuteAction
    Then a valid ExecuteActionResponse is returned

  Scenario Outline: Execute action sync
    Given a Isilon service
    And I induce error <induced>
    When I call SyncExecuteAction
    Then the error contains <errormsg>

    Examples:
      | induced          | errormsg             |
      | "GetPolicyError" | "policy sync failed" |

  Scenario: Execute action failover
    Given a Isilon service
    And I enable quota
    When I call FailoverExecuteAction
    Then a valid ExecuteActionResponse is returned

  Scenario: Execute action unplanned failover
    Given a Isilon service
    When I call FailoverUnplannedExecuteAction
    Then a valid ExecuteActionResponse is returned

  Scenario Outline: Execute action failover
    Given a Isilon service
    When I induce error <induced>
    And I call FailoverExecuteAction
    Then the error contains <errormsg>
    Examples:
      | induced                  | errormsg                                                 |
      | "GetPolicyInternalError" | "failover: encountered error when trying to sync policy" |
      | "GetJobsInternalError"   | "failover: can't allow writes on target site EOF"        |
      | "UpdatePolicyError"      | "failover: can't disable local policy"                   |
      | "Failover"               | "failover: can't create protection policy"               |
      | "FailoverTP"             | "failover: couldn't get target policy"                   |
      | "GetSpgTPErrors"         | "failover: can't allow writes on target site"            |

  Scenario Outline: Execute action unplanned failover
    Given a Isilon service
    When I induce error <induced>
    And I call FailoverUnplannedExecuteAction
    Then the error contains <errormsg>
    Examples:
      | induced                        | errormsg                                                     |
      | "GetTargetPolicyInternalError" | "unplanned failover: can't break association on target site" |

  Scenario Outline: Execute action reprotect
    Given a Isilon service
    When I induce error <induced>
    And I call ReprotectExecuteAction
    Then the error contains <errormsg>
    Examples:
      | induced                        | errormsg                                                    |
      | "GetPolicyInternalError"       | "reprotect: can't get policy"                               |
      | "GetPolicyNotFoundError"       | "reprotect: can't create protection policy EOF"             |
      | "GetTargetPolicyInternalError" | "reprotect: can't find remote replication policy"           |
      | "GetPolicyError"               | "reprotect: can't ensure protection policy exists "         |
      | "Reprotect"                    | "reprotect: policy couldn't reach enabled condition on TGT" |
      | "ReprotectTP"                  | "none" |

  Scenario Outline: Execute action
    Given a Isilon service
    When I induce error <induced>
    And I call ExecuteAction to <systemName> to <clusterNameOne> to <clusterNameTwo> to <remoteSystemName> to <vgname> to <ppname>
    Then the error contains <errormsg>
    Examples:
      | induced                  | systemName   | clusterNameOne | clusterNameTwo | remoteSystemName   | vgname            | ppname                                               | errormsg                              |
      | "GetPolicyInternalError" | "systemName" | "cluster1"     | "cluster1"     | "remoteSystemName" | "VolumeGroupName" | "csi-prov-test-19743d82-192-168-111-25-Five_Minutes" | "suspend: can't disable local policy" |







