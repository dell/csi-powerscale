Feature: Isilon CSI interface
  As a consumer of the CSI interface
  I want to test service methods
  So that they are known to work

@podmon
@v1.0.0
  Scenario: Call ValidateConnectivity
    Given a Isilon service
    When I call Probe
    And I call CreateVolume "volume1"
    And a valid CreateVolumeResponse is returned
    And I call ValidateConnectivity
    Then the error contains "none"

  Scenario: Call ValidateConnectivity with no Node
    Given a Isilon service
    When I call CreateVolume "volume1"
    And I induce error "no-nodeId"
    And I call ValidateConnectivity
    Then the error contains "the NodeID is a required field"


  Scenario: Call ValidateConnectivity with no Volume no Node
    Given a Isilon service
    And I induce error "no-volume-no-nodeId"
    And I call ValidateConnectivity
    Then the error contains "ValidateVolumeHostConnectivity is implemented"

  Scenario: Call Validate Url Status
    Given a Isilon service
    And I call QueryArrayStatus "36443"
    Then the error contains "none"

