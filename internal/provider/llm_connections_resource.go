package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
)

var _ resource.Resource = &llmConnectionsResource{}
var _ resource.ResourceWithConfigValidators = &llmConnectionsResource{}

// llmConnectionsPageSize is the number of results to request per page when listing
// LLM connections. 100 is the practical maximum to minimise round-trips.
const llmConnectionsPageSize = 100

func NewLlmConnectionResource() resource.Resource {
	return &llmConnectionsResource{}
}

type llmConnectionsResourceModel struct {
	ID                types.String `tfsdk:"id"`
	ProjectPublicKey  types.String `tfsdk:"project_public_key"`
	ProjectSecretKey  types.String `tfsdk:"project_secret_key"`
	ConnectionName    types.String `tfsdk:"connection_name"`
	Adapter           types.String `tfsdk:"adapter"`
	SecretKey         types.String `tfsdk:"secret_key"`
	BaseURL           types.String `tfsdk:"base_url"`
	CustomModels      types.List   `tfsdk:"custom_models"`
	ExtraHeaders      types.Map    `tfsdk:"extra_headers"`
	WithDefaultModels types.Bool   `tfsdk:"with_default_models"`
	Config            types.String `tfsdk:"config"`
}

type llmConnectionsResource struct {
	ClientFactory langfuse.ClientFactory
}

func (r *llmConnectionsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *llmConnectionsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_llm_connection"
}

func (r *llmConnectionsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an LLM connection in a Langfuse project. Note: the Langfuse API does not provide a delete endpoint for LLM connections. Destroying this resource only removes it from Terraform state; the connection will remain in Langfuse.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique identifier of the LLM connection (set to the provider value).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_public_key": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "The project public key used to authenticate API calls.",
			},
			"project_secret_key": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "The project secret key used to authenticate API calls.",
			},
			"connection_name": schema.StringAttribute{
				Required:    true,
				Description: "The unique name identifying this LLM connection within the project. Changing this value destroys and recreates the resource, as the connection_name is the upsert key.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"adapter": schema.StringAttribute{
				Required:    true,
				Description: "The LLM service type. Valid values: anthropic, openai, azure, bedrock, google-vertex-ai, google-ai-studio.",
				Validators: []validator.String{
					stringvalidator.OneOf("anthropic", "openai", "azure", "bedrock", "google-vertex-ai", "google-ai-studio"),
				},
			},
			"secret_key": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "The API authentication key for the LLM provider.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"base_url": schema.StringAttribute{
				Optional:    true,
				Description: "Optional base URL override for the LLM provider API.",
			},
			"custom_models": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Optional list of custom model identifiers.",
			},
			"extra_headers": schema.MapAttribute{
				Optional:    true,
				Sensitive:   true,
				ElementType: types.StringType,
				Description: "Optional map of additional HTTP headers for LLM API requests.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
			},
			"with_default_models": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether to include default models. Defaults to true if not set.",
			},
			"config": schema.StringAttribute{
				Optional:    true,
				Description: "Adapter-specific configuration as a JSON string.",
			},
		},
	}
}

type llmConnectionConfigValidator struct{}

func (v llmConnectionConfigValidator) Description(ctx context.Context) string {
	return "Validates adapter-specific config requirements"
}

func (v llmConnectionConfigValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v llmConnectionConfigValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data llmConnectionsResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	adapter := data.Adapter.ValueString()
	configStr := data.Config

	switch adapter {
	case "bedrock":
		if configStr.IsNull() || configStr.IsUnknown() || configStr.ValueString() == "" {
			resp.Diagnostics.AddError(
				"Config required for bedrock adapter",
				"The bedrock adapter requires a config JSON string containing a \"region\" key.",
			)
			return
		}
		var configMap map[string]any
		if err := json.Unmarshal([]byte(configStr.ValueString()), &configMap); err != nil {
			resp.Diagnostics.AddError(
				"Invalid config JSON for bedrock adapter",
				fmt.Sprintf("Failed to parse config as JSON: %s", err.Error()),
			)
			return
		}
		if _, ok := configMap["region"]; !ok {
			resp.Diagnostics.AddError(
				"Missing \"region\" in bedrock config",
				"The bedrock adapter config must contain a \"region\" key.",
			)
		}

	case "google-vertex-ai":
		if !configStr.IsNull() && !configStr.IsUnknown() && configStr.ValueString() != "" {
			var configMap map[string]any
			if err := json.Unmarshal([]byte(configStr.ValueString()), &configMap); err != nil {
				resp.Diagnostics.AddError(
					"Invalid config JSON for google-vertex-ai adapter",
					fmt.Sprintf("Failed to parse config as JSON: %s", err.Error()),
				)
				return
			}
			if _, ok := configMap["location"]; !ok {
				resp.Diagnostics.AddError(
					"Missing \"location\" in google-vertex-ai config",
					"When config is provided for the google-vertex-ai adapter, it must contain a \"location\" key.",
				)
			}
		}

	default:
		if !configStr.IsNull() && !configStr.IsUnknown() && configStr.ValueString() != "" {
			resp.Diagnostics.AddError(
				"Config must be null for this adapter",
				fmt.Sprintf("The %q adapter does not support a config value. Set config to null or remove it.", adapter),
			)
		}
	}
}

func (r *llmConnectionsResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{llmConnectionConfigValidator{}}
}

// mapResponseToState maps an LlmConnection API response to the resource model,
// preserving write-only fields (secretKey, extraHeaders) from the provided prior state.
func mapResponseToState(conn *langfuse.LlmConnection, secretKey types.String, extraHeaders types.Map, projectPublicKey, projectSecretKey types.String) (llmConnectionsResourceModel, error) {
	state := llmConnectionsResourceModel{
		ID:                types.StringValue(conn.Provider),
		ConnectionName:    types.StringValue(conn.Provider),
		Adapter:           types.StringValue(conn.Adapter),
		WithDefaultModels: types.BoolValue(conn.WithDefaultModels),
		SecretKey:         secretKey,
		ExtraHeaders:      extraHeaders,
		ProjectPublicKey:  projectPublicKey,
		ProjectSecretKey:  projectSecretKey,
	}

	if conn.BaseURL != "" {
		state.BaseURL = types.StringValue(conn.BaseURL)
	} else {
		state.BaseURL = types.StringNull()
	}

	if len(conn.CustomModels) > 0 {
		elems := make([]attr.Value, len(conn.CustomModels))
		for i, m := range conn.CustomModels {
			elems[i] = types.StringValue(m)
		}
		state.CustomModels = types.ListValueMust(types.StringType, elems)
	} else {
		state.CustomModels = types.ListNull(types.StringType)
	}

	if len(conn.Config) > 0 {
		configBytes, err := json.Marshal(conn.Config)
		if err != nil {
			return state, fmt.Errorf("failed to marshal config to JSON: %w", err)
		}
		state.Config = types.StringValue(string(configBytes))
	} else {
		state.Config = types.StringNull()
	}

	return state, nil
}

// buildUpsertRequest constructs an UpsertLlmConnectionRequest from a resource model plan.
func buildUpsertRequest(plan llmConnectionsResourceModel) (*langfuse.UpsertLlmConnectionRequest, error) {
	upsertReq := langfuse.UpsertLlmConnectionRequest{
		Adapter:   plan.Adapter.ValueString(),
		Provider:  plan.ConnectionName.ValueString(),
		SecretKey: plan.SecretKey.ValueString(),
	}

	if !plan.BaseURL.IsNull() && !plan.BaseURL.IsUnknown() {
		upsertReq.BaseURL = plan.BaseURL.ValueString()
	}

	if !plan.CustomModels.IsNull() && !plan.CustomModels.IsUnknown() {
		var models []string
		for _, elem := range plan.CustomModels.Elements() {
			if sv, ok := elem.(types.String); ok {
				models = append(models, sv.ValueString())
			}
		}
		upsertReq.CustomModels = models
	}

	if !plan.ExtraHeaders.IsNull() && !plan.ExtraHeaders.IsUnknown() {
		headers := make(map[string]string)
		for k, v := range plan.ExtraHeaders.Elements() {
			if sv, ok := v.(types.String); ok {
				headers[k] = sv.ValueString()
			}
		}
		upsertReq.ExtraHeaders = headers
	}

	if !plan.WithDefaultModels.IsNull() && !plan.WithDefaultModels.IsUnknown() {
		v := plan.WithDefaultModels.ValueBool()
		upsertReq.WithDefaultModels = &v
	}

	if !plan.Config.IsNull() && !plan.Config.IsUnknown() && plan.Config.ValueString() != "" {
		var configMap map[string]any
		if err := json.Unmarshal([]byte(plan.Config.ValueString()), &configMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config JSON: %w", err)
		}
		upsertReq.Config = configMap
	}

	return &upsertReq, nil
}

func (r *llmConnectionsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan llmConnectionsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := r.ClientFactory.NewLlmConnectionsClient(plan.ProjectPublicKey.ValueString(), plan.ProjectSecretKey.ValueString())

	upsertReq, err := buildUpsertRequest(plan)
	if err != nil {
		resp.Diagnostics.AddError("Error building upsert request", err.Error())
		return
	}

	conn, err := client.UpsertLlmConnection(ctx, upsertReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating LLM connection", err.Error())
		return
	}

	state, err := mapResponseToState(conn, plan.SecretKey, plan.ExtraHeaders, plan.ProjectPublicKey, plan.ProjectSecretKey)
	if err != nil {
		resp.Diagnostics.AddError("Error mapping LLM connection response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *llmConnectionsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state llmConnectionsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := r.ClientFactory.NewLlmConnectionsClient(state.ProjectPublicKey.ValueString(), state.ProjectSecretKey.ValueString())

	var found *langfuse.LlmConnection
	page := 1
	pageSize := llmConnectionsPageSize
	for {
		listResp, err := client.ListLlmConnections(ctx, &page, &pageSize)
		if err != nil {
			resp.Diagnostics.AddError("Error listing LLM connections", err.Error())
			return
		}
		for i := range listResp.Data {
			if listResp.Data[i].Provider == state.ConnectionName.ValueString() {
				found = &listResp.Data[i]
				break
			}
		}
		if found != nil || page >= listResp.Meta.TotalPages {
			break
		}
		page++
	}

	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	newState, err := mapResponseToState(found, state.SecretKey, state.ExtraHeaders, state.ProjectPublicKey, state.ProjectSecretKey)
	if err != nil {
		resp.Diagnostics.AddError("Error mapping LLM connection response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *llmConnectionsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan llmConnectionsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := r.ClientFactory.NewLlmConnectionsClient(plan.ProjectPublicKey.ValueString(), plan.ProjectSecretKey.ValueString())

	upsertReq, err := buildUpsertRequest(plan)
	if err != nil {
		resp.Diagnostics.AddError("Error building upsert request", err.Error())
		return
	}

	conn, err := client.UpsertLlmConnection(ctx, upsertReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating LLM connection", err.Error())
		return
	}

	state, err := mapResponseToState(conn, plan.SecretKey, plan.ExtraHeaders, plan.ProjectPublicKey, plan.ProjectSecretKey)
	if err != nil {
		resp.Diagnostics.AddError("Error mapping LLM connection response", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *llmConnectionsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Info(ctx, "LLM connection removed from Terraform state. The connection remains in Langfuse but is no longer managed by Terraform.")
}
