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

var _ resource.Resource = &projectResource{}

func NewProjectResource() resource.Resource {
	return &organizationResource{}
}

type projectResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	Retention              types.Int32  `tfsdk:"retention"`
	OrganizationPublicKey  types.String `tfsdk:"organization_public_key"`
	OrganizationPrivateKey types.String `tfsdk:"organization_private_key"`
}

type projectResource struct {
	ClientFactory langfuse.ClientFactory
}

func (r *projectResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.ClientFactory = req.ProviderData.(langfuse.ClientFactory)
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
			"retention": schema.Int64Attribute{
				Optional:    true,
				Description: "The retention period for the project in days. If not set, or set with a value of 0, data will be stored indefinitely.",
			},
			"organization_public_key": schema.StringAttribute{
				Optional:    false,
				Sensitive:   true,
				Description: "Organization public key to authenticate the call.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_private_key": schema.StringAttribute{
				Optional:    false,
				Sensitive:   true,
				Description: "Organization private key to authenticate the call.",
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

	organizationClient := r.ClientFactory.NewOrganizationClient(data.OrganizationPublicKey.ValueString(), data.OrganizationPrivateKey.ValueString())
	project, err := organizationClient.CreateProject(ctx, langfuse.Project{
		Name:      data.Name.ValueString(),
		Retention: data.Retention.ValueInt32(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating project", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &projectResourceModel{
		ID:                     types.StringValue(project.ID),
		Name:                   types.StringValue(project.Name),
		Retention:              types.Int32Value(project.Retention),
		OrganizationPublicKey:  types.StringValue(data.OrganizationPublicKey.ValueString()),
		OrganizationPrivateKey: types.StringValue(data.OrganizationPrivateKey.ValueString()),
	})...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(data.OrganizationPublicKey.ValueString(), data.OrganizationPrivateKey.ValueString())
	project, err := organizationClient.GetProject(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading project", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &projectResourceModel{
		ID:                     types.StringValue(project.ID),
		Name:                   types.StringValue(project.Name),
		Retention:              types.Int32Value(project.Retention),
		OrganizationPublicKey:  types.StringValue(data.OrganizationPublicKey.ValueString()),
		OrganizationPrivateKey: types.StringValue(data.OrganizationPrivateKey.ValueString()),
	})...)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data projectResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(data.OrganizationPublicKey.ValueString(), data.OrganizationPrivateKey.ValueString())

	opts := langfuse.UpdateProjectRequest{}
	opts.Name = data.Name.ValueString()
	opts.Retention = data.Retention.ValueInt32()

	project, err := organizationClient.UpdateProject(ctx, data.ID.ValueString(), opts)
	if err != nil {
		resp.Diagnostics.AddError("Error updating project", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &projectResourceModel{
		ID:                     types.StringValue(project.ID),
		Name:                   types.StringValue(project.Name),
		Retention:              types.Int32Value(project.Retention),
		OrganizationPublicKey:  types.StringValue(data.OrganizationPublicKey.ValueString()),
		OrganizationPrivateKey: types.StringValue(data.OrganizationPrivateKey.ValueString()),
	})...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(data.OrganizationPublicKey.ValueString(), data.OrganizationPrivateKey.ValueString())
	err := organizationClient.DeleteProject(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting project", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &projectResourceModel{})...)
}
