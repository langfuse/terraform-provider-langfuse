package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
)

var _ datasource.DataSource = &userDataSource{}
var _ datasource.DataSourceWithValidateConfig = &userDataSource{}

func NewUserDataSource() datasource.DataSource {
	return &userDataSource{}
}

type userDataSourceModel struct {
	ID                     types.String `tfsdk:"id"`
	Email                  types.String `tfsdk:"email"`
	UserName               types.String `tfsdk:"user_name"`
	Active                 types.Bool   `tfsdk:"active"`
	OrganizationPublicKey  types.String `tfsdk:"organization_public_key"`
	OrganizationPrivateKey types.String `tfsdk:"organization_private_key"`
}

type userDataSource struct {
	ClientFactory langfuse.ClientFactory
}

func (d *userDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientFactory, ok := req.ProviderData.(langfuse.ClientFactory)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected DataSource Configure Type",
			fmt.Sprintf("Expected langfuse.ClientFactory, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.ClientFactory = clientFactory
}

func (d *userDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *userDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to look up a Langfuse user by ID or email.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The unique identifier of the user (SCIM ID). Exactly one of `id` or `email` must be specified.",
			},
			"email": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The email address (userName) of the user. Exactly one of `id` or `email` must be specified.",
			},
			"user_name": schema.StringAttribute{
				Computed:    true,
				Description: "The SCIM userName of the user (same as email).",
			},
			"active": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user account is active.",
			},
			"organization_public_key": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Organization public key to authenticate the call.",
			},
			"organization_private_key": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Organization private key to authenticate the call.",
			},
		},
	}
}

func (d *userDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var data userDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	idSet := !data.ID.IsNull() && !data.ID.IsUnknown()
	emailSet := !data.Email.IsNull() && !data.Email.IsUnknown()

	if !idSet && !emailSet {
		resp.Diagnostics.AddError(
			"Missing required argument",
			"Exactly one of `id` or `email` must be specified.",
		)
	}

	if idSet && emailSet {
		resp.Diagnostics.AddError(
			"Conflicting arguments",
			"Exactly one of `id` or `email` must be specified, not both.",
		)
	}
}

func (d *userDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data userDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	organizationClient := d.ClientFactory.NewOrganizationClient(
		data.OrganizationPublicKey.ValueString(),
		data.OrganizationPrivateKey.ValueString(),
	)

	var user *langfuse.SCIMUserResponse
	var err error

	if !data.ID.IsNull() && !data.ID.IsUnknown() {
		user, err = organizationClient.GetSCIMUser(ctx, data.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error reading user by ID", err.Error())
			return
		}
	} else {
		user, err = organizationClient.FindSCIMUserByEmail(ctx, data.Email.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error finding user by email", err.Error())
			return
		}
	}

	email := ""
	for _, e := range user.Emails {
		if e.Primary {
			email = e.Value
			break
		}
	}
	if email == "" && len(user.Emails) > 0 {
		email = user.Emails[0].Value
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &userDataSourceModel{
		ID:                     types.StringValue(user.ID),
		Email:                  types.StringValue(email),
		UserName:               types.StringValue(user.UserName),
		Active:                 types.BoolValue(user.Active),
		OrganizationPublicKey:  data.OrganizationPublicKey,
		OrganizationPrivateKey: data.OrganizationPrivateKey,
	})...)
}
