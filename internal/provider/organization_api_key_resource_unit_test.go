package provider

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/cresta/terraform-provider-langfuse/internal/langfuse"
	"github.com/cresta/terraform-provider-langfuse/internal/langfuse/mocks"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	resschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestOrganizationApiKeyResourceMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewOrganizationApiKeyResource()

	var resp resource.MetadataResponse
	r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "langfuse"}, &resp)

	if resp.TypeName != "langfuse_organization_api_key" {
		t.Fatalf("unexpected type name. got %q, want %q", resp.TypeName, "langfuse_organization_api_key")
	}
}

func TestOrganizationApiKeyResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewOrganizationApiKeyResource()

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
		t.Fatalf("'id' must be computed string")
	}
	orgAttr, ok := schemaResp.Schema.Attributes["organization_id"].(resschema.StringAttribute)
	if !ok || !orgAttr.Required {
		t.Fatalf("'organization_id' must be required string")
	}
}

func TestOrganizationApiKeyResourceCRUD(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	r, ok := NewOrganizationApiKeyResource().(*organizationApiKeyResource)
	if !ok {
		t.Fatalf("factory did not return *organizationApiKeyResource")
	}

	clientFactory := mocks.NewMockClientFactory(ctrl)

	var resourceSchema resschema.Schema
	t.Run("Configure", func(t *testing.T) {
		var configureResp resource.ConfigureResponse
		r.Configure(ctx, resource.ConfigureRequest{ProviderData: clientFactory}, &configureResp)
		if configureResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Configure: %v", configureResp.Diagnostics)
		}
		if r.AdminClient == nil {
			t.Fatalf("AdminClient is nil after Configure")
		}
		var schemaResp resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
		if schemaResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Schema: %v", schemaResp.Diagnostics)
		}
		resourceSchema = schemaResp.Schema
	})

	orgID := "org-123"

	var createResp resource.CreateResponse
	t.Run("Create", func(t *testing.T) {
		clientFactory.AdminClient.EXPECT().CreateOrganizationApiKey(ctx, orgID).Return(&langfuse.OrganizationApiKey{ID: "oak-123", PublicKey: "pk-1234", SecretKey: "sk-1234"}, nil)

		createConfig := tfsdk.Config{Raw: buildOrgApiKeyObjectValue(map[string]tftypes.Value{
			"id":              tftypes.NewValue(tftypes.String, nil),
			"organization_id": tftypes.NewValue(tftypes.String, orgID),
			"public_key":      tftypes.NewValue(tftypes.String, nil),
			"secret_key":      tftypes.NewValue(tftypes.String, nil),
		}), Schema: resourceSchema}
		createResp.State.Schema = resourceSchema
		r.Create(ctx, resource.CreateRequest{Config: createConfig}, &createResp)
		if createResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Create: %v", createResp.Diagnostics)
		}
	})

	var readResp resource.ReadResponse
	t.Run("Read", func(t *testing.T) {
		clientFactory.AdminClient.EXPECT().GetOrganizationApiKey(ctx, orgID, "oak-123").Return(&langfuse.OrganizationApiKey{ID: "oak-123", PublicKey: "pk-1234", SecretKey: "sk-1234"}, nil)

		readResp.State.Schema = resourceSchema
		r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)
		if readResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Read: %v", readResp.Diagnostics)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		clientFactory.AdminClient.EXPECT().DeleteOrganizationApiKey(ctx, orgID, "oak-123").Return(nil)

		var deleteResp resource.DeleteResponse
		deleteResp.State.Schema = resourceSchema
		r.Delete(ctx, resource.DeleteRequest{State: readResp.State}, &deleteResp)
		if deleteResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Delete: %v", deleteResp.Diagnostics)
		}
	})
}

func buildOrgApiKeyObjectValue(values map[string]tftypes.Value) tftypes.Value {
	return tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"id":              tftypes.String,
				"organization_id": tftypes.String,
				"public_key":      tftypes.String,
				"secret_key":      tftypes.String,
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
