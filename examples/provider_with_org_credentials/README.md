# Provider with Organization Credentials Example

This example demonstrates using provider-level organization credentials to avoid repeating credentials across resources.

## Benefits

- **No credential repetition**: Set credentials once at the provider level
- **Cleaner code**: Resources don't need org_public_key and org_private_key fields
- **Easier management**: Update credentials in one place
- **Simplified imports**: Import resources using just the resource ID

## Usage

### Using Environment Variables (Recommended)

```bash
export LANGFUSE_ORG_PUBLIC_KEY="pk_..."
export LANGFUSE_ORG_PRIVATE_KEY="sk_..."
export TF_VAR_organization_id="org_..."

terraform init
terraform plan
terraform apply
```

### Using tfvars File

```bash
# Create terraform.tfvars
cat > terraform.tfvars <<EOF
org_public_key  = "pk_..."
org_private_key = "sk_..."
organization_id = "org_..."
EOF

terraform init
terraform plan
terraform apply
```

## Import Examples

With provider-level credentials, you can import resources using simplified syntax:

```bash
# Import project (only need project_id and organization_id)
terraform import langfuse_project.example "proj_123,org_456"

# Import organization membership (only need membership_id)
terraform import langfuse_organization_membership.member "mem_789"

# Import project API key (only need key_id and project_id)
terraform import langfuse_project_api_key.example_key "key_abc,proj_123"
```

## Backward Compatibility

You can still specify credentials at the resource level if needed. Resource-level credentials override provider-level credentials:

```hcl
resource "langfuse_project" "override_example" {
  name                     = "Override Project"
  organization_id          = var.organization_id
  organization_public_key  = var.other_org_public_key
  organization_private_key = var.other_org_private_key
}
```
