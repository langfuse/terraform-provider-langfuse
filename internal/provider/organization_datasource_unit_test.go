package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse/mocks"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestOrganizationDataSourceMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	d := NewOrganizationDataSource().(*organizationDataSource)

	var resp datasource.MetadataResponse
	d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "langfuse"}, &resp)

	expected := "langfuse_organization"
	if resp.TypeName != expected {
		t.Fatalf("unexpected type name. got %q, want %q", resp.TypeName, expected)
	}
}

func TestOrganizationDataSourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	d := NewOrganizationDataSource().(*organizationDataSource)

	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Schema: %v", schemaResp.Diagnostics)
	}

	if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema implementation validation failed: %v", diags)
	}

	idAttrRaw, ok := schemaResp.Schema.Attributes["id"]
	if !ok {
		t.Fatalf("schema is missing 'id' attribute")
	}

	idAttr, ok := idAttrRaw.(dsschema.StringAttribute)
	if !ok {
		t.Fatalf("'id' attribute is not a string attribute as expected")
	}

	if !idAttr.Optional || !idAttr.Computed {
		t.Fatalf("'id' attribute must be Optional=true and Computed=true")
	}

	nameAttrRaw, ok := schemaResp.Schema.Attributes["name"]
	if !ok {
		t.Fatalf("schema is missing 'name' attribute")
	}

	nameAttr, ok := nameAttrRaw.(dsschema.StringAttribute)
	if !ok {
		t.Fatalf("'name' attribute is not a string attribute as expected")
	}

	if !nameAttr.Optional || !nameAttr.Computed {
		t.Fatalf("'name' attribute must be Optional=true and Computed=true")
	}

	metadataAttrRaw, ok := schemaResp.Schema.Attributes["metadata"]
	if !ok {
		t.Fatalf("schema is missing 'metadata' attribute")
	}

	metadataAttr, ok := metadataAttrRaw.(dsschema.MapAttribute)
	if !ok {
		t.Fatalf("'metadata' attribute is not a map attribute as expected")
	}

	if !metadataAttr.Computed {
		t.Fatalf("'metadata' attribute must be Computed=true")
	}
}

func TestOrganizationDataSourceValidateConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	d := NewOrganizationDataSource().(*organizationDataSource)

	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	dataSourceSchema := schemaResp.Schema

	t.Run("NeitherIdNorName", func(t *testing.T) {
		config := tfsdk.Config{
			Raw: buildDataSourceObjectValue(map[string]tftypes.Value{
				"id":       tftypes.NewValue(tftypes.String, nil),
				"name":     tftypes.NewValue(tftypes.String, nil),
				"metadata": tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
			}),
			Schema: dataSourceSchema,
		}

		var resp datasource.ValidateConfigResponse
		d.ValidateConfig(ctx, datasource.ValidateConfigRequest{Config: config}, &resp)

		if !resp.Diagnostics.HasError() {
			t.Fatalf("expected error when neither id nor name is set")
		}
	})

	t.Run("BothIdAndName", func(t *testing.T) {
		config := tfsdk.Config{
			Raw: buildDataSourceObjectValue(map[string]tftypes.Value{
				"id":       tftypes.NewValue(tftypes.String, "org-123"),
				"name":     tftypes.NewValue(tftypes.String, "Acme Inc"),
				"metadata": tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
			}),
			Schema: dataSourceSchema,
		}

		var resp datasource.ValidateConfigResponse
		d.ValidateConfig(ctx, datasource.ValidateConfigRequest{Config: config}, &resp)

		if !resp.Diagnostics.HasError() {
			t.Fatalf("expected error when both id and name are set")
		}
	})

	t.Run("OnlyId", func(t *testing.T) {
		config := tfsdk.Config{
			Raw: buildDataSourceObjectValue(map[string]tftypes.Value{
				"id":       tftypes.NewValue(tftypes.String, "org-123"),
				"name":     tftypes.NewValue(tftypes.String, nil),
				"metadata": tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
			}),
			Schema: dataSourceSchema,
		}

		var resp datasource.ValidateConfigResponse
		d.ValidateConfig(ctx, datasource.ValidateConfigRequest{Config: config}, &resp)

		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected error when only id is set: %v", resp.Diagnostics)
		}
	})

	t.Run("OnlyName", func(t *testing.T) {
		config := tfsdk.Config{
			Raw: buildDataSourceObjectValue(map[string]tftypes.Value{
				"id":       tftypes.NewValue(tftypes.String, nil),
				"name":     tftypes.NewValue(tftypes.String, "Acme Inc"),
				"metadata": tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
			}),
			Schema: dataSourceSchema,
		}

		var resp datasource.ValidateConfigResponse
		d.ValidateConfig(ctx, datasource.ValidateConfigRequest{Config: config}, &resp)

		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected error when only name is set: %v", resp.Diagnostics)
		}
	})
}

func TestOrganizationDataSourceRead(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	d := NewOrganizationDataSource().(*organizationDataSource)

	clientFactory := mocks.NewMockClientFactory(ctrl)

	var dataSourceSchema dsschema.Schema
	t.Run("Configure", func(t *testing.T) {
		var configureResp datasource.ConfigureResponse
		d.Configure(ctx, datasource.ConfigureRequest{ProviderData: clientFactory}, &configureResp)

		if configureResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Configure: %v", configureResp.Diagnostics)
		}

		if d.AdminClient == nil {
			t.Fatalf("Configure did not populate AdminClient on the data source")
		}

		var schemaResp datasource.SchemaResponse
		d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
		if schemaResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Schema: %v", schemaResp.Diagnostics)
		}
		dataSourceSchema = schemaResp.Schema
	})

	t.Run("ReadByID", func(t *testing.T) {
		orgID := "org-123"
		orgName := "Acme Inc"
		orgMetadata := map[string]string{"environment": "test", "team": "platform"}

		clientFactory.AdminClient.EXPECT().
			GetOrganization(ctx, orgID).
			Return(&langfuse.Organization{
				ID:       orgID,
				Name:     orgName,
				Metadata: orgMetadata,
			}, nil)

		readConfig := tfsdk.Config{
			Raw: buildDataSourceObjectValue(map[string]tftypes.Value{
				"id":       tftypes.NewValue(tftypes.String, orgID),
				"name":     tftypes.NewValue(tftypes.String, nil),
				"metadata": tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
			}),
			Schema: dataSourceSchema,
		}

		var readResp datasource.ReadResponse
		readResp.State.Schema = dataSourceSchema

		d.Read(ctx, datasource.ReadRequest{Config: readConfig}, &readResp)

		if readResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Read: %v", readResp.Diagnostics)
		}

		var model organizationDataSourceModel
		diags := readResp.State.Get(ctx, &model)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics getting model from state: %v", diags)
		}

		if model.ID.ValueString() != orgID {
			t.Fatalf("unexpected ID. got %q, want %q", model.ID.ValueString(), orgID)
		}

		if model.Name.ValueString() != orgName {
			t.Fatalf("unexpected name. got %q, want %q", model.Name.ValueString(), orgName)
		}

		if model.Metadata.IsNull() {
			t.Fatalf("metadata should not be null")
		}

		var actualMetadata map[string]string
		diags = model.Metadata.ElementsAs(ctx, &actualMetadata, false)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics extracting metadata: %v", diags)
		}

		for key, expectedValue := range orgMetadata {
			if actualValue, exists := actualMetadata[key]; !exists || actualValue != expectedValue {
				t.Fatalf("unexpected metadata for key %q. got %q, want %q", key, actualValue, expectedValue)
			}
		}
	})

	t.Run("ReadByName", func(t *testing.T) {
		orgID := "org-789"
		orgName := "By Name Org"
		orgMetadata := map[string]string{"tier": "enterprise"}

		clientFactory.AdminClient.EXPECT().
			ListOrganizations(ctx).
			Return([]*langfuse.Organization{
				{ID: "org-other", Name: "Other Org", Metadata: nil},
				{ID: orgID, Name: orgName, Metadata: orgMetadata},
			}, nil)

		readConfig := tfsdk.Config{
			Raw: buildDataSourceObjectValue(map[string]tftypes.Value{
				"id":       tftypes.NewValue(tftypes.String, nil),
				"name":     tftypes.NewValue(tftypes.String, orgName),
				"metadata": tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
			}),
			Schema: dataSourceSchema,
		}

		var readResp datasource.ReadResponse
		readResp.State.Schema = dataSourceSchema

		d.Read(ctx, datasource.ReadRequest{Config: readConfig}, &readResp)

		if readResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Read: %v", readResp.Diagnostics)
		}

		var model organizationDataSourceModel
		diags := readResp.State.Get(ctx, &model)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics getting model from state: %v", diags)
		}

		if model.ID.ValueString() != orgID {
			t.Fatalf("unexpected ID. got %q, want %q", model.ID.ValueString(), orgID)
		}

		if model.Name.ValueString() != orgName {
			t.Fatalf("unexpected name. got %q, want %q", model.Name.ValueString(), orgName)
		}
	})

	t.Run("ReadByName_NotFound", func(t *testing.T) {
		clientFactory.AdminClient.EXPECT().
			ListOrganizations(ctx).
			Return([]*langfuse.Organization{
				{ID: "org-1", Name: "Existing Org", Metadata: nil},
			}, nil)

		readConfig := tfsdk.Config{
			Raw: buildDataSourceObjectValue(map[string]tftypes.Value{
				"id":       tftypes.NewValue(tftypes.String, nil),
				"name":     tftypes.NewValue(tftypes.String, "Nonexistent Org"),
				"metadata": tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
			}),
			Schema: dataSourceSchema,
		}

		var readResp datasource.ReadResponse
		readResp.State.Schema = dataSourceSchema

		d.Read(ctx, datasource.ReadRequest{Config: readConfig}, &readResp)

		if !readResp.Diagnostics.HasError() {
			t.Fatalf("expected error when organization name is not found")
		}

		found := false
		for _, diag := range readResp.Diagnostics.Errors() {
			if diag.Summary() == "Organization not found" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected 'Organization not found' error, got: %v", readResp.Diagnostics)
		}
	})

	t.Run("ReadByID_EmptyMetadata", func(t *testing.T) {
		orgID := "org-456"
		orgName := "Empty Org"

		clientFactory.AdminClient.EXPECT().
			GetOrganization(ctx, orgID).
			Return(&langfuse.Organization{
				ID:       orgID,
				Name:     orgName,
				Metadata: nil,
			}, nil)

		readConfig := tfsdk.Config{
			Raw: buildDataSourceObjectValue(map[string]tftypes.Value{
				"id":       tftypes.NewValue(tftypes.String, orgID),
				"name":     tftypes.NewValue(tftypes.String, nil),
				"metadata": tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
			}),
			Schema: dataSourceSchema,
		}

		var readResp datasource.ReadResponse
		readResp.State.Schema = dataSourceSchema

		d.Read(ctx, datasource.ReadRequest{Config: readConfig}, &readResp)

		if readResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Read: %v", readResp.Diagnostics)
		}

		var model organizationDataSourceModel
		diags := readResp.State.Get(ctx, &model)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics getting model from state: %v", diags)
		}

		if model.ID.ValueString() != orgID {
			t.Fatalf("unexpected ID. got %q, want %q", model.ID.ValueString(), orgID)
		}

		if model.Name.ValueString() != orgName {
			t.Fatalf("unexpected name. got %q, want %q", model.Name.ValueString(), orgName)
		}

		if !model.Metadata.IsNull() {
			t.Fatalf("metadata should be null when empty, got %v", model.Metadata)
		}
	})

	t.Run("ReadByID_Error", func(t *testing.T) {
		orgID := "org-bad"

		clientFactory.AdminClient.EXPECT().
			GetOrganization(ctx, orgID).
			Return(nil, fmt.Errorf("not found"))

		readConfig := tfsdk.Config{
			Raw: buildDataSourceObjectValue(map[string]tftypes.Value{
				"id":       tftypes.NewValue(tftypes.String, orgID),
				"name":     tftypes.NewValue(tftypes.String, nil),
				"metadata": tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, nil),
			}),
			Schema: dataSourceSchema,
		}

		var readResp datasource.ReadResponse
		readResp.State.Schema = dataSourceSchema

		d.Read(ctx, datasource.ReadRequest{Config: readConfig}, &readResp)

		if !readResp.Diagnostics.HasError() {
			t.Fatalf("expected error when GetOrganization fails")
		}
	})
}

func buildDataSourceObjectValue(values map[string]tftypes.Value) tftypes.Value {
	return tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"id":       tftypes.String,
				"name":     tftypes.String,
				"metadata": tftypes.Map{ElementType: tftypes.String},
			},
		},
		values,
	)
}
