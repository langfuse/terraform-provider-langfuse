resource "langfuse_project" "my_project" {
  name                     = "My Project"
  organization_id          = local.langfuse_organization_id
  organization_private_key = local.langfuse_secret_key
  organization_public_key  = local.langfuse_public_key
}
