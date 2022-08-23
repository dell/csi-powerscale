Feature: Isilon CSI interface
  As a consumer of the CSI interface
  I want to test volume expansion service methods
  So that they are known to work

@expandVolume
@v1.1.0
  Scenario: Controller Expand volume good scenario with Quota enabled
    Given a Isilon service
    And I enable quota
    When I call ControllerExpandVolume "volume1=_=_=557=_=_=System" "108589934592"
    Then a valid ControllerExpandVolumeResponse is returned

  Scenario: Controller Expand volume good scenario with Quota enabled and non-existing volume
    Given a Isilon service
    And I enable quota
    When I call ControllerExpandVolume "volume1=_=_=557=_=_=System=_=_=cluster1" "108589934592"
    Then a valid ControllerExpandVolumeResponse is returned

  Scenario: Controller Expand volume negative scenario with Quota enabled
    Given a Isilon service
    And I enable quota
    When I call ControllerExpandVolume "volume1=_=_=557=_=_=System=_=_=cluster2" "108589934592"
    Then the error contains "failed to get cluster config details for clusterName: 'cluster2'"

  Scenario: Controller Expand volume good scenario with Quota disabled
    Given a Isilon service
    When I call ControllerExpandVolume "volume1=_=_=557=_=_=System" "108589934592"
    Then a valid ControllerExpandVolumeResponse is returned

  Scenario: Controller Expand volume idempotent scenario with Quota enabled
    Given a Isilon service
    And I enable quota
    When I call ControllerExpandVolume "volume1=_=_=557=_=_=System" "589934592"
    Then a valid ControllerExpandVolumeResponse is returned

  Scenario Outline: Controller Expand volume with negative arguments and Quota enabled
    Given a Isilon service
    And I enable quota
    When I call ControllerExpandVolume <volumeID> <requiredBytes>
    Then a valid ControllerExpandVolumeResponse is returned

    Examples:
    | volumeID                          | requiredBytes     |
    | "volume1=_=_=557=_=_=System"      | "-108589934592"    |

   Scenario Outline: Controller Expand volume with induced errors and Quota enabled
      Given a Isilon service
      And I enable quota
      When I induce error <induced>
      And I call ControllerExpandVolume "volume1=_=_=557=_=_=System" "108589934592"
      Then the error contains <errormsg>

     Examples:
     | induced                             | errormsg                                           |
     | "UpdateQuotaError"                  | "failed to update quota"                           |

   Scenario: Calling functions with autoProbe failed
      Given a Isilon service
      And I induce error "autoProbeFailed"
      When I call ControllerExpandVolume "volume1=_=_=557=_=_=System" "108589934592"
      Then the error contains "auto probe is not enabled"

   Scenario: Calling functions with invalid volume id
      Given a Isilon service
      When I call ControllerExpandVolume "volume1=_=_=557" "108589934592"
      Then the error contains "volume ID @@ cannot be split into tokens"
