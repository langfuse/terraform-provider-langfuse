# LLM-as-a-judge evaluator with a numeric score.
# Changing prompt, model_config, variable_mapping or output_definition creates a
# new version on the same evaluator; rules automatically pick up the latest one.
resource "langfuse_evaluator" "helpfulness" {
  project_public_key = local.langfuse_project_public_key
  project_secret_key = local.langfuse_project_secret_key

  name        = "Helpfulness"
  description = "Scores how helpful the answer is for the user's question."
  type        = "llm_as_judge"

  prompt = [
    {
      role    = "system"
      content = "You are a strict but fair evaluator."
    },
    {
      role    = "user"
      content = "Question: {{input}}\n\nAnswer: {{output}}\n\nRate the helpfulness of the answer from 0 to 1."
    },
  ]

  # Omit model_config to use the project's default evaluation model.
  model_config = {
    provider = langfuse_llm_connection.openai.provider_name
    model    = "gpt-4.1-mini"
  }

  variable_mapping = [
    { variable = "input", source = "input" },
    { variable = "output", source = "output" },
  ]

  output_definition = {
    data_type                    = "NUMERIC"
    min_value                    = 0
    max_value                    = 1
    score_reasoning_instructions = "Explain the rating in one sentence."
  }
}

# Categorical LLM-as-a-judge evaluator.
resource "langfuse_evaluator" "tone" {
  project_public_key = local.langfuse_project_public_key
  project_secret_key = local.langfuse_project_secret_key

  name = "Tone"
  type = "llm_as_judge"

  prompt = [
    {
      role    = "user"
      content = "Classify the tone of the following answer: {{output}}"
    },
  ]

  output_definition = {
    data_type                     = "CATEGORICAL"
    categories                    = ["friendly", "neutral", "rude"]
    should_allow_multiple_matches = false
  }
}

# Deterministic code evaluator.
resource "langfuse_evaluator" "answer_length" {
  project_public_key = local.langfuse_project_public_key
  project_secret_key = local.langfuse_project_secret_key

  name                 = "Answer length"
  type                 = "code"
  source_code_language = "PYTHON"
  source_code          = <<-PY
    def evaluate(ctx: EvaluationContext) -> EvaluationResult:
        return EvaluationResult(
            scores=[
                Score(
                    name="answer_length_ok",
                    value=len(str(ctx.observation.output or "")) <= 500,
                    data_type="BOOLEAN",
                )
            ]
        )
  PY
}
