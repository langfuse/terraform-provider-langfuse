package provider

import (
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// TestAccLangfuseEvaluators exercises langfuse_evaluator and
// langfuse_evaluation_rule against a real Langfuse instance using the
// project keys seeded by testdata/docker-compose.yml. Unlike the organization
// workflow, this test does not need an enterprise license.
func TestAccLangfuseEvaluators(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("TF_ACC not set - skipping acceptance test")
	}

	testAccEvaluatorsPreCheck(t)

	suffix := rand.Intn(1000000)
	judgeName := fmt.Sprintf("acc-judge-%d", suffix)
	codeName := fmt.Sprintf("acc-code-%d", suffix)
	ruleName := fmt.Sprintf("acc-rule-%d", suffix)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLangfuseResourcesDestroyed,
		Steps: []resource.TestStep{
			// Step 1: create both evaluator types and a disabled draft rule with filters.
			{
				Config: testAccEvaluatorsConfig(judgeName, codeName, ruleName, "Rate {{input}} against {{output}} from 0 to 1.", false, "0.25"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("langfuse_evaluator.judge", "id"),
					resource.TestCheckResourceAttr("langfuse_evaluator.judge", "name", judgeName),
					resource.TestCheckResourceAttr("langfuse_evaluator.judge", "type", "llm_as_judge"),
					resource.TestCheckResourceAttr("langfuse_evaluator.judge", "version", "1"),
					resource.TestCheckResourceAttrSet("langfuse_evaluator.judge", "version_id"),
					resource.TestCheckResourceAttrSet("langfuse_evaluator.judge", "status"),
					resource.TestCheckResourceAttr("langfuse_evaluator.judge", "prompt.#", "2"),
					resource.TestCheckResourceAttr("langfuse_evaluator.judge", "prompt.0.role", "system"),
					resource.TestCheckResourceAttr("langfuse_evaluator.judge", "variables.#", "2"),
					resource.TestCheckResourceAttr("langfuse_evaluator.judge", "variable_mapping.#", "2"),
					resource.TestCheckResourceAttr("langfuse_evaluator.judge", "output_definition.data_type", "NUMERIC"),
					resource.TestCheckResourceAttr("langfuse_evaluator.judge", "output_definition.min_value", "0"),
					resource.TestCheckResourceAttr("langfuse_evaluator.judge", "output_definition.max_value", "1"),
					resource.TestCheckNoResourceAttr("langfuse_evaluator.judge", "model_config"),
					resource.TestCheckNoResourceAttr("langfuse_evaluator.judge", "source_code"),

					resource.TestCheckResourceAttrSet("langfuse_evaluator.code", "id"),
					resource.TestCheckResourceAttr("langfuse_evaluator.code", "type", "code"),
					resource.TestCheckResourceAttr("langfuse_evaluator.code", "source_code_language", "TYPESCRIPT"),
					resource.TestCheckResourceAttr("langfuse_evaluator.code", "version", "1"),
					resource.TestCheckNoResourceAttr("langfuse_evaluator.code", "prompt"),

					resource.TestCheckResourceAttrSet("langfuse_evaluation_rule.rule", "id"),
					resource.TestCheckResourceAttr("langfuse_evaluation_rule.rule", "name", ruleName),
					resource.TestCheckResourceAttr("langfuse_evaluation_rule.rule", "enabled", "false"),
					resource.TestCheckResourceAttr("langfuse_evaluation_rule.rule", "sampling", "0.25"),
					resource.TestCheckResourceAttr("langfuse_evaluation_rule.rule", "filter.#", "3"),
					resource.TestCheckResourceAttr("langfuse_evaluation_rule.rule", "filter.0.values.#", "1"),
					resource.TestCheckResourceAttr("langfuse_evaluation_rule.rule", "filter.1.value", "acceptance-user"),
					resource.TestCheckResourceAttr("langfuse_evaluation_rule.rule", "filter.2.key", "tenant"),
					resource.TestCheckResourceAttr("langfuse_evaluation_rule.rule", "evaluator_assignments.#", "2"),
					resource.TestCheckResourceAttrPair("langfuse_evaluation_rule.rule", "evaluator_assignments.0.evaluator_id", "langfuse_evaluator.judge", "id"),
					resource.TestCheckResourceAttrPair("langfuse_evaluation_rule.rule", "evaluator_assignments.1.evaluator_id", "langfuse_evaluator.code", "id"),
					resource.TestCheckResourceAttr("langfuse_evaluation_rule.rule", "evaluator_assignments.0.variable_mapping.#", "2"),
					resource.TestCheckNoResourceAttr("langfuse_evaluation_rule.rule", "evaluator_assignments.1.variable_mapping"),
				),
			},
			// Step 2: re-apply the same config; there must be no drift.
			{
				Config: testAccEvaluatorsConfig(judgeName, codeName, ruleName, "Rate {{input}} against {{output}} from 0 to 1.", false, "0.25"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			// Step 3: change the prompt (new evaluator version, same id), enable the rule and change sampling.
			{
				Config: testAccEvaluatorsConfig(judgeName, codeName, ruleName, "Judge {{input}} versus {{output}}. Return a value between 0 and 1.", true, "1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("langfuse_evaluator.judge", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("langfuse_evaluator.code", plancheck.ResourceActionNoop),
						plancheck.ExpectResourceAction("langfuse_evaluation_rule.rule", plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue("langfuse_evaluator.judge", tfjsonpath.New("id"), knownvalue.NotNull()),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_evaluator.judge", "version", "2"),
					resource.TestCheckResourceAttr("langfuse_evaluator.judge", "prompt.1.content", "Judge {{input}} versus {{output}}. Return a value between 0 and 1."),
					resource.TestCheckResourceAttr("langfuse_evaluation_rule.rule", "enabled", "true"),
					resource.TestCheckResourceAttr("langfuse_evaluation_rule.rule", "sampling", "1"),
				),
			},
			// Step 4: rename the evaluator only; metadata updates must not create a version.
			{
				Config: testAccEvaluatorsConfig(judgeName+"-renamed", codeName, ruleName, "Judge {{input}} versus {{output}}. Return a value between 0 and 1.", true, "1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_evaluator.judge", "name", judgeName+"-renamed"),
					resource.TestCheckResourceAttr("langfuse_evaluator.judge", "version", "2"),
				),
			},
			// Step 5: import both resources.
			{
				ResourceName:      "langfuse_evaluator.judge",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccProjectScopedImportID("langfuse_evaluator.judge"),
			},
			{
				ResourceName:      "langfuse_evaluator.code",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccProjectScopedImportID("langfuse_evaluator.code"),
			},
			{
				ResourceName:      "langfuse_evaluation_rule.rule",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccProjectScopedImportID("langfuse_evaluation_rule.rule"),
			},
		},
	})
}

// TestAccLangfuseEvaluatorTypeChangeReplaces verifies that changing an
// evaluator's type destroys and recreates it.
func TestAccLangfuseEvaluatorTypeChangeReplaces(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("TF_ACC not set - skipping acceptance test")
	}

	testAccEvaluatorsPreCheck(t)

	name := fmt.Sprintf("acc-replace-%d", rand.Intn(1000000))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLangfuseResourcesDestroyed,
		Steps: []resource.TestStep{
			{
				Config: testAccCodeEvaluatorConfig(name, "TYPESCRIPT"),
				Check:  resource.TestCheckResourceAttr("langfuse_evaluator.single", "source_code_language", "TYPESCRIPT"),
			},
			{
				Config: testAccJudgeOnlyConfig(name),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("langfuse_evaluator.single", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.TestCheckResourceAttr("langfuse_evaluator.single", "type", "llm_as_judge"),
			},
		},
	})
}

func testAccEvaluatorsPreCheck(t *testing.T) {
	t.Helper()
	if v := os.Getenv("LANGFUSE_HOST"); v == "" {
		t.Fatal("LANGFUSE_HOST must be set for acceptance tests")
	}
	if os.Getenv("LANGFUSE_PROJECT_PUBLIC_KEY") == "" || os.Getenv("LANGFUSE_PROJECT_SECRET_KEY") == "" {
		t.Skip("LANGFUSE_PROJECT_PUBLIC_KEY and LANGFUSE_PROJECT_SECRET_KEY must be set - skipping evaluator acceptance tests")
	}
}

// testAccProjectScopedImportID builds <project_public_key>:<project_secret_key>:<id>
// from the state of the given resource.
func testAccProjectScopedImportID(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("resource %s not found in state", resourceName)
		}
		return fmt.Sprintf("%s:%s:%s",
			rs.Primary.Attributes["project_public_key"],
			rs.Primary.Attributes["project_secret_key"],
			rs.Primary.ID,
		), nil
	}
}

func testAccEvaluatorsProviderBlock() string {
	return fmt.Sprintf(`
provider "langfuse" {
  host          = "%s"
  admin_api_key = "%s"
}

locals {
  project_public_key = "%s"
  project_secret_key = "%s"
}
`,
		os.Getenv("LANGFUSE_HOST"),
		os.Getenv("LANGFUSE_ADMIN_KEY"),
		os.Getenv("LANGFUSE_PROJECT_PUBLIC_KEY"),
		os.Getenv("LANGFUSE_PROJECT_SECRET_KEY"),
	)
}

func testAccEvaluatorsConfig(judgeName, codeName, ruleName, userPrompt string, enabled bool, sampling string) string {
	return testAccEvaluatorsProviderBlock() + fmt.Sprintf(`
resource "langfuse_evaluator" "judge" {
  project_public_key = local.project_public_key
  project_secret_key = local.project_secret_key

  name        = "%s"
  description = "Acceptance test judge"
  type        = "llm_as_judge"

  prompt = [
    { role = "system", content = "You are a strict evaluator." },
    { role = "user", content = "%s" },
  ]

  variable_mapping = [
    { variable = "input", source = "input" },
    { variable = "output", source = "output" },
  ]

  output_definition = {
    data_type                    = "NUMERIC"
    min_value                    = 0
    max_value                    = 1
    score_reasoning_instructions = "One sentence."
  }
}

resource "langfuse_evaluator" "code" {
  project_public_key = local.project_public_key
  project_secret_key = local.project_secret_key

  name                 = "%s"
  type                 = "code"
  source_code_language = "TYPESCRIPT"
  source_code          = "function evaluate({ observation: { output } }: EvaluationContext): EvaluationResult {\n  return { scores: [{ name: \"answer_length_ok\", value: String(output ?? \"\").length < 500, dataType: \"BOOLEAN\" }] };\n}\n"
}

resource "langfuse_evaluation_rule" "rule" {
  project_public_key = local.project_public_key
  project_secret_key = local.project_secret_key

  name     = "%s"
  enabled  = %t
  sampling = %s

  filter = [
    { type = "stringOptions", column = "type", operator = "any of", values = ["GENERATION"] },
    { type = "string", column = "userId", operator = "=", value = "acceptance-user" },
    { type = "stringObject", column = "metadata", key = "tenant", operator = "=", value = "acme" },
  ]

  evaluator_assignments = [
    {
      evaluator_id = langfuse_evaluator.judge.id
      variable_mapping = [
        { variable = "input", source = "input" },
        { variable = "output", source = "metadata", json_path = "$.answer" },
      ]
    },
    {
      evaluator_id = langfuse_evaluator.code.id
    },
  ]
}
`, judgeName, userPrompt, codeName, ruleName, enabled, sampling)
}

func testAccCodeEvaluatorConfig(name, language string) string {
	return testAccEvaluatorsProviderBlock() + fmt.Sprintf(`
resource "langfuse_evaluator" "single" {
  project_public_key   = local.project_public_key
  project_secret_key   = local.project_secret_key
  name                 = "%s"
  type                 = "code"
  source_code_language = "%s"
  source_code          = "function evaluate(ctx: EvaluationContext): EvaluationResult {\n  return { scores: [{ name: \"always_pass\", value: true, dataType: \"BOOLEAN\" }] };\n}\n"
}
`, name, language)
}

func testAccJudgeOnlyConfig(name string) string {
	return testAccEvaluatorsProviderBlock() + fmt.Sprintf(`
resource "langfuse_evaluator" "single" {
  project_public_key = local.project_public_key
  project_secret_key = local.project_secret_key
  name               = "%s"
  type               = "llm_as_judge"

  prompt = [
    { role = "user", content = "Is {{output}} polite? Answer true or false." },
  ]

  output_definition = {
    data_type = "BOOLEAN"
  }
}
`, name)
}
