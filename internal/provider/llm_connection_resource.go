package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
)

var _ resource.Resource = &llmConnectionResource{}
var _ resource.ResourceWithImportState = &llmConnectionResource{}

func NewLlmConnectionResource() resource.Resource {
	return &llmConnectionResource{}
}

type llmConnectionResourceModel struct {
	ID               types.String `tfsdk:"id"`
	ProjectPublicKey types.String `tfsdk:"project_public_key"`
	ProjectSecretKey types.String `tfsdk:"project_secret_key"`

	ProviderName types.String `tfsdk:"provider_name"`
	Adapter      types.String `tfsdk:"adapter"`

	SecretKey types.String `tfsdk:"secret_key"`

	BaseURL           types.String `tfsdk:"base_url"`
	CustomModels      types.List   `tfsdk:"custom_models"`
	WithDefaultModels types.Bool   `tfsdk:"with_default_models"`
	ExtraHeaders      types.Map    `tfsdk:"extra_headers"`
	ConfigJSON        types.String `tfsdk:"config_json"`

	DisplaySecretKey types.String `tfsdk:"display_secret_key"`
	ExtraHeaderKeys  types.List   `tfsdk:"extra_header_keys"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
	IgnoreDestroy    types.Bool   `tfsdk:"ignore_destroy"`
}

type llmConnectionResource struct {
	ClientFactory langfuse.ClientFactory
}

func (r *llmConnectionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.ClientFactory = req.ProviderData.(langfuse.ClientFactory)
}

func (r *llmConnectionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_llm_connection"
}

func (r *llmConnectionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_public_key": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Project public key to authenticate the call.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"project_secret_key": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Project secret key to authenticate the call.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"provider_name": schema.StringAttribute{
				Required:    true,
				Description: "Provider name (e.g., 'openai', 'my-gateway'). Must be unique in project.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"adapter": schema.StringAttribute{
				Required:    true,
				Description: "Adapter used to interface with the LLM (e.g., openai, anthropic, azure, bedrock).",
			},
			"secret_key": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Secret key for the LLM API. Not returned by Langfuse after upsert.",
				PlanModifiers: []planmodifier.String{
					// Keep the value that is already in state because Read() will never be able to fetch it.
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"base_url": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Custom base URL for the LLM API.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"custom_models": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of custom model names available for this connection.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"with_default_models": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether to include default models for this adapter. Default is true.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"extra_headers": schema.MapAttribute{
				Optional:    true,
				Sensitive:   true,
				ElementType: types.StringType,
				Description: "Extra headers to send with requests. Values are not returned by Langfuse after upsert.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
			},
			"config_json": schema.StringAttribute{
				Optional:    true,
				Description: "Adapter-specific configuration as a JSON object (e.g. for bedrock: {\"region\":\"us-east-1\"}).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"display_secret_key": schema.StringAttribute{
				Computed:    true,
				Description: "Masked version of the secret key for display purposes.",
			},
			"extra_header_keys": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Keys of extra headers sent with requests (values excluded for security).",
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the connection was created.",
			},
			"updated_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the connection was last updated.",
			},
			"ignore_destroy": schema.BoolAttribute{
				Optional:    true,
				Description: "When true, the resource will not be deleted in Langfuse when destroyed via Terraform. Defaults to false.",
			},
		},
	}
}

func (r *llmConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data llmConnectionResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	conn, err := r.upsert(ctx, &data, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError("Error creating LLM connection", err.Error())
		return
	}

	state, diags := llmConnectionStateFromAPI(ctx, conn, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *llmConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data llmConnectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(data.ProjectPublicKey.ValueString(), data.ProjectSecretKey.ValueString())
	conn, err := findLlmConnectionByProvider(ctx, organizationClient, data.ProviderName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading LLM connection", err.Error())
		return
	}
	if conn == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state, diags := llmConnectionStateFromAPI(ctx, conn, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *llmConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data llmConnectionResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	conn, err := r.upsert(ctx, &data, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError("Error updating LLM connection", err.Error())
		return
	}

	state, diags := llmConnectionStateFromAPI(ctx, conn, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *llmConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data llmConnectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if !data.IgnoreDestroy.IsNull() && data.IgnoreDestroy.ValueBool() {
		return
	}

	resp.Diagnostics.AddWarning(
		"LLM connection deletion skipped",
		"The Langfuse API does not expose an endpoint to delete LLM connections. The resource will be removed from state but remain in Langfuse.",
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, emptyLlmConnectionState())...)
}

func (r *llmConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: provider,project_public_key,project_secret_key,secret_key
	// Example: terraform import langfuse_llm_connection.example "openai,pk_123,sk_456,OPENAI_KEY"

	importParts := strings.Split(req.ID, ",")
	if len(importParts) != 4 {
		resp.Diagnostics.AddError("Invalid import format", "Import ID must be in format: provider,project_public_key,project_secret_key,secret_key")
		return
	}

	provider := importParts[0]
	projectPublicKey := importParts[1]
	projectSecretKey := importParts[2]
	secretKey := importParts[3]

	organizationClient := r.ClientFactory.NewOrganizationClient(projectPublicKey, projectSecretKey)
	conn, err := findLlmConnectionByProvider(ctx, organizationClient, provider)
	if err != nil {
		resp.Diagnostics.AddError("Error importing LLM connection", err.Error())
		return
	}
	if conn == nil {
		resp.Diagnostics.AddError("Error importing LLM connection", fmt.Sprintf("Could not find LLM connection with provider %q", provider))
		return
	}

	state := llmConnectionResourceModel{
		ID:               types.StringValue(conn.ID),
		ProjectPublicKey: types.StringValue(projectPublicKey),
		ProjectSecretKey: types.StringValue(projectSecretKey),
		ProviderName:     types.StringValue(conn.Provider),
		Adapter:          types.StringValue(conn.Adapter),
		SecretKey:        types.StringValue(secretKey),
		ExtraHeaders:     types.MapNull(types.StringType),
		ConfigJSON:       types.StringNull(),
	}

	fullState, diags := llmConnectionStateFromAPI(ctx, conn, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &fullState)...)
	resource.ImportStatePassthroughID(ctx, path.Root("id"), resource.ImportStateRequest{ID: conn.ID}, resp)
}

func (r *llmConnectionResource) upsert(ctx context.Context, data *llmConnectionResourceModel, diags *diag.Diagnostics) (*langfuse.LlmConnection, error) {
	var baseURL *string
	if !data.BaseURL.IsNull() && !data.BaseURL.IsUnknown() && data.BaseURL.ValueString() != "" {
		v := data.BaseURL.ValueString()
		baseURL = &v
	}

	var customModels []string
	if !data.CustomModels.IsNull() && !data.CustomModels.IsUnknown() {
		diags.Append(data.CustomModels.ElementsAs(ctx, &customModels, false)...)
		if diags.HasError() {
			return nil, fmt.Errorf("invalid custom_models")
		}
	}

	var withDefaultModels *bool
	if !data.WithDefaultModels.IsNull() && !data.WithDefaultModels.IsUnknown() {
		v := data.WithDefaultModels.ValueBool()
		withDefaultModels = &v
	}

	extraHeaders := make(map[string]string)
	if !data.ExtraHeaders.IsNull() && !data.ExtraHeaders.IsUnknown() {
		diags.Append(data.ExtraHeaders.ElementsAs(ctx, &extraHeaders, false)...)
		if diags.HasError() {
			return nil, fmt.Errorf("invalid extra_headers")
		}
	}

	var config map[string]any
	if !data.ConfigJSON.IsNull() && !data.ConfigJSON.IsUnknown() && data.ConfigJSON.ValueString() != "" {
		raw := []byte(data.ConfigJSON.ValueString())
		if !json.Valid(raw) {
			return nil, fmt.Errorf("config_json must be valid JSON")
		}
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return nil, fmt.Errorf("config_json must be valid JSON: %w", err)
		}
		m, ok := decoded.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("config_json must be a JSON object")
		}
		config = m
	}

	upsertReq := &langfuse.UpsertLlmConnectionRequest{
		Provider:          data.ProviderName.ValueString(),
		Adapter:           data.Adapter.ValueString(),
		SecretKey:         data.SecretKey.ValueString(),
		BaseURL:           baseURL,
		CustomModels:      customModels,
		WithDefaultModels: withDefaultModels,
	}
	if len(extraHeaders) > 0 {
		upsertReq.ExtraHeaders = extraHeaders
	}
	if config != nil {
		upsertReq.Config = config
	}

	organizationClient := r.ClientFactory.NewOrganizationClient(data.ProjectPublicKey.ValueString(), data.ProjectSecretKey.ValueString())
	return organizationClient.UpsertLlmConnection(ctx, upsertReq)
}

func findLlmConnectionByProvider(ctx context.Context, client langfuse.OrganizationClient, provider string) (*langfuse.LlmConnection, error) {
	page := 1
	limit := 100
	for {
		resp, err := client.ListLlmConnections(ctx, page, limit)
		if err != nil {
			return nil, err
		}
		for _, c := range resp.Data {
			if c.Provider == provider {
				conn := c
				return &conn, nil
			}
		}
		if page >= resp.Meta.TotalPages {
			return nil, nil
		}
		page++
	}
}

func llmConnectionStateFromAPI(ctx context.Context, conn *langfuse.LlmConnection, prior *llmConnectionResourceModel) (llmConnectionResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	state := *prior
	state.ID = types.StringValue(conn.ID)
	state.ProviderName = types.StringValue(conn.Provider)
	state.Adapter = types.StringValue(conn.Adapter)
	state.DisplaySecretKey = types.StringValue(conn.DisplaySecretKey)
	state.CreatedAt = types.StringValue(conn.CreatedAt)
	state.UpdatedAt = types.StringValue(conn.UpdatedAt)

	if conn.BaseURL != nil {
		state.BaseURL = types.StringValue(*conn.BaseURL)
	} else {
		state.BaseURL = types.StringNull()
	}

	customModels, d := types.ListValueFrom(ctx, types.StringType, conn.CustomModels)
	diags.Append(d...)
	if diags.HasError() {
		return llmConnectionResourceModel{}, diags
	}
	state.CustomModels = customModels

	state.WithDefaultModels = types.BoolValue(conn.WithDefaultModels)

	extraHeaderKeys, d := types.ListValueFrom(ctx, types.StringType, conn.ExtraHeaderKeys)
	diags.Append(d...)
	if diags.HasError() {
		return llmConnectionResourceModel{}, diags
	}
	state.ExtraHeaderKeys = extraHeaderKeys

	// Secrets are not returned by the API; keep prior values.
	if state.SecretKey.IsNull() {
		state.SecretKey = prior.SecretKey
	}
	if state.ExtraHeaders.IsNull() {
		state.ExtraHeaders = prior.ExtraHeaders
	}
	if state.ConfigJSON.IsNull() {
		state.ConfigJSON = prior.ConfigJSON
	}

	return state, diags
}

func emptyLlmConnectionState() *llmConnectionResourceModel {
	return &llmConnectionResourceModel{
		ID:                types.StringValue(""),
		ProjectPublicKey:  types.StringValue(""),
		ProjectSecretKey:  types.StringValue(""),
		ProviderName:      types.StringValue(""),
		Adapter:           types.StringValue(""),
		SecretKey:         types.StringValue(""),
		BaseURL:           types.StringNull(),
		CustomModels:      types.ListNull(types.StringType),
		WithDefaultModels: types.BoolNull(),
		ExtraHeaders:      types.MapNull(types.StringType),
		ConfigJSON:        types.StringNull(),
		DisplaySecretKey:  types.StringValue(""),
		ExtraHeaderKeys:   types.ListNull(types.StringType),
		CreatedAt:         types.StringValue(""),
		UpdatedAt:         types.StringValue(""),
		IgnoreDestroy:     types.BoolNull(),
	}
}
