Feature: Spending limit enforcement
  As a finance team
  I need spending limits enforced across authorizations
  So that card spending stays within approved budgets

  Background:
    Given a card account with spending limit of 1000 EUR

  Scenario: Single authorization within limit
    When I authorize 500 EUR for merchant "office-supplies"
    Then the authorization should be created with status "authorized"
    And the rolling spend should be 500 EUR
    And the available spending limit should be 500 EUR

  Scenario: Multiple authorizations within cumulative limit
    When I authorize 300 EUR for merchant "coffee-shop"
    And I authorize 300 EUR for merchant "lunch-spot"
    And I authorize 300 EUR for merchant "office-supplies"
    Then all authorizations should be created with status "authorized"
    And the rolling spend should be 900 EUR
    And the available spending limit should be 100 EUR

  Scenario: Cumulative authorizations exceed limit
    Given I have an authorization for 600 EUR
    When I authorize 600 EUR for merchant "electronics-store"
    Then the authorization should be declined with error "spending limit exceeded"
    And the rolling spend should remain 600 EUR

  Scenario: Reversal restores available limit
    Given I have an authorization for 500 EUR
    When I reverse the authorization
    Then the rolling spend should be 0 EUR
    And the available spending limit should be 1000 EUR

  Scenario: Reject authorization with currency mismatch
    When I authorize 100 USD for merchant "foreign-vendor"
    Then the authorization should be declined with error "currency mismatch"

  Scenario: Idempotent authorization request
    When I authorize 100 EUR for merchant "coffee-shop" with idempotency key "order-123"
    And I authorize 100 EUR for merchant "coffee-shop" with idempotency key "order-123"
    Then only one authorization should be created
    And the rolling spend should be 100 EUR
