package provider

import (
	"context"
	"encoding/json"
	"errors"
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

func setupEvaluatorResource(t *testing.T, ctrl *gomock.Controller) (*evaluatorResource, *mocks.MockEvaluatorsClient, resschema.Schema) {
	t.Helper()

	ctx := context.Background()

	r, ok := NewEvaluatorResource().(*evaluatorResource)
	if !ok {
		t.Fatalf("NewEvaluatorResource did not return *evaluatorResource")
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

// rawFromModel encodes a fully-typed model through the schema so tests can
// build plan, state and config values without hand-writing tftypes objects.
func rawFromModel(t *testing.T, s resschema.Schema, model any) tfsdk.State {
	t.Helper()
	st := tfsdk.State{Schema: s}
	if diags := st.Set(context.Background(), model); diags.HasError() {
		t.Fatalf("unexpected diagnostics encoding model: %v", diags)
	}
	return st
}

func planFromModel(t *testing.T, s resschema.Schema, model any) tfsdk.Plan {
	t.Helper()
	st := rawFromModel(t, s, model)
	return tfsdk.Plan{Raw: st.Raw, Schema: s}
}

func configFromModel(t *testing.T, s resschema.Schema, model any) tfsdk.Config {
	t.Helper()
	st := rawFromModel(t, s, model)
	return tfsdk.Config{Raw: st.Raw, Schema: s}
}

// baseEvaluatorModel returns a model with every attribute set to a typed null,
// ready to be customised by a test.
func baseEvaluatorModel() evaluatorResourceModel {
	return evaluatorResourceModel{
		ID:                 types.StringNull(),
		ProjectPublicKey:   types.StringValue("pk-test"),
		ProjectSecretKey:   types.StringValue("sk-test"),
		Name:               types.StringValue("Helpfulness"),
		Description:        types.StringNull(),
		Type:               types.StringNull(),
		Prompt:             types.ListNull(promptMessageObjectType),
		ModelConfig:        types.ObjectNull(modelConfigAttrTypes),
		VariableMapping:    types.ListNull(variableMappingObjectType),
		OutputDefinition:   types.ObjectNull(outputDefinitionAttrTypes),
		SourceCode:         types.StringNull(),
		SourceCodeLanguage: types.StringNull(),
		Version:            types.Int64Null(),
		VersionID:          types.StringNull(),
		Status:             types.StringNull(),
		Variables:          types.ListNull(types.StringType),
	}
}

func llmJudgeEvaluatorModel(t *testing.T) evaluatorResourceModel {
	t.Helper()
	ctx := context.Background()

	m := baseEvaluatorModel()
	m.Type = types.StringValue(langfuse.EvaluatorTypeLlmAsJudge)
	m.Description = types.StringValue("Judges helpfulness")

	prompt, diags := types.ListValueFrom(ctx, promptMessageObjectType, []promptMessageModel{
		{Role: types.StringValue("system"), Content: types.StringValue("You are a strict judge.")},
		{Role: types.StringValue("user"), Content: types.StringValue("Rate {{input}} against {{output}}")},
	})
	if diags.HasError() {
		t.Fatalf("prompt: %v", diags)
	}
	m.Prompt = prompt

	mc, diags := types.ObjectValueFrom(ctx, modelConfigAttrTypes, modelConfigModel{
		Provider: types.StringValue("openai"),
		Model:    types.StringValue("gpt-4.1-mini"),
	})
	if diags.HasError() {
		t.Fatalf("model config: %v", diags)
	}
	m.ModelConfig = mc

	mapping, diags := types.ListValueFrom(ctx, variableMappingObjectType, []variableMappingModel{
		{Variable: types.StringValue("input"), Source: types.StringValue("input"), JSONPath: types.StringNull()},
		{Variable: types.StringValue("output"), Source: types.StringValue("metadata"), JSONPath: types.StringValue("$.answer")},
	})
	if diags.HasError() {
		t.Fatalf("mapping: %v", diags)
	}
	m.VariableMapping = mapping

	out, diags := types.ObjectValueFrom(ctx, outputDefinitionAttrTypes, outputDefinitionModel{
		DataType:                   types.StringValue("NUMERIC"),
		MinValue:                   types.Float64Value(0),
		MaxValue:                   types.Float64Value(1),
		Categories:                 types.ListNull(types.StringType),
		ShouldAllowMultipleMatches: types.BoolValue(false),
		ScoreReasoningInstructions: types.StringValue("Explain briefly."),
		ScoreValueInstructions:     types.StringNull(),
	})
	if diags.HasError() {
		t.Fatalf("output definition: %v", diags)
	}
	m.OutputDefinition = out

	return m
}

func codeEvaluatorModel() evaluatorResourceModel {
	m := baseEvaluatorModel()
	m.Name = types.StringValue("Length check")
	m.Type = types.StringValue(langfuse.EvaluatorTypeCode)
	m.SourceCode = types.StringValue("def evaluate(ctx: EvaluationContext) -> EvaluationResult:\n    return EvaluationResult(scores=[Score(name=\"short\", value=len(str(ctx.observation.output)) < 100, data_type=\"BOOLEAN\")])\n")
	m.SourceCodeLanguage = types.StringValue("PYTHON")
	return m
}

func llmJudgeAPIResponse() *langfuse.Evaluator {
	desc := "Judges helpfulness"
	reasoning := "Explain briefly."
	minV, maxV := 0.0, 1.0
	jsonPath := "$.answer"
	return &langfuse.Evaluator{
		ID:          "ev-123",
		Name:        "Helpfulness",
		Description: &desc,
		Type:        langfuse.EvaluatorTypeLlmAsJudge,
		Status:      "active",
		Version:     1,
		VersionID:   "ver-1",
		Prompt: []langfuse.EvaluatorChatMessage{
			{Role: "system", Content: "You are a strict judge."},
			{Role: "user", Content: "Rate {{input}} against {{output}}"},
		},
		Variables: []string{"input", "output"},
		VariableMapping: []langfuse.PromptVariableMapping{
			{Variable: "input", Source: "input"},
			{Variable: "output", Source: "metadata", JSONPath: &jsonPath},
		},
		ModelConfig: &langfuse.EvaluatorModelConfig{Provider: "openai", Model: "gpt-4.1-mini"},
		OutputDefinition: &langfuse.EvaluatorOutputDefinition{
			DataType:                   "NUMERIC",
			MinValue:                   &minV,
			MaxValue:                   &maxV,
			ScoreReasoningInstructions: &reasoning,
		},
	}
}

func TestEvaluatorResource_Create_LlmAsJudge(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, client, s := setupEvaluatorResource(t, ctrl)

	plan := llmJudgeEvaluatorModel(t)
	plan.Version = types.Int64Unknown()
	plan.VersionID = types.StringUnknown()
	plan.Status = types.StringUnknown()
	plan.Variables = types.ListUnknown(types.StringType)

	var captured *langfuse.EvaluatorRequest
	client.EXPECT().
		CreateEvaluator(ctx, gomock.Any()).
		DoAndReturn(func(_ context.Context, req *langfuse.EvaluatorRequest) (*langfuse.Evaluator, error) {
			captured = req
			return llmJudgeAPIResponse(), nil
		})

	var createResp resource.CreateResponse
	createResp.State.Schema = s
	r.Create(ctx, resource.CreateRequest{Plan: planFromModel(t, s, plan)}, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Create: %v", createResp.Diagnostics)
	}

	// Request body must be the flattened v2 shape.
	body, _ := json.Marshal(captured)
	var got map[string]any
	_ = json.Unmarshal(body, &got)
	if got["type"] != "llm_as_judge" || got["name"] != "Helpfulness" || got["description"] != "Judges helpfulness" {
		t.Errorf("unexpected top-level request fields: %s", body)
	}
	if _, ok := got["sourceCode"]; ok {
		t.Errorf("sourceCode must be omitted for llm_as_judge: %s", body)
	}
	if got["modelConfig"].(map[string]any)["model"] != "gpt-4.1-mini" {
		t.Errorf("unexpected modelConfig: %s", body)
	}
	out := got["outputDefinition"].(map[string]any)
	if out["dataType"] != "NUMERIC" || out["minValue"] != 0.0 || out["maxValue"] != 1.0 {
		t.Errorf("unexpected outputDefinition: %s", body)
	}
	if _, ok := out["shouldAllowMultipleMatches"]; ok {
		t.Errorf("shouldAllowMultipleMatches must be omitted for NUMERIC: %s", body)
	}
	mapping := got["variableMapping"].([]any)
	if len(mapping) != 2 || mapping[1].(map[string]any)["jsonPath"] != "$.answer" {
		t.Errorf("unexpected variableMapping: %s", body)
	}

	var model evaluatorResourceModel
	if diags := createResp.State.Get(ctx, &model); diags.HasError() {
		t.Fatalf("unexpected diagnostics getting state: %v", diags)
	}
	if model.ID.ValueString() != "ev-123" || model.Version.ValueInt64() != 1 || model.VersionID.ValueString() != "ver-1" || model.Status.ValueString() != "active" {
		t.Errorf("unexpected computed attributes: %+v", model)
	}
	if model.ProjectPublicKey.ValueString() != "pk-test" || model.ProjectSecretKey.ValueString() != "sk-test" {
		t.Errorf("project credentials must be preserved from plan")
	}
	if !model.Prompt.Equal(plan.Prompt) || !model.ModelConfig.Equal(plan.ModelConfig) || !model.VariableMapping.Equal(plan.VariableMapping) || !model.OutputDefinition.Equal(plan.OutputDefinition) {
		t.Errorf("definition attributes must round-trip unchanged\nplan:  %+v\nstate: %+v", plan, model)
	}
	if len(model.Variables.Elements()) != 2 {
		t.Errorf("expected 2 variables, got %v", model.Variables)
	}
	if !model.SourceCode.IsNull() || !model.SourceCodeLanguage.IsNull() {
		t.Errorf("code attributes must be null for llm_as_judge")
	}
}

func TestEvaluatorResource_Create_Code(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, client, s := setupEvaluatorResource(t, ctrl)

	plan := codeEvaluatorModel()
	plan.Version = types.Int64Unknown()
	plan.VersionID = types.StringUnknown()
	plan.Status = types.StringUnknown()
	plan.Variables = types.ListUnknown(types.StringType)

	client.EXPECT().
		CreateEvaluator(ctx, gomock.Any()).
		DoAndReturn(func(_ context.Context, req *langfuse.EvaluatorRequest) (*langfuse.Evaluator, error) {
			body, _ := json.Marshal(req)
			var got map[string]any
			_ = json.Unmarshal(body, &got)
			if got["type"] != "code" || got["sourceCodeLanguage"] != "PYTHON" {
				t.Errorf("unexpected request: %s", body)
			}
			for _, k := range []string{"prompt", "modelConfig", "variableMapping", "outputDefinition"} {
				if _, ok := got[k]; ok {
					t.Errorf("%s must be omitted for code evaluators: %s", k, body)
				}
			}
			if got["description"] != nil {
				t.Errorf("description must be sent as null when unset: %s", body)
			}
			return &langfuse.Evaluator{
				ID: "ev-code", Name: "Length check", Type: "code", Status: "active", Version: 1, VersionID: "ver-c1",
				SourceCode: req.SourceCode, SourceCodeLanguage: "PYTHON",
			}, nil
		})

	var createResp resource.CreateResponse
	createResp.State.Schema = s
	r.Create(ctx, resource.CreateRequest{Plan: planFromModel(t, s, plan)}, &createResp)
	if createResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Create: %v", createResp.Diagnostics)
	}

	var model evaluatorResourceModel
	if diags := createResp.State.Get(ctx, &model); diags.HasError() {
		t.Fatalf("unexpected diagnostics getting state: %v", diags)
	}
	if !model.SourceCode.Equal(plan.SourceCode) || model.SourceCodeLanguage.ValueString() != "PYTHON" {
		t.Errorf("code attributes must round-trip: %+v", model)
	}
	if !model.Prompt.IsNull() || !model.OutputDefinition.IsNull() || !model.ModelConfig.IsNull() || !model.VariableMapping.IsNull() {
		t.Errorf("llm attributes must be null for code evaluators: %+v", model)
	}
	if !model.Variables.IsNull() {
		t.Errorf("variables must be null when the API returns none, got %v", model.Variables)
	}
}

func TestEvaluatorResource_Read_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, client, s := setupEvaluatorResource(t, ctrl)

	state := codeEvaluatorModel()
	state.ID = types.StringValue("ev-gone")
	state.Version = types.Int64Value(1)
	state.VersionID = types.StringValue("ver-1")
	state.Status = types.StringValue("active")

	client.EXPECT().
		GetEvaluator(ctx, "ev-gone").
		Return(nil, &langfuse.APIError{StatusCode: http.StatusNotFound, Code: "resource_not_found", Message: "gone"})

	readResp := resource.ReadResponse{State: rawFromModel(t, s, state)}
	r.Read(ctx, resource.ReadRequest{State: rawFromModel(t, s, state)}, &readResp)
	if readResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Read: %v", readResp.Diagnostics)
	}
	if !readResp.State.Raw.IsNull() {
		t.Errorf("expected resource to be removed from state on 404")
	}
}

func TestEvaluatorResource_Read_Error(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, client, s := setupEvaluatorResource(t, ctrl)

	state := codeEvaluatorModel()
	state.ID = types.StringValue("ev-1")
	state.Version = types.Int64Value(1)
	state.VersionID = types.StringValue("ver-1")
	state.Status = types.StringValue("active")

	client.EXPECT().
		GetEvaluator(ctx, "ev-1").
		Return(nil, errors.New("boom"))

	readResp := resource.ReadResponse{State: rawFromModel(t, s, state)}
	r.Read(ctx, resource.ReadRequest{State: rawFromModel(t, s, state)}, &readResp)
	if !readResp.Diagnostics.HasError() {
		t.Fatalf("expected an error diagnostic from Read")
	}
}

func TestEvaluatorResource_Update_MetadataOnlyDoesNotCreateVersion(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, client, s := setupEvaluatorResource(t, ctrl)

	state := llmJudgeEvaluatorModel(t)
	state.ID = types.StringValue("ev-123")
	state.Version = types.Int64Value(1)
	state.VersionID = types.StringValue("ver-1")
	state.Status = types.StringValue("active")
	state.Variables = stringSliceToList([]string{"input", "output"})

	plan := llmJudgeEvaluatorModel(t)
	plan.ID = types.StringValue("ev-123")
	plan.Name = types.StringValue("Helpfulness v2")
	plan.Description = types.StringNull()
	plan.Version = types.Int64Unknown()
	plan.VersionID = types.StringUnknown()
	plan.Status = types.StringUnknown()
	plan.Variables = types.ListUnknown(types.StringType)

	client.EXPECT().
		UpdateEvaluatorMetadata(ctx, "ev-123", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, req *langfuse.UpdateEvaluatorMetadataRequest) (*langfuse.Evaluator, error) {
			if req.Name != "Helpfulness v2" || req.Description != nil {
				t.Errorf("unexpected metadata request: %+v", req)
			}
			resp := llmJudgeAPIResponse()
			resp.Name = req.Name
			resp.Description = nil
			return resp, nil
		})

	updateResp := resource.UpdateResponse{State: rawFromModel(t, s, state)}
	r.Update(ctx, resource.UpdateRequest{Plan: planFromModel(t, s, plan), State: rawFromModel(t, s, state)}, &updateResp)
	if updateResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Update: %v", updateResp.Diagnostics)
	}

	var model evaluatorResourceModel
	if diags := updateResp.State.Get(ctx, &model); diags.HasError() {
		t.Fatalf("unexpected diagnostics getting state: %v", diags)
	}
	if model.Name.ValueString() != "Helpfulness v2" || !model.Description.IsNull() || model.Version.ValueInt64() != 1 {
		t.Errorf("unexpected state after metadata update: %+v", model)
	}
}

func TestEvaluatorResource_Update_DefinitionReplacesAndBumpsVersion(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, client, s := setupEvaluatorResource(t, ctrl)

	state := llmJudgeEvaluatorModel(t)
	state.ID = types.StringValue("ev-123")
	state.Version = types.Int64Value(1)
	state.VersionID = types.StringValue("ver-1")
	state.Status = types.StringValue("active")
	state.Variables = stringSliceToList([]string{"input", "output"})

	plan := llmJudgeEvaluatorModel(t)
	plan.ID = types.StringValue("ev-123")
	plan.ModelConfig = types.ObjectNull(modelConfigAttrTypes) // switch to project default model
	plan.Version = types.Int64Unknown()
	plan.VersionID = types.StringUnknown()
	plan.Status = types.StringUnknown()
	plan.Variables = types.ListUnknown(types.StringType)

	client.EXPECT().
		UpdateEvaluator(ctx, "ev-123", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, req *langfuse.EvaluatorRequest) (*langfuse.Evaluator, error) {
			if req.Type != "llm_as_judge" || req.ModelConfig != nil || len(req.Prompt) != 2 || req.OutputDefinition == nil {
				t.Errorf("full definition must be sent on update: %+v", req)
			}
			resp := llmJudgeAPIResponse()
			resp.ModelConfig = nil
			resp.Version = 2
			resp.VersionID = "ver-2"
			return resp, nil
		})

	updateResp := resource.UpdateResponse{State: rawFromModel(t, s, state)}
	r.Update(ctx, resource.UpdateRequest{Plan: planFromModel(t, s, plan), State: rawFromModel(t, s, state)}, &updateResp)
	if updateResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Update: %v", updateResp.Diagnostics)
	}

	var model evaluatorResourceModel
	if diags := updateResp.State.Get(ctx, &model); diags.HasError() {
		t.Fatalf("unexpected diagnostics getting state: %v", diags)
	}
	if model.Version.ValueInt64() != 2 || model.VersionID.ValueString() != "ver-2" || !model.ModelConfig.IsNull() {
		t.Errorf("unexpected state after definition update: %+v", model)
	}
}

func TestEvaluatorResource_Update_Error(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, client, s := setupEvaluatorResource(t, ctrl)

	state := codeEvaluatorModel()
	state.ID = types.StringValue("ev-code")
	state.Version = types.Int64Value(1)
	state.VersionID = types.StringValue("ver-1")
	state.Status = types.StringValue("active")

	plan := codeEvaluatorModel()
	plan.ID = types.StringValue("ev-code")
	plan.SourceCode = types.StringValue("def evaluate(ctx: EvaluationContext) -> EvaluationResult:\n    return EvaluationResult(scores=[])\n")
	plan.Version = types.Int64Unknown()
	plan.VersionID = types.StringUnknown()
	plan.Status = types.StringUnknown()
	plan.Variables = types.ListUnknown(types.StringType)

	client.EXPECT().
		UpdateEvaluator(ctx, "ev-code", gomock.Any()).
		Return(nil, &langfuse.APIError{StatusCode: 409, Code: "conflict", Message: "type cannot change"})

	updateResp := resource.UpdateResponse{State: rawFromModel(t, s, state)}
	r.Update(ctx, resource.UpdateRequest{Plan: planFromModel(t, s, plan), State: rawFromModel(t, s, state)}, &updateResp)
	if !updateResp.Diagnostics.HasError() {
		t.Fatalf("expected an error diagnostic from Update")
	}
}

func TestEvaluatorResource_Delete(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, client, s := setupEvaluatorResource(t, ctrl)

	state := codeEvaluatorModel()
	state.ID = types.StringValue("ev-code")
	state.Version = types.Int64Value(1)
	state.VersionID = types.StringValue("ver-1")
	state.Status = types.StringValue("active")

	client.EXPECT().DeleteEvaluator(ctx, "ev-code").Return(nil)

	var deleteResp resource.DeleteResponse
	r.Delete(ctx, resource.DeleteRequest{State: rawFromModel(t, s, state)}, &deleteResp)
	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Delete: %v", deleteResp.Diagnostics)
	}
}

func TestEvaluatorResource_ValidateConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	cases := map[string]struct {
		mutate    func(t *testing.T) evaluatorResourceModel
		wantError bool
	}{
		"valid llm_as_judge": {
			mutate:    llmJudgeEvaluatorModel,
			wantError: false,
		},
		"valid code": {
			mutate:    func(*testing.T) evaluatorResourceModel { return codeEvaluatorModel() },
			wantError: false,
		},
		"llm_as_judge without prompt": {
			mutate: func(t *testing.T) evaluatorResourceModel {
				m := llmJudgeEvaluatorModel(t)
				m.Prompt = types.ListNull(promptMessageObjectType)
				return m
			},
			wantError: true,
		},
		"llm_as_judge without output_definition": {
			mutate: func(t *testing.T) evaluatorResourceModel {
				m := llmJudgeEvaluatorModel(t)
				m.OutputDefinition = types.ObjectNull(outputDefinitionAttrTypes)
				return m
			},
			wantError: true,
		},
		"llm_as_judge with source_code": {
			mutate: func(t *testing.T) evaluatorResourceModel {
				m := llmJudgeEvaluatorModel(t)
				m.SourceCode = types.StringValue("print(1)")
				return m
			},
			wantError: true,
		},
		"code with prompt": {
			mutate: func(t *testing.T) evaluatorResourceModel {
				m := codeEvaluatorModel()
				m.Prompt = llmJudgeEvaluatorModel(t).Prompt
				return m
			},
			wantError: true,
		},
		"code without language": {
			mutate: func(*testing.T) evaluatorResourceModel {
				m := codeEvaluatorModel()
				m.SourceCodeLanguage = types.StringNull()
				return m
			},
			wantError: true,
		},
		"categorical without categories": {
			mutate: func(t *testing.T) evaluatorResourceModel {
				m := llmJudgeEvaluatorModel(t)
				out, _ := types.ObjectValueFrom(ctx, outputDefinitionAttrTypes, outputDefinitionModel{
					DataType:                   types.StringValue("CATEGORICAL"),
					MinValue:                   types.Float64Null(),
					MaxValue:                   types.Float64Null(),
					Categories:                 types.ListNull(types.StringType),
					ShouldAllowMultipleMatches: types.BoolValue(false),
					ScoreReasoningInstructions: types.StringNull(),
					ScoreValueInstructions:     types.StringNull(),
				})
				m.OutputDefinition = out
				return m
			},
			wantError: true,
		},
		"numeric with categories": {
			mutate: func(t *testing.T) evaluatorResourceModel {
				m := llmJudgeEvaluatorModel(t)
				out, _ := types.ObjectValueFrom(ctx, outputDefinitionAttrTypes, outputDefinitionModel{
					DataType:                   types.StringValue("NUMERIC"),
					MinValue:                   types.Float64Null(),
					MaxValue:                   types.Float64Null(),
					Categories:                 stringSliceToList([]string{"a", "b"}),
					ShouldAllowMultipleMatches: types.BoolValue(false),
					ScoreReasoningInstructions: types.StringNull(),
					ScoreValueInstructions:     types.StringNull(),
				})
				m.OutputDefinition = out
				return m
			},
			wantError: true,
		},
		"numeric with min greater than max": {
			mutate: func(t *testing.T) evaluatorResourceModel {
				m := llmJudgeEvaluatorModel(t)
				out, _ := types.ObjectValueFrom(ctx, outputDefinitionAttrTypes, outputDefinitionModel{
					DataType:                   types.StringValue("NUMERIC"),
					MinValue:                   types.Float64Value(5),
					MaxValue:                   types.Float64Value(1),
					Categories:                 types.ListNull(types.StringType),
					ShouldAllowMultipleMatches: types.BoolValue(false),
					ScoreReasoningInstructions: types.StringNull(),
					ScoreValueInstructions:     types.StringNull(),
				})
				m.OutputDefinition = out
				return m
			},
			wantError: true,
		},
		"should_allow_multiple_matches true on NUMERIC": {
			mutate: func(t *testing.T) evaluatorResourceModel {
				m := llmJudgeEvaluatorModel(t)
				out, _ := types.ObjectValueFrom(ctx, outputDefinitionAttrTypes, outputDefinitionModel{
					DataType:                   types.StringValue("NUMERIC"),
					MinValue:                   types.Float64Null(),
					MaxValue:                   types.Float64Null(),
					Categories:                 types.ListNull(types.StringType),
					ShouldAllowMultipleMatches: types.BoolValue(true),
					ScoreReasoningInstructions: types.StringNull(),
					ScoreValueInstructions:     types.StringNull(),
				})
				m.OutputDefinition = out
				return m
			},
			wantError: true,
		},
		"unknown forbidden attribute is deferred": {
			mutate: func(t *testing.T) evaluatorResourceModel {
				m := llmJudgeEvaluatorModel(t)
				m.SourceCode = types.StringUnknown() // may resolve to null at apply time
				return m
			},
			wantError: false,
		},
		"unknown categories on NUMERIC is deferred": {
			mutate: func(t *testing.T) evaluatorResourceModel {
				m := llmJudgeEvaluatorModel(t)
				out, _ := types.ObjectValueFrom(ctx, outputDefinitionAttrTypes, outputDefinitionModel{
					DataType:                   types.StringValue("NUMERIC"),
					MinValue:                   types.Float64Unknown(),
					MaxValue:                   types.Float64Null(),
					Categories:                 types.ListUnknown(types.StringType),
					ShouldAllowMultipleMatches: types.BoolUnknown(),
					ScoreReasoningInstructions: types.StringNull(),
					ScoreValueInstructions:     types.StringNull(),
				})
				m.OutputDefinition = out
				return m
			},
			wantError: false,
		},
		"unknown type is skipped": {
			mutate: func(*testing.T) evaluatorResourceModel {
				m := baseEvaluatorModel()
				m.Type = types.StringUnknown()
				return m
			},
			wantError: false,
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			r, _, s := setupEvaluatorResource(t, ctrl)

			var resp resource.ValidateConfigResponse
			r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: configFromModel(t, s, tc.mutate(t))}, &resp)
			if resp.Diagnostics.HasError() != tc.wantError {
				t.Errorf("wantError=%v, got diagnostics: %v", tc.wantError, resp.Diagnostics)
			}
		})
	}
}

func TestEvaluatorResource_ImportState(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, client, s := setupEvaluatorResource(t, ctrl)

	client.EXPECT().GetEvaluator(ctx, "ev-123").Return(llmJudgeAPIResponse(), nil)

	importResp := resource.ImportStateResponse{State: tfsdk.State{Schema: s}}
	importResp.State.Raw = rawFromModel(t, s, baseEvaluatorModel()).Raw
	r.ImportState(ctx, resource.ImportStateRequest{ID: "pk-imp:sk-imp:ev-123"}, &importResp)
	if importResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from ImportState: %v", importResp.Diagnostics)
	}

	var model evaluatorResourceModel
	if diags := importResp.State.Get(ctx, &model); diags.HasError() {
		t.Fatalf("unexpected diagnostics getting state: %v", diags)
	}
	if model.ID.ValueString() != "ev-123" || model.ProjectPublicKey.ValueString() != "pk-imp" || model.ProjectSecretKey.ValueString() != "sk-imp" {
		t.Errorf("unexpected imported state: %+v", model)
	}
	if model.Type.ValueString() != "llm_as_judge" || len(model.Prompt.Elements()) != 2 {
		t.Errorf("imported definition not mapped: %+v", model)
	}
}

func TestEvaluatorResource_ImportState_InvalidID(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, _, s := setupEvaluatorResource(t, ctrl)

	for _, id := range []string{"", "pk-only", "pk:sk", "pk:sk:", ":sk:id"} {
		importResp := resource.ImportStateResponse{State: tfsdk.State{Schema: s}}
		r.ImportState(ctx, resource.ImportStateRequest{ID: id}, &importResp)
		if !importResp.Diagnostics.HasError() {
			t.Errorf("expected error for import ID %q", id)
		}
	}
}

func TestEvaluatorResource_TypeRequiresReplace(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	_, _, s := setupEvaluatorResource(t, ctrl)

	typeAttr, ok := s.Attributes["type"].(resschema.StringAttribute)
	if !ok {
		t.Fatalf("type attribute is not a StringAttribute")
	}
	found := false
	for _, pm := range typeAttr.PlanModifiers {
		if pm.Description(context.Background()) == "If the value of this attribute changes, Terraform will destroy and recreate the resource." {
			found = true
		}
	}
	if !found {
		t.Errorf("type must carry a RequiresReplace plan modifier")
	}

	// Sanity check the shared helper.
	got := stringSliceToList([]string{"a"})
	elems := got.Elements()
	if len(elems) != 1 || elems[0].(types.String).ValueString() != "a" {
		t.Errorf("stringSliceToList mismatch: %v", got)
	}
}
