package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
)

var _ provider.Provider = &langfuseProvider{}

type langfuseProvider struct {
	version string
}

type langfuseProviderModel struct {
	Host                   types.String `tfsdk:"host"`
	AdminAPIKey            types.String `tfsdk:"admin_api_key"`
	OrganizationPublicKey  types.String `tfsdk:"organization_public_key"`
	OrganizationPrivateKey types.String `tfsdk:"organization_private_key"`
	TLSServerName          types.String `tfsdk:"tls_server_name"`
}

func (p *langfuseProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "langfuse"
	resp.Version = p.version
}

func (p *langfuseProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Optional:    true,
				Description: "Base URI of the Langfuse instance (defaults to https://app.langfuse.com).",
			},
			"admin_api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Instance Management API key, i.e. the ADMIN_API_KEY configured on a self-hosted Langfuse instance. Only needed for langfuse_organization and langfuse_organization_api_key. The Instance Management API is not available on Langfuse Cloud, so omit this there. Can also come from LANGFUSE_ADMIN_KEY.",
			},
			"organization_public_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Public key of an organization-scoped API key. Used by langfuse_project and langfuse_project_membership when they don't set organization_public_key themselves, and lets them be imported without keys in the import ID. Can also come from LANGFUSE_ORGANIZATION_PUBLIC_KEY.",
			},
			"organization_private_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Secret key of an organization-scoped API key. Used by langfuse_project and langfuse_project_membership when they don't set organization_private_key themselves, and lets them be imported without keys in the import ID. Can also come from LANGFUSE_ORGANIZATION_PRIVATE_KEY.",
			},
			"tls_server_name": schema.StringAttribute{
				Optional:    true,
				Description: "Hostname to verify the TLS certificate against, also sent as the SNI value. Set this when the instance is reached through a port forward or tunnel, where the host above (typically localhost) cannot match the certificate. Certificate verification remains enabled.",
			},
		},
	}
}

func (p *langfuseProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config langfuseProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if config.Host.IsUnknown() || config.AdminAPIKey.IsUnknown() || config.TLSServerName.IsUnknown() ||
		config.OrganizationPublicKey.IsUnknown() || config.OrganizationPrivateKey.IsUnknown() {
		return
	}

	host := "https://app.langfuse.com"
	if !config.Host.IsNull() && config.Host.ValueString() != "" {
		host = config.Host.ValueString()
	}

	apiKey := os.Getenv("LANGFUSE_ADMIN_KEY")
	if !config.AdminAPIKey.IsNull() && config.AdminAPIKey.ValueString() != "" {
		apiKey = config.AdminAPIKey.ValueString()
	}

	orgPublicKey := os.Getenv("LANGFUSE_ORGANIZATION_PUBLIC_KEY")
	if !config.OrganizationPublicKey.IsNull() && config.OrganizationPublicKey.ValueString() != "" {
		orgPublicKey = config.OrganizationPublicKey.ValueString()
	}

	orgPrivateKey := os.Getenv("LANGFUSE_ORGANIZATION_PRIVATE_KEY")
	if !config.OrganizationPrivateKey.IsNull() && config.OrganizationPrivateKey.ValueString() != "" {
		orgPrivateKey = config.OrganizationPrivateKey.ValueString()
	}

	clientFactory := langfuse.NewClientFactory(host, apiKey, orgPublicKey, orgPrivateKey, config.TLSServerName.ValueString())
	resp.DataSourceData = clientFactory
	resp.ResourceData = clientFactory
}

func (p *langfuseProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (p *langfuseProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewOrganizationResource,
		NewOrganizationApiKeyResource,
		NewOrganizationMembershipResource,
		NewProjectResource,
		NewProjectApiKeyResource,
		NewProjectMembershipResource,
		NewLlmConnectionResource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &langfuseProvider{version: version}
	}
}
