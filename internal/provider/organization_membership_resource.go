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

var _ resource.Resource = &organizationMembershipResource{}
var _ resource.ResourceWithImportState = &organizationMembershipResource{}

func NewOrganizationMembershipResource() resource.Resource {
	return &organizationMembershipResource{}
}

type organizationMembershipResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	Email                  types.String `tfsdk:"email"`
	Role                   types.String `tfsdk:"role"`
	Status                 types.String `tfsdk:"status"`
	UserID                 types.String `tfsdk:"user_id"`
	Username               types.String `tfsdk:"username"`
	OrganizationPublicKey  types.String `tfsdk:"organization_public_key"`
	OrganizationPrivateKey types.String `tfsdk:"organization_private_key"`
}

type organizationMembershipResource struct {
	ClientFactory langfuse.ClientFactory
}

func (r *organizationMembershipResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *organizationMembershipResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_membership"
}

func (r *organizationMembershipResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages membership in a Langfuse organization.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the membership.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"email": schema.StringAttribute{
				Description: "The email address of the user to invite.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": schema.StringAttribute{
				Description: "The role to assign to the user. Valid values are: ADMIN, MEMBER, VIEWER.",
				Required:    true,
			},
			"status": schema.StringAttribute{
				Description: "The status of the membership invitation.",
				Computed:    true,
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

func (r *organizationMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan organizationMembershipResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate role is one of the allowed values
	validRoles := []string{"OWNER","ADMIN", "MEMBER", "VIEWER","NONE"}
	role := plan.Role.ValueString()
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

	organizationClient := r.ClientFactory.NewOrganizationClient(plan.OrganizationPublicKey.ValueString(), plan.OrganizationPrivateKey.ValueString())

	email := plan.Email.ValueString()

	// Check if the user already exists in the organization
	memberships, err := organizationClient.ListMemberships(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing current memberships", err.Error())
		return
	}

	var existingMembership *langfuse.OrganizationMembership
	for i := range memberships {
		if memberships[i].Email == email {
			existingMembership = &memberships[i]
			break
		}
	}

	// If user doesn't exist in organization, create them via SCIM
	if existingMembership == nil {
		scimRequest := &langfuse.SCIMUserRequest{
			UserName: email,
			Active:   true,
			Emails: []struct {
				Value   string `json:"value"`
				Primary bool   `json:"primary"`
			}{
				{
					Value:   email,
					Primary: true,
				},
			},
		}

		scimUser, err := organizationClient.CreateSCIMUser(ctx, scimRequest)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error creating user via SCIM",
				fmt.Sprintf("Failed to create user with email %s: %v. User may already exist in Langfuse system.", email, err),
			)
			return
		}

		// Refresh membership list to find the newly created user membership
		memberships, err := organizationClient.ListMemberships(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Error listing memberships after SCIM user creation", err.Error())
			return
		}

		var newMembership *langfuse.OrganizationMembership
		for i := range memberships {
			if memberships[i].UserID == scimUser.ID {
				newMembership = &memberships[i]
				break
			}
		}

		if newMembership == nil {
			resp.Diagnostics.AddError(
				"Error finding new membership",
				fmt.Sprintf("User was created via SCIM but membership not found in organization. UserID: %s", scimUser.ID),
			)
			return
		}

		// Now update their role via membership endpoint
		updateRequest := &langfuse.UpdateMembershipRequest{
			UserID: scimUser.ID,
			Role:   role,
		}

		membership, err := organizationClient.UpdateMembership(ctx, newMembership.ID, updateRequest)
		if err != nil {
			resp.Diagnostics.AddError("Error updating membership role", err.Error())
			return
		}

		// The API may not return membership ID, so use UserID as the resource ID
		membershipID := membership.ID
		if membershipID == "" {
			membershipID = membership.UserID
		}

		plan.ID = types.StringValue(membershipID)
		plan.Email = types.StringValue(membership.Email)
		plan.Role = types.StringValue(membership.Role)
		plan.Status = types.StringValue(membership.Status)
		plan.UserID = types.StringValue(membership.UserID)
		plan.Username = types.StringValue(membership.Username)
	} else {
		// User already exists in organization, update their role
		updateRequest := &langfuse.UpdateMembershipRequest{
			UserID: existingMembership.UserID,
			Role:   role,
		}

		membership, err := organizationClient.UpdateMembership(ctx, existingMembership.ID, updateRequest)
		if err != nil {
			resp.Diagnostics.AddError("Error updating membership role", err.Error())
			return
		}

		// The API may not return membership ID, so use UserID as the resource ID
		membershipID := membership.ID
		if membershipID == "" {
			membershipID = membership.UserID
		}

		plan.ID = types.StringValue(membershipID)
		plan.Email = types.StringValue(membership.Email)
		plan.Role = types.StringValue(membership.Role)
		plan.Status = types.StringValue(membership.Status)
		plan.UserID = types.StringValue(membership.UserID)
		plan.Username = types.StringValue(membership.Username)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *organizationMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state organizationMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(state.OrganizationPublicKey.ValueString(), state.OrganizationPrivateKey.ValueString())

	membership, err := organizationClient.GetMembership(ctx, state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "cannot find membership") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading membership", err.Error())
		return
	}

	state.Email = types.StringValue(membership.Email)
	state.Role = types.StringValue(membership.Role)
	state.Status = types.StringValue(membership.Status)
	state.UserID = types.StringValue(membership.UserID)
	state.Username = types.StringValue(membership.Username)

	// The API may not return membership ID, so use UserID as the resource ID
	if membership.ID != "" {
		state.ID = types.StringValue(membership.ID)
	} else {
		state.ID = types.StringValue(membership.UserID)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *organizationMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan organizationMembershipResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state organizationMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate role is one of the allowed values
	validRoles := []string{"OWNER", "ADMIN", "MEMBER", "VIEWER","NONE"}
	role := plan.Role.ValueString()
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

	organizationClient := r.ClientFactory.NewOrganizationClient(state.OrganizationPublicKey.ValueString(), state.OrganizationPrivateKey.ValueString())

	updateRequest := &langfuse.UpdateMembershipRequest{
		Role: role,
	}

	membership, err := organizationClient.UpdateMembership(ctx, state.ID.ValueString(), updateRequest)
	if err != nil {
		resp.Diagnostics.AddError("Error updating membership", err.Error())
		return
	}

	// The API may not return membership ID, so use UserID as the resource ID
	membershipID := membership.ID
	if membershipID == "" {
		membershipID = membership.UserID
	}

	plan.ID = types.StringValue(membershipID)
	plan.Email = types.StringValue(membership.Email)
	plan.Role = types.StringValue(membership.Role)
	plan.Status = types.StringValue(membership.Status)
	plan.UserID = types.StringValue(membership.UserID)
	plan.Username = types.StringValue(membership.Username)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *organizationMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state organizationMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(state.OrganizationPublicKey.ValueString(), state.OrganizationPrivateKey.ValueString())

	err := organizationClient.RemoveMember(ctx, state.UserID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error removing member", err.Error())
		return
	}
}

func (r *organizationMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: membership_id,organization_public_key,organization_private_key
	// Example: terraform import langfuse_organization_membership.example "mem_123,pk_456,sk_789"

	importParts := strings.Split(req.ID, ",")
	if len(importParts) != 3 {
		resp.Diagnostics.AddError("Invalid import format",
			"Import ID must be in format: membership_id,organization_public_key,organization_private_key")
		return
	}

	membershipID := importParts[0]
	orgPublicKey := importParts[1]
	orgPrivateKey := importParts[2]

	// Validate we can fetch the membership with the provided credentials
	organizationClient := r.ClientFactory.NewOrganizationClient(orgPublicKey, orgPrivateKey)

	membership, err := organizationClient.GetMembership(ctx, membershipID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing membership",
			fmt.Sprintf("Could not read membership %s: %s", membershipID, err.Error()))
		return
	}

	// Set the imported state with all required information
	resp.Diagnostics.Append(resp.State.Set(ctx, &organizationMembershipResourceModel{
		ID:                     types.StringValue(membershipID),
		Email:                  types.StringValue(membership.Email),
		Role:                   types.StringValue(membership.Role),
		Status:                 types.StringValue(membership.Status),
		UserID:                 types.StringValue(membership.UserID),
		Username:               types.StringValue(membership.Username),
		OrganizationPublicKey:  types.StringValue(orgPublicKey),
		OrganizationPrivateKey: types.StringValue(orgPrivateKey),
	})...)
}
