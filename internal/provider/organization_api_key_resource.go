package provider

import (
	"context"

	"github.com/cresta/terraform-provider-langfuse/internal/langfuse"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &organizationApiKeyResource{}

func NewOrganizationApiKeyResource() resource.Resource {
	return &organizationApiKeyResource{}
}

type organizationApiKeyResourceModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	PublicKey      types.String `tfsdk:"public_key"`
	SecretKey      types.String `tfsdk:"secret_key"`
}

type organizationApiKeyResource struct {
	AdminClient langfuse.AdminClient
}

func (r *organizationApiKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.AdminClient = req.ProviderData.(langfuse.ClientFactory).NewAdminClient()
}

func (r *organizationApiKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_api_key"
}

func (r *organizationApiKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"organization_id": schema.StringAttribute{
				Required:    true,
				Description: "The Langfuse organization the key belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(), // changing org → new key
				},
			},
			"public_key": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "The public value of the API key (only returned at creation time).",
				PlanModifiers: []planmodifier.String{
					// keep the value that is already in state because
					// Read() will never be able to fetch it again
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"secret_key": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "The secret value of the API key (only returned at creation time).",
				PlanModifiers: []planmodifier.String{
					// keep the value that is already in state because
					// Read() will never be able to fetch it again
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *organizationApiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data organizationApiKeyResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	orgKey, err := r.AdminClient.CreateOrganizationApiKey(ctx, data.OrganizationID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error creating organization API key", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &organizationApiKeyResourceModel{
		ID:             types.StringValue(orgKey.ID),
		OrganizationID: types.StringValue(data.OrganizationID.ValueString()),
		PublicKey:      types.StringValue(orgKey.PublicKey),
		SecretKey:      types.StringValue(orgKey.SecretKey),
	})...)
}

func (r *organizationApiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data organizationApiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.AdminClient.GetOrganizationApiKey(ctx, data.OrganizationID.ValueString(), data.ID.ValueString())
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *organizationApiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// No update
}

func (r *organizationApiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data organizationApiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.AdminClient.DeleteOrganizationApiKey(ctx, data.OrganizationID.ValueString(), data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting organization API key", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &organizationApiKeyResourceModel{})...)
}
