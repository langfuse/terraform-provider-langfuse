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

var _ resource.Resource = &organizationApiKeyResource{}

func NewOrganizationApiKeyResource() resource.Resource {
	return &organizationApiKeyResource{}
}

type organizationApiKeyResourceModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Note           types.String `tfsdk:"note"`
	PublicKey      types.String `tfsdk:"public_key"`
	SecretKey      types.String `tfsdk:"secret_key"`
}

type organizationApiKeyResource struct {
	AdminClient langfuse.AdminClient
}

func (r *organizationApiKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

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
			"note": schema.StringAttribute{
				Optional: true,
				Description: "Optional note for the API key (POST /api/admin/organizations/{organizationId}/apiKeys). " +
					"Because the Langfuse admin API only accepts a note at creation time, changing this attribute forces replacement: the old key is deleted and a new one is created (new id and credentials).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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

	orgKey, err := r.AdminClient.CreateOrganizationApiKey(ctx, data.OrganizationID.ValueString(), planNoteToCreateOrganizationApiKeyRequest(data.Note))
	if err != nil {
		resp.Diagnostics.AddError("Error creating organization API key", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &organizationApiKeyResourceModel{
		ID:             types.StringValue(orgKey.ID),
		OrganizationID: types.StringValue(data.OrganizationID.ValueString()),
		Note:           organizationApiKeyNoteToTF(orgKey.Note),
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

	if r.AdminClient == nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	key, err := r.AdminClient.GetOrganizationApiKey(ctx, data.OrganizationID.ValueString(), data.ID.ValueString())
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Note = organizationApiKeyNoteToTF(key.Note)

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

func organizationApiKeyNoteToTF(note *string) types.String {
	if note == nil {
		return types.StringNull()
	}
	return types.StringValue(*note)
}

func planNoteToCreateOrganizationApiKeyRequest(note types.String) *langfuse.CreateOrganizationApiKeyRequest {
	if note.IsUnknown() || note.IsNull() {
		return nil
	}
	s := note.ValueString()
	return &langfuse.CreateOrganizationApiKeyRequest{Note: &s}
}
