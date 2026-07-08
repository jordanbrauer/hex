Feature: A counter that adds and subtracts

  Scenario: adding numbers
    Given a counter starting at 0
    When I add 5
    And I add 3
    Then the counter is 8

  Scenario: adding then subtracting
    Given a counter starting at 10
    When I add 4
    And I subtract 2
    Then the counter is 12

  Scenario: starting nonzero
    Given a counter starting at 100
    Then the counter is 100
