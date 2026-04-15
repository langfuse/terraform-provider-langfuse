package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
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
	UserID                 types.String `tfsdk:"user_id"`
	Role                   types.String `tfsdk:"role"`
	Email                  types.String `tfsdk:"email"`
	OrganizationPublicKey  types.String `tfsdk:"organization_public_key"`
	OrganizationPrivateKey types.String `tfsdk:"organization_private_key"`
	IgnoreDestroy          types.Bool   `tfsdk:"ignore_destroy"`
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
		Description: "Manages a user's membership in a Langfuse project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique identifier of the project membership (composed of project_id and user_id).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the project.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"user_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the user to add to the project.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": schema.StringAttribute{
				Required:    true,
				Description: "The role to assign to the user in the project. Valid values: OWNER, ADMIN, MEMBER, VIEWER, NONE.",
			},
			"email": schema.StringAttribute{
				Computed:    true,
				Description: "The email address of the user.",
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
			"ignore_destroy": schema.BoolAttribute{
				Optional:    true,
				Description: "When true, the project membership will not be removed when the resource is destroyed. Defaults to false.",
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

	validRoles := []string{"OWNER", "ADMIN", "MEMBER", "VIEWER", "NONE"}
	role := plan.Role.ValueString()
	isValidRole := false
	for _, vr := range validRoles {
		if role == vr {
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

	upsertReq := &langfuse.UpsertProjectMemberRequest{
		UserID: plan.UserID.ValueString(),
		Role:   role,
	}

	membership, err := organizationClient.UpsertProjectMembership(ctx, plan.ProjectID.ValueString(), upsertReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating project membership", err.Error())
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s/%s", plan.ProjectID.ValueString(), plan.UserID.ValueString()))
	plan.Role = types.StringValue(membership.Role)
	if membership.Email != "" {
		plan.Email = types.StringValue(membership.Email)
	} else {
		plan.Email = types.StringValue("")
	}

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

	membership, err := organizationClient.GetProjectMembership(ctx, state.ProjectID.ValueString(), state.UserID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "cannot find project membership") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading project membership", err.Error())
		return
	}

	state.Role = types.StringValue(membership.Role)
	if membership.Email != "" {
		state.Email = types.StringValue(membership.Email)
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

	validRoles := []string{"OWNER", "ADMIN", "MEMBER", "VIEWER", "NONE"}
	role := plan.Role.ValueString()
	isValidRole := false
	for _, vr := range validRoles {
		if role == vr {
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

	upsertReq := &langfuse.UpsertProjectMemberRequest{
		UserID: state.UserID.ValueString(),
		Role:   role,
	}

	membership, err := organizationClient.UpsertProjectMembership(ctx, state.ProjectID.ValueString(), upsertReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating project membership", err.Error())
		return
	}

	plan.ID = state.ID
	plan.Role = types.StringValue(membership.Role)
	if membership.Email != "" {
		plan.Email = types.StringValue(membership.Email)
	} else {
		plan.Email = state.Email
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *projectMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !state.IgnoreDestroy.IsNull() && state.IgnoreDestroy.ValueBool() {
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(
		state.OrganizationPublicKey.ValueString(),
		state.OrganizationPrivateKey.ValueString(),
	)

	err := organizationClient.RemoveProjectMember(ctx, state.ProjectID.ValueString(), state.UserID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error removing project member", err.Error())
		return
	}
}

func (r *projectMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: project_id/user_id
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
