package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse/mocks"
)

func TestProjectMembershipResourceMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewProjectMembershipResource()

	var resp resource.MetadataResponse
	r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "langfuse"}, &resp)

	if resp.TypeName != "langfuse_project_membership" {
		t.Fatalf("unexpected type name. got %q, want %q", resp.TypeName, "langfuse_project_membership")
	}
}

func TestProjectMembershipResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewProjectMembershipResource()

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

	projectIDAttr, ok := schemaResp.Schema.Attributes["project_id"].(resschema.StringAttribute)
	if !ok || !projectIDAttr.Required {
		t.Fatalf("'project_id' attribute must be required string")
	}

	emailAttr, ok := schemaResp.Schema.Attributes["email"].(resschema.StringAttribute)
	if !ok || !emailAttr.Required {
		t.Fatalf("'email' attribute must be required string")
	}

	roleAttr, ok := schemaResp.Schema.Attributes["role"].(resschema.StringAttribute)
	if !ok || !roleAttr.Required {
		t.Fatalf("'role' attribute must be required string")
	}

	userIDAttr, ok := schemaResp.Schema.Attributes["user_id"].(resschema.StringAttribute)
	if !ok || !userIDAttr.Computed {
		t.Fatalf("'user_id' attribute must be a computed string")
	}

	nameAttr, ok := schemaResp.Schema.Attributes["name"].(resschema.StringAttribute)
	if !ok || !nameAttr.Computed {
		t.Fatalf("'name' attribute must be a computed string")
	}
}

func TestProjectMembershipResource_RoleValidation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	r := NewProjectMembershipResource()

	schemaResp := resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	roleAttr := schemaResp.Schema.Attributes["role"].(resschema.StringAttribute)

	tests := []struct {
		name        string
		role        string
		shouldError bool
	}{
		{"valid_owner", "OWNER", false},
		{"valid_admin", "ADMIN", false},
		{"valid_member", "MEMBER", false},
		{"valid_viewer", "VIEWER", false},
		{"valid_none", "NONE", false},
		{"invalid_super_admin", "SUPER_ADMIN", true},
		{"invalid_invalid_role", "INVALID_ROLE", true},
		{"invalid_lowercase", "admin", true},
		{"invalid_empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validator.StringRequest{
				ConfigValue: types.StringValue(tt.role),
			}
			resp := validator.StringResponse{}

			for _, v := range roleAttr.Validators {
				v.ValidateString(ctx, req, &resp)
			}

			hasError := resp.Diagnostics.HasError()
			if hasError != tt.shouldError {
				if tt.shouldError {
					t.Errorf("expected validation error for role %q, but got none", tt.role)
				} else {
					t.Errorf("expected no validation error for role %q, but got: %v", tt.role, resp.Diagnostics)
				}
			}
		})
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

	projectID := "proj-123"
	userEmail := "developer@company.com"
	publicKey := "pk-1234"
	privateKey := "sk-1234"

	var createResp resource.CreateResponse
	t.Run("Create", func(t *testing.T) {
		// First, ListMemberships is called to resolve email to UserID
		clientFactory.OrganizationClient.EXPECT().
			ListMemberships(ctx).
			Return([]langfuse.OrganizationMembership{
				{
					ID:     "orgmem-123",
					Email:  userEmail,
					UserID: "user-789",
				},
			}, nil)

		clientFactory.OrganizationClient.EXPECT().
			CreateOrUpdateProjectMembership(ctx, projectID, &langfuse.CreateProjectMembershipRequest{
				UserID: "user-789",
				Role:   "MEMBER",
			}).
			Return(&langfuse.ProjectMembership{
				UserID: "user-789",
				Role:   "MEMBER",
				Email:  userEmail,
				Name:   "developer",
			}, nil)

		createConfig := tfsdk.Config{
			Raw:    buildProjectMembershipObjectValue(projectID, userEmail, "MEMBER", publicKey, privateKey),
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
		clientFactory.OrganizationClient.EXPECT().
			GetProjectMembership(ctx, projectID, "user-789").
			Return(&langfuse.ProjectMembership{
				UserID: "user-789",
				Role:   "MEMBER",
				Email:  userEmail,
				Name:   "developer",
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
				UserID: "user-789",
				Role:   "ADMIN",
			}).
			Return(&langfuse.ProjectMembership{
				UserID: "user-789",
				Role:   "ADMIN",
				Email:  userEmail,
				Name:   "developer",
			}, nil)

		updateConfig := tfsdk.Config{
			Raw:    buildProjectMembershipStateValue(projectID, userEmail, "ADMIN", "user-789", "developer", publicKey, privateKey),
			Schema: resourceSchema,
		}
		updateResp.State.Schema = resourceSchema

		r.Update(ctx, resource.UpdateRequest{Config: updateConfig, State: readResp.State}, &updateResp)

		if updateResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Update: %v", updateResp.Diagnostics)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().
			DeleteProjectMembership(ctx, projectID, "user-789").
			Return(nil)

		var deleteResp resource.DeleteResponse
		deleteResp.State.Schema = resourceSchema

		r.Delete(ctx, resource.DeleteRequest{State: updateResp.State}, &deleteResp)

		if deleteResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from Delete: %v", deleteResp.Diagnostics)
		}
	})
}

func TestProjectMembershipResourceErrorHandling(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	r, ok := NewProjectMembershipResource().(*projectMembershipResource)
	if !ok {
		t.Fatalf("NewProjectMembershipResource did not return *projectMembershipResource")
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
	resourceSchema := schemaResp.Schema

	projectID := "proj-123"
	userEmail := "developer@company.com"
	publicKey := "pk-1234"
	privateKey := "sk-1234"

	t.Run("Create_UserNotFoundInOrganization", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().
			ListMemberships(ctx).
			Return([]langfuse.OrganizationMembership{
				{
					ID:     "orgmem-999",
					Email:  "other@company.com",
					UserID: "user-999",
				},
			}, nil)

		createConfig := tfsdk.Config{
			Raw:    buildProjectMembershipObjectValue(projectID, userEmail, "MEMBER", publicKey, privateKey),
			Schema: resourceSchema,
		}

		var createResp resource.CreateResponse
		createResp.State.Schema = resourceSchema

		r.Create(ctx, resource.CreateRequest{Config: createConfig}, &createResp)

		if !createResp.Diagnostics.HasError() {
			t.Fatal("expected error when user email not found in organization, but got none")
		}

		hasExpectedError := false
		for _, diag := range createResp.Diagnostics.Errors() {
			if diag.Summary() == "User not found" {
				hasExpectedError = true
				break
			}
		}

		if !hasExpectedError {
			t.Errorf("expected 'User not found' error, got: %v", createResp.Diagnostics)
		}
	})

	t.Run("Create_ListMembershipsError", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().
			ListMemberships(ctx).
			Return(nil, fmt.Errorf("API error: rate limit exceeded"))

		createConfig := tfsdk.Config{
			Raw:    buildProjectMembershipObjectValue(projectID, userEmail, "MEMBER", publicKey, privateKey),
			Schema: resourceSchema,
		}

		var createResp resource.CreateResponse
		createResp.State.Schema = resourceSchema

		r.Create(ctx, resource.CreateRequest{Config: createConfig}, &createResp)

		if !createResp.Diagnostics.HasError() {
			t.Fatal("expected error when ListMemberships fails, but got none")
		}

		hasExpectedError := false
		for _, diag := range createResp.Diagnostics.Errors() {
			if diag.Summary() == "Error listing organization memberships" {
				hasExpectedError = true
				break
			}
		}

		if !hasExpectedError {
			t.Errorf("expected 'Error listing organization memberships' error, got: %v", createResp.Diagnostics)
		}
	})

	t.Run("Create_CreateOrUpdateProjectMembershipError", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().
			ListMemberships(ctx).
			Return([]langfuse.OrganizationMembership{
				{
					ID:     "orgmem-123",
					Email:  userEmail,
					UserID: "user-789",
				},
			}, nil)

		clientFactory.OrganizationClient.EXPECT().
			CreateOrUpdateProjectMembership(ctx, projectID, &langfuse.CreateProjectMembershipRequest{
				UserID: "user-789",
				Role:   "MEMBER",
			}).
			Return(nil, fmt.Errorf("API error: insufficient permissions"))

		createConfig := tfsdk.Config{
			Raw:    buildProjectMembershipObjectValue(projectID, userEmail, "MEMBER", publicKey, privateKey),
			Schema: resourceSchema,
		}

		var createResp resource.CreateResponse
		createResp.State.Schema = resourceSchema

		r.Create(ctx, resource.CreateRequest{Config: createConfig}, &createResp)

		if !createResp.Diagnostics.HasError() {
			t.Fatal("expected error when CreateOrUpdateProjectMembership fails, but got none")
		}

		hasExpectedError := false
		for _, diag := range createResp.Diagnostics.Errors() {
			if diag.Summary() == "Error creating project membership" {
				hasExpectedError = true
				break
			}
		}

		if !hasExpectedError {
			t.Errorf("expected 'Error creating project membership' error, got: %v", createResp.Diagnostics)
		}
	})

	t.Run("Read_GetProjectMembershipError", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().
			GetProjectMembership(ctx, projectID, "user-789").
			Return(nil, fmt.Errorf("API error: service unavailable"))

		state := tfsdk.State{
			Raw:    buildProjectMembershipStateValue(projectID, userEmail, "MEMBER", "user-789", "developer", publicKey, privateKey),
			Schema: resourceSchema,
		}

		var readResp resource.ReadResponse
		readResp.State.Schema = resourceSchema

		r.Read(ctx, resource.ReadRequest{State: state}, &readResp)

		if !readResp.Diagnostics.HasError() {
			t.Fatal("expected error when GetProjectMembership fails, but got none")
		}

		hasExpectedError := false
		for _, diag := range readResp.Diagnostics.Errors() {
			if diag.Summary() == "Error reading project membership" {
				hasExpectedError = true
				break
			}
		}

		if !hasExpectedError {
			t.Errorf("expected 'Error reading project membership' error, got: %v", readResp.Diagnostics)
		}
	})

	t.Run("Read_MembershipNotFound_RemovesResource", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().
			GetProjectMembership(ctx, projectID, "user-789").
			Return(nil, fmt.Errorf("cannot find project membership"))

		state := tfsdk.State{
			Raw:    buildProjectMembershipStateValue(projectID, userEmail, "MEMBER", "user-789", "developer", publicKey, privateKey),
			Schema: resourceSchema,
		}

		var readResp resource.ReadResponse
		readResp.State.Schema = resourceSchema
		readResp.State = state

		r.Read(ctx, resource.ReadRequest{State: state}, &readResp)

		if readResp.Diagnostics.HasError() {
			t.Fatalf("unexpected error when membership not found: %v", readResp.Diagnostics)
		}

		// Verify the state was removed
		if !readResp.State.Raw.IsNull() {
			t.Error("expected state to be removed when membership not found, but it was not")
		}
	})

	t.Run("Update_CreateOrUpdateProjectMembershipError", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().
			CreateOrUpdateProjectMembership(ctx, projectID, &langfuse.CreateProjectMembershipRequest{
				UserID: "user-789",
				Role:   "ADMIN",
			}).
			Return(nil, fmt.Errorf("API error: invalid role transition"))

		state := tfsdk.State{
			Raw:    buildProjectMembershipStateValue(projectID, userEmail, "MEMBER", "user-789", "developer", publicKey, privateKey),
			Schema: resourceSchema,
		}

		config := tfsdk.Config{
			Raw:    buildProjectMembershipStateValue(projectID, userEmail, "ADMIN", "user-789", "developer", publicKey, privateKey),
			Schema: resourceSchema,
		}

		var updateResp resource.UpdateResponse
		updateResp.State.Schema = resourceSchema

		r.Update(ctx, resource.UpdateRequest{Config: config, State: state}, &updateResp)

		if !updateResp.Diagnostics.HasError() {
			t.Fatal("expected error when CreateOrUpdateProjectMembership fails during update, but got none")
		}

		hasExpectedError := false
		for _, diag := range updateResp.Diagnostics.Errors() {
			if diag.Summary() == "Error updating project membership" {
				hasExpectedError = true
				break
			}
		}

		if !hasExpectedError {
			t.Errorf("expected 'Error updating project membership' error, got: %v", updateResp.Diagnostics)
		}
	})

	t.Run("Delete_DeleteProjectMembershipError", func(t *testing.T) {
		clientFactory.OrganizationClient.EXPECT().
			DeleteProjectMembership(ctx, projectID, "user-789").
			Return(fmt.Errorf("API error: cannot remove last owner"))

		state := tfsdk.State{
			Raw:    buildProjectMembershipStateValue(projectID, userEmail, "OWNER", "user-789", "developer", publicKey, privateKey),
			Schema: resourceSchema,
		}

		var deleteResp resource.DeleteResponse
		deleteResp.State.Schema = resourceSchema

		r.Delete(ctx, resource.DeleteRequest{State: state}, &deleteResp)

		if !deleteResp.Diagnostics.HasError() {
			t.Fatal("expected error when DeleteProjectMembership fails, but got none")
		}

		hasExpectedError := false
		for _, diag := range deleteResp.Diagnostics.Errors() {
			if diag.Summary() == "Error removing project member" {
				hasExpectedError = true
				break
			}
		}

		if !hasExpectedError {
			t.Errorf("expected 'Error removing project member' error, got: %v", deleteResp.Diagnostics)
		}
	})
}

func TestProjectMembershipResourceImport(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	r, ok := NewProjectMembershipResource().(*projectMembershipResource)
	if !ok {
		t.Fatalf("NewProjectMembershipResource did not return *projectMembershipResource")
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

	projectID := "proj-123"
	userID := "user-789"
	publicKey := "pk-import"
	privateKey := "sk-import"

	t.Run("Successful import", func(t *testing.T) {
		importID := projectID + "," + userID + "," + publicKey + "," + privateKey

		clientFactory.OrganizationClient.EXPECT().
			GetProjectMembership(ctx, projectID, userID).
			Return(&langfuse.ProjectMembership{
				UserID: userID,
				Role:   "VIEWER",
				Email:  "imported@example.com",
				Name:   "imported_user",
			}, nil)

		var importResp resource.ImportStateResponse
		importResp.State.Schema = schemaResp.Schema

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

		if model.ProjectID.ValueString() != projectID {
			t.Errorf("expected ProjectID %q, got %q", projectID, model.ProjectID.ValueString())
		}

		if model.Role.ValueString() != "VIEWER" {
			t.Errorf("expected Role %q, got %q", "VIEWER", model.Role.ValueString())
		}

		if model.Email.ValueString() != "imported@example.com" {
			t.Errorf("expected Email %q, got %q", "imported@example.com", model.Email.ValueString())
		}

		if model.OrganizationPublicKey.ValueString() != publicKey {
			t.Errorf("expected OrganizationPublicKey %q, got %q", publicKey, model.OrganizationPublicKey.ValueString())
		}

		if model.OrganizationPrivateKey.ValueString() != privateKey {
			t.Errorf("expected OrganizationPrivateKey %q, got %q", privateKey, model.OrganizationPrivateKey.ValueString())
		}
	})

	t.Run("Invalid import format", func(t *testing.T) {
		testCases := []struct {
			name     string
			importID string
		}{
			{"too_few_parts", "proj-123,user-789"},
			{"too_many_parts", "proj-123,user-789,pk-1234,sk-1234,extra"},
			{"single_part", "proj-123"},
			{"empty_string", ""},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				var importResp resource.ImportStateResponse
				importResp.State.Schema = schemaResp.Schema

				r.ImportState(ctx, resource.ImportStateRequest{ID: tc.importID}, &importResp)

				if !importResp.Diagnostics.HasError() {
					t.Fatalf("expected error for invalid import format %q, but got none", tc.importID)
				}

				hasExpectedError := false
				for _, diag := range importResp.Diagnostics.Errors() {
					if diag.Summary() == "Invalid import format" {
						hasExpectedError = true
						break
					}
				}

				if !hasExpectedError {
					t.Errorf("expected 'Invalid import format' error, got: %v", importResp.Diagnostics)
				}
			})
		}
	})

	t.Run("Import with API error", func(t *testing.T) {
		importID := projectID + "," + userID + "," + publicKey + "," + privateKey

		clientFactory.OrganizationClient.EXPECT().
			GetProjectMembership(ctx, projectID, userID).
			Return(nil, fmt.Errorf("API error: unauthorized"))

		var importResp resource.ImportStateResponse
		importResp.State.Schema = schemaResp.Schema

		r.ImportState(ctx, resource.ImportStateRequest{ID: importID}, &importResp)

		if !importResp.Diagnostics.HasError() {
			t.Fatal("expected error when GetProjectMembership fails during import, but got none")
		}

		hasExpectedError := false
		for _, diag := range importResp.Diagnostics.Errors() {
			if diag.Summary() == "Error importing project membership" {
				hasExpectedError = true
				break
			}
		}

		if !hasExpectedError {
			t.Errorf("expected 'Error importing project membership' error, got: %v", importResp.Diagnostics)
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
				"name":                     tftypes.String,
				"organization_public_key":  tftypes.String,
				"organization_private_key": tftypes.String,
			},
			OptionalAttributes: map[string]struct{}{
				"id":      {},
				"user_id": {},
				"name":    {},
			},
		},
		map[string]tftypes.Value{
			"id":                       tftypes.NewValue(tftypes.String, nil),
			"project_id":               tftypes.NewValue(tftypes.String, projectID),
			"email":                    tftypes.NewValue(tftypes.String, email),
			"role":                     tftypes.NewValue(tftypes.String, role),
			"user_id":                  tftypes.NewValue(tftypes.String, nil),
			"name":                     tftypes.NewValue(tftypes.String, nil),
			"organization_public_key":  tftypes.NewValue(tftypes.String, publicKey),
			"organization_private_key": tftypes.NewValue(tftypes.String, privateKey),
		},
	)
}

func buildProjectMembershipStateValue(projectID, email, role, userID, name, publicKey, privateKey string) tftypes.Value {
	return tftypes.NewValue(
		tftypes.Object{
			AttributeTypes: map[string]tftypes.Type{
				"id":                       tftypes.String,
				"project_id":               tftypes.String,
				"email":                    tftypes.String,
				"role":                     tftypes.String,
				"user_id":                  tftypes.String,
				"name":                     tftypes.String,
				"organization_public_key":  tftypes.String,
				"organization_private_key": tftypes.String,
			},
			OptionalAttributes: map[string]struct{}{
				"id":      {},
				"user_id": {},
				"name":    {},
			},
		},
		map[string]tftypes.Value{
			"id":                       tftypes.NewValue(tftypes.String, userID),
			"project_id":               tftypes.NewValue(tftypes.String, projectID),
			"email":                    tftypes.NewValue(tftypes.String, email),
			"role":                     tftypes.NewValue(tftypes.String, role),
			"user_id":                  tftypes.NewValue(tftypes.String, userID),
			"name":                     tftypes.NewValue(tftypes.String, name),
			"organization_public_key":  tftypes.NewValue(tftypes.String, publicKey),
			"organization_private_key": tftypes.NewValue(tftypes.String, privateKey),
		},
	)
}
