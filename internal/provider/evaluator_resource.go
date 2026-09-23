package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
)

var _ resource.Resource = &evaluatorResource{}
var _ resource.ResourceWithConfigure = &evaluatorResource{}
var _ resource.ResourceWithValidateConfig = &evaluatorResource{}
var _ resource.ResourceWithImportState = &evaluatorResource{}

func NewEvaluatorResource() resource.Resource {
	return &evaluatorResource{}
}

type evaluatorResource struct {
	ClientFactory langfuse.ClientFactory
}

type evaluatorResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	ProjectPublicKey   types.String `tfsdk:"project_public_key"`
	ProjectSecretKey   types.String `tfsdk:"project_secret_key"`
	Name               types.String `tfsdk:"name"`
	Description        types.String `tfsdk:"description"`
	Type               types.String `tfsdk:"type"`
	Prompt             types.List   `tfsdk:"prompt"`
	ModelConfig        types.Object `tfsdk:"model_config"`
	VariableMapping    types.List   `tfsdk:"variable_mapping"`
	OutputDefinition   types.Object `tfsdk:"output_definition"`
	SourceCode         types.String `tfsdk:"source_code"`
	SourceCodeLanguage types.String `tfsdk:"source_code_language"`
	Version            types.Int64  `tfsdk:"version"`
	VersionID          types.String `tfsdk:"version_id"`
	Status             types.String `tfsdk:"status"`
	Variables          types.List   `tfsdk:"variables"`
}

type promptMessageModel struct {
	Role    types.String `tfsdk:"role"`
	Content types.String `tfsdk:"content"`
}

var promptMessageAttrTypes = map[string]attr.Type{
	"role":    types.StringType,
	"content": types.StringType,
}

var promptMessageObjectType = types.ObjectType{AttrTypes: promptMessageAttrTypes}

type modelConfigModel struct {
	Provider types.String `tfsdk:"provider"`
	Model    types.String `tfsdk:"model"`
}

var modelConfigAttrTypes = map[string]attr.Type{
	"provider": types.StringType,
	"model":    types.StringType,
}

type outputDefinitionModel struct {
	DataType                   types.String  `tfsdk:"data_type"`
	MinValue                   types.Float64 `tfsdk:"min_value"`
	MaxValue                   types.Float64 `tfsdk:"max_value"`
	Categories                 types.List    `tfsdk:"categories"`
	ShouldAllowMultipleMatches types.Bool    `tfsdk:"should_allow_multiple_matches"`
	ScoreReasoningInstructions types.String  `tfsdk:"score_reasoning_instructions"`
	ScoreValueInstructions     types.String  `tfsdk:"score_value_instructions"`
}

var outputDefinitionAttrTypes = map[string]attr.Type{
	"data_type":                     types.StringType,
	"min_value":                     types.Float64Type,
	"max_value":                     types.Float64Type,
	"categories":                    types.ListType{ElemType: types.StringType},
	"should_allow_multiple_matches": types.BoolType,
	"score_reasoning_instructions":  types.StringType,
	"score_value_instructions":      types.StringType,
}

func (r *evaluatorResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *evaluatorResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_evaluator"
}

func (r *evaluatorResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an evaluator in a Langfuse project using the stable evaluators API (`/api/public/v2/evaluators`). " +
			"An evaluator defines how Langfuse scores observations: either an LLM-as-a-judge prompt with a structured output, " +
			"or deterministic source code. Changing any definition attribute creates a new evaluator version on the same stable ID; " +
			"evaluation rules referencing this evaluator automatically use the latest version. Changing only `name` or `description` does not create a version.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The stable identifier of the evaluator, shared across all of its versions.",
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
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Human-readable evaluator name. Names are not identifiers and do not need to be unique.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Optional human-readable evaluator description.",
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "The evaluator type. Valid values: `llm_as_judge`, `code`. The type of an existing evaluator cannot change, so changing it recreates the resource.",
				Validators: []validator.String{
					stringvalidator.OneOf(langfuse.EvaluatorTypeLlmAsJudge, langfuse.EvaluatorTypeCode),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"prompt": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Ordered chat messages used by an `llm_as_judge` evaluator. Variables use `{{variable}}` syntax. A `system` message is only allowed as the first message. Required for `llm_as_judge`; must not be set for `code`.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"role": schema.StringAttribute{
							Required:    true,
							Description: "Role of the message. Valid values: `system`, `user`, `assistant`.",
							Validators: []validator.String{
								stringvalidator.OneOf("system", "user", "assistant"),
							},
						},
						"content": schema.StringAttribute{
							Required:    true,
							Description: "Message content. Evaluator variables use `{{variable}}` syntax.",
						},
					},
				},
			},
			"model_config": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Explicit model configuration for an `llm_as_judge` evaluator. Omit to use the project's default evaluation model. The provider must match a `provider_name` of an existing `langfuse_llm_connection` in the project.",
				Attributes: map[string]schema.Attribute{
					"provider": schema.StringAttribute{
						Required:    true,
						Description: "Provider identifier, for example `openai` or `anthropic`, matching an LLM connection in the project.",
					},
					"model": schema.StringAttribute{
						Required:    true,
						Description: "Model identifier exposed by the provider, for example `gpt-4.1-mini`.",
					},
				},
			},
			"variable_mapping": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Default prompt-variable mapping for an `llm_as_judge` evaluator. Each variable extracted from the prompt must be mapped exactly once. Evaluation rules may override this per assignment. Must not be set for `code`.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: variableMappingNestedObject(),
			},
			"output_definition": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Structured output schema returned by an `llm_as_judge` evaluator. Required for `llm_as_judge`; must not be set for `code`.",
				Attributes: map[string]schema.Attribute{
					"data_type": schema.StringAttribute{
						Required:    true,
						Description: "Score type. Valid values: `NUMERIC`, `BOOLEAN`, `CATEGORICAL`.",
						Validators: []validator.String{
							stringvalidator.OneOf("NUMERIC", "BOOLEAN", "CATEGORICAL"),
						},
					},
					"min_value": schema.Float64Attribute{
						Optional:    true,
						Description: "Optional inclusive minimum value. Only valid for `NUMERIC`.",
					},
					"max_value": schema.Float64Attribute{
						Optional:    true,
						Description: "Optional inclusive maximum value. Only valid for `NUMERIC`. Must not be lower than `min_value`.",
					},
					"categories": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Description: "Allowed category values. Required for `CATEGORICAL` (at least two unique values); must not be set otherwise.",
						Validators: []validator.List{
							listvalidator.SizeAtLeast(2),
							listvalidator.UniqueValues(),
						},
					},
					"should_allow_multiple_matches": schema.BoolAttribute{
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
						Description: "Whether a `CATEGORICAL` evaluator may return more than one category. Defaults to `false`.",
					},
					"score_reasoning_instructions": schema.StringAttribute{
						Optional:    true,
						Description: "Optional instructions for deriving the reasoning returned with the score.",
					},
					"score_value_instructions": schema.StringAttribute{
						Optional:    true,
						Description: "Optional instructions for deriving the score value.",
					},
				},
			},
			"source_code": schema.StringAttribute{
				Optional:    true,
				Description: "Source code executed for each matched observation by a `code` evaluator. Required for `code`; must not be set for `llm_as_judge`.",
			},
			"source_code_language": schema.StringAttribute{
				Optional:    true,
				Description: "Runtime language of a `code` evaluator. Valid values: `PYTHON`, `TYPESCRIPT`. Required for `code`; must not be set for `llm_as_judge`.",
				Validators: []validator.String{
					stringvalidator.OneOf("PYTHON", "TYPESCRIPT"),
				},
			},
			"version": schema.Int64Attribute{
				Computed:    true,
				Description: "The latest evaluator version number. Increases each time a definition attribute changes.",
			},
			"version_id": schema.StringAttribute{
				Computed:    true,
				Description: "The stable identifier of the latest evaluator version.",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Effective runtime status after Langfuse validates the configuration: `active` or `paused`.",
			},
			"variables": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Variables extracted from the prompt of an `llm_as_judge` evaluator. Use these to build `variable_mapping` entries.",
			},
		},
	}
}

// variableMappingNestedObject is the nested attribute object shared by the
// evaluator default mapping and the evaluation rule per-assignment mapping.
func variableMappingNestedObject() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"variable": schema.StringAttribute{
				Required:    true,
				Description: "Prompt variable name without braces. For the prompt `Judge {{input}}` use `input`.",
			},
			"source": schema.StringAttribute{
				Required:    true,
				Description: "Observation field that populates the variable. Valid values: `input`, `output`, `metadata`, `tool_calls`, `expected_output`, `experiment_item_metadata`.",
				Validators: []validator.String{
					stringvalidator.OneOf(promptVariableMappingSources...),
				},
			},
			"json_path": schema.StringAttribute{
				Optional:    true,
				Description: "Optional JSONPath selector, for example `$.answer`, applied to the source before it is inserted into the prompt. Validated by the API.",
			},
		},
	}
}

func (r *evaluatorResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data evaluatorResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Type.IsUnknown() || data.Type.IsNull() {
		return
	}

	requireSet := func(v attr.Value, name string) {
		if v.IsNull() {
			resp.Diagnostics.AddAttributeError(
				path.Root(name),
				fmt.Sprintf("%q is required for type %q", name, data.Type.ValueString()),
				fmt.Sprintf("Evaluators of type %q must set %q.", data.Type.ValueString(), name),
			)
		}
	}
	// Unknown values may still resolve to null, so only known non-null values are rejected.
	requireUnset := func(v attr.Value, name string) {
		if !v.IsNull() && !v.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				path.Root(name),
				fmt.Sprintf("%q is not allowed for type %q", name, data.Type.ValueString()),
				fmt.Sprintf("Evaluators of type %q must not set %q. Remove the attribute.", data.Type.ValueString(), name),
			)
		}
	}

	switch data.Type.ValueString() {
	case langfuse.EvaluatorTypeLlmAsJudge:
		requireSet(data.Prompt, "prompt")
		requireSet(data.OutputDefinition, "output_definition")
		requireUnset(data.SourceCode, "source_code")
		requireUnset(data.SourceCodeLanguage, "source_code_language")
	case langfuse.EvaluatorTypeCode:
		requireSet(data.SourceCode, "source_code")
		requireSet(data.SourceCodeLanguage, "source_code_language")
		requireUnset(data.Prompt, "prompt")
		requireUnset(data.ModelConfig, "model_config")
		requireUnset(data.VariableMapping, "variable_mapping")
		requireUnset(data.OutputDefinition, "output_definition")
	}

	if data.OutputDefinition.IsNull() || data.OutputDefinition.IsUnknown() {
		return
	}

	var out outputDefinitionModel
	resp.Diagnostics.Append(data.OutputDefinition.As(ctx, &out, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() || out.DataType.IsUnknown() || out.DataType.IsNull() {
		return
	}

	outPath := path.Root("output_definition")
	dataType := out.DataType.ValueString()
	isKnownSet := func(v attr.Value) bool { return !v.IsNull() && !v.IsUnknown() }

	if dataType == "CATEGORICAL" {
		if out.Categories.IsNull() {
			resp.Diagnostics.AddAttributeError(
				outPath.AtName("categories"),
				"\"categories\" is required for CATEGORICAL output",
				"A CATEGORICAL output definition must list at least two unique categories.",
			)
		}
	} else {
		if isKnownSet(out.Categories) {
			resp.Diagnostics.AddAttributeError(
				outPath.AtName("categories"),
				"\"categories\" is only valid for CATEGORICAL output",
				fmt.Sprintf("Remove categories from a %s output definition.", dataType),
			)
		}
		// The API only stores this flag for CATEGORICAL output; accepting true elsewhere
		// would be silently dropped and produce a perpetual diff.
		if isKnownSet(out.ShouldAllowMultipleMatches) && out.ShouldAllowMultipleMatches.ValueBool() {
			resp.Diagnostics.AddAttributeError(
				outPath.AtName("should_allow_multiple_matches"),
				"\"should_allow_multiple_matches\" is only valid for CATEGORICAL output",
				fmt.Sprintf("Remove should_allow_multiple_matches (or set it to false) for a %s output definition.", dataType),
			)
		}
	}

	if dataType == "NUMERIC" {
		if isKnownSet(out.MinValue) && isKnownSet(out.MaxValue) && out.MinValue.ValueFloat64() > out.MaxValue.ValueFloat64() {
			resp.Diagnostics.AddAttributeError(
				outPath.AtName("max_value"),
				"\"max_value\" must not be lower than \"min_value\"",
				fmt.Sprintf("min_value is %v but max_value is %v.", out.MinValue.ValueFloat64(), out.MaxValue.ValueFloat64()),
			)
		}
	} else if isKnownSet(out.MinValue) || isKnownSet(out.MaxValue) {
		resp.Diagnostics.AddAttributeError(
			outPath,
			"\"min_value\" and \"max_value\" are only valid for NUMERIC output",
			fmt.Sprintf("Remove min_value and max_value from a %s output definition.", dataType),
		)
	}
}

func (r *evaluatorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan evaluatorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := r.ClientFactory.NewEvaluatorsClient(plan.ProjectPublicKey.ValueString(), plan.ProjectSecretKey.ValueString())

	apiReq, diags := buildEvaluatorRequest(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ev, err := client.CreateEvaluator(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating evaluator", err.Error())
		return
	}

	state, diags := mapEvaluatorToState(ctx, ev, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *evaluatorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state evaluatorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.ClientFactory == nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	client := r.ClientFactory.NewEvaluatorsClient(state.ProjectPublicKey.ValueString(), state.ProjectSecretKey.ValueString())

	ev, err := client.GetEvaluator(ctx, state.ID.ValueString())
	if err != nil {
		if langfuse.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading evaluator", err.Error())
		return
	}

	newState, diags := mapEvaluatorToState(ctx, ev, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *evaluatorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state evaluatorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := r.ClientFactory.NewEvaluatorsClient(plan.ProjectPublicKey.ValueString(), plan.ProjectSecretKey.ValueString())

	var ev *langfuse.Evaluator
	var err error
	if evaluatorDefinitionChanged(plan, state) {
		apiReq, diags := buildEvaluatorRequest(ctx, plan)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		ev, err = client.UpdateEvaluator(ctx, state.ID.ValueString(), apiReq)
	} else {
		// Only name and/or description changed: a metadata-only update does not
		// create a new evaluator version.
		ev, err = client.UpdateEvaluatorMetadata(ctx, state.ID.ValueString(), &langfuse.UpdateEvaluatorMetadataRequest{
			Name:        plan.Name.ValueString(),
			Description: optionalString(plan.Description),
		})
	}
	if err != nil {
		resp.Diagnostics.AddError("Error updating evaluator", err.Error())
		return
	}

	newState, diags := mapEvaluatorToState(ctx, ev, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *evaluatorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state evaluatorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := r.ClientFactory.NewEvaluatorsClient(state.ProjectPublicKey.ValueString(), state.ProjectSecretKey.ValueString())

	if err := client.DeleteEvaluator(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting evaluator", err.Error())
		return
	}

	tflog.Info(ctx, "Evaluator deleted", map[string]any{"id": state.ID.ValueString()})
}

// ImportState imports an existing evaluator by its project credentials and stable ID.
// The import ID format is: <project_public_key>:<project_secret_key>:<evaluator_id>
func (r *evaluatorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	publicKey, secretKey, evaluatorID, err := parseProjectScopedImportID(req.ID, "evaluator")
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	client := r.ClientFactory.NewEvaluatorsClient(publicKey, secretKey)

	ev, err := client.GetEvaluator(ctx, evaluatorID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing evaluator", err.Error())
		return
	}

	prior := evaluatorResourceModel{
		ProjectPublicKey: types.StringValue(publicKey),
		ProjectSecretKey: types.StringValue(secretKey),
	}
	state, diags := mapEvaluatorToState(ctx, ev, prior)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// evaluatorDefinitionChanged reports whether any attribute that is part of the
// versioned evaluator definition differs between plan and state.
func evaluatorDefinitionChanged(plan, state evaluatorResourceModel) bool {
	return !plan.Type.Equal(state.Type) ||
		!plan.Prompt.Equal(state.Prompt) ||
		!plan.ModelConfig.Equal(state.ModelConfig) ||
		!plan.VariableMapping.Equal(state.VariableMapping) ||
		!plan.OutputDefinition.Equal(state.OutputDefinition) ||
		!plan.SourceCode.Equal(state.SourceCode) ||
		!plan.SourceCodeLanguage.Equal(state.SourceCodeLanguage)
}

// buildEvaluatorRequest converts the planned model into the flattened API request.
func buildEvaluatorRequest(ctx context.Context, plan evaluatorResourceModel) (*langfuse.EvaluatorRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	apiReq := &langfuse.EvaluatorRequest{
		Name:        plan.Name.ValueString(),
		Description: optionalString(plan.Description),
		Type:        plan.Type.ValueString(),
	}

	switch plan.Type.ValueString() {
	case langfuse.EvaluatorTypeLlmAsJudge:
		var messages []promptMessageModel
		diags.Append(plan.Prompt.ElementsAs(ctx, &messages, false)...)
		if diags.HasError() {
			return nil, diags
		}
		apiReq.Prompt = make([]langfuse.EvaluatorChatMessage, 0, len(messages))
		for _, m := range messages {
			apiReq.Prompt = append(apiReq.Prompt, langfuse.EvaluatorChatMessage{
				Role:    m.Role.ValueString(),
				Content: m.Content.ValueString(),
			})
		}

		if !plan.ModelConfig.IsNull() && !plan.ModelConfig.IsUnknown() {
			var mc modelConfigModel
			diags.Append(plan.ModelConfig.As(ctx, &mc, basetypes.ObjectAsOptions{})...)
			if diags.HasError() {
				return nil, diags
			}
			apiReq.ModelConfig = &langfuse.EvaluatorModelConfig{
				Provider: mc.Provider.ValueString(),
				Model:    mc.Model.ValueString(),
			}
		}

		mappings, mDiags := variableMappingsFromList(ctx, plan.VariableMapping)
		diags.Append(mDiags...)
		if diags.HasError() {
			return nil, diags
		}
		apiReq.VariableMapping = mappings

		var out outputDefinitionModel
		diags.Append(plan.OutputDefinition.As(ctx, &out, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return nil, diags
		}
		def := &langfuse.EvaluatorOutputDefinition{
			DataType:                   out.DataType.ValueString(),
			ScoreReasoningInstructions: optionalString(out.ScoreReasoningInstructions),
			ScoreValueInstructions:     optionalString(out.ScoreValueInstructions),
		}
		switch def.DataType {
		case "NUMERIC":
			if !out.MinValue.IsNull() && !out.MinValue.IsUnknown() {
				v := out.MinValue.ValueFloat64()
				def.MinValue = &v
			}
			if !out.MaxValue.IsNull() && !out.MaxValue.IsUnknown() {
				v := out.MaxValue.ValueFloat64()
				def.MaxValue = &v
			}
		case "CATEGORICAL":
			categories, cDiags := stringListToSlice(ctx, out.Categories)
			diags.Append(cDiags...)
			if diags.HasError() {
				return nil, diags
			}
			def.Categories = categories
			multi := out.ShouldAllowMultipleMatches.ValueBool()
			def.ShouldAllowMultipleMatches = &multi
		}
		apiReq.OutputDefinition = def

	case langfuse.EvaluatorTypeCode:
		apiReq.SourceCode = plan.SourceCode.ValueString()
		apiReq.SourceCodeLanguage = plan.SourceCodeLanguage.ValueString()
	}

	return apiReq, diags
}

// mapEvaluatorToState converts an API evaluator into resource state. The prior
// model supplies the project credentials, which the API never returns.
func mapEvaluatorToState(ctx context.Context, ev *langfuse.Evaluator, prior evaluatorResourceModel) (evaluatorResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	state := evaluatorResourceModel{
		ID:                 types.StringValue(ev.ID),
		ProjectPublicKey:   prior.ProjectPublicKey,
		ProjectSecretKey:   prior.ProjectSecretKey,
		Name:               types.StringValue(ev.Name),
		Description:        stringPointerToValue(ev.Description),
		Type:               types.StringValue(ev.Type),
		Prompt:             types.ListNull(promptMessageObjectType),
		ModelConfig:        types.ObjectNull(modelConfigAttrTypes),
		VariableMapping:    types.ListNull(variableMappingObjectType),
		OutputDefinition:   types.ObjectNull(outputDefinitionAttrTypes),
		SourceCode:         types.StringNull(),
		SourceCodeLanguage: types.StringNull(),
		Version:            types.Int64Value(ev.Version),
		VersionID:          types.StringValue(ev.VersionID),
		Status:             types.StringValue(ev.Status),
		Variables:          stringSliceToList(ev.Variables),
	}

	switch ev.Type {
	case langfuse.EvaluatorTypeLlmAsJudge:
		messages := make([]promptMessageModel, 0, len(ev.Prompt))
		for _, m := range ev.Prompt {
			messages = append(messages, promptMessageModel{
				Role:    types.StringValue(m.Role),
				Content: types.StringValue(m.Content),
			})
		}
		prompt, pDiags := types.ListValueFrom(ctx, promptMessageObjectType, messages)
		diags.Append(pDiags...)
		state.Prompt = prompt

		if ev.ModelConfig != nil {
			mc, mDiags := types.ObjectValueFrom(ctx, modelConfigAttrTypes, modelConfigModel{
				Provider: types.StringValue(ev.ModelConfig.Provider),
				Model:    types.StringValue(ev.ModelConfig.Model),
			})
			diags.Append(mDiags...)
			state.ModelConfig = mc
		}

		mapping, vDiags := variableMappingsToList(ctx, ev.VariableMapping)
		diags.Append(vDiags...)
		state.VariableMapping = mapping

		if ev.OutputDefinition != nil {
			out := outputDefinitionModel{
				DataType:                   types.StringValue(ev.OutputDefinition.DataType),
				MinValue:                   types.Float64Null(),
				MaxValue:                   types.Float64Null(),
				Categories:                 types.ListNull(types.StringType),
				ShouldAllowMultipleMatches: types.BoolValue(false),
				ScoreReasoningInstructions: stringPointerToValue(ev.OutputDefinition.ScoreReasoningInstructions),
				ScoreValueInstructions:     stringPointerToValue(ev.OutputDefinition.ScoreValueInstructions),
			}
			if ev.OutputDefinition.MinValue != nil {
				out.MinValue = types.Float64Value(*ev.OutputDefinition.MinValue)
			}
			if ev.OutputDefinition.MaxValue != nil {
				out.MaxValue = types.Float64Value(*ev.OutputDefinition.MaxValue)
			}
			if ev.OutputDefinition.Categories != nil {
				out.Categories = stringSliceToList(ev.OutputDefinition.Categories)
			}
			if ev.OutputDefinition.ShouldAllowMultipleMatches != nil {
				out.ShouldAllowMultipleMatches = types.BoolValue(*ev.OutputDefinition.ShouldAllowMultipleMatches)
			}
			obj, oDiags := types.ObjectValueFrom(ctx, outputDefinitionAttrTypes, out)
			diags.Append(oDiags...)
			state.OutputDefinition = obj
		}

	case langfuse.EvaluatorTypeCode:
		state.SourceCode = types.StringValue(ev.SourceCode)
		state.SourceCodeLanguage = types.StringValue(ev.SourceCodeLanguage)
	}

	return state, diags
}
