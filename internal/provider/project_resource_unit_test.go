package provider

import (
	"context"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/cresta/terraform-provider-langfuse/internal/langfuse"
	"github.com/cresta/terraform-provider-langfuse/internal/langfuse/mocks"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	resschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestProjectResourceMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewProjectResource()

	var resp resource.MetadataResponse
	r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "langfuse"}, &resp)

	if resp.TypeName != "langfuse_project" {
		t.Fatalf("unexpected type name. got %q, want %q", resp.TypeName, "langfuse_project")
	}
}

func TestProjectResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewProjectResource()

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

func TestProjectResourceCRUD(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	r, ok := NewProjectResource().(*projectResource)
	if !ok {
		t.Fatalf("NewProjectResource did not return a *projectResource as expected")
	}

	clientFactory := mocks.NewMockClientFactory(ctrl)

	var resourceSchema resschema.Schema
	t.Run("Configure", func(t *testing.T) {
		var configureResp resource.ConfigureResponse
		r.Configure(ctx, resource.ConfigureRequest{ProviderData: clientFactory}, &configureResp)

		if configureResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Configure: %v", configureResp.Diagnostics)
		}
		if r.ClientFactory == nil {
			t.Fatalf("Configure did not populate ClientFactory on the resource")
		}

		var schemaResp resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
		if schemaResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Schema: %v", schemaResp.Diagnostics)
		}
		resourceSchema = schemaResp.Schema
	})

	createName := "ChatQA"
	createMetadata := map[string]string{"environment": "test", "team": "ai"}
	projectID := "proj-123"
	organizationID := "org-123"
	publicKey := "pk-1234"
	privateKey := "sk-1234"

	var createResp resource.CreateResponse
	t.Run("Create", func(t *testing.T) {
		expectedProject := &langfuse.CreateProjectRequest{
			Name:          createName,
			RetentionDays: 0,
			Metadata:      createMetadata,
		}
		clientFactory.OrganizationClient.EXPECT().CreateProject(ctx, expectedProject).Return(&langfuse.Project{
			ID:            projectID,
			Name:          createName,
			RetentionDays: 0,
			Metadata:      createMetadata,
		}, nil)

		metadataValue := tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, map[string]tftypes.Value{
			"environment": tftypes.NewValue(tftypes.String, "test"),
			"team":        tftypes.NewValue(tftypes.String, "ai"),
		})

		createConfig := tfsdk.Config{
			Raw: buildProjectObjectValue(map[string]tftypes.Value{
				"id":                       tftypes.NewValue(tftypes.String, nil),
				"name":                     tftypes.NewValue(tftypes.String, createName),
				"retention_days":           tftypes.NewValue(tftypes.Number, big.NewFloat(0)),
				"metadata":                 metadataValue,
				"organization_id":          tftypes.NewValue(tftypes.String, organizationID),
				"organization_public_key":  tftypes.NewValue(tftypes.String, publicKey),
				"organization_private_key": tftypes.NewValue(tftypes.String, privateKey),
			}),
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
		clientFactory.OrganizationClient.EXPECT().GetProject(ctx, "proj-123").Return(&langfuse.Project{
			ID:            "proj-123",
			Name:          createName,
			RetentionDays: 0,
			Metadata:      createMetadata,
		}, nil)

		readResp.State.Schema = resourceSchema
		r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)
		if readResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Read: %v", readResp.Diagnostics)
		}
	})

	var updateResp resource.UpdateResponse
	t.Run("Update", func(t *testing.T) {
		newName := "ChatQA Plus"
		newRetention := int32(30)
		newMetadata := map[string]string{"environment": "production", "team": "ai", "version": "2.0"}
		clientFactory.OrganizationClient.EXPECT().UpdateProject(ctx, "proj-123", &langfuse.UpdateProjectRequest{
			Name:          newName,
			RetentionDays: newRetention,
			Metadata:      newMetadata,
		}).Return(&langfuse.Project{
			ID:            "proj-123",
			Name:          newName,
			RetentionDays: newRetention,
			Metadata:      newMetadata,
		}, nil)

		newMetadataValue := tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, map[string]tftypes.Value{
			"environment": tftypes.NewValue(tftypes.String, "production"),
			"team":        tftypes.NewValue(tftypes.String, "ai"),
			"version":     tftypes.NewValue(tftypes.String, "2.0"),
		})

		updateConfig := tfsdk.Config{
			Raw: buildProjectObjectValue(map[string]tftypes.Value{
				"id":                       tftypes.NewValue(tftypes.String, "proj-123"),
				"name":                     tftypes.NewValue(tftypes.String, newName),
				"retention_days":           tftypes.NewValue(tftypes.Number, big.NewFloat(float64(newRetention))),
				"metadata":                 newMetadataValue,
				"organization_id":          tftypes.NewValue(tftypes.String, organizationID),
				"organization_public_key":  tftypes.NewValue(tftypes.String, publicKey),
				"organization_private_key": tftypes.NewValue(tftypes.String, privateKey),
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
		clientFactory.OrganizationClient.EXPECT().DeleteProject(ctx, "proj-123").Return(nil)

		var deleteResp resource.DeleteResponse
		deleteResp.State.Schema = resourceSchema
		r.Delete(ctx, resource.DeleteRequest{State: updateResp.State}, &deleteResp)
		if deleteResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Delete: %v", deleteResp.Diagnostics)
		}
	})

	// Test that retention_days is preserved from state during Read (since API doesn't return it)
	t.Run("Read preserves retention_days from state", func(t *testing.T) {
		ctx := context.Background()
		r := &projectResource{}

		clientFactory := mocks.NewMockClientFactory(ctrl)

		clientFactory.OrganizationClient.EXPECT().GetProject(ctx, "proj-123").Return(&langfuse.Project{
			ID:            "proj-123",
			Name:          "test-project",
			RetentionDays: 0, // API returns 0 (doesn't return actual value)
			Metadata:      map[string]string{"test": "value"},
		}, nil)

		r.ClientFactory = clientFactory

		testMetadataValue := tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, map[string]tftypes.Value{
			"test": tftypes.NewValue(tftypes.String, "value"),
		})

		state := buildProjectObjectValue(map[string]tftypes.Value{
			"id":                       tftypes.NewValue(tftypes.String, "proj-123"),
			"name":                     tftypes.NewValue(tftypes.String, "test-project"),
			"retention_days":           tftypes.NewValue(tftypes.Number, big.NewFloat(30)),
			"metadata":                 testMetadataValue,
			"organization_id":          tftypes.NewValue(tftypes.String, organizationID),
			"organization_public_key":  tftypes.NewValue(tftypes.String, "pub-key"),
			"organization_private_key": tftypes.NewValue(tftypes.String, "priv-key"),
		})

		var readResp resource.ReadResponse
		readResp.State.Raw = state
		readResp.State.Schema = resourceSchema

		r.Read(ctx, resource.ReadRequest{State: readResp.State}, &readResp)

		if readResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Read: %v", readResp.Diagnostics)
		}

		// Verify retention_days is preserved as 30, not overwritten with 0 from API
		var stateData projectResourceModel
		readResp.State.Get(ctx, &stateData)

		if stateData.RetentionDays.ValueInt32() != 30 {
			t.Errorf("expected retention_days to be preserved as 30, got %d", stateData.RetentionDays.ValueInt32())
		}
	})
}

func buildProjectObjectValue(values map[string]tftypes.Value) tftypes.Value {
	return tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"id":                       tftypes.String,
				"name":                     tftypes.String,
				"retention_days":           tftypes.Number,
				"metadata":                 tftypes.Map{ElementType: tftypes.String},
				"organization_id":          tftypes.String,
				"organization_public_key":  tftypes.String,
				"organization_private_key": tftypes.String,
			},
			OptionalAttributes: map[string]struct{}{
				"id":                       {},
				"retention_days":           {},
				"metadata":                 {},
				"organization_id":          {},
				"organization_public_key":  {},
				"organization_private_key": {},
			},
		},
		values,
	)
}
