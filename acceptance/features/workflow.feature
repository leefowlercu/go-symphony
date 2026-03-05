Feature: Workflow loading
  Symphony loads WORKFLOW.md contracts from disk.

  Scenario: Load a workflow with front matter and prompt body
    Given a workflow file with tracker kind "linear"
    When I load the workflow definition
    Then the workflow config value "tracker.kind" equals "linear"
    And the workflow prompt contains "Issue"

  Scenario: Missing workflow file returns typed error
    Given a missing workflow file path
    When I load the workflow definition
    Then the workflow error code equals "missing_workflow_file"
