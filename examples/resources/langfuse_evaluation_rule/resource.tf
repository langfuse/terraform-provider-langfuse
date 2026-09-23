# Evaluate half of all production generations with two evaluators.
resource "langfuse_evaluation_rule" "production_generations" {
  project_public_key = local.langfuse_project_public_key
  project_secret_key = local.langfuse_project_secret_key

  name     = "Production generations"
  enabled  = true
  sampling = 0.5

  # Omit filter to match every incoming observation.
  filter = [
    {
      type     = "stringOptions"
      column   = "type"
      operator = "any of"
      values   = ["GENERATION"]
    },
    {
      type     = "string"
      column   = "promptName"
      operator = "="
      value    = "chat-answer"
    },
    {
      type     = "stringObject"
      column   = "metadata"
      key      = "tenant"
      operator = "="
      value    = "acme"
    },
  ]

  evaluator_assignments = [
    {
      # Inherits the evaluator's default variable_mapping.
      evaluator_id = langfuse_evaluator.helpfulness.id
    },
    {
      evaluator_id = langfuse_evaluator.tone.id
      # Rule-specific mapping: every variable of the evaluator must appear exactly once.
      variable_mapping = [
        { variable = "output", source = "metadata", json_path = "$.final_answer" },
      ]
    },
    {
      # Code evaluators use a fixed runtime mapping; do not set variable_mapping.
      evaluator_id = langfuse_evaluator.answer_length.id
    },
  ]
}

# Draft rule: disabled rules may omit evaluator_assignments.
resource "langfuse_evaluation_rule" "experiment_draft" {
  project_public_key = local.langfuse_project_public_key
  project_secret_key = local.langfuse_project_secret_key

  name    = "Experiment runs (draft)"
  enabled = false

  filter = [
    {
      type     = "stringOptions"
      column   = "datasetId" # dataset ID, not the dataset name
      operator = "any of"
      values   = ["cm1dataset000000000000000"]
    },
  ]
}
