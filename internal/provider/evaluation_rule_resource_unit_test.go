package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse/mocks"
)

func setupEvaluationRuleResource(t *testing.T, ctrl *gomock.Controller) (*evaluationRuleResource, *mocks.MockEvaluatorsClient, resschema.Schema) {
	t.Helper()

	ctx := context.Background()

	r, ok := NewEvaluationRuleResource().(*evaluationRuleResource)
	if !ok {
		t.Fatalf("NewEvaluationRuleResource did not return *evaluationRuleResource")
	}

	clientFactory := mocks.NewMockClientFactory(ctrl)

	var configureResp resource.ConfigureResponse
	r.Configure(ctx, resource.ConfigureRequest{ProviderData: clientFactory}, &configureResp)
	if configureResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Configure: %v", configureResp.Diagnostics)
	}

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Schema: %v", schemaResp.Diagnostics)
	}

	return r, clientFactory.EvaluatorsClient, schemaResp.Schema
}

func baseRuleModel() evaluationRuleResourceModel {
	return evaluationRuleResourceModel{
		ID:                   types.StringNull(),
		ProjectPublicKey:     types.StringValue("pk-test"),
		ProjectSecretKey:     types.StringValue("sk-test"),
		Name:                 types.StringValue("Production generations"),
		Enabled:              types.BoolValue(true),
		Sampling:             types.Float64Value(0.5),
		Filter:               types.ListNull(ruleFilterObjectType),
		EvaluatorAssignments: types.ListNull(evaluatorAssignmentObjectType),
	}
}

func filterModel(filterType, column, key, operator, value string, values []string) ruleFilterModel {
	m := ruleFilterModel{
		Type:     types.StringValue(filterType),
		Column:   types.StringValue(column),
		Key:      types.StringNull(),
		Operator: types.StringValue(operator),
		Value:    types.StringNull(),
		Values:   types.ListNull(types.StringType),
	}
	if key != "" {
		m.Key = types.StringValue(key)
	}
	if value != "" {
		m.Value = types.StringValue(value)
	}
	if values != nil {
		m.Values = stringSliceToList(values)
	}
	return m
}

func fullRuleModel(t *testing.T) evaluationRuleResourceModel {
	t.Helper()
	ctx := context.Background()

	m := baseRuleModel()

	filters, diags := types.ListValueFrom(ctx, ruleFilterObjectType, []ruleFilterModel{
		filterModel("string", "environment", "", "=", "production", nil),
		filterModel("stringOptions", "type", "", "any of", "", []string{"GENERATION", "AGENT"}),
		filterModel("number", "latency", "", ">", "1.5", nil),
		filterModel("boolean", "level", "", "=", "true", nil),
		filterModel("null", "parentObservationId", "", "is null", "", nil),
		filterModel("stringObject", "metadata", "tenant", "contains", "acme", nil),
	})
	if diags.HasError() {
		t.Fatalf("filters: %v", diags)
	}
	m.Filter = filters

	mapping, diags := types.ListValueFrom(ctx, variableMappingObjectType, []variableMappingModel{
		{Variable: types.StringValue("input"), Source: types.StringValue("input"), JSONPath: types.StringNull()},
		{Variable: types.StringValue("output"), Source: types.StringValue("output"), JSONPath: types.StringNull()},
	})
	if diags.HasError() {
		t.Fatalf("mapping: %v", diags)
	}
	assignments, diags := types.ListValueFrom(ctx, evaluatorAssignmentObjectType, []evaluatorAssignmentModel{
		{EvaluatorID: types.StringValue("ev-llm"), VariableMapping: mapping},
		{EvaluatorID: types.StringValue("ev-code"), VariableMapping: types.ListNull(variableMappingObjectType)},
	})
	if diags.HasError() {
		t.Fatalf("assignments: %v", diags)
	}
	m.EvaluatorAssignments = assignments

	return m
}

// ruleAPIResponse mirrors what the API returns for fullRuleModel: filters are
// echoed verbatim with native JSON types.
func ruleAPIResponse(req *langfuse.EvaluationRuleRequest) *langfuse.EvaluationRule {
	sampling := 1.0
	if req.Sampling != nil {
		sampling = *req.Sampling
	}
	// Round-trip through JSON so values take the shapes json.Unmarshal produces.
	body, _ := json.Marshal(req)
	var echoed langfuse.EvaluationRuleRequest
	_ = json.Unmarshal(body, &echoed)
	return &langfuse.EvaluationRule{
		ID:                   "rule-1",
		Name:                 echoed.Name,
		Enabled:              echoed.Enabled,
		Sampling:             sampling,
		Filter:               echoed.Filter,
		EvaluatorAssignments: echoed.EvaluatorAssignments,
	}
}

func TestEvaluationRuleResource_Create(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, client, s := setupEvaluationRuleResource(t, ctrl)

	plan := fullRuleModel(t)

	client.EXPECT().
		CreateEvaluationRule(ctx, gomock.Any()).
		DoAndReturn(func(_ context.Context, req *langfuse.EvaluationRuleRequest) (*langfuse.EvaluationRule, error) {
			body, _ := json.Marshal(req)
			var got map[string]any
			_ = json.Unmarshal(body, &got)

			filters := got["filter"].([]any)
			if len(filters) != 6 {
				t.Fatalf("expected 6 filters, got %s", body)
			}
			if filters[0].(map[string]any)["value"] != "production" {
				t.Errorf("string filter value: %s", body)
			}
			if opts, ok := filters[1].(map[string]any)["value"].([]any); !ok || len(opts) != 2 {
				t.Errorf("stringOptions filter must send an array: %s", body)
			}
			if filters[2].(map[string]any)["value"] != 1.5 {
				t.Errorf("number filter must send a JSON number: %s", body)
			}
			if filters[3].(map[string]any)["value"] != true {
				t.Errorf("boolean filter must send a JSON boolean: %s", body)
			}
			if filters[4].(map[string]any)["value"] != "" {
				t.Errorf("null filter must send an empty-string placeholder: %s", body)
			}
			if filters[5].(map[string]any)["key"] != "tenant" {
				t.Errorf("stringObject filter must send key: %s", body)
			}
			if _, ok := filters[0].(map[string]any)["key"]; ok {
				t.Errorf("key must be omitted when unset: %s", body)
			}

			assignments := got["evaluatorAssignments"].([]any)
			if len(assignments) != 2 {
				t.Fatalf("expected 2 assignments, got %s", body)
			}
			if assignments[1].(map[string]any)["variableMapping"] != nil {
				t.Errorf("omitted variable_mapping must be sent as null to inherit: %s", body)
			}
			if got["sampling"] != 0.5 || got["enabled"] != true {
				t.Errorf("unexpected sampling/enabled: %s", body)
			}
			return ruleAPIResponse(req), nil
		})

	var createResp resource.CreateResponse
	createResp.State.Schema = s
	r.Create(ctx, resource.CreateRequest{Plan: planFromModel(t, s, plan)}, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Create: %v", createResp.Diagnostics)
	}

	var model evaluationRuleResourceModel
	if diags := createResp.State.Get(ctx, &model); diags.HasError() {
		t.Fatalf("unexpected diagnostics getting state: %v", diags)
	}
	if model.ID.ValueString() != "rule-1" || model.Sampling.ValueFloat64() != 0.5 {
		t.Errorf("unexpected state: %+v", model)
	}
	if !model.Filter.Equal(plan.Filter) {
		t.Errorf("filters must round-trip unchanged\nplan:  %v\nstate: %v", plan.Filter, model.Filter)
	}
	if !model.EvaluatorAssignments.Equal(plan.EvaluatorAssignments) {
		t.Errorf("assignments must round-trip unchanged\nplan:  %v\nstate: %v", plan.EvaluatorAssignments, model.EvaluatorAssignments)
	}
}

func TestEvaluationRuleResource_Create_NoFilterStaysNull(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, client, s := setupEvaluationRuleResource(t, ctrl)

	plan := baseRuleModel()
	plan.Enabled = types.BoolValue(false)
	plan.Sampling = types.Float64Value(1)

	client.EXPECT().
		CreateEvaluationRule(ctx, gomock.Any()).
		DoAndReturn(func(_ context.Context, req *langfuse.EvaluationRuleRequest) (*langfuse.EvaluationRule, error) {
			body, _ := json.Marshal(req)
			var got map[string]any
			_ = json.Unmarshal(body, &got)
			if f, ok := got["filter"].([]any); !ok || len(f) != 0 {
				t.Errorf("filter must be sent as an empty array, got %s", body)
			}
			if a, ok := got["evaluatorAssignments"].([]any); !ok || len(a) != 0 {
				t.Errorf("evaluatorAssignments must be sent as an empty array, got %s", body)
			}
			return ruleAPIResponse(req), nil
		})

	var createResp resource.CreateResponse
	createResp.State.Schema = s
	r.Create(ctx, resource.CreateRequest{Plan: planFromModel(t, s, plan)}, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Create: %v", createResp.Diagnostics)
	}

	var model evaluationRuleResourceModel
	if diags := createResp.State.Get(ctx, &model); diags.HasError() {
		t.Fatalf("unexpected diagnostics getting state: %v", diags)
	}
	if !model.Filter.IsNull() || !model.EvaluatorAssignments.IsNull() {
		t.Errorf("omitted lists must stay null in state: %+v", model)
	}
}

func TestEvaluationRuleResource_Read_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, client, s := setupEvaluationRuleResource(t, ctrl)

	state := fullRuleModel(t)
	state.ID = types.StringValue("rule-gone")

	client.EXPECT().
		GetEvaluationRule(ctx, "rule-gone").
		Return(nil, &langfuse.APIError{StatusCode: http.StatusNotFound, Code: "resource_not_found"})

	readResp := resource.ReadResponse{State: rawFromModel(t, s, state)}
	r.Read(ctx, resource.ReadRequest{State: rawFromModel(t, s, state)}, &readResp)
	if readResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Read: %v", readResp.Diagnostics)
	}
	if !readResp.State.Raw.IsNull() {
		t.Errorf("expected resource to be removed from state on 404")
	}
}

func TestEvaluationRuleResource_Read_PreservesNumericSpelling(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, client, s := setupEvaluationRuleResource(t, ctrl)

	state := baseRuleModel()
	state.ID = types.StringValue("rule-1")
	filters, _ := types.ListValueFrom(ctx, ruleFilterObjectType, []ruleFilterModel{
		filterModel("number", "latency", "", ">=", "2.0", nil),
	})
	state.Filter = filters
	assignments, _ := types.ListValueFrom(ctx, evaluatorAssignmentObjectType, []evaluatorAssignmentModel{
		{EvaluatorID: types.StringValue("ev-1"), VariableMapping: types.ListNull(variableMappingObjectType)},
	})
	state.EvaluatorAssignments = assignments

	client.EXPECT().
		GetEvaluationRule(ctx, "rule-1").
		Return(&langfuse.EvaluationRule{
			ID: "rule-1", Name: "Production generations", Enabled: true, Sampling: 0.5,
			Filter:               []langfuse.EvaluationRuleFilter{{Type: "number", Column: "latency", Operator: ">=", Value: float64(2)}},
			EvaluatorAssignments: []langfuse.EvaluatorAssignment{{EvaluatorID: "ev-1"}},
		}, nil)

	readResp := resource.ReadResponse{State: rawFromModel(t, s, state)}
	r.Read(ctx, resource.ReadRequest{State: rawFromModel(t, s, state)}, &readResp)
	if readResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Read: %v", readResp.Diagnostics)
	}

	var model evaluationRuleResourceModel
	if diags := readResp.State.Get(ctx, &model); diags.HasError() {
		t.Fatalf("unexpected diagnostics getting state: %v", diags)
	}
	if !model.Filter.Equal(state.Filter) {
		t.Errorf("numeric spelling \"2.0\" must be preserved when the API returns 2\nstate: %v\nread:  %v", state.Filter, model.Filter)
	}
}

func TestEvaluationRuleResource_Update(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, client, s := setupEvaluationRuleResource(t, ctrl)

	state := fullRuleModel(t)
	state.ID = types.StringValue("rule-1")

	plan := fullRuleModel(t)
	plan.ID = types.StringValue("rule-1")
	plan.Enabled = types.BoolValue(false)
	plan.Sampling = types.Float64Value(1)
	plan.Filter = types.ListNull(ruleFilterObjectType)

	client.EXPECT().
		UpdateEvaluationRule(ctx, "rule-1", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, req *langfuse.EvaluationRuleRequest) (*langfuse.EvaluationRule, error) {
			if req.Enabled || *req.Sampling != 1 || len(req.Filter) != 0 || len(req.EvaluatorAssignments) != 2 {
				t.Errorf("update must send the full replacement: %+v", req)
			}
			return ruleAPIResponse(req), nil
		})

	updateResp := resource.UpdateResponse{State: rawFromModel(t, s, state)}
	r.Update(ctx, resource.UpdateRequest{Plan: planFromModel(t, s, plan), State: rawFromModel(t, s, state)}, &updateResp)
	if updateResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Update: %v", updateResp.Diagnostics)
	}

	var model evaluationRuleResourceModel
	if diags := updateResp.State.Get(ctx, &model); diags.HasError() {
		t.Fatalf("unexpected diagnostics getting state: %v", diags)
	}
	if model.Enabled.ValueBool() || !model.Filter.IsNull() || model.Sampling.ValueFloat64() != 1 {
		t.Errorf("unexpected state after update: %+v", model)
	}
}

func TestEvaluationRuleResource_Delete(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, client, s := setupEvaluationRuleResource(t, ctrl)

	state := fullRuleModel(t)
	state.ID = types.StringValue("rule-1")

	client.EXPECT().DeleteEvaluationRule(ctx, "rule-1").Return(nil)

	var deleteResp resource.DeleteResponse
	r.Delete(ctx, resource.DeleteRequest{State: rawFromModel(t, s, state)}, &deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Delete: %v", deleteResp.Diagnostics)
	}
}

func TestEvaluationRuleResource_ValidateConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	withFilter := func(f ruleFilterModel) evaluationRuleResourceModel {
		m := fullRuleModel(t)
		filters, _ := types.ListValueFrom(ctx, ruleFilterObjectType, []ruleFilterModel{f})
		m.Filter = filters
		return m
	}

	cases := map[string]struct {
		model     evaluationRuleResourceModel
		wantError bool
	}{
		"valid": {model: fullRuleModel(t)},
		"enabled without assignments": {
			model: func() evaluationRuleResourceModel {
				m := baseRuleModel()
				m.Enabled = types.BoolValue(true)
				return m
			}(),
			wantError: true,
		},
		"disabled draft without assignments": {
			model: func() evaluationRuleResourceModel {
				m := baseRuleModel()
				m.Enabled = types.BoolValue(false)
				return m
			}(),
		},
		"number filter with non-numeric value":  {model: withFilter(filterModel("number", "latency", "", ">", "fast", nil)), wantError: true},
		"boolean filter with non-boolean value": {model: withFilter(filterModel("boolean", "level", "", "=", "yes", nil)), wantError: true},
		"string filter without value":           {model: withFilter(filterModel("string", "name", "", "=", "", nil)), wantError: true},
		"string filter with values":             {model: withFilter(filterModel("string", "name", "", "=", "x", []string{"a"})), wantError: true},
		"options filter with value":             {model: withFilter(filterModel("stringOptions", "type", "", "any of", "x", nil)), wantError: true},
		"options filter without values":         {model: withFilter(filterModel("stringOptions", "type", "", "any of", "", nil)), wantError: true},
		"stringObject without key":              {model: withFilter(filterModel("stringObject", "metadata", "", "=", "x", nil)), wantError: true},
		"string filter with key":                {model: withFilter(filterModel("string", "name", "k", "=", "x", nil)), wantError: true},
		"null filter with value":                {model: withFilter(filterModel("null", "parentObservationId", "", "is null", "x", nil)), wantError: true},
		"null filter with empty-string value": {
			model: func() evaluationRuleResourceModel {
				f := filterModel("null", "parentObservationId", "", "is null", "", nil)
				f.Value = types.StringValue("")
				return withFilter(f)
			}(),
			wantError: true,
		},
		"filter with unknown value is deferred": {
			model: func() evaluationRuleResourceModel {
				f := filterModel("stringOptions", "type", "", "any of", "", nil)
				f.Value = types.StringUnknown() // would be rejected if known and non-null
				return withFilter(f)
			}(),
		},
		"categoryOptions with key and values": {model: withFilter(filterModel("categoryOptions", "metadata", "tier", "any of", "", []string{"gold"}))},
		"datetime filter":                     {model: withFilter(filterModel("datetime", "startTime", "", ">", "2026-01-01T00:00:00Z", nil))},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			r, _, s := setupEvaluationRuleResource(t, ctrl)

			var resp resource.ValidateConfigResponse
			r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: configFromModel(t, s, tc.model)}, &resp)
			if resp.Diagnostics.HasError() != tc.wantError {
				t.Errorf("wantError=%v, got diagnostics: %v", tc.wantError, resp.Diagnostics)
			}
		})
	}
}

func TestEvaluationRuleResource_ImportState(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, client, s := setupEvaluationRuleResource(t, ctrl)

	client.EXPECT().
		GetEvaluationRule(ctx, "rule-1").
		Return(&langfuse.EvaluationRule{
			ID: "rule-1", Name: "Imported", Enabled: true, Sampling: 1,
			Filter:               []langfuse.EvaluationRuleFilter{{Type: "stringOptions", Column: "type", Operator: "any of", Value: []any{"GENERATION"}}},
			EvaluatorAssignments: []langfuse.EvaluatorAssignment{{EvaluatorID: "ev-1"}},
		}, nil)

	importResp := resource.ImportStateResponse{State: tfsdk.State{Schema: s}}
	importResp.State.Raw = rawFromModel(t, s, baseRuleModel()).Raw
	r.ImportState(ctx, resource.ImportStateRequest{ID: "pk-imp:sk-imp:rule-1"}, &importResp)
	if importResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from ImportState: %v", importResp.Diagnostics)
	}

	var model evaluationRuleResourceModel
	if diags := importResp.State.Get(ctx, &model); diags.HasError() {
		t.Fatalf("unexpected diagnostics getting state: %v", diags)
	}
	if model.ID.ValueString() != "rule-1" || model.ProjectPublicKey.ValueString() != "pk-imp" || len(model.Filter.Elements()) != 1 || len(model.EvaluatorAssignments.Elements()) != 1 {
		t.Errorf("unexpected imported state: %+v", model)
	}

	var badResp resource.ImportStateResponse
	badResp.State.Schema = s
	r.ImportState(ctx, resource.ImportStateRequest{ID: "no-separators"}, &badResp)
	if !badResp.Diagnostics.HasError() {
		t.Errorf("expected error for malformed import ID")
	}
}
