Feature: Authorization lifecycle
  As a spend management system
  I need to manage authorization states correctly
  So that merchants can hold and settle funds appropriately

  Background:
    Given a card account with spending limit of 1000 EUR

  Scenario: Authorize amount within spending limit
    When I authorize 100 EUR for merchant "coffee-shop"
    Then the authorization should be created with status "authorized"
    And the available spending limit should be 900 EUR

  Scenario: Reject authorization exceeding spending limit
    When I authorize 1500 EUR for merchant "electronics-store"
    Then the authorization should be declined with error "spending limit exceeded"
    And the available spending limit should remain 1000 EUR

  Scenario: Capture authorized amount
    Given I have an authorization for 100 EUR
    When I capture 100 EUR from the authorization
    Then the authorization status should be "captured"

  Scenario: Partial capture of authorized amount
    Given I have an authorization for 100 EUR
    When I capture 50 EUR from the authorization
    Then the authorization status should be "captured"
    And the captured amount should be 50 EUR

  Scenario: Reject capture exceeding authorized amount
    Given I have an authorization for 100 EUR
    When I attempt to capture 150 EUR from the authorization
    Then the capture should be declined with error "exceeds authorized amount"
    And the authorization status should remain "authorized"

  Scenario: Reject double capture
    Given I have an authorization for 100 EUR
    And I have captured 100 EUR from the authorization
    When I attempt to capture 50 EUR from the authorization
    Then the capture should be declined with error "already captured"

  Scenario: Reverse authorized amount
    Given I have an authorization for 100 EUR
    When I reverse the authorization
    Then the authorization status should be "reversed"
    And the available spending limit should be 1000 EUR

  Scenario: Reject reversal of captured authorization
    Given I have an authorization for 100 EUR
    And I have captured 100 EUR from the authorization
    When I attempt to reverse the authorization
    Then the reversal should be declined with error "invalid state transition"
