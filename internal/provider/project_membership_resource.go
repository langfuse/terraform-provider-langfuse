package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
)

var _ resource.Resource = &projectMembershipResource{}
var _ resource.ResourceWithImportState = &projectMembershipResource{}

func NewProjectMembershipResource() resource.Resource {
	return &projectMembershipResource{}
}

type projectMembershipResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	ProjectID              types.String `tfsdk:"project_id"`
	Email                  types.String `tfsdk:"email"`
	Role                   types.String `tfsdk:"role"`
	UserID                 types.String `tfsdk:"user_id"`
	Username               types.String `tfsdk:"username"`
	OrganizationPublicKey  types.String `tfsdk:"organization_public_key"`
	OrganizationPrivateKey types.String `tfsdk:"organization_private_key"`
}

type projectMembershipResource struct {
	ClientFactory langfuse.ClientFactory
}

func (r *projectMembershipResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientFactory, ok := req.ProviderData.(langfuse.ClientFactory)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected langfuse.ClientFactory, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.ClientFactory = clientFactory
}

func (r *projectMembershipResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_membership"
}

func (r *projectMembershipResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages membership in a Langfuse project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the project membership.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: "The ID of the project.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"email": schema.StringAttribute{
				Description: "The email address of the user to add to the project.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": schema.StringAttribute{
				Description: "The role to assign to the user. Valid values are: OWNER, ADMIN, MEMBER, VIEWER, NONE.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("OWNER", "ADMIN", "MEMBER", "VIEWER", "NONE"),
				},
			},
			"user_id": schema.StringAttribute{
				Description: "The unique identifier of the user.",
				Computed:    true,
			},
			"username": schema.StringAttribute{
				Description: "The username of the user.",
				Computed:    true,
			},
			"organization_public_key": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Organization public key to authenticate the call.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"organization_private_key": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Organization private key to authenticate the call.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *projectMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectMembershipResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role := plan.Role.ValueString()

	// Validate role is one of the allowed values
	validRoles := []string{"OWNER", "ADMIN", "MEMBER", "VIEWER", "NONE"}
	isValidRole := false
	for _, validRole := range validRoles {
		if role == validRole {
			isValidRole = true
			break
		}
	}
	if !isValidRole {
		resp.Diagnostics.AddError(
			"Invalid Role",
			fmt.Sprintf("Role must be one of: %s. Got: %s", strings.Join(validRoles, ", "), role),
		)
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(
		plan.OrganizationPublicKey.ValueString(),
		plan.OrganizationPrivateKey.ValueString(),
	)

	// Look up user ID from email via organization memberships
	memberships, err := organizationClient.ListMemberships(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing organization memberships", err.Error())
		return
	}

	var userID string
	email := plan.Email.ValueString()
	for _, m := range memberships {
		if m.Email == email {
			userID = m.UserID
			break
		}
	}
	if userID == "" {
		resp.Diagnostics.AddError(
			"User not found",
			fmt.Sprintf("No organization member found with email: %s", email),
		)
		return
	}

	createRequest := &langfuse.CreateProjectMembershipRequest{
		UserID: userID,
		Role:   role,
	}

	membership, err := organizationClient.CreateOrUpdateProjectMembership(ctx, plan.ProjectID.ValueString(), createRequest)
	if err != nil {
		resp.Diagnostics.AddError("Error creating project membership", err.Error())
		return
	}

	// Use UserID as ID if membership.ID is empty (following org membership pattern)
	membershipID := membership.ID
	if membershipID == "" {
		membershipID = membership.UserID
	}

	plan.ID = types.StringValue(membershipID)
	plan.UserID = types.StringValue(membership.UserID)
	plan.Username = types.StringValue(membership.Username)
	plan.Role = types.StringValue(membership.Role)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *projectMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(
		state.OrganizationPublicKey.ValueString(),
		state.OrganizationPrivateKey.ValueString(),
	)

	membership, err := organizationClient.GetProjectMembership(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "cannot find project membership") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading project membership", err.Error())
		return
	}

	// Update state with current values
	state.Role = types.StringValue(membership.Role)
	state.UserID = types.StringValue(membership.UserID)
	state.Username = types.StringValue(membership.Username)

	if membership.ID != "" {
		state.ID = types.StringValue(membership.ID)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *projectMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectMembershipResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state projectMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role := plan.Role.ValueString()

	// Validate role is one of the allowed values
	validRoles := []string{"OWNER", "ADMIN", "MEMBER", "VIEWER", "NONE"}
	isValidRole := false
	for _, validRole := range validRoles {
		if role == validRole {
			isValidRole = true
			break
		}
	}
	if !isValidRole {
		resp.Diagnostics.AddError(
			"Invalid Role",
			fmt.Sprintf("Role must be one of: %s. Got: %s", strings.Join(validRoles, ", "), role),
		)
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(
		state.OrganizationPublicKey.ValueString(),
		state.OrganizationPrivateKey.ValueString(),
	)

	updateRequest := &langfuse.CreateProjectMembershipRequest{
		UserID: state.UserID.ValueString(),
		Role:   role,
	}

	membership, err := organizationClient.CreateOrUpdateProjectMembership(ctx, state.ProjectID.ValueString(), updateRequest)
	if err != nil {
		resp.Diagnostics.AddError("Error updating project membership", err.Error())
		return
	}

	membershipID := membership.ID
	if membershipID == "" {
		membershipID = membership.UserID
	}

	plan.ID = types.StringValue(membershipID)
	plan.UserID = types.StringValue(membership.UserID)
	plan.Username = types.StringValue(membership.Username)
	plan.Role = types.StringValue(membership.Role)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *projectMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(
		state.OrganizationPublicKey.ValueString(),
		state.OrganizationPrivateKey.ValueString(),
	)

	err := organizationClient.DeleteProjectMembership(ctx, state.ProjectID.ValueString(), state.UserID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error removing project member", err.Error())
		return
	}
}

func (r *projectMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: project_id,membership_id,organization_public_key,organization_private_key
	// Example: terraform import langfuse_project_membership.example "proj_123,mem_456,pk_789,sk_012"

	importParts := strings.Split(req.ID, ",")
	if len(importParts) != 4 {
		resp.Diagnostics.AddError("Invalid import format",
			"Import ID must be in format: project_id,membership_id,organization_public_key,organization_private_key")
		return
	}

	projectID := importParts[0]
	membershipID := importParts[1]
	organizationPublicKey := importParts[2]
	organizationPrivateKey := importParts[3]

	organizationClient := r.ClientFactory.NewOrganizationClient(organizationPublicKey, organizationPrivateKey)
	membership, err := organizationClient.GetProjectMembership(ctx, projectID, membershipID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing project membership",
			"Could not read project membership "+membershipID+" in project "+projectID+": "+err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &projectMembershipResourceModel{
		ID:                     types.StringValue(membershipID),
		ProjectID:              types.StringValue(projectID),
		Email:                  types.StringValue(membership.Email),
		Role:                   types.StringValue(membership.Role),
		UserID:                 types.StringValue(membership.UserID),
		Username:               types.StringValue(membership.Username),
		OrganizationPublicKey:  types.StringValue(organizationPublicKey),
		OrganizationPrivateKey: types.StringValue(organizationPrivateKey),
	})...)

	resource.ImportStatePassthroughID(ctx, path.Root("id"), resource.ImportStateRequest{ID: membershipID}, resp)
}
