package provider

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse/mocks"
)

func TestLlmConnectionResourceMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewLlmConnectionResource()

	var resp resource.MetadataResponse
	r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "langfuse"}, &resp)

	if resp.TypeName != "langfuse_llm_connection" {
		t.Fatalf("unexpected type name. got %q, want %q", resp.TypeName, "langfuse_llm_connection")
	}
}

func TestLlmConnectionResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewLlmConnectionResource()

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Schema: %v", schemaResp.Diagnostics)
	}

	if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema implementation validation failed: %v", diags)
	}

	idAttr, ok := schemaResp.Schema.Attributes["id"].(resschema.StringAttribute)
	if !ok || !idAttr.Computed {
		t.Fatalf("'id' attribute must be a computed string")
	}

	providerAttr, ok := schemaResp.Schema.Attributes["provider_name"].(resschema.StringAttribute)
	if !ok || !providerAttr.Required {
		t.Fatalf("'provider_name' attribute must be a required string")
	}
}

func TestLlmConnectionResourceCRUD(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	r, ok := NewLlmConnectionResource().(*llmConnectionResource)
	if !ok {
		t.Fatalf("factory did not return *llmConnectionResource")
	}

	clientFactory := mocks.NewMockClientFactory(ctrl)

	var resourceSchema resschema.Schema
	t.Run("Configure", func(t *testing.T) {
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
		resourceSchema = schemaResp.Schema
	})

	projectPublicKey := "pk-test"
	projectSecretKey := "sk-test"
	providerName := "openai"
	adapter := "openai"
	secretKey := "llm-secret"

	connID := "llm-conn-123"

	var createResp resource.CreateResponse
	t.Run("Create", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().UpsertLlmConnection(ctx, &langfuse.UpsertLlmConnectionRequest{
			Provider:  providerName,
			Adapter:   adapter,
			SecretKey: secretKey,
		}).Return(&langfuse.LlmConnection{
			ID:                connID,
			Provider:          providerName,
			Adapter:           adapter,
			DisplaySecretKey:  "****",
			CustomModels:      []string{},
			WithDefaultModels: true,
			ExtraHeaderKeys:   []string{},
			CreatedAt:         "2026-01-01T00:00:00Z",
			UpdatedAt:         "2026-01-01T00:00:00Z",
		}, nil)

		createConfig := tfsdk.Config{Raw: buildLlmConnectionObjectValue(map[string]tftypes.Value{
			"id":                  tftypes.NewValue(tftypes.String, nil),
			"project_public_key":  tftypes.NewValue(tftypes.String, projectPublicKey),
			"project_secret_key":  tftypes.NewValue(tftypes.String, projectSecretKey),
			"provider_name":       tftypes.NewValue(tftypes.String, providerName),
			"adapter":             tftypes.NewValue(tftypes.String, adapter),
			"secret_key":          tftypes.NewValue(tftypes.String, secretKey),
			"base_url":            tftypes.NewValue(tftypes.String, nil),
			"custom_models":       tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
			"with_default_models": tftypes.NewValue(tftypes.Bool, nil),
			"extra_headers":       tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
			"config_json":         tftypes.NewValue(tftypes.String, nil),
			"display_secret_key":  tftypes.NewValue(tftypes.String, nil),
			"extra_header_keys":   tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, nil),
			"created_at":          tftypes.NewValue(tftypes.String, nil),
			"updated_at":          tftypes.NewValue(tftypes.String, nil),
			"ignore_destroy":      tftypes.NewValue(tftypes.Bool, nil),
		}), Schema: resourceSchema}
		createResp.State.Schema = resourceSchema

		r.Create(ctx, resource.CreateRequest{Config: createConfig}, &createResp)
		if createResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Create: %v", createResp.Diagnostics)
		}
	})

	var readResp resource.ReadResponse
	t.Run("Read", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().ListLlmConnections(ctx, 1, 100).Return(&langfuse.PaginatedLlmConnections{
			Data: []langfuse.LlmConnection{{
				ID:                connID,
				Provider:          providerName,
				Adapter:           adapter,
				DisplaySecretKey:  "****",
				CustomModels:      []string{"gpt-4o"},
				WithDefaultModels: true,
				ExtraHeaderKeys:   []string{"x-test"},
				CreatedAt:         "2026-01-01T00:00:00Z",
				UpdatedAt:         "2026-01-02T00:00:00Z",
			}},
			Meta: langfuse.MetaResponse{Page: 1, Limit: 100, TotalItems: 1, TotalPages: 1},
		}, nil)

		readResp.State.Schema = resourceSchema
		r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)
		if readResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Read: %v", readResp.Diagnostics)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		var deleteResp resource.DeleteResponse
		deleteResp.State.Schema = resourceSchema
		r.Delete(ctx, resource.DeleteRequest{State: readResp.State}, &deleteResp)
		if deleteResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Delete: %v", deleteResp.Diagnostics)
		}
	})
}

func buildLlmConnectionObjectValue(values map[string]tftypes.Value) tftypes.Value {
	return tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"id":                  tftypes.String,
				"project_public_key":  tftypes.String,
				"project_secret_key":  tftypes.String,
				"provider_name":       tftypes.String,
				"adapter":             tftypes.String,
				"secret_key":          tftypes.String,
				"base_url":            tftypes.String,
				"custom_models":       tftypes.List{ElementType: tftypes.String},
				"with_default_models": tftypes.Bool,
				"extra_headers":       tftypes.Map{ElementType: tftypes.String},
				"config_json":         tftypes.String,
				"display_secret_key":  tftypes.String,
				"extra_header_keys":   tftypes.List{ElementType: tftypes.String},
				"created_at":          tftypes.String,
				"updated_at":          tftypes.String,
				"ignore_destroy":      tftypes.Bool,
			},
			OptionalAttributes: map[string]struct{}{
				"id":                  {},
				"base_url":            {},
				"custom_models":       {},
				"with_default_models": {},
				"extra_headers":       {},
				"config_json":         {},
				"display_secret_key":  {},
				"extra_header_keys":   {},
				"created_at":          {},
				"updated_at":          {},
				"ignore_destroy":      {},
			},
		},
		values,
	)
}
