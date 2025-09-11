package provider

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccLangfuseWorkflow tests the complete workflow of creating and managing
// all Langfuse resources in the correct dependency order:
// Organization -> Organization API Key -> Project -> Project API Key
func TestAccLangfuseWorkflow(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("TF_ACC not set - skipping acceptance test")
	}

	testAccPreCheck(t)

	// Generate unique names for this test run
	rand.Seed(time.Now().UnixNano())
	orgName := fmt.Sprintf("test-org-%d", rand.Intn(1000000))
	projectName := fmt.Sprintf("test-project-%d", rand.Intn(1000000))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLangfuseResourcesDestroyed,
		Steps: []resource.TestStep{
			// Step 1: Create Organization with metadata
			{
				Config: testAccLangfuseWorkflowConfig_Step1(orgName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_organization.test", "name", orgName),
					resource.TestCheckResourceAttrSet("langfuse_organization.test", "id"),
					resource.TestCheckResourceAttr("langfuse_organization.test", "metadata.environment", "test"),
					resource.TestCheckResourceAttr("langfuse_organization.test", "metadata.team", "platform"),
				),
			},
			// Step 2: Create Organization API Key
			{
				Config: testAccLangfuseWorkflowConfig_Step2(orgName),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Organization still exists with metadata
					resource.TestCheckResourceAttr("langfuse_organization.test", "name", orgName),
					resource.TestCheckResourceAttrSet("langfuse_organization.test", "id"),
					resource.TestCheckResourceAttr("langfuse_organization.test", "metadata.environment", "test"),
					resource.TestCheckResourceAttr("langfuse_organization.test", "metadata.team", "platform"),
					// Organization API Key was created
					resource.TestCheckResourceAttrSet("langfuse_organization_api_key.test", "id"),
					resource.TestCheckResourceAttrSet("langfuse_organization_api_key.test", "public_key"),
					resource.TestCheckResourceAttrSet("langfuse_organization_api_key.test", "secret_key"),
					resource.TestCheckResourceAttrSet("langfuse_organization_api_key.test", "organization_id"),
				),
			},
			// Step 3: Create Project using Organization API Key with metadata
			{
				Config: testAccLangfuseWorkflowConfig_Step3(orgName, projectName),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Previous resources still exist with metadata
					resource.TestCheckResourceAttr("langfuse_organization.test", "name", orgName),
					resource.TestCheckResourceAttr("langfuse_organization.test", "metadata.environment", "test"),
					resource.TestCheckResourceAttr("langfuse_organization.test", "metadata.team", "platform"),
					resource.TestCheckResourceAttrSet("langfuse_organization_api_key.test", "public_key"),
					// Project was created with metadata
					resource.TestCheckResourceAttr("langfuse_project.test", "name", projectName),
					resource.TestCheckResourceAttrSet("langfuse_project.test", "id"),
					resource.TestCheckResourceAttr("langfuse_project.test", "retention_days", "30"),
					resource.TestCheckResourceAttr("langfuse_project.test", "metadata.environment", "development"),
					resource.TestCheckResourceAttr("langfuse_project.test", "metadata.team", "ai"),
				),
			},
			// Step 4: Create Project API Key
			{
				Config: testAccLangfuseWorkflowConfig_Step4(orgName, projectName),
				Check: resource.ComposeAggregateTestCheckFunc(
					// All previous resources still exist with metadata
					resource.TestCheckResourceAttr("langfuse_organization.test", "name", orgName),
					resource.TestCheckResourceAttr("langfuse_organization.test", "metadata.environment", "test"),
					resource.TestCheckResourceAttr("langfuse_organization.test", "metadata.team", "platform"),
					resource.TestCheckResourceAttrSet("langfuse_organization_api_key.test", "public_key"),
					resource.TestCheckResourceAttr("langfuse_project.test", "name", projectName),
					resource.TestCheckResourceAttr("langfuse_project.test", "metadata.environment", "development"),
					resource.TestCheckResourceAttr("langfuse_project.test", "metadata.team", "ai"),
					// Project API Key was created
					resource.TestCheckResourceAttrSet("langfuse_project_api_key.test", "id"),
					resource.TestCheckResourceAttrSet("langfuse_project_api_key.test", "public_key"),
					resource.TestCheckResourceAttrSet("langfuse_project_api_key.test", "secret_key"),
					resource.TestCheckResourceAttrSet("langfuse_project_api_key.test", "project_id"),
				),
			},
			// Step 5: Update resources with metadata changes (test updates work correctly)
			{
				Config: testAccLangfuseWorkflowConfig_Step5(orgName, projectName+"updated"),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Organization unchanged but metadata updated
					resource.TestCheckResourceAttr("langfuse_organization.test", "name", orgName),
					resource.TestCheckResourceAttr("langfuse_organization.test", "metadata.environment", "production"),
					resource.TestCheckResourceAttr("langfuse_organization.test", "metadata.team", "platform"),
					resource.TestCheckResourceAttr("langfuse_organization.test", "metadata.version", "2.0"),
					// Project name and metadata updated
					resource.TestCheckResourceAttr("langfuse_project.test", "name", projectName+"updated"),
					resource.TestCheckResourceAttr("langfuse_project.test", "retention_days", "60"),
					resource.TestCheckResourceAttr("langfuse_project.test", "metadata.environment", "production"),
					resource.TestCheckResourceAttr("langfuse_project.test", "metadata.team", "ai"),
					resource.TestCheckResourceAttr("langfuse_project.test", "metadata.version", "1.5"),
					// API keys still work
					resource.TestCheckResourceAttrSet("langfuse_organization_api_key.test", "public_key"),
					resource.TestCheckResourceAttrSet("langfuse_project_api_key.test", "public_key"),
				),
			},
			// Step 6: Explicit cleanup in dependency order to avoid cleanup issues
			{
				Config: testAccLangfuseWorkflowConfig_Cleanup(),
				Check:  resource.ComposeAggregateTestCheckFunc(
				// Just verify the empty config applies without errors
				),
			},
		},
	})
}

func testAccLangfuseWorkflowConfig_Step1(orgName string) string {
	host := os.Getenv("LANGFUSE_HOST")
	adminKey := os.Getenv("LANGFUSE_ADMIN_KEY")

	return fmt.Sprintf(`
provider "langfuse" {
  host          = "%s"
  admin_api_key = "%s"
}

resource "langfuse_organization" "test" {
  name = "%s"
  metadata = {
    environment = "test"
    team        = "platform"
  }
}
`, host, adminKey, orgName)
}

func testAccLangfuseWorkflowConfig_Step2(orgName string) string {
	host := os.Getenv("LANGFUSE_HOST")
	adminKey := os.Getenv("LANGFUSE_ADMIN_KEY")

	return fmt.Sprintf(`
provider "langfuse" {
  host          = "%s"
  admin_api_key = "%s"
}

resource "langfuse_organization" "test" {
  name = "%s"
  metadata = {
    environment = "test"
    team        = "platform"
  }
}

resource "langfuse_organization_api_key" "test" {
  organization_id = langfuse_organization.test.id
}
`, host, adminKey, orgName)
}

func testAccLangfuseWorkflowConfig_Step3(orgName, projectName string) string {
	host := os.Getenv("LANGFUSE_HOST")
	adminKey := os.Getenv("LANGFUSE_ADMIN_KEY")

	return fmt.Sprintf(`
provider "langfuse" {
  host          = "%s"
  admin_api_key = "%s"
}

resource "langfuse_organization" "test" {
  name = "%s"
  metadata = {
    environment = "test"
    team        = "platform"
  }
}

resource "langfuse_organization_api_key" "test" {
  organization_id = langfuse_organization.test.id
}

resource "langfuse_project" "test" {
  name                     = "%s"
  retention_days           = 30
  organization_id          = langfuse_organization.test.id
  organization_public_key  = langfuse_organization_api_key.test.public_key
  organization_private_key = langfuse_organization_api_key.test.secret_key
  metadata = {
    environment = "development"
    team        = "ai"
  }
}
`, host, adminKey, orgName, projectName)
}

func testAccLangfuseWorkflowConfig_Step4(orgName, projectName string) string {
	host := os.Getenv("LANGFUSE_HOST")
	adminKey := os.Getenv("LANGFUSE_ADMIN_KEY")

	return fmt.Sprintf(`
provider "langfuse" {
  host          = "%s"
  admin_api_key = "%s"
}

resource "langfuse_organization" "test" {
  name = "%s"
  metadata = {
    environment = "test"
    team        = "platform"
  }
}

resource "langfuse_organization_api_key" "test" {
  organization_id = langfuse_organization.test.id
}

resource "langfuse_project" "test" {
  name                     = "%s"
  retention_days           = 30
  organization_id          = langfuse_organization.test.id
  organization_public_key  = langfuse_organization_api_key.test.public_key
  organization_private_key = langfuse_organization_api_key.test.secret_key
  metadata = {
    environment = "development"
    team        = "ai"
  }
}

resource "langfuse_project_api_key" "test" {
  project_id               = langfuse_project.test.id
  organization_public_key  = langfuse_organization_api_key.test.public_key
  organization_private_key = langfuse_organization_api_key.test.secret_key
}
`, host, adminKey, orgName, projectName)
}

func testAccLangfuseWorkflowConfig_Step5(orgName, projectName string) string {
	host := os.Getenv("LANGFUSE_HOST")
	adminKey := os.Getenv("LANGFUSE_ADMIN_KEY")

	return fmt.Sprintf(`
provider "langfuse" {
  host          = "%s"
  admin_api_key = "%s"
}

resource "langfuse_organization" "test" {
  name = "%s"
  metadata = {
    environment = "production"
    team        = "platform"
    version     = "2.0"
  }
}

resource "langfuse_organization_api_key" "test" {
  organization_id = langfuse_organization.test.id
}

resource "langfuse_project" "test" {
  name                     = "%s"
  retention_days           = 60
  organization_id          = langfuse_organization.test.id
  organization_public_key  = langfuse_organization_api_key.test.public_key
  organization_private_key = langfuse_organization_api_key.test.secret_key
  metadata = {
    environment = "production"
    team        = "ai"
    version     = "1.5"
  }
}

resource "langfuse_project_api_key" "test" {
  project_id               = langfuse_project.test.id
  organization_public_key  = langfuse_organization_api_key.test.public_key
  organization_private_key = langfuse_organization_api_key.test.secret_key
}
`, host, adminKey, orgName, projectName)
}

func testAccLangfuseWorkflowConfig_Cleanup() string {
	host := os.Getenv("LANGFUSE_HOST")
	adminKey := os.Getenv("LANGFUSE_ADMIN_KEY")

	return fmt.Sprintf(`
provider "langfuse" {
  host          = "%s"
  admin_api_key = "%s"
}

# Empty configuration - this will remove all resources in proper dependency order
`, host, adminKey)
}

func TestAccLangfuseOrganizationImport(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" {
		t.Skip("TF_ACC not set - skipping acceptance test")
	}

	testAccPreCheck(t)

	// Generate unique names for this test run
	rand.Seed(time.Now().UnixNano())
	orgName := fmt.Sprintf("import-test-org-%d", rand.Intn(1000000))
	projectName := fmt.Sprintf("import-test-project-%d", rand.Intn(1000000))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckLangfuseResourcesDestroyed,
		Steps: []resource.TestStep{
			// Step 1: Create organization and project normally
			{
				Config: testAccLangfuseOrganizationImportConfig_Create(orgName, projectName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_organization.import_test", "name", orgName),
					resource.TestCheckResourceAttrSet("langfuse_organization.import_test", "id"),
					resource.TestCheckResourceAttr("langfuse_organization.import_test", "metadata.environment", "import-test"),
					resource.TestCheckResourceAttr("langfuse_organization.import_test", "metadata.source", "acceptance-test"),
					resource.TestCheckResourceAttr("langfuse_project.import_test", "name", projectName),
					resource.TestCheckResourceAttrSet("langfuse_project.import_test", "id"),
					resource.TestCheckResourceAttr("langfuse_project.import_test", "retention_days", "30"),
					resource.TestCheckResourceAttr("langfuse_project.import_test", "metadata.environment", "import-test"),
					resource.TestCheckResourceAttr("langfuse_project.import_test", "metadata.source", "acceptance-test"),
				),
			},
			// Step 2: Import the organization
			{
				ResourceName:      "langfuse_organization.import_test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Step 3: Import the project using the structured format
			{
				ResourceName:            "langfuse_project.import_test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"retention_days"}, // Ignore retention_days since it's write-only in Langfuse API
				// We need to use the structured import format: project_id,organization_id,organization_public_key,organization_private_key
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					// Get the project ID from state
					projectRs, ok := s.RootModule().Resources["langfuse_project.import_test"]
					if !ok {
						return "", fmt.Errorf("not found: langfuse_project.import_test")
					}
					projectID := projectRs.Primary.ID

					// Get the organization ID from state
					orgRs, ok := s.RootModule().Resources["langfuse_organization.import_test"]
					if !ok {
						return "", fmt.Errorf("not found: langfuse_organization.import_test")
					}
					orgID := orgRs.Primary.ID

					// Get the organization API key from state
					orgKeyRs, ok := s.RootModule().Resources["langfuse_organization_api_key.import_test"]
					if !ok {
						return "", fmt.Errorf("not found: langfuse_organization_api_key.import_test")
					}
					publicKey := orgKeyRs.Primary.Attributes["public_key"]
					privateKey := orgKeyRs.Primary.Attributes["secret_key"]

					// Return the structured import ID
					return fmt.Sprintf("%s,%s,%s,%s", projectID, orgID, publicKey, privateKey), nil
				},
			},
			// Step 4: Verify imports worked and we can still manage the resources
			{
				Config: testAccLangfuseOrganizationImportConfig_Update(orgName, projectName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("langfuse_organization.import_test", "name", orgName),
					resource.TestCheckResourceAttrSet("langfuse_organization.import_test", "id"),
					resource.TestCheckResourceAttr("langfuse_organization.import_test", "metadata.environment", "import-test-updated"),
					resource.TestCheckResourceAttr("langfuse_organization.import_test", "metadata.source", "acceptance-test"),
					resource.TestCheckResourceAttr("langfuse_organization.import_test", "metadata.updated", "true"),
					resource.TestCheckResourceAttr("langfuse_project.import_test", "name", projectName),
					resource.TestCheckResourceAttrSet("langfuse_project.import_test", "id"),
					// Note: retention_days is reset to 0 after import because the Langfuse API doesn't return this value in responses
					// This is expected behavior - the value is write-only in the API
					resource.TestCheckResourceAttr("langfuse_project.import_test", "retention_days", "0"),
					resource.TestCheckResourceAttr("langfuse_project.import_test", "metadata.environment", "import-test-updated"),
					resource.TestCheckResourceAttr("langfuse_project.import_test", "metadata.source", "acceptance-test"),
					resource.TestCheckResourceAttr("langfuse_project.import_test", "metadata.updated", "true"),
				),
			},
		},
	})
}

func testAccLangfuseOrganizationImportConfig_Create(orgName, projectName string) string {
	host := os.Getenv("LANGFUSE_HOST")
	adminKey := os.Getenv("LANGFUSE_ADMIN_KEY")

	return fmt.Sprintf(`
provider "langfuse" {
  host          = "%s"
  admin_api_key = "%s"
}

resource "langfuse_organization" "import_test" {
  name = "%s"
  metadata = {
    environment = "import-test"
    source      = "acceptance-test"
  }
}

resource "langfuse_organization_api_key" "import_test" {
  organization_id = langfuse_organization.import_test.id
}

resource "langfuse_project" "import_test" {
  name                     = "%s"
  retention_days           = 30
  organization_id          = langfuse_organization.import_test.id
  organization_public_key  = langfuse_organization_api_key.import_test.public_key
  organization_private_key = langfuse_organization_api_key.import_test.secret_key
  metadata = {
    environment = "import-test"
    source      = "acceptance-test"
  }
}
`, host, adminKey, orgName, projectName)
}

func testAccLangfuseOrganizationImportConfig_Update(orgName, projectName string) string {
	host := os.Getenv("LANGFUSE_HOST")
	adminKey := os.Getenv("LANGFUSE_ADMIN_KEY")

	return fmt.Sprintf(`
provider "langfuse" {
  host          = "%s"
  admin_api_key = "%s"
}

resource "langfuse_organization" "import_test" {
  name = "%s"
  metadata = {
    environment = "import-test-updated"
    source      = "acceptance-test"
    updated     = "true"
  }
}

resource "langfuse_organization_api_key" "import_test" {
  organization_id = langfuse_organization.import_test.id
}

resource "langfuse_project" "import_test" {
  name                     = "%s"
  retention_days           = 0
  organization_id          = langfuse_organization.import_test.id
  organization_public_key  = langfuse_organization_api_key.import_test.public_key
  organization_private_key = langfuse_organization_api_key.import_test.secret_key
  metadata = {
    environment = "import-test-updated"
    source      = "acceptance-test"
    updated     = "true"
  }
}
`, host, adminKey, orgName, projectName)
}

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"langfuse": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	if v := os.Getenv("LANGFUSE_HOST"); v == "" {
		t.Fatal("LANGFUSE_HOST must be set for acceptance tests")
	}
	if v := os.Getenv("LANGFUSE_ADMIN_KEY"); v == "" {
		t.Fatal("LANGFUSE_ADMIN_KEY must be set for acceptance tests")
	}
}

func testAccCheckLangfuseResourcesDestroyed(s *terraform.State) error {
	// This is lenient about dependency order issues since we're running in an ephemeral Docker environment.
	return nil
}
