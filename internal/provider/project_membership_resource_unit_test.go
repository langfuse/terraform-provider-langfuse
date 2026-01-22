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

	expectedAttributes := []string{
		"id", "project_id", "email", "role", "user_id", "username",
		"organization_public_key", "organization_private_key",
	}

	for _, expectedAttr := range expectedAttributes {
		if _, exists := schemaResp.Schema.Attributes[expectedAttr]; !exists {
			t.Errorf("expected attribute %q not found in schema", expectedAttr)
		}
	}
}

func TestProjectMembershipResourceCRUD(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	r, ok := NewProjectMembershipResource().(*projectMembershipResource)
	if !ok {
		t.Fatalf("NewProjectMembershipResource did not return *projectMembershipResource")
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
	userEmail := "developer@company.com"
	publicKey := "pk-1234"
	privateKey := "sk-1234"

	var createResp resource.CreateResponse
	t.Run("Create", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().
			CreateOrUpdateProjectMembership(ctx, projectID, &langfuse.CreateProjectMembershipRequest{
				Email: userEmail,
				Role:  "MEMBER",
			}).
			Return(&langfuse.ProjectMembership{
				ID:       "mem-456",
				UserID:   "user-789",
				Role:     "MEMBER",
				Email:    userEmail,
				Username: "developer",
			}, nil)

		createPlan := tfsdk.Plan{
			Raw:    buildProjectMembershipObjectValue(projectID, userEmail, "MEMBER", publicKey, privateKey),
			Schema: resourceSchema,
		}
		createResp.State.Schema = resourceSchema

		r.Create(ctx, resource.CreateRequest{Plan: createPlan}, &createResp)

		if createResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Create: %v", createResp.Diagnostics)
		}
	})

	var readResp resource.ReadResponse
	t.Run("Read", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().
			GetProjectMembership(ctx, projectID, "mem-456").
			Return(&langfuse.ProjectMembership{
				ID:       "mem-456",
				UserID:   "user-789",
				Role:     "MEMBER",
				Email:    userEmail,
				Username: "developer",
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
			CreateOrUpdateProjectMembership(ctx, projectID, &langfuse.CreateProjectMembershipRequest{
				Email: userEmail,
				Role:  "ADMIN",
			}).
			Return(&langfuse.ProjectMembership{
				ID:       "mem-456",
				UserID:   "user-789",
				Role:     "ADMIN",
				Email:    userEmail,
				Username: "developer",
			}, nil)

		updatePlan := tfsdk.Plan{
			Raw:    buildProjectMembershipObjectValue(projectID, userEmail, "ADMIN", publicKey, privateKey),
			Schema: resourceSchema,
		}
		updateResp.State.Schema = resourceSchema

		r.Update(ctx, resource.UpdateRequest{Plan: updatePlan, State: readResp.State}, &updateResp)

		if updateResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Update: %v", updateResp.Diagnostics)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().
			DeleteProjectMembership(ctx, projectID, userEmail).
			Return(nil)

		var deleteResp resource.DeleteResponse
		deleteResp.State.Schema = resourceSchema

		r.Delete(ctx, resource.DeleteRequest{State: updateResp.State}, &deleteResp)

		if deleteResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Delete: %v", deleteResp.Diagnostics)
		}
	})

	t.Run("ImportState", func(t *testing.T) {
		importID := "proj-123,mem-456,pk-import,sk-import"

		clientFactory.OrganizationClient.EXPECT().
			GetProjectMembership(ctx, "proj-123", "mem-456").
			Return(&langfuse.ProjectMembership{
				ID:       "mem-456",
				UserID:   "user-789",
				Role:     "VIEWER",
				Email:    "imported@example.com",
				Username: "imported_user",
			}, nil)

		var importResp resource.ImportStateResponse
		importResp.State.Schema = resourceSchema

		r.ImportState(ctx, resource.ImportStateRequest{ID: importID}, &importResp)

		if importResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from ImportState: %v", importResp.Diagnostics)
		}

		// Verify that the state was set correctly
		var model projectMembershipResourceModel
		diags := importResp.State.Get(ctx, &model)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics getting model from imported state: %v", diags)
		}

		if model.ProjectID.ValueString() != "proj-123" {
			t.Fatalf("unexpected project_id in imported state. got %q, want %q", model.ProjectID.ValueString(), "proj-123")
		}

		if model.Role.ValueString() != "VIEWER" {
			t.Fatalf("unexpected role in imported state. got %q, want %q", model.Role.ValueString(), "VIEWER")
		}

		if model.Email.ValueString() != "imported@example.com" {
			t.Fatalf("unexpected email in imported state. got %q, want %q", model.Email.ValueString(), "imported@example.com")
		}
	})
}

func buildProjectMembershipObjectValue(projectID, email, role, publicKey, privateKey string) tftypes.Value {
	return tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"id":                       tftypes.String,
				"project_id":               tftypes.String,
				"email":                    tftypes.String,
				"role":                     tftypes.String,
				"user_id":                  tftypes.String,
				"username":                 tftypes.String,
				"organization_public_key":  tftypes.String,
				"organization_private_key": tftypes.String,
			},
		},
		map[string]tftypes.Value{
			"id":                       tftypes.NewValue(tftypes.String, nil),
			"project_id":               tftypes.NewValue(tftypes.String, projectID),
			"email":                    tftypes.NewValue(tftypes.String, email),
			"role":                     tftypes.NewValue(tftypes.String, role),
			"user_id":                  tftypes.NewValue(tftypes.String, nil),
			"username":                 tftypes.NewValue(tftypes.String, nil),
			"organization_public_key":  tftypes.NewValue(tftypes.String, publicKey),
			"organization_private_key": tftypes.NewValue(tftypes.String, privateKey),
		},
	)
}
