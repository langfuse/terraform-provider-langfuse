package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
)

// newOrganizationClient returns an organization client authenticated with the
// resource-level organization keys when they are set, otherwise with the
// provider-level ones.
func newOrganizationClient(cf langfuse.ClientFactory, publicKey, privateKey types.String) (langfuse.OrganizationClient, diag.Diagnostics) {
	var diags diag.Diagnostics

	pub, priv := publicKey.ValueString(), privateKey.ValueString()
	if pub == "" && priv == "" {
		pub, priv = cf.OrganizationKeys()
	}

	if pub == "" || priv == "" {
		diags.AddError("Missing organization credentials",
			"Set both organization_public_key and organization_private_key, either on the resource or on the provider "+
				"(or through the LANGFUSE_ORGANIZATION_PUBLIC_KEY and LANGFUSE_ORGANIZATION_PRIVATE_KEY environment variables).")
		return nil, diags
	}

	return cf.NewOrganizationClient(pub, priv), diags
}

// organizationKeysInImportIDWarning is returned when an import ID carries
// organization API keys. Terraform does not treat import IDs as sensitive:
// it prints them in plan output and in `terraform import` output.
func organizationKeysInImportIDWarning(keyFreeFormat string) (string, string) {
	return "Import ID contains organization API keys",
		"Terraform does not treat import IDs as sensitive and prints them in plan output, saved plan files and " +
			"`terraform import` output, so the organization keys in this import ID are now visible in plaintext, " +
			"for example in CI logs. Rotate the organization API key if that output was stored or shared. " +
			"To avoid this, set organization_public_key and organization_private_key on the provider (or through the " +
			"LANGFUSE_ORGANIZATION_PUBLIC_KEY and LANGFUSE_ORGANIZATION_PRIVATE_KEY environment variables) and import with " +
			"the key-free format \"" + keyFreeFormat + "\". The import ID format with keys is deprecated."
}
