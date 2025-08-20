package provider

import (
	"context"

	"github.com/cresta/terraform-provider-langfuse/internal/langfuse"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &organizationResource{}

func NewOrganizationResource() resource.Resource {
	return &organizationResource{}
}

type organizationResourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

type organizationResource struct {
	AdminClient langfuse.AdminClient
}

func (r *organizationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.AdminClient = req.ProviderData.(langfuse.ClientFactory).NewAdminClient()
}

func (r *organizationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (r *organizationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The display name of the organization.",
			},
		},
	}
}

func (r *organizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data organizationResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	org, err := r.AdminClient.CreateOrganization(ctx, &langfuse.CreateOrganizationRequest{Name: data.Name.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Error creating organization", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &organizationResourceModel{
		ID:   types.StringValue(org.ID),
		Name: types.StringValue(org.Name),
	})...)
}

func (r *organizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data organizationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	org, err := r.AdminClient.GetOrganization(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading organization", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &organizationResourceModel{
		ID:   types.StringValue(org.ID),
		Name: types.StringValue(org.Name),
	})...)
}

func (r *organizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data organizationResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	request := &langfuse.UpdateOrganizationRequest{
		Name: data.Name.ValueString(),
	}

	org, err := r.AdminClient.UpdateOrganization(ctx, data.ID.ValueString(), request)
	if err != nil {
		resp.Diagnostics.AddError("Error updating organization", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &organizationResourceModel{
		ID:   types.StringValue(org.ID),
		Name: types.StringValue(org.Name),
	})...)
}

func (r *organizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data organizationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.AdminClient.DeleteOrganization(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting organization", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &organizationResourceModel{})...)
}
