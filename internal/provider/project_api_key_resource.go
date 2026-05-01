package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
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
	Note                   types.String `tfsdk:"note"`
	PublicKey              types.String `tfsdk:"public_key"`
	SecretKey              types.String `tfsdk:"secret_key"`
}

type projectApiKeyResource struct {
	ClientFactory langfuse.ClientFactory
}

func (r *projectApiKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

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
			"note": schema.StringAttribute{
				Optional: true,
				Description: "Optional note for the API key (POST /api/public/projects/{projectId}/apiKeys). " +
					"Because the Langfuse public API only accepts a note at creation time, changing this attribute forces replacement: the old key is deleted and a new one is created (new id and credentials).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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
	createReq := planNoteToCreateRequest(data.Note)
	projectApiKey, err := organizationClient.CreateProjectApiKey(ctx, data.ProjectID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating project API key", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &projectApiKeyResourceModel{
		ID:                     types.StringValue(projectApiKey.ID),
		OrganizationPublicKey:  types.StringValue(data.OrganizationPublicKey.ValueString()),
		OrganizationPrivateKey: types.StringValue(data.OrganizationPrivateKey.ValueString()),
		ProjectID:              types.StringValue(data.ProjectID.ValueString()),
		Note:                   projectApiKeyNoteToTF(projectApiKey.Note),
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

	if r.ClientFactory == nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(data.OrganizationPublicKey.ValueString(), data.OrganizationPrivateKey.ValueString())
	key, err := organizationClient.GetProjectApiKey(ctx, data.ProjectID.ValueString(), data.ID.ValueString())
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Note = projectApiKeyNoteToTF(key.Note)

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
	err := organizationClient.DeleteProjectApiKey(ctx, data.ProjectID.ValueString(), data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting project API key", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &projectApiKeyResourceModel{})...)
}

func projectApiKeyNoteToTF(note *string) types.String {
	if note == nil {
		return types.StringNull()
	}
	return types.StringValue(*note)
}

func planNoteToCreateRequest(note types.String) *langfuse.CreateProjectApiKeyRequest {
	if note.IsUnknown() || note.IsNull() {
		return nil
	}
	s := note.ValueString()
	return &langfuse.CreateProjectApiKeyRequest{Note: &s}
}
