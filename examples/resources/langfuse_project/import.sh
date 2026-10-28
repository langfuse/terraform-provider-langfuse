# Import with "<project_id>,<organization_id>". The organization keys come from the
# provider's organization_public_key and organization_private_key (or the
# LANGFUSE_ORGANIZATION_PUBLIC_KEY and LANGFUSE_ORGANIZATION_PRIVATE_KEY environment variables).
#
# Don't put the keys in the import ID: Terraform prints import IDs in plaintext.
# The deprecated "<project_id>,<organization_id>,<organization_public_key>,<organization_private_key>"
# format still works but raises a warning.
terraform import langfuse_project.my_project "proj_123,org_456"
