package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse/mocks"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	resschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestProjectMembershipResourceMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewProjectMembershipResource().(*projectMembershipResource)

	var resp resource.MetadataResponse
	r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "langfuse"}, &resp)

	expected := "langfuse_project_membership"
	if resp.TypeName != expected {
		t.Fatalf("unexpected type name. got %q, want %q", resp.TypeName, expected)
	}
}

func TestProjectMembershipResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewProjectMembershipResource().(*projectMembershipResource)

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Schema: %v", schemaResp.Diagnostics)
	}
	if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema implementation validation failed: %v", diags)
	}

	schema := schemaResp.Schema

	expectedAttributes := []string{
		"id", "project_id", "user_id", "role", "email",
		"organization_public_key", "organization_private_key", "ignore_destroy",
	}

	for _, expectedAttr := range expectedAttributes {
		if _, exists := schema.Attributes[expectedAttr]; !exists {
			t.Errorf("expected attribute %q not found in schema", expectedAttr)
		}
	}

	idAttr, ok := schema.Attributes["id"].(resschema.StringAttribute)
	if !ok || !idAttr.Computed {
		t.Fatalf("'id' must be a computed string attribute")
	}

	projectIDAttr, ok := schema.Attributes["project_id"].(resschema.StringAttribute)
	if !ok || !projectIDAttr.Required {
		t.Fatalf("'project_id' must be a required string attribute")
	}

	userIDAttr, ok := schema.Attributes["user_id"].(resschema.StringAttribute)
	if !ok || !userIDAttr.Required {
		t.Fatalf("'user_id' must be a required string attribute")
	}

	roleAttr, ok := schema.Attributes["role"].(resschema.StringAttribute)
	if !ok || !roleAttr.Required {
		t.Fatalf("'role' must be a required string attribute")
	}

	emailAttr, ok := schema.Attributes["email"].(resschema.StringAttribute)
	if !ok || !emailAttr.Computed {
		t.Fatalf("'email' must be a computed string attribute")
	}
}

func TestProjectMembershipResourceCRUD(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	r, ok := NewProjectMembershipResource().(*projectMembershipResource)
	if !ok {
		t.Fatalf("factory did not return *projectMembershipResource")
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
			t.Fatalf("ClientFactory is nil after Configure")
		}
		var schemaResp resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
		if schemaResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Schema: %v", schemaResp.Diagnostics)
		}
		resourceSchema = schemaResp.Schema
	})

	projectID := "proj-123"
	userID := "user-456"

	var createResp resource.CreateResponse
	t.Run("Create", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().
			UpsertProjectMembership(ctx, projectID, &langfuse.UpsertProjectMemberRequest{
				UserID: userID,
				Role:   "MEMBER",
			}).
			Return(&langfuse.ProjectMembership{
				UserID: userID,
				Role:   "MEMBER",
				Email:  "user@example.com",
			}, nil)

		createResp.State.Schema = resourceSchema
		r.Create(ctx, resource.CreateRequest{
			Plan: tfsdk.Plan{
				Schema: resourceSchema,
				Raw: buildProjectMembershipObjectValue(map[string]tftypes.Value{
					"id":                       tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
					"project_id":               tftypes.NewValue(tftypes.String, projectID),
					"user_id":                  tftypes.NewValue(tftypes.String, userID),
					"role":                     tftypes.NewValue(tftypes.String, "MEMBER"),
					"email":                    tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
					"organization_public_key":  tftypes.NewValue(tftypes.String, "pub-key"),
					"organization_private_key": tftypes.NewValue(tftypes.String, "priv-key"),
					"ignore_destroy":           tftypes.NewValue(tftypes.Bool, nil),
				}),
			},
		}, &createResp)
		if createResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Create: %v", createResp.Diagnostics)
		}
	})

	var readResp resource.ReadResponse
	t.Run("Read", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().
			GetProjectMembership(ctx, projectID, userID).
			Return(&langfuse.ProjectMembership{
				UserID: userID,
				Role:   "MEMBER",
				Email:  "user@example.com",
			}, nil)

		readResp.State.Schema = resourceSchema
		r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)
		if readResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Read: %v", readResp.Diagnostics)
		}
	})

	var updateResp resource.UpdateResponse
	t.Run("Update", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().
			UpsertProjectMembership(ctx, projectID, &langfuse.UpsertProjectMemberRequest{
				UserID: userID,
				Role:   "ADMIN",
			}).
			Return(&langfuse.ProjectMembership{
				UserID: userID,
				Role:   "ADMIN",
				Email:  "user@example.com",
			}, nil)

		updateResp.State.Schema = resourceSchema
		r.Update(ctx, resource.UpdateRequest{
			Plan: tfsdk.Plan{
				Schema: resourceSchema,
				Raw: buildProjectMembershipObjectValue(map[string]tftypes.Value{
					"id":                       tftypes.NewValue(tftypes.String, fmt.Sprintf("%s/%s", projectID, userID)),
					"project_id":               tftypes.NewValue(tftypes.String, projectID),
					"user_id":                  tftypes.NewValue(tftypes.String, userID),
					"role":                     tftypes.NewValue(tftypes.String, "ADMIN"),
					"email":                    tftypes.NewValue(tftypes.String, "user@example.com"),
					"organization_public_key":  tftypes.NewValue(tftypes.String, "pub-key"),
					"organization_private_key": tftypes.NewValue(tftypes.String, "priv-key"),
					"ignore_destroy":           tftypes.NewValue(tftypes.Bool, nil),
				}),
			},
			State: readResp.State,
		}, &updateResp)
		if updateResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Update: %v", updateResp.Diagnostics)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().
			RemoveProjectMember(ctx, projectID, userID).
			Return(nil)

		var deleteResp resource.DeleteResponse
		deleteResp.State.Schema = resourceSchema
		r.Delete(ctx, resource.DeleteRequest{State: updateResp.State}, &deleteResp)
		if deleteResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Delete: %v", deleteResp.Diagnostics)
		}
	})
}

func TestProjectMembershipResource_Read_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	clientFactory := mocks.NewMockClientFactory(ctrl)
	r := &projectMembershipResource{}
	var configureResp resource.ConfigureResponse
	r.Configure(ctx, resource.ConfigureRequest{ProviderData: clientFactory}, &configureResp)

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	state := tfsdk.State{
		Schema: schemaResp.Schema,
		Raw: buildProjectMembershipObjectValue(map[string]tftypes.Value{
			"id":                       tftypes.NewValue(tftypes.String, "proj-123/user-456"),
			"project_id":               tftypes.NewValue(tftypes.String, "proj-123"),
			"user_id":                  tftypes.NewValue(tftypes.String, "user-456"),
			"role":                     tftypes.NewValue(tftypes.String, "MEMBER"),
			"email":                    tftypes.NewValue(tftypes.String, "user@example.com"),
			"organization_public_key":  tftypes.NewValue(tftypes.String, "pub-key"),
			"organization_private_key": tftypes.NewValue(tftypes.String, "priv-key"),
			"ignore_destroy":           tftypes.NewValue(tftypes.Bool, nil),
		}),
	}

	clientFactory.OrganizationClient.EXPECT().
		GetProjectMembership(ctx, "proj-123", "user-456").
		Return(nil, fmt.Errorf("cannot find project membership for user user-456 in project proj-123"))

	var resp resource.ReadResponse
	resp.State.Schema = schemaResp.Schema
	r.Read(ctx, resource.ReadRequest{State: state}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no diagnostics, got: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Fatal("expected state to be removed (null) when membership is not found")
	}
}

func TestProjectMembershipResource_Read_Error(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	clientFactory := mocks.NewMockClientFactory(ctrl)
	r := &projectMembershipResource{}
	var configureResp resource.ConfigureResponse
	r.Configure(ctx, resource.ConfigureRequest{ProviderData: clientFactory}, &configureResp)

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	state := tfsdk.State{
		Schema: schemaResp.Schema,
		Raw: buildProjectMembershipObjectValue(map[string]tftypes.Value{
			"id":                       tftypes.NewValue(tftypes.String, "proj-123/user-456"),
			"project_id":               tftypes.NewValue(tftypes.String, "proj-123"),
			"user_id":                  tftypes.NewValue(tftypes.String, "user-456"),
			"role":                     tftypes.NewValue(tftypes.String, "MEMBER"),
			"email":                    tftypes.NewValue(tftypes.String, "user@example.com"),
			"organization_public_key":  tftypes.NewValue(tftypes.String, "pub-key"),
			"organization_private_key": tftypes.NewValue(tftypes.String, "priv-key"),
			"ignore_destroy":           tftypes.NewValue(tftypes.Bool, nil),
		}),
	}

	clientFactory.OrganizationClient.EXPECT().
		GetProjectMembership(ctx, "proj-123", "user-456").
		Return(nil, fmt.Errorf("internal server error"))

	var resp resource.ReadResponse
	resp.State.Schema = schemaResp.Schema
	r.Read(ctx, resource.ReadRequest{State: state}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error diagnostic for non-404 error, got none")
	}
	errs := resp.Diagnostics.Errors()
	if errs[0].Summary() != "Error reading project membership" {
		t.Fatalf("unexpected error summary: %q", errs[0].Summary())
	}
}

func TestProjectMembershipResource_Delete_IgnoreDestroy(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	// No calls expected on the client — ignore_destroy=true skips deletion.
	clientFactory := mocks.NewMockClientFactory(ctrl)
	r := &projectMembershipResource{}
	var configureResp resource.ConfigureResponse
	r.Configure(ctx, resource.ConfigureRequest{ProviderData: clientFactory}, &configureResp)

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	state := tfsdk.State{
		Schema: schemaResp.Schema,
		Raw: buildProjectMembershipObjectValue(map[string]tftypes.Value{
			"id":                       tftypes.NewValue(tftypes.String, "proj-123/user-456"),
			"project_id":               tftypes.NewValue(tftypes.String, "proj-123"),
			"user_id":                  tftypes.NewValue(tftypes.String, "user-456"),
			"role":                     tftypes.NewValue(tftypes.String, "MEMBER"),
			"email":                    tftypes.NewValue(tftypes.String, "user@example.com"),
			"organization_public_key":  tftypes.NewValue(tftypes.String, "pub-key"),
			"organization_private_key": tftypes.NewValue(tftypes.String, "priv-key"),
			"ignore_destroy":           tftypes.NewValue(tftypes.Bool, true),
		}),
	}

	var deleteResp resource.DeleteResponse
	deleteResp.State.Schema = schemaResp.Schema
	r.Delete(ctx, resource.DeleteRequest{State: state}, &deleteResp)

	if deleteResp.Diagnostics.HasError() {
		t.Fatalf("expected no diagnostics when ignore_destroy=true, got: %v", deleteResp.Diagnostics)
	}
}

func buildProjectMembershipObjectValue(values map[string]tftypes.Value) tftypes.Value {
	return tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"id":                       tftypes.String,
				"project_id":               tftypes.String,
				"user_id":                  tftypes.String,
				"role":                     tftypes.String,
				"email":                    tftypes.String,
				"organization_public_key":  tftypes.String,
				"organization_private_key": tftypes.String,
				"ignore_destroy":           tftypes.Bool,
			},
			OptionalAttributes: map[string]struct{}{
				"id":             {},
				"email":          {},
				"ignore_destroy": {},
			},
		},
		values,
	)
}
