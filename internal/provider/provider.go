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
	Host        types.String `tfsdk:"host"`
	AdminAPIKey types.String `tfsdk:"admin_api_key"`
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
				Description: "Admin API key. Only needed when managing organizations. Can also come from LANGFUSE_ADMIN_KEY.",
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

	host := "https://app.langfuse.com"
	if !config.Host.IsNull() && !config.Host.IsUnknown() && config.Host.ValueString() != "" {
		host = config.Host.ValueString()
	}

	apiKey := os.Getenv("LANGFUSE_ADMIN_KEY")
	if !config.AdminAPIKey.IsNull() && !config.AdminAPIKey.IsUnknown() && config.AdminAPIKey.ValueString() != "" {
		apiKey = config.AdminAPIKey.ValueString()
	}

	clientFactory := langfuse.NewClientFactory(host, apiKey)
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
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &langfuseProvider{version: version}
	}
}
