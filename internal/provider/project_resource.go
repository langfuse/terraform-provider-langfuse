package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
)

var _ resource.Resource = &projectResource{}
var _ resource.ResourceWithImportState = &projectResource{}

func NewProjectResource() resource.Resource {
	return &projectResource{}
}

type projectResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	RetentionDays          types.Int32  `tfsdk:"retention_days"`
	Metadata               types.Map    `tfsdk:"metadata"`
	OrganizationID         types.String `tfsdk:"organization_id"`
	OrganizationPublicKey  types.String `tfsdk:"organization_public_key"`
	OrganizationPrivateKey types.String `tfsdk:"organization_private_key"`
}

type projectResource struct {
	ClientFactory langfuse.ClientFactory
}

func (r *projectResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.ClientFactory = req.ProviderData.(langfuse.ClientFactory)
}

// resolveOrgCredentials returns the org credentials to use, implementing the fallback pattern:
// 1. Use resource-level credentials if provided
// 2. Fall back to provider-level credentials if available
// 3. Error if neither is available
func (r *projectResource) resolveOrgCredentials(
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

func (r *projectResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *projectResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The display name of the project.",
			},
			"retention_days": schema.Int32Attribute{
				Optional:    true,
				Description: "The retention period for the project in days. If not set, or set with a value of 0, data will be stored indefinitely.",
			},
			"metadata": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Metadata for the project as key-value pairs.",
			},
			"organization_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the organization that owns this project.",
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
		},
	}
}

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data projectResourceModel
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

	metadata := make(map[string]string)
	if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
		resp.Diagnostics.Append(data.Metadata.ElementsAs(ctx, &metadata, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(publicKey, privateKey)
	project, err := organizationClient.CreateProject(ctx, &langfuse.CreateProjectRequest{
		Name:          data.Name.ValueString(),
		RetentionDays: data.RetentionDays.ValueInt32(),
		Metadata:      metadata,
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating project", err.Error())
		return
	}

	var metadataMap types.Map
	if len(project.Metadata) > 0 {
		var diags diag.Diagnostics
		metadataMap, diags = types.MapValueFrom(ctx, types.StringType, project.Metadata)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	} else {
		metadataMap = types.MapNull(types.StringType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &projectResourceModel{
		ID:                     types.StringValue(project.ID),
		Name:                   types.StringValue(project.Name),
		RetentionDays:          types.Int32Value(project.RetentionDays),
		Metadata:               metadataMap,
		OrganizationID:         types.StringValue(data.OrganizationID.ValueString()),
		OrganizationPublicKey:  types.StringValue(publicKey),
		OrganizationPrivateKey: types.StringValue(privateKey),
	})...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data projectResourceModel
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
	project, err := organizationClient.GetProject(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading project", err.Error())
		return
	}

	var metadataMap types.Map
	if len(project.Metadata) > 0 {
		var diags diag.Diagnostics
		metadataMap, diags = types.MapValueFrom(ctx, types.StringType, project.Metadata)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	} else {
		metadataMap = types.MapNull(types.StringType)
	}

	// Note: retention_days is write-only in the Langfuse API and not returned in responses.
	resp.Diagnostics.Append(resp.State.Set(ctx, &projectResourceModel{
		ID:                     types.StringValue(project.ID),
		Name:                   types.StringValue(project.Name),
		RetentionDays:          data.RetentionDays,
		Metadata:               metadataMap,
		OrganizationID:         types.StringValue(data.OrganizationID.ValueString()),
		OrganizationPublicKey:  types.StringValue(publicKey),
		OrganizationPrivateKey: types.StringValue(privateKey),
	})...)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data projectResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Get ID from current state (ID is not in config during updates)
	var currentState projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &currentState)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve credentials with fallback
	publicKey, privateKey, err := r.resolveOrgCredentials(ctx, data.OrganizationPublicKey, data.OrganizationPrivateKey)
	if err != nil {
		resp.Diagnostics.AddError("Missing Organization Credentials", err.Error())
		return
	}

	projectID := currentState.ID.ValueString()

	metadata := make(map[string]string)
	if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
		resp.Diagnostics.Append(data.Metadata.ElementsAs(ctx, &metadata, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(publicKey, privateKey)

	request := &langfuse.UpdateProjectRequest{
		Name:          data.Name.ValueString(),
		RetentionDays: data.RetentionDays.ValueInt32(),
		Metadata:      metadata,
	}

	project, err := organizationClient.UpdateProject(ctx, projectID, request)
	if err != nil {
		resp.Diagnostics.AddError("Error updating project", err.Error())
		return
	}

	var metadataMap types.Map
	if len(project.Metadata) > 0 {
		var diags diag.Diagnostics
		metadataMap, diags = types.MapValueFrom(ctx, types.StringType, project.Metadata)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	} else {
		metadataMap = types.MapNull(types.StringType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &projectResourceModel{
		ID:                     types.StringValue(project.ID),
		Name:                   types.StringValue(project.Name),
		RetentionDays:          data.RetentionDays, // Use from config, not API response
		Metadata:               metadataMap,
		OrganizationID:         types.StringValue(data.OrganizationID.ValueString()),
		OrganizationPublicKey:  types.StringValue(publicKey),
		OrganizationPrivateKey: types.StringValue(privateKey),
	})...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data projectResourceModel
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
	err = organizationClient.DeleteProject(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting project", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &projectResourceModel{
		ID:                     types.StringValue(""),
		Name:                   types.StringValue(""),
		RetentionDays:          types.Int32Value(0),
		Metadata:               types.MapNull(types.StringType),
		OrganizationID:         types.StringValue(""),
		OrganizationPublicKey:  types.StringValue(""),
		OrganizationPrivateKey: types.StringValue(""),
	})...)
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importParts := strings.Split(req.ID, ",")

	var projectID, organizationID, organizationPublicKey, organizationPrivateKey string

	switch len(importParts) {
	case 2:
		// New format: "project_id,organization_id" - use provider credentials
		projectID = importParts[0]
		organizationID = importParts[1]

		if !r.ClientFactory.HasDefaultOrgCredentials() {
			resp.Diagnostics.AddError(
				"Missing Organization Credentials for Import",
				"Import format 'project_id,organization_id' requires provider-level org credentials. "+
					"Either:\n"+
					"1. Configure provider with org_public_key and org_private_key, or\n"+
					"2. Use import format: project_id,organization_id,org_public_key,org_private_key",
			)
			return
		}

		organizationPublicKey = r.ClientFactory.GetDefaultOrgPublicKey()
		organizationPrivateKey = r.ClientFactory.GetDefaultOrgPrivateKey()

	case 4:
		// Legacy format: "project_id,organization_id,public_key,private_key"
		projectID = importParts[0]
		organizationID = importParts[1]
		organizationPublicKey = importParts[2]
		organizationPrivateKey = importParts[3]

	default:
		resp.Diagnostics.AddError("Invalid import format",
			"Import ID must be in one of these formats:\n"+
				"1. project_id,organization_id (requires provider-level credentials)\n"+
				"2. project_id,organization_id,organization_public_key,organization_private_key")
		return
	}

	// Get the project details using the provided organization credentials
	organizationClient := r.ClientFactory.NewOrganizationClient(organizationPublicKey, organizationPrivateKey)
	project, err := organizationClient.GetProject(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing project",
			"Could not read project "+projectID+": "+err.Error())
		return
	}

	// Convert metadata to the appropriate type
	var metadataMap types.Map
	if len(project.Metadata) > 0 {
		var diags diag.Diagnostics
		metadataMap, diags = types.MapValueFrom(ctx, types.StringType, project.Metadata)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	} else {
		metadataMap = types.MapNull(types.StringType)
	}

	// Set the imported state with all required information
	resp.Diagnostics.Append(resp.State.Set(ctx, &projectResourceModel{
		ID:                     types.StringValue(project.ID),
		Name:                   types.StringValue(project.Name),
		RetentionDays:          types.Int32Value(0), // Default value since retention_days is write-only in Langfuse API
		Metadata:               metadataMap,
		OrganizationID:         types.StringValue(organizationID),
		OrganizationPublicKey:  types.StringValue(organizationPublicKey),
		OrganizationPrivateKey: types.StringValue(organizationPrivateKey),
	})...)

	// Set the ID attribute explicitly to just the project ID (not the full import string)
	resource.ImportStatePassthroughID(ctx, path.Root("id"), resource.ImportStateRequest{ID: projectID}, resp)
}
