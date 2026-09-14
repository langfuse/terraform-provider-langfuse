package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
)

var _ resource.Resource = &evaluationRuleResource{}
var _ resource.ResourceWithConfigure = &evaluationRuleResource{}
var _ resource.ResourceWithValidateConfig = &evaluationRuleResource{}
var _ resource.ResourceWithImportState = &evaluationRuleResource{}

func NewEvaluationRuleResource() resource.Resource {
	return &evaluationRuleResource{}
}

type evaluationRuleResource struct {
	ClientFactory langfuse.ClientFactory
}

type evaluationRuleResourceModel struct {
	ID                   types.String  `tfsdk:"id"`
	ProjectPublicKey     types.String  `tfsdk:"project_public_key"`
	ProjectSecretKey     types.String  `tfsdk:"project_secret_key"`
	Name                 types.String  `tfsdk:"name"`
	Enabled              types.Bool    `tfsdk:"enabled"`
	Sampling             types.Float64 `tfsdk:"sampling"`
	Filter               types.List    `tfsdk:"filter"`
	EvaluatorAssignments types.List    `tfsdk:"evaluator_assignments"`
}

type ruleFilterModel struct {
	Type     types.String `tfsdk:"type"`
	Column   types.String `tfsdk:"column"`
	Key      types.String `tfsdk:"key"`
	Operator types.String `tfsdk:"operator"`
	Value    types.String `tfsdk:"value"`
	Values   types.List   `tfsdk:"values"`
}

var ruleFilterAttrTypes = map[string]attr.Type{
	"type":     types.StringType,
	"column":   types.StringType,
	"key":      types.StringType,
	"operator": types.StringType,
	"value":    types.StringType,
	"values":   types.ListType{ElemType: types.StringType},
}

var ruleFilterObjectType = types.ObjectType{AttrTypes: ruleFilterAttrTypes}

type evaluatorAssignmentModel struct {
	EvaluatorID     types.String `tfsdk:"evaluator_id"`
	VariableMapping types.List   `tfsdk:"variable_mapping"`
}

var evaluatorAssignmentAttrTypes = map[string]attr.Type{
	"evaluator_id":     types.StringType,
	"variable_mapping": types.ListType{ElemType: variableMappingObjectType},
}

var evaluatorAssignmentObjectType = types.ObjectType{AttrTypes: evaluatorAssignmentAttrTypes}

// Filter type discriminators accepted by the API, grouped by the shape of value they take.
const (
	filterTypeDatetime        = "datetime"
	filterTypeString          = "string"
	filterTypeNumber          = "number"
	filterTypeStringOptions   = "stringOptions"
	filterTypeCategoryOptions = "categoryOptions"
	filterTypeArrayOptions    = "arrayOptions"
	filterTypeStringObject    = "stringObject"
	filterTypeNumberObject    = "numberObject"
	filterTypeBoolean         = "boolean"
	filterTypeNull            = "null"
)

var ruleFilterTypes = []string{
	filterTypeDatetime, filterTypeString, filterTypeNumber, filterTypeStringOptions, filterTypeCategoryOptions,
	filterTypeArrayOptions, filterTypeStringObject, filterTypeNumberObject, filterTypeBoolean, filterTypeNull,
}

func filterTypeUsesValues(t string) bool {
	return t == filterTypeStringOptions || t == filterTypeCategoryOptions || t == filterTypeArrayOptions
}

func filterTypeRequiresKey(t string) bool {
	return t == filterTypeStringObject || t == filterTypeNumberObject || t == filterTypeCategoryOptions
}

func (r *evaluationRuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *evaluationRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_evaluation_rule"
}

func (r *evaluationRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an evaluation rule in a Langfuse project using the stable evaluation-rules API (`/api/public/v2/evaluation-rules`). " +
			"A rule defines which incoming observations are evaluated, by which evaluators, how often, and how prompt variables are populated. " +
			"Rules always use the latest version of each assigned evaluator.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The stable identifier of the evaluation rule.",
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
				Description: "Human-readable rule name. Names are not identifiers and do not need to be unique.",
			},
			"enabled": schema.BoolAttribute{
				Required:    true,
				Description: "Whether live execution is enabled. An enabled rule requires at least one evaluator assignment.",
			},
			"sampling": schema.Float64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     float64default.StaticFloat64(1),
				Description: "Fraction of matching observations to evaluate, between `0` and `1`. Defaults to `1`, which evaluates every match.",
				Validators: []validator.Float64{
					float64validator.Between(0, 1),
				},
			},
			"filter": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Conditions used to select observations. Omit to match every incoming observation. Scalar conditions use `value`; option conditions (`stringOptions`, `arrayOptions`, `categoryOptions`) use `values`.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Required:    true,
							Description: "Filter type. Valid values: `datetime`, `string`, `number`, `stringOptions`, `categoryOptions`, `arrayOptions`, `stringObject`, `numberObject`, `boolean`, `null`.",
							Validators: []validator.String{
								stringvalidator.OneOf(ruleFilterTypes...),
							},
						},
						"column": schema.StringAttribute{
							Required: true,
							Description: "Observation column to filter on. The API accepts a fixed set of column/type pairs, for example " +
								"`type`, `name`, `level`, `traceName`, `experimentId`, `datasetId` (dataset ID, not name) with `stringOptions`; " +
								"`version`, `release`, `statusMessage`, `providedModelName`, `promptName`, `userId`, `sessionId`, `experimentName` with `string`; " +
								"`promptVersion`, `toolCalls` with `number`; `tags`, `calledToolNames` with `arrayOptions`; `metadata` with `stringObject`; " +
								"`isRootObservation`, `isExperimentItemRootSpan` with `boolean`; `parentObservationId` with `null`.",
						},
						"key": schema.StringAttribute{
							Optional:    true,
							Description: "Key inside an object-valued column. Required for `stringObject`, `numberObject` and `categoryOptions`.",
						},
						"operator": schema.StringAttribute{
							Required:    true,
							Description: "Comparison operator, for example `=`, `contains`, `>`, `any of`, `none of`, `all of`, `<>`, `is null`, `is not null`. Valid operators depend on `type`.",
						},
						"value": schema.StringAttribute{
							Optional:    true,
							Description: "Scalar comparison value as a string. Numbers (`number`, `numberObject`) and booleans (`boolean`) are parsed from this string. Datetimes are RFC 3339. Not used for option types or `null`.",
						},
						"values": schema.ListAttribute{
							Optional:    true,
							ElementType: types.StringType,
							Description: "Option values for `stringOptions`, `arrayOptions` and `categoryOptions`. Not used for other types.",
							Validators: []validator.List{
								listvalidator.SizeAtLeast(1),
							},
						},
					},
				},
			},
			"evaluator_assignments": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Evaluators attached to this rule. Required (at least one) when `enabled` is `true`; a disabled rule may omit it as a draft.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"evaluator_id": schema.StringAttribute{
							Required:    true,
							Description: "Stable evaluator identifier. The rule automatically uses that evaluator's latest version.",
						},
						"variable_mapping": schema.ListNestedAttribute{
							Optional:    true,
							Description: "Rule-specific prompt-variable mapping overriding the evaluator's default mapping. Omit to inherit the default. Code evaluators use a fixed runtime mapping and must omit it.",
							Validators: []validator.List{
								listvalidator.SizeAtLeast(1),
							},
							NestedObject: variableMappingNestedObject(),
						},
					},
				},
			},
		},
	}
}

func (r *evaluationRuleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data evaluationRuleResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !data.Enabled.IsUnknown() && data.Enabled.ValueBool() && !data.EvaluatorAssignments.IsUnknown() {
		if data.EvaluatorAssignments.IsNull() || len(data.EvaluatorAssignments.Elements()) == 0 {
			resp.Diagnostics.AddAttributeError(
				path.Root("evaluator_assignments"),
				"Enabled rules require at least one evaluator assignment",
				"Set enabled = false to keep the rule as a draft, or add at least one evaluator_assignments entry.",
			)
		}
	}

	if data.Filter.IsNull() || data.Filter.IsUnknown() {
		return
	}

	var filters []ruleFilterModel
	resp.Diagnostics.Append(data.Filter.ElementsAs(ctx, &filters, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	for i, f := range filters {
		// Shape validation needs every field to be known; unknown values may still
		// resolve to null, so defer to apply time in that case.
		if f.Type.IsUnknown() || f.Type.IsNull() || f.Key.IsUnknown() || f.Value.IsUnknown() || f.Values.IsUnknown() {
			continue
		}
		p := path.Root("filter").AtListIndex(i)
		if _, err := ruleFilterToAPI(ctx, f); err != nil {
			resp.Diagnostics.AddAttributeError(p, "Invalid filter", err.Error())
		}
	}
}

func (r *evaluationRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan evaluationRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := r.ClientFactory.NewEvaluatorsClient(plan.ProjectPublicKey.ValueString(), plan.ProjectSecretKey.ValueString())

	apiReq, diags := buildEvaluationRuleRequest(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := client.CreateEvaluationRule(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating evaluation rule", err.Error())
		return
	}

	state, diags := mapEvaluationRuleToState(ctx, rule, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *evaluationRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state evaluationRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.ClientFactory == nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	client := r.ClientFactory.NewEvaluatorsClient(state.ProjectPublicKey.ValueString(), state.ProjectSecretKey.ValueString())

	rule, err := client.GetEvaluationRule(ctx, state.ID.ValueString())
	if err != nil {
		if langfuse.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading evaluation rule", err.Error())
		return
	}

	newState, diags := mapEvaluationRuleToState(ctx, rule, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *evaluationRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state evaluationRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := r.ClientFactory.NewEvaluatorsClient(plan.ProjectPublicKey.ValueString(), plan.ProjectSecretKey.ValueString())

	apiReq, diags := buildEvaluationRuleRequest(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := client.UpdateEvaluationRule(ctx, state.ID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating evaluation rule", err.Error())
		return
	}

	newState, diags := mapEvaluationRuleToState(ctx, rule, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *evaluationRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state evaluationRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := r.ClientFactory.NewEvaluatorsClient(state.ProjectPublicKey.ValueString(), state.ProjectSecretKey.ValueString())

	if err := client.DeleteEvaluationRule(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting evaluation rule", err.Error())
		return
	}

	tflog.Info(ctx, "Evaluation rule deleted", map[string]any{"id": state.ID.ValueString()})
}

// ImportState imports an existing evaluation rule by its project credentials and stable ID.
// The import ID format is: <project_public_key>:<project_secret_key>:<evaluation_rule_id>
func (r *evaluationRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	publicKey, secretKey, ruleID, err := parseProjectScopedImportID(req.ID, "evaluation_rule")
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	client := r.ClientFactory.NewEvaluatorsClient(publicKey, secretKey)

	rule, err := client.GetEvaluationRule(ctx, ruleID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing evaluation rule", err.Error())
		return
	}

	prior := evaluationRuleResourceModel{
		ProjectPublicKey:     types.StringValue(publicKey),
		ProjectSecretKey:     types.StringValue(secretKey),
		Filter:               types.ListNull(ruleFilterObjectType),
		EvaluatorAssignments: types.ListNull(evaluatorAssignmentObjectType),
	}
	state, diags := mapEvaluationRuleToState(ctx, rule, prior)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// buildEvaluationRuleRequest converts the planned model into the API request.
// Filter and assignments are always non-nil so they serialise as JSON arrays.
func buildEvaluationRuleRequest(ctx context.Context, plan evaluationRuleResourceModel) (*langfuse.EvaluationRuleRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	apiReq := &langfuse.EvaluationRuleRequest{
		Name:                 plan.Name.ValueString(),
		Enabled:              plan.Enabled.ValueBool(),
		Filter:               make([]langfuse.EvaluationRuleFilter, 0),
		EvaluatorAssignments: make([]langfuse.EvaluatorAssignment, 0),
	}

	if !plan.Sampling.IsNull() && !plan.Sampling.IsUnknown() {
		s := plan.Sampling.ValueFloat64()
		apiReq.Sampling = &s
	}

	if !plan.Filter.IsNull() && !plan.Filter.IsUnknown() {
		var filters []ruleFilterModel
		diags.Append(plan.Filter.ElementsAs(ctx, &filters, false)...)
		if diags.HasError() {
			return nil, diags
		}
		for i, f := range filters {
			apiFilter, err := ruleFilterToAPI(ctx, f)
			if err != nil {
				diags.AddAttributeError(path.Root("filter").AtListIndex(i), "Invalid filter", err.Error())
				return nil, diags
			}
			apiReq.Filter = append(apiReq.Filter, apiFilter)
		}
	}

	if !plan.EvaluatorAssignments.IsNull() && !plan.EvaluatorAssignments.IsUnknown() {
		var assignments []evaluatorAssignmentModel
		diags.Append(plan.EvaluatorAssignments.ElementsAs(ctx, &assignments, false)...)
		if diags.HasError() {
			return nil, diags
		}
		for _, a := range assignments {
			mapping, mDiags := variableMappingsFromList(ctx, a.VariableMapping)
			diags.Append(mDiags...)
			if diags.HasError() {
				return nil, diags
			}
			apiReq.EvaluatorAssignments = append(apiReq.EvaluatorAssignments, langfuse.EvaluatorAssignment{
				EvaluatorID:     a.EvaluatorID.ValueString(),
				VariableMapping: mapping,
			})
		}
	}

	return apiReq, diags
}

// ruleFilterToAPI converts one filter into its typed API shape, validating
// that the value attributes match the filter type.
func ruleFilterToAPI(ctx context.Context, f ruleFilterModel) (langfuse.EvaluationRuleFilter, error) {
	filterType := f.Type.ValueString()
	apiFilter := langfuse.EvaluationRuleFilter{
		Type:     filterType,
		Column:   f.Column.ValueString(),
		Operator: f.Operator.ValueString(),
	}

	hasValue := !f.Value.IsNull()
	hasValues := !f.Values.IsNull()
	hasKey := !f.Key.IsNull()

	if filterTypeRequiresKey(filterType) {
		if !hasKey {
			return apiFilter, fmt.Errorf("filter type %q requires \"key\"", filterType)
		}
		apiFilter.Key = f.Key.ValueString()
	} else if hasKey {
		return apiFilter, fmt.Errorf("filter type %q does not accept \"key\"", filterType)
	}

	if filterTypeUsesValues(filterType) {
		if hasValue {
			return apiFilter, fmt.Errorf("filter type %q uses \"values\", not \"value\"", filterType)
		}
		if !hasValues {
			return apiFilter, fmt.Errorf("filter type %q requires \"values\"", filterType)
		}
		if f.Values.IsUnknown() {
			return apiFilter, nil
		}
		values, diags := stringListToSlice(ctx, f.Values)
		if diags.HasError() {
			return apiFilter, fmt.Errorf("invalid \"values\": %s", diags.Errors()[0].Detail())
		}
		apiFilter.Value = values
		return apiFilter, nil
	}

	if hasValues {
		return apiFilter, fmt.Errorf("filter type %q uses \"value\", not \"values\"", filterType)
	}

	if filterType == filterTypeNull {
		// The API requires an empty-string placeholder, which is always synthesised here
		// and never read back into state, so any configured value (even "") would drift.
		if hasValue {
			return apiFilter, fmt.Errorf("filter type %q does not accept a \"value\"; omit it", filterType)
		}
		apiFilter.Value = ""
		return apiFilter, nil
	}

	if !hasValue {
		return apiFilter, fmt.Errorf("filter type %q requires \"value\"", filterType)
	}
	if f.Value.IsUnknown() {
		return apiFilter, nil
	}
	raw := f.Value.ValueString()

	switch filterType {
	case filterTypeNumber, filterTypeNumberObject:
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return apiFilter, fmt.Errorf("filter type %q requires a numeric \"value\", got %q", filterType, raw)
		}
		apiFilter.Value = n
	case filterTypeBoolean:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return apiFilter, fmt.Errorf("filter type %q requires a boolean \"value\" (true or false), got %q", filterType, raw)
		}
		apiFilter.Value = b
	default:
		apiFilter.Value = raw
	}

	return apiFilter, nil
}

// ruleFilterFromAPI converts a stored filter back into the resource model. The
// prior model, when present, preserves the user's spelling of numeric values
// (for example "1.0") when it denotes the same number the API returned.
func ruleFilterFromAPI(f langfuse.EvaluationRuleFilter, prior *ruleFilterModel) ruleFilterModel {
	model := ruleFilterModel{
		Type:     types.StringValue(f.Type),
		Column:   types.StringValue(f.Column),
		Key:      types.StringNull(),
		Operator: types.StringValue(f.Operator),
		Value:    types.StringNull(),
		Values:   types.ListNull(types.StringType),
	}
	if f.Key != "" {
		model.Key = types.StringValue(f.Key)
	}

	switch v := f.Value.(type) {
	case string:
		if f.Type != filterTypeNull {
			model.Value = types.StringValue(v)
		}
	case float64:
		formatted := strconv.FormatFloat(v, 'f', -1, 64)
		if prior != nil && !prior.Value.IsNull() && !prior.Value.IsUnknown() {
			if p, err := strconv.ParseFloat(prior.Value.ValueString(), 64); err == nil && p == v {
				formatted = prior.Value.ValueString()
			}
		}
		model.Value = types.StringValue(formatted)
	case bool:
		model.Value = types.StringValue(strconv.FormatBool(v))
	case []any:
		values := make([]string, 0, len(v))
		for _, item := range v {
			values = append(values, fmt.Sprint(item))
		}
		model.Values = stringSliceToList(values)
	}

	return model
}

// mapEvaluationRuleToState converts an API rule into resource state. The prior
// model supplies the project credentials and keeps omitted lists null.
func mapEvaluationRuleToState(ctx context.Context, rule *langfuse.EvaluationRule, prior evaluationRuleResourceModel) (evaluationRuleResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	state := evaluationRuleResourceModel{
		ID:                   types.StringValue(rule.ID),
		ProjectPublicKey:     prior.ProjectPublicKey,
		ProjectSecretKey:     prior.ProjectSecretKey,
		Name:                 types.StringValue(rule.Name),
		Enabled:              types.BoolValue(rule.Enabled),
		Sampling:             types.Float64Value(rule.Sampling),
		Filter:               types.ListNull(ruleFilterObjectType),
		EvaluatorAssignments: types.ListNull(evaluatorAssignmentObjectType),
	}

	if len(rule.Filter) > 0 {
		var priorFilters []ruleFilterModel
		if !prior.Filter.IsNull() && !prior.Filter.IsUnknown() {
			diags.Append(prior.Filter.ElementsAs(ctx, &priorFilters, false)...)
		}
		filters := make([]ruleFilterModel, 0, len(rule.Filter))
		for i, f := range rule.Filter {
			var priorFilter *ruleFilterModel
			if i < len(priorFilters) {
				priorFilter = &priorFilters[i]
			}
			filters = append(filters, ruleFilterFromAPI(f, priorFilter))
		}
		list, lDiags := types.ListValueFrom(ctx, ruleFilterObjectType, filters)
		diags.Append(lDiags...)
		state.Filter = list
	} else if !prior.Filter.IsNull() && !prior.Filter.IsUnknown() && len(prior.Filter.Elements()) == 0 {
		state.Filter = types.ListValueMust(ruleFilterObjectType, []attr.Value{})
	}

	if len(rule.EvaluatorAssignments) > 0 {
		assignments := make([]evaluatorAssignmentModel, 0, len(rule.EvaluatorAssignments))
		for _, a := range rule.EvaluatorAssignments {
			mapping, mDiags := variableMappingsToList(ctx, a.VariableMapping)
			diags.Append(mDiags...)
			assignments = append(assignments, evaluatorAssignmentModel{
				EvaluatorID:     types.StringValue(a.EvaluatorID),
				VariableMapping: mapping,
			})
		}
		list, lDiags := types.ListValueFrom(ctx, evaluatorAssignmentObjectType, assignments)
		diags.Append(lDiags...)
		state.EvaluatorAssignments = list
	} else if !prior.EvaluatorAssignments.IsNull() && !prior.EvaluatorAssignments.IsUnknown() && len(prior.EvaluatorAssignments.Elements()) == 0 {
		state.EvaluatorAssignments = types.ListValueMust(evaluatorAssignmentObjectType, []attr.Value{})
	}

	return state, diags
}
