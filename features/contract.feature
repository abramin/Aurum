Feature: Aurum service contract
  Scenario: Health check is available
    Given the service is running
    When I request the health endpoint
    Then the response status should be 200
