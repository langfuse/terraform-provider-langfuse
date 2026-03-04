package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
)

var _ resource.Resource = &projectApiKeyResource{}
var _ resource.ResourceWithImportState = &projectApiKeyResource{}

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
	if req.ProviderData == nil {
		return
	}

	r.ClientFactory = req.ProviderData.(langfuse.ClientFactory)
}

// resolveOrgCredentials returns the org credentials to use, implementing the fallback pattern:
// 1. Use resource-level credentials if provided
// 2. Fall back to provider-level credentials if available
// 3. Error if neither is available
func (r *projectApiKeyResource) resolveOrgCredentials(
	ctx context.Context,
	resourcePublicKey, resourcePrivateKey types.String,
) (publicKey string, privateKey string, err error) {

	// Check if resource has explicit credentials
	hasResourceCreds := !resourcePublicKey.IsNull() &&
		!resourcePublicKey.IsUnknown() &&
		resourcePublicKey.ValueString() != "" &&
		!resourcePrivateKey.IsNull() &&
		!resourcePrivateKey.IsUnknown() &&
		resourcePrivateKey.ValueString() != ""

	if hasResourceCreds {
		return resourcePublicKey.ValueString(), resourcePrivateKey.ValueString(), nil
	}

	// Fall back to provider-level credentials
	if r.ClientFactory.HasDefaultOrgCredentials() {
		return r.ClientFactory.GetDefaultOrgPublicKey(),
			r.ClientFactory.GetDefaultOrgPrivateKey(),
			nil
	}

	// Neither available - return error
	return "", "", fmt.Errorf(
		"organization credentials required: provide org_public_key and org_private_key " +
			"either at resource level or configure at provider level",
	)
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
				Optional:    true,
				Sensitive:   true,
				Description: "Organization public key to authenticate the call. If not provided, uses provider-level org_public_key.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_private_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Organization private key to authenticate the call. If not provided, uses provider-level org_private_key.",
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

	// Resolve credentials with fallback
	publicKey, privateKey, err := r.resolveOrgCredentials(ctx, data.OrganizationPublicKey, data.OrganizationPrivateKey)
	if err != nil {
		resp.Diagnostics.AddError("Missing Organization Credentials", err.Error())
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(publicKey, privateKey)
	projectApiKey, err := organizationClient.CreateProjectApiKey(ctx, data.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error creating project API key", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &projectApiKeyResourceModel{
		ID:                     types.StringValue(projectApiKey.ID),
		OrganizationPublicKey:  types.StringValue(publicKey),
		OrganizationPrivateKey: types.StringValue(privateKey),
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

	// Resolve credentials with fallback
	publicKey, privateKey, err := r.resolveOrgCredentials(ctx, data.OrganizationPublicKey, data.OrganizationPrivateKey)
	if err != nil {
		resp.Diagnostics.AddError("Missing Organization Credentials", err.Error())
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(publicKey, privateKey)
	_, err = organizationClient.GetProjectApiKey(ctx, data.ProjectID.ValueString(), data.ID.ValueString())
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

	// Resolve credentials with fallback
	publicKey, privateKey, err := r.resolveOrgCredentials(ctx, data.OrganizationPublicKey, data.OrganizationPrivateKey)
	if err != nil {
		resp.Diagnostics.AddError("Missing Organization Credentials", err.Error())
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(publicKey, privateKey)
	err = organizationClient.DeleteProjectApiKey(ctx, data.ProjectID.ValueString(), data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting project API key", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &projectApiKeyResourceModel{})...)
}

func (r *projectApiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importParts := strings.Split(req.ID, ",")

	var keyID, projectID, orgPublicKey, orgPrivateKey string

	switch len(importParts) {
	case 2:
		// New format: "key_id,project_id" - use provider credentials
		keyID = importParts[0]
		projectID = importParts[1]

		if !r.ClientFactory.HasDefaultOrgCredentials() {
			resp.Diagnostics.AddError(
				"Missing Organization Credentials for Import",
				"Import format 'key_id,project_id' requires provider-level org credentials. "+
					"Either:\n"+
					"1. Configure provider with org_public_key and org_private_key, or\n"+
					"2. Use import format: key_id,project_id,org_public_key,org_private_key",
			)
			return
		}

		orgPublicKey = r.ClientFactory.GetDefaultOrgPublicKey()
		orgPrivateKey = r.ClientFactory.GetDefaultOrgPrivateKey()

	case 4:
		// Legacy format: "key_id,project_id,public_key,private_key"
		keyID = importParts[0]
		projectID = importParts[1]
		orgPublicKey = importParts[2]
		orgPrivateKey = importParts[3]

	default:
		resp.Diagnostics.AddError(
			"Invalid import format",
			"Import ID must be in one of these formats:\n"+
				"1. key_id,project_id (requires provider-level credentials)\n"+
				"2. key_id,project_id,organization_public_key,organization_private_key",
		)
		return
	}

	// Validate we can fetch the project API key with the provided credentials
	organizationClient := r.ClientFactory.NewOrganizationClient(orgPublicKey, orgPrivateKey)

	// Note: The API key secret is not retrievable after creation, so we can only verify the key exists
	_, err := organizationClient.GetProjectApiKey(ctx, projectID, keyID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing project API key",
			fmt.Sprintf("Could not read project API key %s: %s", keyID, err.Error()))
		return
	}

	// Set the imported state with all available information
	// Note: public_key and secret_key are not available on import since they're only returned at creation
	resp.Diagnostics.Append(resp.State.Set(ctx, &projectApiKeyResourceModel{
		ID:                     types.StringValue(keyID),
		ProjectID:              types.StringValue(projectID),
		OrganizationPublicKey:  types.StringValue(orgPublicKey),
		OrganizationPrivateKey: types.StringValue(orgPrivateKey),
		PublicKey:              types.StringNull(), // Not retrievable after creation
		SecretKey:              types.StringNull(), // Not retrievable after creation
	})...)
}
