package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	resschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse/mocks"
)

// buildLlmConnectionObjectValue builds a tftypes.Value for the LLM connection schema
// without the id field (for plan/config use).
func buildLlmConnectionObjectValue(values map[string]tftypes.Value) tftypes.Value {
	return tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"id":                  tftypes.String,
				"project_public_key":  tftypes.String,
				"project_secret_key":  tftypes.String,
				"provider":            tftypes.String,
				"adapter":             tftypes.String,
				"secret_key":          tftypes.String,
				"base_url":            tftypes.String,
				"custom_models":       tftypes.List{ElementType: tftypes.String},
				"extra_headers":       tftypes.Map{ElementType: tftypes.String},
				"with_default_models": tftypes.Bool,
				"config":              tftypes.String,
			},
			OptionalAttributes: map[string]struct{}{
				"id":                  {},
				"base_url":            {},
				"custom_models":       {},
				"extra_headers":       {},
				"with_default_models": {},
				"config":              {},
			},
		},
		values,
	)
}

// buildLlmConnectionStateValue builds a tftypes.Value for the LLM connection schema
// including all fields (for state use).
func buildLlmConnectionStateValue(id, projectPublicKey, projectSecretKey, provider, adapter, secretKey string, extraHeaders map[string]tftypes.Value) tftypes.Value {
	extraHeadersVal := tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, extraHeaders)

	return buildLlmConnectionObjectValue(map[string]tftypes.Value{
		"id":                  tftypes.NewValue(tftypes.String, id),
		"project_public_key":  tftypes.NewValue(tftypes.String, projectPublicKey),
		"project_secret_key":  tftypes.NewValue(tftypes.String, projectSecretKey),
		"provider":            tftypes.NewValue(tftypes.String, provider),
		"adapter":             tftypes.NewValue(tftypes.String, adapter),
		"secret_key":          tftypes.NewValue(tftypes.String, secretKey),
		"base_url":            tftypes.NewValue(tftypes.String, nil),
		"custom_models":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
		"extra_headers":       extraHeadersVal,
		"with_default_models": tftypes.NewValue(tftypes.Bool, nil),
		"config":              tftypes.NewValue(tftypes.String, nil),
	})
}

func setupLlmConnectionResource(t *testing.T, ctrl *gomock.Controller) (*llmConnectionsResource, *mocks.MockLlmConnectionsClient, resschema.Schema) {
	t.Helper()

	ctx := context.Background()

	r, ok := NewLlmConnectionResource().(*llmConnectionsResource)
	if !ok {
		t.Fatalf("NewLlmConnectionResource did not return *llmConnectionsResource")
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

	return r, clientFactory.LlmConnectionsClient, schemaResp.Schema
}

// TestLlmConnectionsResource_Create verifies that Create calls UpsertLlmConnection
// and maps the response to state, preserving secret_key from the plan.
func TestLlmConnectionsResource_Create(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, llmClient, resourceSchema := setupLlmConnectionResource(t, ctrl)

	llmClient.EXPECT().
		UpsertLlmConnection(ctx, gomock.Any()).
		Return(&langfuse.LlmConnection{
			ID:                "conn-123",
			Provider:          "openai-prod",
			Adapter:           "openai",
			DisplaySecretKey:  "sk-...xyz",
			WithDefaultModels: true,
		}, nil)

	createPlan := tfsdk.Plan{
		Raw: buildLlmConnectionObjectValue(map[string]tftypes.Value{
			"id":                  tftypes.NewValue(tftypes.String, nil),
			"project_public_key":  tftypes.NewValue(tftypes.String, "pk-test"),
			"project_secret_key":  tftypes.NewValue(tftypes.String, "sk-test"),
			"provider":            tftypes.NewValue(tftypes.String, "openai-prod"),
			"adapter":             tftypes.NewValue(tftypes.String, "openai"),
			"secret_key":          tftypes.NewValue(tftypes.String, "my-api-key"),
			"base_url":            tftypes.NewValue(tftypes.String, nil),
			"custom_models":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
			"extra_headers":       tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
			"with_default_models": tftypes.NewValue(tftypes.Bool, nil),
			"config":              tftypes.NewValue(tftypes.String, nil),
		}),
		Schema: resourceSchema,
	}

	var createResp resource.CreateResponse
	createResp.State.Schema = resourceSchema

	r.Create(ctx, resource.CreateRequest{Plan: createPlan}, &createResp)

	if createResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Create: %v", createResp.Diagnostics)
	}

	var model llmConnectionsResourceModel
	diags := createResp.State.Get(ctx, &model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics getting model from state: %v", diags)
	}

	if model.Provider.ValueString() != "openai-prod" {
		t.Errorf("expected provider %q, got %q", "openai-prod", model.Provider.ValueString())
	}
	if model.Adapter.ValueString() != "openai" {
		t.Errorf("expected adapter %q, got %q", "openai", model.Adapter.ValueString())
	}
	if model.ID.ValueString() != "openai-prod" {
		t.Errorf("expected id %q, got %q", "openai-prod", model.ID.ValueString())
	}
	// secret_key must be preserved from plan, not overwritten by API response
	if model.SecretKey.ValueString() != "my-api-key" {
		t.Errorf("expected secret_key %q, got %q", "my-api-key", model.SecretKey.ValueString())
	}
}

// TestLlmConnectionsResource_Read_NotFound verifies that when the provider is not
// found in the list response, the resource is removed from state.
func TestLlmConnectionsResource_Read_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, llmClient, resourceSchema := setupLlmConnectionResource(t, ctrl)

	llmClient.EXPECT().
		ListLlmConnections(ctx, gomock.Any(), gomock.Any()).
		Return(&langfuse.ListLlmConnectionsResponse{
			Data: []langfuse.LlmConnection{
				{
					ID:       "conn-999",
					Provider: "other-provider",
					Adapter:  "anthropic",
				},
			},
			Meta: langfuse.PaginationMeta{Page: 1, Limit: 100, TotalItems: 1, TotalPages: 1},
		}, nil)

	priorState := tfsdk.State{
		Raw:    buildLlmConnectionStateValue("openai-prod", "pk-test", "sk-test", "openai-prod", "openai", "my-api-key", nil),
		Schema: resourceSchema,
	}

	var readResp resource.ReadResponse
	readResp.State.Schema = resourceSchema
	readResp.State = priorState

	r.Read(ctx, resource.ReadRequest{State: priorState}, &readResp)

	if readResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Read: %v", readResp.Diagnostics)
	}

	if !readResp.State.Raw.IsNull() {
		t.Error("expected state to be null (resource removed) when provider not found, but it was not")
	}
}

// TestLlmConnectionsResource_Delete_NoApiCall verifies that Delete does not call
// any client method — it is state-only.
func TestLlmConnectionsResource_Delete_NoApiCall(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	// No mock expectations set — gomock will fail if any method is called.
	r, _, resourceSchema := setupLlmConnectionResource(t, ctrl)
	state := tfsdk.State{
		Raw:    buildLlmConnectionStateValue("openai-prod", "pk-test", "sk-test", "openai-prod", "openai", "my-api-key", nil),
		Schema: resourceSchema,
	}

	var deleteResp resource.DeleteResponse
	deleteResp.State.Schema = resourceSchema

	r.Delete(ctx, resource.DeleteRequest{State: state}, &deleteResp)

	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Delete: %v", deleteResp.Diagnostics)
	}
}

// TestLlmConnectionsResource_ConfigValidator tests the adapter-specific config validation rules.
func TestLlmConnectionsResource_ConfigValidator(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	r := NewLlmConnectionResource()
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	resourceSchema := schemaResp.Schema

	validator := llmConnectionConfigValidator{}

	tests := []struct {
		name         string
		adapter      string
		config       tftypes.Value
		expectError  bool
		errorSummary string
	}{
		{
			name:         "bedrock_no_config",
			adapter:      "bedrock",
			config:       tftypes.NewValue(tftypes.String, nil),
			expectError:  true,
			errorSummary: "Config required for bedrock adapter",
		},
		{
			name:         "bedrock_missing_region",
			adapter:      "bedrock",
			config:       tftypes.NewValue(tftypes.String, `{"zone":"us-east"}`),
			expectError:  true,
			errorSummary: "Missing \"region\" in bedrock config",
		},
		{
			name:        "bedrock_valid",
			adapter:     "bedrock",
			config:      tftypes.NewValue(tftypes.String, `{"region":"us-east-1"}`),
			expectError: false,
		},
		{
			name:         "google_vertex_ai_missing_location",
			adapter:      "google-vertex-ai",
			config:       tftypes.NewValue(tftypes.String, `{"zone":"us-central1"}`),
			expectError:  true,
			errorSummary: "Missing \"location\" in google-vertex-ai config",
		},
		{
			name:        "google_vertex_ai_valid",
			adapter:     "google-vertex-ai",
			config:      tftypes.NewValue(tftypes.String, `{"location":"us-central1"}`),
			expectError: false,
		},
		{
			name:        "google_vertex_ai_no_config",
			adapter:     "google-vertex-ai",
			config:      tftypes.NewValue(tftypes.String, nil),
			expectError: false,
		},
		{
			name:         "google_vertex_ai_invalid_json",
			adapter:      "google-vertex-ai",
			config:       tftypes.NewValue(tftypes.String, `not-valid-json`),
			expectError:  true,
			errorSummary: "Invalid config JSON for google-vertex-ai adapter",
		},
		{
			name:         "openai_with_config",
			adapter:      "openai",
			config:       tftypes.NewValue(tftypes.String, `{"some":"value"}`),
			expectError:  true,
			errorSummary: "Config must be null for this adapter",
		},
		{
			name:        "openai_no_config",
			adapter:     "openai",
			config:      tftypes.NewValue(tftypes.String, nil),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configVal := buildLlmConnectionObjectValue(map[string]tftypes.Value{
				"id":                  tftypes.NewValue(tftypes.String, nil),
				"project_public_key":  tftypes.NewValue(tftypes.String, "pk-test"),
				"project_secret_key":  tftypes.NewValue(tftypes.String, "sk-test"),
				"provider":            tftypes.NewValue(tftypes.String, "test-provider"),
				"adapter":             tftypes.NewValue(tftypes.String, tt.adapter),
				"secret_key":          tftypes.NewValue(tftypes.String, "my-api-key"),
				"base_url":            tftypes.NewValue(tftypes.String, nil),
				"custom_models":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
				"extra_headers":       tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
				"with_default_models": tftypes.NewValue(tftypes.Bool, nil),
				"config":              tt.config,
			})

			config := tfsdk.Config{
				Raw:    configVal,
				Schema: resourceSchema,
			}

			var validateResp resource.ValidateConfigResponse
			validator.ValidateResource(ctx, resource.ValidateConfigRequest{Config: config}, &validateResp)

			hasError := validateResp.Diagnostics.HasError()
			if hasError != tt.expectError {
				if tt.expectError {
					t.Errorf("expected validation error for %q, but got none", tt.name)
				} else {
					t.Errorf("expected no validation error for %q, but got: %v", tt.name, validateResp.Diagnostics)
				}
				return
			}

			if tt.expectError && tt.errorSummary != "" {
				found := false
				for _, diag := range validateResp.Diagnostics.Errors() {
					if diag.Summary() == tt.errorSummary {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error with summary %q, got: %v", tt.errorSummary, validateResp.Diagnostics)
				}
			}
		})
	}
}

// TestLlmConnectionsResource_WriteOnlyFieldsPreserved verifies that secret_key and
// extra_headers are preserved from prior state after a Read operation.
func TestLlmConnectionsResource_WriteOnlyFieldsPreserved(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, llmClient, resourceSchema := setupLlmConnectionResource(t, ctrl)

	llmClient.EXPECT().
		ListLlmConnections(ctx, gomock.Any(), gomock.Any()).
		Return(&langfuse.ListLlmConnectionsResponse{
			Data: []langfuse.LlmConnection{
				{
					ID:                "conn-123",
					Provider:          "openai-prod",
					Adapter:           "openai",
					DisplaySecretKey:  "sk-...xyz",
					WithDefaultModels: true,
				},
			},
			Meta: langfuse.PaginationMeta{Page: 1, Limit: 100, TotalItems: 1, TotalPages: 1},
		}, nil)

	extraHeadersMap := map[string]tftypes.Value{
		"X-Custom": tftypes.NewValue(tftypes.String, "value"),
	}

	priorState := tfsdk.State{
		Raw:    buildLlmConnectionStateValue("openai-prod", "pk-test", "sk-test", "openai-prod", "openai", "original-secret", extraHeadersMap),
		Schema: resourceSchema,
	}

	var readResp resource.ReadResponse
	readResp.State.Schema = resourceSchema
	readResp.State = priorState

	r.Read(ctx, resource.ReadRequest{State: priorState}, &readResp)

	if readResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Read: %v", readResp.Diagnostics)
	}

	var model llmConnectionsResourceModel
	diags := readResp.State.Get(ctx, &model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics getting model from state: %v", diags)
	}

	if model.SecretKey.ValueString() != "original-secret" {
		t.Errorf("expected secret_key %q to be preserved, got %q", "original-secret", model.SecretKey.ValueString())
	}

	extraHeadersElements := model.ExtraHeaders.Elements()
	if len(extraHeadersElements) == 0 {
		t.Fatal("expected extra_headers to be preserved, but it is empty")
	}

	xCustomVal, ok := extraHeadersElements["X-Custom"]
	if !ok {
		t.Fatalf("expected extra_headers to contain key %q, but it did not. Keys: %v", "X-Custom", extraHeadersElements)
	}

	// The value is a framework type; compare its string representation
	if xCustomVal.String() != `"value"` {
		t.Errorf("expected extra_headers[\"X-Custom\"] = %q, got %q", "value", xCustomVal.String())
	}
}

// TestLlmConnectionsResource_Update verifies that Update calls UpsertLlmConnection
// and maps the response to state, preserving write-only fields from the plan.
func TestLlmConnectionsResource_Update(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, llmClient, resourceSchema := setupLlmConnectionResource(t, ctrl)

	llmClient.EXPECT().
		UpsertLlmConnection(ctx, gomock.Any()).
		Return(&langfuse.LlmConnection{
			ID:                "conn-123",
			Provider:          "openai-prod",
			Adapter:           "openai",
			DisplaySecretKey:  "sk-...xyz",
			WithDefaultModels: false,
		}, nil)

	updatePlan := tfsdk.Plan{
		Raw: buildLlmConnectionObjectValue(map[string]tftypes.Value{
			"id":                  tftypes.NewValue(tftypes.String, "openai-prod"),
			"project_public_key":  tftypes.NewValue(tftypes.String, "pk-test"),
			"project_secret_key":  tftypes.NewValue(tftypes.String, "sk-test"),
			"provider":            tftypes.NewValue(tftypes.String, "openai-prod"),
			"adapter":             tftypes.NewValue(tftypes.String, "openai"),
			"secret_key":          tftypes.NewValue(tftypes.String, "updated-api-key"),
			"base_url":            tftypes.NewValue(tftypes.String, nil),
			"custom_models":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
			"extra_headers":       tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
			"with_default_models": tftypes.NewValue(tftypes.Bool, false),
			"config":              tftypes.NewValue(tftypes.String, nil),
		}),
		Schema: resourceSchema,
	}

	priorState := tfsdk.State{
		Raw:    buildLlmConnectionStateValue("openai-prod", "pk-test", "sk-test", "openai-prod", "openai", "old-api-key", nil),
		Schema: resourceSchema,
	}

	var updateResp resource.UpdateResponse
	updateResp.State.Schema = resourceSchema

	r.Update(ctx, resource.UpdateRequest{Plan: updatePlan, State: priorState}, &updateResp)

	if updateResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Update: %v", updateResp.Diagnostics)
	}

	var model llmConnectionsResourceModel
	diags := updateResp.State.Get(ctx, &model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics getting model from state: %v", diags)
	}

	if model.Provider.ValueString() != "openai-prod" {
		t.Errorf("expected provider %q, got %q", "openai-prod", model.Provider.ValueString())
	}
	// secret_key must be taken from plan, not API response
	if model.SecretKey.ValueString() != "updated-api-key" {
		t.Errorf("expected secret_key %q, got %q", "updated-api-key", model.SecretKey.ValueString())
	}
	if model.WithDefaultModels.ValueBool() != false {
		t.Errorf("expected with_default_models false, got %v", model.WithDefaultModels.ValueBool())
	}
}

// TestLlmConnectionsResource_Create_WithOptionalFields verifies that base_url,
// custom_models, extra_headers, with_default_models, and config are all forwarded
// in the upsert request and correctly mapped back from the response.
func TestLlmConnectionsResource_Create_WithOptionalFields(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, llmClient, resourceSchema := setupLlmConnectionResource(t, ctrl)

	expectedCustomModels := []string{"my-model-v1", "my-model-v2"}
	responseConn := &langfuse.LlmConnection{
		ID:                "conn-456",
		Provider:          "bedrock-prod",
		Adapter:           "bedrock",
		DisplaySecretKey:  "AK...xyz",
		BaseURL:           "https://custom.bedrock.example.com",
		CustomModels:      expectedCustomModels,
		WithDefaultModels: true,
		Config:            map[string]any{"region": "us-east-1"},
	}

	llmClient.EXPECT().
		UpsertLlmConnection(ctx, gomock.Any()).
		Return(responseConn, nil)

	customModelsList := tftypes.NewValue(
		tftypes.List{ElementType: tftypes.String},
		[]tftypes.Value{
			tftypes.NewValue(tftypes.String, "my-model-v1"),
			tftypes.NewValue(tftypes.String, "my-model-v2"),
		},
	)

	createPlan := tfsdk.Plan{
		Raw: buildLlmConnectionObjectValue(map[string]tftypes.Value{
			"id":                  tftypes.NewValue(tftypes.String, nil),
			"project_public_key":  tftypes.NewValue(tftypes.String, "pk-test"),
			"project_secret_key":  tftypes.NewValue(tftypes.String, "sk-test"),
			"provider":            tftypes.NewValue(tftypes.String, "bedrock-prod"),
			"adapter":             tftypes.NewValue(tftypes.String, "bedrock"),
			"secret_key":          tftypes.NewValue(tftypes.String, "my-aws-key"),
			"base_url":            tftypes.NewValue(tftypes.String, "https://custom.bedrock.example.com"),
			"custom_models":       customModelsList,
			"extra_headers":       tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
			"with_default_models": tftypes.NewValue(tftypes.Bool, true),
			"config":              tftypes.NewValue(tftypes.String, `{"region":"us-east-1"}`),
		}),
		Schema: resourceSchema,
	}

	var createResp resource.CreateResponse
	createResp.State.Schema = resourceSchema

	r.Create(ctx, resource.CreateRequest{Plan: createPlan}, &createResp)

	if createResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Create: %v", createResp.Diagnostics)
	}

	var model llmConnectionsResourceModel
	diags := createResp.State.Get(ctx, &model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics getting model from state: %v", diags)
	}

	if model.BaseURL.ValueString() != "https://custom.bedrock.example.com" {
		t.Errorf("expected base_url %q, got %q", "https://custom.bedrock.example.com", model.BaseURL.ValueString())
	}
	if model.Config.ValueString() != `{"region":"us-east-1"}` {
		t.Errorf("expected config %q, got %q", `{"region":"us-east-1"}`, model.Config.ValueString())
	}
	models := model.CustomModels.Elements()
	if len(models) != 2 {
		t.Errorf("expected 2 custom_models, got %d", len(models))
	}
	if model.WithDefaultModels.ValueBool() != true {
		t.Errorf("expected with_default_models true, got %v", model.WithDefaultModels.ValueBool())
	}
}

// TestLlmConnectionsResource_Create_Error verifies that an API error during Create
// surfaces as a diagnostic error and does not write any state.
func TestLlmConnectionsResource_Create_Error(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, llmClient, resourceSchema := setupLlmConnectionResource(t, ctrl)

	llmClient.EXPECT().
		UpsertLlmConnection(ctx, gomock.Any()).
		Return(nil, fmt.Errorf("upstream API error"))

	createPlan := tfsdk.Plan{
		Raw: buildLlmConnectionObjectValue(map[string]tftypes.Value{
			"id":                  tftypes.NewValue(tftypes.String, nil),
			"project_public_key":  tftypes.NewValue(tftypes.String, "pk-test"),
			"project_secret_key":  tftypes.NewValue(tftypes.String, "sk-test"),
			"provider":            tftypes.NewValue(tftypes.String, "openai-prod"),
			"adapter":             tftypes.NewValue(tftypes.String, "openai"),
			"secret_key":          tftypes.NewValue(tftypes.String, "my-api-key"),
			"base_url":            tftypes.NewValue(tftypes.String, nil),
			"custom_models":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
			"extra_headers":       tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
			"with_default_models": tftypes.NewValue(tftypes.Bool, nil),
			"config":              tftypes.NewValue(tftypes.String, nil),
		}),
		Schema: resourceSchema,
	}

	var createResp resource.CreateResponse
	createResp.State.Schema = resourceSchema

	r.Create(ctx, resource.CreateRequest{Plan: createPlan}, &createResp)

	if !createResp.Diagnostics.HasError() {
		t.Fatal("expected diagnostics error from Create on API failure, but got none")
	}
}

// TestLlmConnectionsResource_Read_Error verifies that an API error during Read
// surfaces as a diagnostic error.
func TestLlmConnectionsResource_Read_Error(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, llmClient, resourceSchema := setupLlmConnectionResource(t, ctrl)

	llmClient.EXPECT().
		ListLlmConnections(ctx, gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("network error"))

	priorState := tfsdk.State{
		Raw:    buildLlmConnectionStateValue("openai-prod", "pk-test", "sk-test", "openai-prod", "openai", "my-api-key", nil),
		Schema: resourceSchema,
	}

	var readResp resource.ReadResponse
	readResp.State.Schema = resourceSchema
	readResp.State = priorState

	r.Read(ctx, resource.ReadRequest{State: priorState}, &readResp)

	if !readResp.Diagnostics.HasError() {
		t.Fatal("expected diagnostics error from Read on API failure, but got none")
	}
}

// TestLlmConnectionsResource_Update_Error verifies that an API error during Update
// surfaces as a diagnostic error.
func TestLlmConnectionsResource_Update_Error(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, llmClient, resourceSchema := setupLlmConnectionResource(t, ctrl)

	llmClient.EXPECT().
		UpsertLlmConnection(ctx, gomock.Any()).
		Return(nil, fmt.Errorf("upstream API error"))

	updatePlan := tfsdk.Plan{
		Raw: buildLlmConnectionObjectValue(map[string]tftypes.Value{
			"id":                  tftypes.NewValue(tftypes.String, "openai-prod"),
			"project_public_key":  tftypes.NewValue(tftypes.String, "pk-test"),
			"project_secret_key":  tftypes.NewValue(tftypes.String, "sk-test"),
			"provider":            tftypes.NewValue(tftypes.String, "openai-prod"),
			"adapter":             tftypes.NewValue(tftypes.String, "openai"),
			"secret_key":          tftypes.NewValue(tftypes.String, "updated-api-key"),
			"base_url":            tftypes.NewValue(tftypes.String, nil),
			"custom_models":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
			"extra_headers":       tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
			"with_default_models": tftypes.NewValue(tftypes.Bool, nil),
			"config":              tftypes.NewValue(tftypes.String, nil),
		}),
		Schema: resourceSchema,
	}

	priorState := tfsdk.State{
		Raw:    buildLlmConnectionStateValue("openai-prod", "pk-test", "sk-test", "openai-prod", "openai", "old-api-key", nil),
		Schema: resourceSchema,
	}

	var updateResp resource.UpdateResponse
	updateResp.State.Schema = resourceSchema

	r.Update(ctx, resource.UpdateRequest{Plan: updatePlan, State: priorState}, &updateResp)

	if !updateResp.Diagnostics.HasError() {
		t.Fatal("expected diagnostics error from Update on API failure, but got none")
	}
}

// TestLlmConnectionsResource_Read_Pagination verifies that Read iterates through
// multiple pages until the target provider is found.
func TestLlmConnectionsResource_Read_Pagination(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	r, llmClient, resourceSchema := setupLlmConnectionResource(t, ctrl)

	// Page 1: does not contain the target provider
	gomock.InOrder(
		llmClient.EXPECT().
			ListLlmConnections(ctx, gomock.Any(), gomock.Any()).
			Return(&langfuse.ListLlmConnectionsResponse{
				Data: []langfuse.LlmConnection{
					{ID: "conn-1", Provider: "other-1", Adapter: "openai"},
				},
				Meta: langfuse.PaginationMeta{Page: 1, Limit: 100, TotalItems: 2, TotalPages: 2},
			}, nil),
		// Page 2: contains the target provider
		llmClient.EXPECT().
			ListLlmConnections(ctx, gomock.Any(), gomock.Any()).
			Return(&langfuse.ListLlmConnectionsResponse{
				Data: []langfuse.LlmConnection{
					{
						ID:                "conn-2",
						Provider:          "openai-prod",
						Adapter:           "openai",
						WithDefaultModels: true,
					},
				},
				Meta: langfuse.PaginationMeta{Page: 2, Limit: 100, TotalItems: 2, TotalPages: 2},
			}, nil),
	)

	priorState := tfsdk.State{
		Raw:    buildLlmConnectionStateValue("openai-prod", "pk-test", "sk-test", "openai-prod", "openai", "my-api-key", nil),
		Schema: resourceSchema,
	}

	var readResp resource.ReadResponse
	readResp.State.Schema = resourceSchema
	readResp.State = priorState

	r.Read(ctx, resource.ReadRequest{State: priorState}, &readResp)

	if readResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Read: %v", readResp.Diagnostics)
	}

	if readResp.State.Raw.IsNull() {
		t.Error("expected resource to be found on page 2, but state was removed")
	}

	var model llmConnectionsResourceModel
	diags := readResp.State.Get(ctx, &model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics getting model from state: %v", diags)
	}
	if model.Provider.ValueString() != "openai-prod" {
		t.Errorf("expected provider %q, got %q", "openai-prod", model.Provider.ValueString())
	}
}

// TestLlmConnectionsResource_ProviderFieldRequiresReplace verifies that the schema
// declares RequiresReplace on the provider attribute, ensuring Terraform will
// destroy-and-recreate rather than in-place update when the provider name changes.
func TestLlmConnectionsResource_ProviderFieldRequiresReplace(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewLlmConnectionResource()

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	providerAttr, ok := schemaResp.Schema.Attributes["provider"]
	if !ok {
		t.Fatal("expected schema to have a \"provider\" attribute")
	}

	strAttr, ok := providerAttr.(resschema.StringAttribute)
	if !ok {
		t.Fatalf("expected \"provider\" to be a StringAttribute, got %T", providerAttr)
	}

	wantDescription := "RequiresReplace"
	found := false
	for _, pm := range strAttr.PlanModifiers {
		// stringplanmodifier.RequiresReplace() returns a modifier whose
		// Description is "RequiresReplace".
		desc := pm.(planmodifier.String).Description(ctx)
		if desc == stringplanmodifier.RequiresReplace().Description(ctx) {
			found = true
			break
		}
		_ = desc
	}
	_ = wantDescription
	if !found {
		t.Error("expected \"provider\" attribute to have RequiresReplace plan modifier, but it was not found")
	}
}
