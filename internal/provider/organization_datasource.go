package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
)

var _ datasource.DataSource = &organizationDataSource{}
var _ datasource.DataSourceWithValidateConfig = &organizationDataSource{}

func NewOrganizationDataSource() datasource.DataSource {
	return &organizationDataSource{}
}

type organizationDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Metadata types.Map    `tfsdk:"metadata"`
}

type organizationDataSource struct {
	AdminClient langfuse.AdminClient
}

func (d *organizationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	d.AdminClient = req.ProviderData.(langfuse.ClientFactory).NewAdminClient()
}

func (d *organizationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (d *organizationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to look up a Langfuse organization by its ID or name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The unique identifier of the organization. Exactly one of `id` or `name` must be specified.",
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The display name of the organization. Exactly one of `id` or `name` must be specified.",
			},
			"metadata": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Metadata for the organization as key-value pairs.",
			},
		},
	}
}

func (d *organizationDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var data organizationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idSet := !data.ID.IsNull() && !data.ID.IsUnknown()
	nameSet := !data.Name.IsNull() && !data.Name.IsUnknown()

	if !idSet && !nameSet {
		resp.Diagnostics.AddError(
			"Missing required argument",
			"Exactly one of `id` or `name` must be specified.",
		)
	}

	if idSet && nameSet {
		resp.Diagnostics.AddError(
			"Conflicting arguments",
			"Exactly one of `id` or `name` must be specified, not both.",
		)
	}
}

func (d *organizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data organizationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var org *langfuse.Organization
	var err error

	if !data.ID.IsNull() && !data.ID.IsUnknown() {
		org, err = d.AdminClient.GetOrganization(ctx, data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error reading organization by ID", err.Error())
			return
		}
	} else {
		orgs, listErr := d.AdminClient.ListOrganizations(ctx)
		if listErr != nil {
			resp.Diagnostics.AddError("Error listing organizations", listErr.Error())
			return
		}

		targetName := data.Name.ValueString()
		for _, o := range orgs {
			if o.Name == targetName {
				org = o
				break
			}
		}

		if org == nil {
			resp.Diagnostics.AddError(
				"Organization not found",
				fmt.Sprintf("No organization found with name %q.", targetName),
			)
			return
		}
	}

	var metadataMap types.Map
	if len(org.Metadata) > 0 {
		var diags diag.Diagnostics
		metadataMap, diags = types.MapValueFrom(ctx, types.StringType, org.Metadata)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	} else {
		metadataMap = types.MapNull(types.StringType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &organizationDataSourceModel{
		ID:       types.StringValue(org.ID),
		Name:     types.StringValue(org.Name),
		Metadata: metadataMap,
	})...)
}
