Feature: Isilon CSI interface
    As a consumer of the CSI interface
    I want to test service methods
    So that they are known to work

    Scenario: Calling create quota in isiService with negative sizeInBytes
      Given a Isilon service
      When I call CreateQuota in isiService with negative sizeInBytes
      Then the error contains "none"

    Scenario: Calling get export with no result
      Given a Isilon service
      When I induce error "GetExportInternalError"
      And I call get export related functions in isiService
      Then the error contains "EOF"
