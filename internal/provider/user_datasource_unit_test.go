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

func buildUserDataSourceObjectValue(values map[string]tftypes.Value) tftypes.Value {
	return tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"id":                      tftypes.String,
				"email":                   tftypes.String,
				"user_name":               tftypes.String,
				"active":                  tftypes.Bool,
				"organization_public_key":  tftypes.String,
				"organization_private_key": tftypes.String,
			},
		},
		values,
	)
}

func TestUserDataSourceMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	d := NewUserDataSource().(*userDataSource)

	var resp datasource.MetadataResponse
	d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "langfuse"}, &resp)

	expected := "langfuse_user"
	if resp.TypeName != expected {
		t.Fatalf("unexpected type name. got %q, want %q", resp.TypeName, expected)
	}
}

func TestUserDataSourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	d := NewUserDataSource().(*userDataSource)

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
		t.Fatalf("'id' attribute is not a StringAttribute")
	}
	if !idAttr.Optional || !idAttr.Computed {
		t.Fatalf("'id' attribute must be Optional=true and Computed=true")
	}

	emailAttrRaw, ok := schemaResp.Schema.Attributes["email"]
	if !ok {
		t.Fatalf("schema is missing 'email' attribute")
	}
	emailAttr, ok := emailAttrRaw.(dsschema.StringAttribute)
	if !ok {
		t.Fatalf("'email' attribute is not a StringAttribute")
	}
	if !emailAttr.Optional || !emailAttr.Computed {
		t.Fatalf("'email' attribute must be Optional=true and Computed=true")
	}

	userNameAttrRaw, ok := schemaResp.Schema.Attributes["user_name"]
	if !ok {
		t.Fatalf("schema is missing 'user_name' attribute")
	}
	userNameAttr, ok := userNameAttrRaw.(dsschema.StringAttribute)
	if !ok {
		t.Fatalf("'user_name' attribute is not a StringAttribute")
	}
	if !userNameAttr.Computed {
		t.Fatalf("'user_name' attribute must be Computed=true")
	}

	activeAttrRaw, ok := schemaResp.Schema.Attributes["active"]
	if !ok {
		t.Fatalf("schema is missing 'active' attribute")
	}
	activeAttr, ok := activeAttrRaw.(dsschema.BoolAttribute)
	if !ok {
		t.Fatalf("'active' attribute is not a BoolAttribute")
	}
	if !activeAttr.Computed {
		t.Fatalf("'active' attribute must be Computed=true")
	}

	pubKeyAttrRaw, ok := schemaResp.Schema.Attributes["organization_public_key"]
	if !ok {
		t.Fatalf("schema is missing 'organization_public_key' attribute")
	}
	pubKeyAttr, ok := pubKeyAttrRaw.(dsschema.StringAttribute)
	if !ok {
		t.Fatalf("'organization_public_key' attribute is not a StringAttribute")
	}
	if !pubKeyAttr.Required || !pubKeyAttr.Sensitive {
		t.Fatalf("'organization_public_key' attribute must be Required=true and Sensitive=true")
	}

	privKeyAttrRaw, ok := schemaResp.Schema.Attributes["organization_private_key"]
	if !ok {
		t.Fatalf("schema is missing 'organization_private_key' attribute")
	}
	privKeyAttr, ok := privKeyAttrRaw.(dsschema.StringAttribute)
	if !ok {
		t.Fatalf("'organization_private_key' attribute is not a StringAttribute")
	}
	if !privKeyAttr.Required || !privKeyAttr.Sensitive {
		t.Fatalf("'organization_private_key' attribute must be Required=true and Sensitive=true")
	}
}

func TestUserDataSourceValidateConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	d := NewUserDataSource().(*userDataSource)

	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	dataSourceSchema := schemaResp.Schema

	pubKey := tftypes.NewValue(tftypes.String, "pk-test")
	privKey := tftypes.NewValue(tftypes.String, "sk-test")

	t.Run("NeitherIdNorEmail", func(t *testing.T) {
		config := tfsdk.Config{
			Raw: buildUserDataSourceObjectValue(map[string]tftypes.Value{
				"id":                      tftypes.NewValue(tftypes.String, nil),
				"email":                   tftypes.NewValue(tftypes.String, nil),
				"user_name":               tftypes.NewValue(tftypes.String, nil),
				"active":                  tftypes.NewValue(tftypes.Bool, nil),
				"organization_public_key":  pubKey,
				"organization_private_key": privKey,
			}),
			Schema: dataSourceSchema,
		}

		var resp datasource.ValidateConfigResponse
		d.ValidateConfig(ctx, datasource.ValidateConfigRequest{Config: config}, &resp)

		if !resp.Diagnostics.HasError() {
			t.Fatalf("expected error when neither id nor email is set")
		}
	})

	t.Run("BothIdAndEmail", func(t *testing.T) {
		config := tfsdk.Config{
			Raw: buildUserDataSourceObjectValue(map[string]tftypes.Value{
				"id":                      tftypes.NewValue(tftypes.String, "user-123"),
				"email":                   tftypes.NewValue(tftypes.String, "user@example.com"),
				"user_name":               tftypes.NewValue(tftypes.String, nil),
				"active":                  tftypes.NewValue(tftypes.Bool, nil),
				"organization_public_key":  pubKey,
				"organization_private_key": privKey,
			}),
			Schema: dataSourceSchema,
		}

		var resp datasource.ValidateConfigResponse
		d.ValidateConfig(ctx, datasource.ValidateConfigRequest{Config: config}, &resp)

		if !resp.Diagnostics.HasError() {
			t.Fatalf("expected error when both id and email are set")
		}
	})

	t.Run("OnlyId", func(t *testing.T) {
		config := tfsdk.Config{
			Raw: buildUserDataSourceObjectValue(map[string]tftypes.Value{
				"id":                      tftypes.NewValue(tftypes.String, "user-123"),
				"email":                   tftypes.NewValue(tftypes.String, nil),
				"user_name":               tftypes.NewValue(tftypes.String, nil),
				"active":                  tftypes.NewValue(tftypes.Bool, nil),
				"organization_public_key":  pubKey,
				"organization_private_key": privKey,
			}),
			Schema: dataSourceSchema,
		}

		var resp datasource.ValidateConfigResponse
		d.ValidateConfig(ctx, datasource.ValidateConfigRequest{Config: config}, &resp)

		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected error when only id is set: %v", resp.Diagnostics)
		}
	})

	t.Run("OnlyEmail", func(t *testing.T) {
		config := tfsdk.Config{
			Raw: buildUserDataSourceObjectValue(map[string]tftypes.Value{
				"id":                      tftypes.NewValue(tftypes.String, nil),
				"email":                   tftypes.NewValue(tftypes.String, "user@example.com"),
				"user_name":               tftypes.NewValue(tftypes.String, nil),
				"active":                  tftypes.NewValue(tftypes.Bool, nil),
				"organization_public_key":  pubKey,
				"organization_private_key": privKey,
			}),
			Schema: dataSourceSchema,
		}

		var resp datasource.ValidateConfigResponse
		d.ValidateConfig(ctx, datasource.ValidateConfigRequest{Config: config}, &resp)

		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected error when only email is set: %v", resp.Diagnostics)
		}
	})
}

func TestUserDataSourceRead_ByID(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	d := NewUserDataSource().(*userDataSource)

	clientFactory := mocks.NewMockClientFactory(ctrl)

	var configureResp datasource.ConfigureResponse
	d.Configure(ctx, datasource.ConfigureRequest{ProviderData: clientFactory}, &configureResp)
	if configureResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Configure: %v", configureResp.Diagnostics)
	}

	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	dataSourceSchema := schemaResp.Schema

	userID := "user-abc"
	userName := "user@example.com"

	clientFactory.OrganizationClient.EXPECT().
		GetSCIMUser(ctx, userID).
		Return(&langfuse.SCIMUserResponse{
			ID:       userID,
			UserName: userName,
			Emails: []struct {
				Value   string `json:"value"`
				Primary bool   `json:"primary"`
			}{
				{Value: userName, Primary: true},
			},
			Active: true,
		}, nil)

	readConfig := tfsdk.Config{
		Raw: buildUserDataSourceObjectValue(map[string]tftypes.Value{
			"id":                      tftypes.NewValue(tftypes.String, userID),
			"email":                   tftypes.NewValue(tftypes.String, nil),
			"user_name":               tftypes.NewValue(tftypes.String, nil),
			"active":                  tftypes.NewValue(tftypes.Bool, nil),
			"organization_public_key":  tftypes.NewValue(tftypes.String, "pk-test"),
			"organization_private_key": tftypes.NewValue(tftypes.String, "sk-test"),
		}),
		Schema: dataSourceSchema,
	}

	var readResp datasource.ReadResponse
	readResp.State.Schema = dataSourceSchema

	d.Read(ctx, datasource.ReadRequest{Config: readConfig}, &readResp)

	if readResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Read: %v", readResp.Diagnostics)
	}

	var model userDataSourceModel
	diags := readResp.State.Get(ctx, &model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics getting model from state: %v", diags)
	}

	if model.ID.ValueString() != userID {
		t.Fatalf("unexpected ID. got %q, want %q", model.ID.ValueString(), userID)
	}
	if model.Email.ValueString() != userName {
		t.Fatalf("unexpected email. got %q, want %q", model.Email.ValueString(), userName)
	}
	if model.UserName.ValueString() != userName {
		t.Fatalf("unexpected user_name. got %q, want %q", model.UserName.ValueString(), userName)
	}
	if !model.Active.ValueBool() {
		t.Fatalf("expected active=true")
	}
}

func TestUserDataSourceRead_ByEmail(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	d := NewUserDataSource().(*userDataSource)

	clientFactory := mocks.NewMockClientFactory(ctrl)

	var configureResp datasource.ConfigureResponse
	d.Configure(ctx, datasource.ConfigureRequest{ProviderData: clientFactory}, &configureResp)
	if configureResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Configure: %v", configureResp.Diagnostics)
	}

	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	dataSourceSchema := schemaResp.Schema

	userID := "user-xyz"
	userEmail := "alice@example.com"

	clientFactory.OrganizationClient.EXPECT().
		FindSCIMUserByEmail(ctx, userEmail).
		Return(&langfuse.SCIMUserResponse{
			ID:       userID,
			UserName: userEmail,
			Emails: []struct {
				Value   string `json:"value"`
				Primary bool   `json:"primary"`
			}{
				{Value: userEmail, Primary: true},
			},
			Active: false,
		}, nil)

	readConfig := tfsdk.Config{
		Raw: buildUserDataSourceObjectValue(map[string]tftypes.Value{
			"id":                      tftypes.NewValue(tftypes.String, nil),
			"email":                   tftypes.NewValue(tftypes.String, userEmail),
			"user_name":               tftypes.NewValue(tftypes.String, nil),
			"active":                  tftypes.NewValue(tftypes.Bool, nil),
			"organization_public_key":  tftypes.NewValue(tftypes.String, "pk-test"),
			"organization_private_key": tftypes.NewValue(tftypes.String, "sk-test"),
		}),
		Schema: dataSourceSchema,
	}

	var readResp datasource.ReadResponse
	readResp.State.Schema = dataSourceSchema

	d.Read(ctx, datasource.ReadRequest{Config: readConfig}, &readResp)

	if readResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Read: %v", readResp.Diagnostics)
	}

	var model userDataSourceModel
	diags := readResp.State.Get(ctx, &model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics getting model from state: %v", diags)
	}

	if model.ID.ValueString() != userID {
		t.Fatalf("unexpected ID. got %q, want %q", model.ID.ValueString(), userID)
	}
	if model.Email.ValueString() != userEmail {
		t.Fatalf("unexpected email. got %q, want %q", model.Email.ValueString(), userEmail)
	}
	if model.UserName.ValueString() != userEmail {
		t.Fatalf("unexpected user_name. got %q, want %q", model.UserName.ValueString(), userEmail)
	}
	if model.Active.ValueBool() {
		t.Fatalf("expected active=false")
	}
}

func TestUserDataSourceRead_Error(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	d := NewUserDataSource().(*userDataSource)

	clientFactory := mocks.NewMockClientFactory(ctrl)

	var configureResp datasource.ConfigureResponse
	d.Configure(ctx, datasource.ConfigureRequest{ProviderData: clientFactory}, &configureResp)
	if configureResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Configure: %v", configureResp.Diagnostics)
	}

	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	dataSourceSchema := schemaResp.Schema

	userID := "user-bad"

	clientFactory.OrganizationClient.EXPECT().
		GetSCIMUser(ctx, userID).
		Return(nil, fmt.Errorf("not found"))

	readConfig := tfsdk.Config{
		Raw: buildUserDataSourceObjectValue(map[string]tftypes.Value{
			"id":                      tftypes.NewValue(tftypes.String, userID),
			"email":                   tftypes.NewValue(tftypes.String, nil),
			"user_name":               tftypes.NewValue(tftypes.String, nil),
			"active":                  tftypes.NewValue(tftypes.Bool, nil),
			"organization_public_key":  tftypes.NewValue(tftypes.String, "pk-test"),
			"organization_private_key": tftypes.NewValue(tftypes.String, "sk-test"),
		}),
		Schema: dataSourceSchema,
	}

	var readResp datasource.ReadResponse
	readResp.State.Schema = dataSourceSchema

	d.Read(ctx, datasource.ReadRequest{Config: readConfig}, &readResp)

	if !readResp.Diagnostics.HasError() {
		t.Fatalf("expected error when GetSCIMUser fails")
	}

	found := false
	for _, diag := range readResp.Diagnostics.Errors() {
		if diag.Summary() == "Error reading user by ID" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'Error reading user by ID' diagnostic, got: %v", readResp.Diagnostics)
	}
}
