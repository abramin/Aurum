Feature: Authorization management
  As a card issuer
  I want to authorize and capture spending
  So that I can control card transactions

  Background:
    Given a tenant "acme-corp"

  Scenario: Create authorization with idempotency
    Given an idempotency key "auth-001"
    When I create an authorization for 100.00 EUR
    Then the authorization should be in "Authorized" state
    And repeating the request returns the same authorization

  Scenario: Full capture of authorization
    Given an authorization for 100.00 EUR in "Authorized" state
    When I capture 100.00 EUR
    Then the authorization should be in "Captured" state

  Scenario: Partial capture of authorization
    Given an authorization for 100.00 EUR in "Authorized" state
    When I capture 60.00 EUR
    Then the authorization should be in "Captured" state
    And the captured amount should be 60.00 EUR

  Scenario: Double capture is rejected
    Given an authorization for 100.00 EUR in "Captured" state
    When I attempt to capture 50.00 EUR
    Then the capture should be rejected with "already captured"

  Scenario: Capture exceeding authorized amount is rejected
    Given an authorization for 100.00 EUR in "Authorized" state
    When I attempt to capture 150.00 EUR
    Then the capture should be rejected with "exceeds authorized amount"

  Scenario: Spending limit enforcement
    Given a card account with spending limit 500.00 EUR
    And existing authorizations totaling 450.00 EUR
    When I attempt to create an authorization for 100.00 EUR
    Then the authorization should be rejected with "spending limit exceeded"
