package provider

import (
	"context"

	"github.com/cresta/terraform-provider-langfuse/langfuse"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &projectApiKeyResource{}

func NewProjectApiKeyResource() resource.Resource {
	return &projectApiKeyResource{}
}

type projectApiKeyResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	OrganizationPublicKey  types.String `tfsdk:"organization_public_key"`
	OrganizationPrivateKey types.String `tfsdk:"organization_private_key"`
	ProjectID              types.String `tfsdk:"project_id"`
	PublicKey              types.String `tfsdk:"public_key"`
	SecretKey              types.String `tfsdk:"secret_key"`
}

type projectApiKeyResource struct {
	ClientFactory langfuse.ClientFactory
}

func (r *projectApiKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.ClientFactory = req.ProviderData.(langfuse.ClientFactory)
}

func (r *projectApiKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_api_key"
}

func (r *projectApiKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"project_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the project the key belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"organization_public_key": schema.StringAttribute{
				Required:    true,
				Description: "Organization public key to authenticate the call.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_private_key": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Organization private key to authenticate the call.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"public_key": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "The public value of the API key (only returned at creation time).",
				PlanModifiers: []planmodifier.String{
					// Keep the value that is already in state because Read() will never be able to fetch it again.
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"secret_key": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "The secret value of the API key (only returned at creation time).",
				PlanModifiers: []planmodifier.String{
					// Keep the value that is already in state because Read() will never be able to fetch it again.
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *projectApiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data projectApiKeyResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(data.OrganizationPublicKey.ValueString(), data.OrganizationPrivateKey.ValueString())
	projectApiKey, err := organizationClient.CreateProjectApiKey(ctx, data.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error creating project API key", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &projectApiKeyResourceModel{
		ID:                     types.StringValue(projectApiKey.ID),
		OrganizationPublicKey:  types.StringValue(data.OrganizationPublicKey.ValueString()),
		OrganizationPrivateKey: types.StringValue(data.OrganizationPrivateKey.ValueString()),
		ProjectID:              types.StringValue(data.ProjectID.ValueString()),
		PublicKey:              types.StringValue(projectApiKey.PublicKey),
		SecretKey:              types.StringValue(projectApiKey.SecretKey),
	})...)
}

func (r *projectApiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data projectApiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(data.OrganizationPublicKey.ValueString(), data.OrganizationPrivateKey.ValueString())
	_, err := organizationClient.GetProject(ctx, data.ProjectID.ValueString())
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *projectApiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// No updates are supported; keys are immutable. Any change should force recreation.
}

func (r *projectApiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data projectApiKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(data.OrganizationPublicKey.ValueString(), data.OrganizationPrivateKey.ValueString())
	err := organizationClient.DeleteProjectApiKey(ctx, data.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting project API key", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &projectApiKeyResourceModel{})...)
}
