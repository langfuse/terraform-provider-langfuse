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

func TestOrganizationResourceMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewOrganizationResource().(*organizationResource)

	var resp resource.MetadataResponse
	r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "langfuse"}, &resp)

	expected := "langfuse_organization"
	if resp.TypeName != expected {
		t.Fatalf("unexpected type name. got %q, want %q", resp.TypeName, expected)
	}
}

func TestOrganizationResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	r := NewOrganizationResource().(*organizationResource)

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Schema: %v", schemaResp.Diagnostics)
	}

	if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema implementation validation failed: %v", diags)
	}

	idAttrRaw, ok := schemaResp.Schema.Attributes["id"]
	if !ok {
		t.Fatalf("schema is missing mandatory 'id' attribute")
	}

	idAttr, ok := idAttrRaw.(resschema.StringAttribute)
	if !ok {
		t.Fatalf("'id' attribute is not a string attribute as expected")
	}

	if !idAttr.Computed {
		t.Fatalf("'id' attribute must be Computed=true")
	}

	nameAttrRaw, ok := schemaResp.Schema.Attributes["name"]
	if !ok {
		t.Fatalf("schema is missing mandatory 'name' attribute")
	}

	nameAttr, ok := nameAttrRaw.(resschema.StringAttribute)
	if !ok {
		t.Fatalf("'name' attribute is not a string attribute as expected")
	}

	if !nameAttr.Required {
		t.Fatalf("'name' attribute must be Required=true")
	}
}

func TestOrganizationResourceCRUD(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	r := NewOrganizationResource().(*organizationResource)

	clientFactory := mocks.NewMockClientFactory(ctrl)

	var resourceSchema resschema.Schema
	t.Run("Configure", func(t *testing.T) {
		var configureResp resource.ConfigureResponse
		r.Configure(ctx, resource.ConfigureRequest{ProviderData: clientFactory}, &configureResp)

		if configureResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Configure: %v", configureResp.Diagnostics)
		}

		if r.AdminClient == nil {
			t.Fatalf("Configure did not populate AdminClient on the resource")
		}

		var schemaResp resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
		if schemaResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Schema: %v", schemaResp.Diagnostics)
		}
		resourceSchema = schemaResp.Schema
	})

	createName := "Acme Inc"
	var createResp resource.CreateResponse
	t.Run("Create", func(t *testing.T) {
		clientFactory.AdminClient.EXPECT().
			CreateOrganization(ctx, &langfuse.CreateOrganizationRequest{Name: createName}).
			Return(&langfuse.Organization{ID: "org-123", Name: createName}, nil)

		createConfig := tfsdk.Config{
			Raw: buildObjectValue(map[string]tftypes.Value{
				"id":   tftypes.NewValue(tftypes.String, nil),
				"name": tftypes.NewValue(tftypes.String, createName)}),
			Schema: resourceSchema,
		}

		createResp.State.Schema = resourceSchema

		r.Create(ctx, resource.CreateRequest{Config: createConfig}, &createResp)

		if createResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Create: %v", createResp.Diagnostics)
		}
	})

	var readResp resource.ReadResponse
	t.Run("Read", func(t *testing.T) {
		clientFactory.AdminClient.EXPECT().
			GetOrganization(ctx, "org-123").
			Return(&langfuse.Organization{ID: "org-123", Name: createName}, nil)

		readResp.State.Schema = resourceSchema

		r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)

		if readResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Read: %v", readResp.Diagnostics)
		}
	})

	var updateResp resource.UpdateResponse
	t.Run("Update", func(t *testing.T) {
		newName := "Acme Corporation"
		clientFactory.AdminClient.EXPECT().
			UpdateOrganization(ctx, "org-123", &langfuse.UpdateOrganizationRequest{Name: newName}).
			Return(&langfuse.Organization{ID: "org-123", Name: newName}, nil)

		updateConfig := tfsdk.Config{
			Raw: buildObjectValue(map[string]tftypes.Value{
				"id":   tftypes.NewValue(tftypes.String, "org-123"),
				"name": tftypes.NewValue(tftypes.String, newName),
			}),
			Schema: resourceSchema,
		}

		updateResp.State.Schema = resourceSchema

		r.Update(ctx, resource.UpdateRequest{Config: updateConfig, State: readResp.State}, &updateResp)

		if updateResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Update: %v", updateResp.Diagnostics)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		clientFactory.AdminClient.EXPECT().
			DeleteOrganization(ctx, "org-123").
			Return(nil)

		var deleteResp resource.DeleteResponse
		deleteResp.State.Schema = resourceSchema

		r.Delete(ctx, resource.DeleteRequest{State: updateResp.State}, &deleteResp)

		if deleteResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Delete: %v", deleteResp.Diagnostics)
		}
	})
}

func buildObjectValue(values map[string]tftypes.Value) tftypes.Value {
	return tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"id":   tftypes.String,
				"name": tftypes.String,
			},
			OptionalAttributes: map[string]struct{}{"id": {}},
		},
		values,
	)
}
