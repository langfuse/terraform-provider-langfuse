package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	Name                   types.String `tfsdk:"name"`
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
			"name": schema.StringAttribute{
				Description: "The name of the user.",
				Computed:    true,
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

func (r *projectMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data projectMembershipResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role := data.Role.ValueString()

	organizationClient := r.ClientFactory.NewOrganizationClient(
		data.OrganizationPublicKey.ValueString(),
		data.OrganizationPrivateKey.ValueString(),
	)

	// Look up user ID from email via organization memberships
	memberships, err := organizationClient.ListMemberships(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing organization memberships", err.Error())
		return
	}

	var userID string
	email := data.Email.ValueString()
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

	membership, err := organizationClient.CreateOrUpdateProjectMembership(ctx, data.ProjectID.ValueString(), createRequest)
	if err != nil {
		resp.Diagnostics.AddError("Error creating project membership", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &projectMembershipResourceModel{
		ID:                     types.StringValue(membership.UserID),
		ProjectID:              data.ProjectID,
		Email:                  data.Email,
		Role:                   types.StringValue(membership.Role),
		UserID:                 types.StringValue(membership.UserID),
		Name:                   types.StringValue(membership.Name),
		OrganizationPublicKey:  data.OrganizationPublicKey,
		OrganizationPrivateKey: data.OrganizationPrivateKey,
	})...)
}

func (r *projectMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.ClientFactory == nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(
		state.OrganizationPublicKey.ValueString(),
		state.OrganizationPrivateKey.ValueString(),
	)

	membership, err := organizationClient.GetProjectMembership(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if errors.Is(err, langfuse.ErrProjectMembershipNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading project membership", err.Error())
		return
	}

	// Update state with current values
	state.Role = types.StringValue(membership.Role)
	state.UserID = types.StringValue(membership.UserID)
	state.Name = types.StringValue(membership.Name)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *projectMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data projectMembershipResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state projectMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role := data.Role.ValueString()

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

	resp.Diagnostics.Append(resp.State.Set(ctx, &projectMembershipResourceModel{
		ID:                     types.StringValue(membership.UserID),
		ProjectID:              state.ProjectID,
		Email:                  data.Email,
		Role:                   types.StringValue(membership.Role),
		UserID:                 types.StringValue(membership.UserID),
		Name:                   types.StringValue(membership.Name),
		OrganizationPublicKey:  state.OrganizationPublicKey,
		OrganizationPrivateKey: state.OrganizationPrivateKey,
	})...)
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
	// Import format: project_id,user_id,organization_public_key,organization_private_key
	// Example: terraform import langfuse_project_membership.example "proj_123,mem_456,pk_789,sk_012"

	importParts := strings.Split(req.ID, ",")
	if len(importParts) != 4 {
		resp.Diagnostics.AddError("Invalid import format",
			"Import ID must be in format: project_id,user_id,organization_public_key,organization_private_key")
		return
	}

	projectID := importParts[0]
	userID := importParts[1]
	organizationPublicKey := importParts[2]
	organizationPrivateKey := importParts[3]

	organizationClient := r.ClientFactory.NewOrganizationClient(organizationPublicKey, organizationPrivateKey)
	membership, err := organizationClient.GetProjectMembership(ctx, projectID, userID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing project membership",
			"Could not read project membership for user "+userID+" in project "+projectID+": "+err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &projectMembershipResourceModel{
		ID:                     types.StringValue(membership.UserID),
		ProjectID:              types.StringValue(projectID),
		Email:                  types.StringValue(membership.Email),
		Role:                   types.StringValue(membership.Role),
		UserID:                 types.StringValue(membership.UserID),
		Name:                   types.StringValue(membership.Name),
		OrganizationPublicKey:  types.StringValue(organizationPublicKey),
		OrganizationPrivateKey: types.StringValue(organizationPrivateKey),
	})...)
}
