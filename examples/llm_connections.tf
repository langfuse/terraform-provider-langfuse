# Example: Manage LLM Connections in Langfuse

terraform {
  required_providers {
    langfuse = {
      source = "langfuse/langfuse"
    }
  }
}

provider "langfuse" {
  # You can set the host if using a self-hosted instance
  # host = "https://your-langfuse-instance.com"

  # Admin API key for organization management
  # Can also be set via LANGFUSE_ADMIN_KEY environment variable
  # admin_api_key = "your-admin-api-key"
}

# Create an LLM connection (OpenAI)
resource "langfuse_llm_connection" "openai_prod" {
  project_public_key  = "your-project-public-key"
  project_secret_key  = "your-project-secret-key"
  provider            = "openai-prod"
  adapter             = "openai"
  secret_key          = "sk-..."
  with_default_models = true
}

# Create an LLM connection (Bedrock)
resource "langfuse_llm_connection" "bedrock_prod" {
  project_public_key  = "your-project-public-key"
  project_secret_key  = "your-project-secret-key"
  provider            = "bedrock-prod"
  adapter             = "bedrock"
  secret_key          = "AK..."
  base_url            = "https://bedrock.aws.example.com"
  custom_models       = ["my-bedrock-model"]
  config              = jsonencode({ region = "us-east-1" })
  with_default_models = false
}

# Create an LLM connection (Google Vertex AI)
resource "langfuse_llm_connection" "vertex_ai_prod" {
  project_public_key  = "your-project-public-key"
  project_secret_key  = "your-project-secret-key"
  provider            = "vertex-ai-prod"
  adapter             = "google-vertex-ai"
  secret_key          = "vertex-key"
  config              = jsonencode({ location = "us-central1" })
  with_default_models = true
}

# Create an LLM connection (Anthropic)
resource "langfuse_llm_connection" "anthropic_prod" {
  project_public_key  = "your-project-public-key"
  project_secret_key  = "your-project-secret-key"
  provider            = "anthropic-prod"
  adapter             = "anthropic"
  secret_key          = "anthropic-key"
  with_default_models = true
}

# Create an LLM connection (Azure)
resource "langfuse_llm_connection" "azure_prod" {
  project_public_key  = "your-project-public-key"
  project_secret_key  = "your-project-secret-key"
  provider            = "azure-prod"
  adapter             = "azure"
  secret_key          = "azure-key"
  with_default_models = true
}
