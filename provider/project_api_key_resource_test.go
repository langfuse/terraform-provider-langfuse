package provider

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/cresta/terraform-provider-langfuse/langfuse"
	"github.com/cresta/terraform-provider-langfuse/langfuse/mocks"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	resschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestProjectApiKeyResourceMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewProjectApiKeyResource()

	var resp resource.MetadataResponse
	r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "langfuse"}, &resp)

	if resp.TypeName != "langfuse_project_api_key" {
		t.Fatalf("unexpected type name. got %q, want %q", resp.TypeName, "langfuse_project_api_key")
	}
}

func TestProjectApiKeyResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewProjectApiKeyResource()

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

	pkAttr, ok := schemaResp.Schema.Attributes["public_key"].(resschema.StringAttribute)
	if !ok || !pkAttr.Computed {
		t.Fatalf("'public_key' attribute must be a computed string")
	}

	skAttr, ok := schemaResp.Schema.Attributes["secret_key"].(resschema.StringAttribute)
	if !ok || !skAttr.Computed {
		t.Fatalf("'secret_key' attribute must be a computed string")
	}

	projIDAttr, ok := schemaResp.Schema.Attributes["project_id"].(resschema.StringAttribute)
	if !ok || !projIDAttr.Required {
		t.Fatalf("'project_id' attribute must be required string")
	}
}

func TestProjectApiKeyResourceCRUD(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	r, ok := NewProjectApiKeyResource().(*projectApiKeyResource)
	if !ok {
		t.Fatalf("factory did not return *projectApiKeyResource")
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

	projectID := "proj-123"
	publicKey := "pk_test_123"
	privateKey := "sk_test_456"

	var createResp resource.CreateResponse
	t.Run("Create", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().CreateProjectApiKey(ctx, projectID).Return(&langfuse.ProjectApiKey{ID: "pak-123", ProjectID: projectID, PublicKey: "pub", SecretKey: "sec"}, nil)

		createConfig := tfsdk.Config{Raw: buildApiKeyObjectValue(map[string]tftypes.Value{
			"id":                       tftypes.NewValue(tftypes.String, nil),
			"project_id":               tftypes.NewValue(tftypes.String, projectID),
			"organization_public_key":  tftypes.NewValue(tftypes.String, publicKey),
			"organization_private_key": tftypes.NewValue(tftypes.String, privateKey),
			"public_key":               tftypes.NewValue(tftypes.String, nil),
			"secret_key":               tftypes.NewValue(tftypes.String, nil),
		}), Schema: resourceSchema}
		createResp.State.Schema = resourceSchema

		r.Create(ctx, resource.CreateRequest{Config: createConfig}, &createResp)
		if createResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Create: %v", createResp.Diagnostics)
		}
	})

	var readResp resource.ReadResponse
	t.Run("Read", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().GetProjectApiKey(ctx, projectID, "pak-123").Return(&langfuse.ProjectApiKey{ID: "pak-123", ProjectID: projectID, PublicKey: "pub", SecretKey: "sec"}, nil)

		readResp.State.Schema = resourceSchema
		r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)
		if readResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Read: %v", readResp.Diagnostics)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().DeleteProjectApiKey(ctx, projectID).Return(nil)

		var deleteResp resource.DeleteResponse
		deleteResp.State.Schema = resourceSchema
		r.Delete(ctx, resource.DeleteRequest{State: readResp.State}, &deleteResp)
		if deleteResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Delete: %v", deleteResp.Diagnostics)
		}
	})
}

func buildApiKeyObjectValue(values map[string]tftypes.Value) tftypes.Value {
	return tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"id":                       tftypes.String,
				"organization_public_key":  tftypes.String,
				"organization_private_key": tftypes.String,
				"project_id":               tftypes.String,
				"public_key":               tftypes.String,
				"secret_key":               tftypes.String,
			},
			OptionalAttributes: map[string]struct{}{
				"id":         {},
				"public_key": {},
				"secret_key": {},
			},
		},
		values,
	)
}
