# Multi-Organization with Provider Aliases Example

This example demonstrates managing multiple organizations using provider aliases.

## Use Cases

- **Multi-tenant SaaS**: Manage different customer organizations
- **Development/Staging/Production**: Separate environments with different org credentials
- **Organizational separation**: Keep different teams' resources isolated

## Usage

### Using Environment Variables

```bash
export TF_VAR_org1_public_key="pk_org1_..."
export TF_VAR_org1_private_key="sk_org1_..."
export TF_VAR_org1_id="org_1"

export TF_VAR_org2_public_key="pk_org2_..."
export TF_VAR_org2_private_key="sk_org2_..."
export TF_VAR_org2_id="org_2"

terraform init
terraform plan
terraform apply
```

### Using tfvars File

```bash
# Create terraform.tfvars
cat > terraform.tfvars <<EOF
org1_public_key  = "pk_org1_..."
org1_private_key = "sk_org1_..."
org1_id          = "org_1"

org2_public_key  = "pk_org2_..."
org2_private_key = "sk_org2_..."
org2_id          = "org_2"
EOF

terraform init
terraform plan
terraform apply
```

## Import with Specific Provider

When importing resources with multiple providers, specify which provider to use:

```bash
# Import Org 1 project
terraform import -provider=langfuse.org1 langfuse_project.org1_project "proj_org1,org_1"

# Import Org 2 project
terraform import -provider=langfuse.org2 langfuse_project.org2_project "proj_org2,org_2"

# Import Org 1 membership
terraform import -provider=langfuse.org1 langfuse_organization_membership.org1_member "mem_org1"

# Import Org 2 membership
terraform import -provider=langfuse.org2 langfuse_organization_membership.org2_member "mem_org2"
```

## Best Practices

1. **Naming Convention**: Use clear prefixes (e.g., `org1_`, `org2_`) for resources
2. **Separate State Files**: Consider using separate workspaces or state files for each org
3. **Security**: Never commit credentials to version control
4. **Tagging**: Use consistent tagging/metadata to identify which org resources belong to
