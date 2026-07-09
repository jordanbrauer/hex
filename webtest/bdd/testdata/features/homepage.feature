Feature: homepage

  Scenario: welcomes the user
    When I visit "/"
    Then the response status should be 200
    And I should see "Welcome, Alice"
    And "h1" should have text "Welcome"
    And there should be 3 elements matching ".user-card"
    And ".user-card" should have class "active"
    And "button" should have attribute "data-testid" = "add-user"
    And ".missing" should not exist
