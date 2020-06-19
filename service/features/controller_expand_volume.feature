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
    Then the error contains <errormsg>

    Examples:
    | volumeID                      | reqiuredBytes   | errormsg                          |
    | "volume1"                     | "108589934592"  | "cannot match the expected"       |

   Scenario Outline: Controller Expand volume with induced errors and Quota enabled
      Given a Isilon service
      And I enable quota
      When I induce error <induced>
      And I call ControllerExpandVolume "volume1=_=_=557=_=_=System" "108589934592"
      Then the error contains <errormsg>

     Examples:
     | induced                             | errormsg                                           |
     | "UpdateQuotaError"                  | "failed to update quota"                           |
