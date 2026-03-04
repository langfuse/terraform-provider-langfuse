package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse/mocks"
)

func TestOrganizationMembershipResourceMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewOrganizationMembershipResource().(*organizationMembershipResource)

	var resp resource.MetadataResponse
	r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "langfuse"}, &resp)

	expected := "langfuse_organization_membership"
	if resp.TypeName != expected {
		t.Fatalf("unexpected type name. got %q, want %q", resp.TypeName, expected)
	}
}

func TestOrganizationMembershipResourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	r := NewOrganizationMembershipResource().(*organizationMembershipResource)

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
		"id", "email", "role", "status", "user_id", "username",
		"organization_public_key", "organization_private_key",
	}

	for _, expectedAttr := range expectedAttributes {
		if _, exists := schema.Attributes[expectedAttr]; !exists {
			t.Errorf("expected attribute %q not found in schema", expectedAttr)
		}
	}
}

func TestOrganizationMembershipResource_Create_InvalidRole(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create resource
	r := NewOrganizationMembershipResource().(*organizationMembershipResource)

	// Create request
	req := resource.CreateRequest{}
	resp := &resource.CreateResponse{}

	// Set up plan data with invalid role
	planValue := map[string]tftypes.Value{
		"id":                       tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"email":                    tftypes.NewValue(tftypes.String, "test@example.com"),
		"role":                     tftypes.NewValue(tftypes.String, "INVALID_ROLE"),
		"status":                   tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"user_id":                  tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"username":                 tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"organization_public_key":  tftypes.NewValue(tftypes.String, "test-public"),
		"organization_private_key": tftypes.NewValue(tftypes.String, "test-private"),
	}

	schemaResp := resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	req.Plan = tfsdk.Plan{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), planValue),
	}

	// Call Create
	r.Create(ctx, req, resp)

	// Assert error occurred
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for invalid role, but got none")
	}

	errorSummary := resp.Diagnostics.Errors()[0].Summary()
	if errorSummary != "Invalid Role" {
		t.Fatalf("unexpected error summary. got %q, want %q", errorSummary, "Invalid Role")
	}
}

func TestOrganizationMembershipResource_Update_InvalidRole(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create resource
	r := NewOrganizationMembershipResource().(*organizationMembershipResource)

	// Create request
	req := resource.UpdateRequest{}
	resp := &resource.UpdateResponse{}

	// Set up plan data with invalid role
	planValue := map[string]tftypes.Value{
		"id":                       tftypes.NewValue(tftypes.String, "membership-123"),
		"email":                    tftypes.NewValue(tftypes.String, "test@example.com"),
		"role":                     tftypes.NewValue(tftypes.String, "SUPER_ADMIN"),
		"status":                   tftypes.NewValue(tftypes.String, "ACTIVE"),
		"user_id":                  tftypes.NewValue(tftypes.String, "user-123"),
		"username":                 tftypes.NewValue(tftypes.String, "testuser"),
		"organization_public_key":  tftypes.NewValue(tftypes.String, "test-public"),
		"organization_private_key": tftypes.NewValue(tftypes.String, "test-private"),
	}

	stateValue := map[string]tftypes.Value{
		"id":                       tftypes.NewValue(tftypes.String, "membership-123"),
		"email":                    tftypes.NewValue(tftypes.String, "test@example.com"),
		"role":                     tftypes.NewValue(tftypes.String, "MEMBER"),
		"status":                   tftypes.NewValue(tftypes.String, "ACTIVE"),
		"user_id":                  tftypes.NewValue(tftypes.String, "user-123"),
		"username":                 tftypes.NewValue(tftypes.String, "testuser"),
		"organization_public_key":  tftypes.NewValue(tftypes.String, "test-public"),
		"organization_private_key": tftypes.NewValue(tftypes.String, "test-private"),
	}

	schemaResp := resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	req.Plan = tfsdk.Plan{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), planValue),
	}

	req.State = tfsdk.State{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), stateValue),
	}

	// Call Update
	r.Update(ctx, req, resp)

	// Assert error occurred
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error for invalid role, but got none")
	}

	errorSummary := resp.Diagnostics.Errors()[0].Summary()
	if errorSummary != "Invalid Role" {
		t.Fatalf("unexpected error summary. got %q, want %q", errorSummary, "Invalid Role")
	}
}

func TestOrganizationMembershipResourceImport(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	r := NewOrganizationMembershipResource().(*organizationMembershipResource)

	clientFactory := mocks.NewMockClientFactory(ctrl)

	// Configure the resource
	var configureResp resource.ConfigureResponse
	r.Configure(ctx, resource.ConfigureRequest{ProviderData: clientFactory}, &configureResp)
	if configureResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Configure: %v", configureResp.Diagnostics)
	}

	// Get the schema
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics from Schema: %v", schemaResp.Diagnostics)
	}

	t.Run("Successful import", func(t *testing.T) {
		membershipID := "mem-123"
		publicKey := "pk-456"
		privateKey := "sk-789"

		// Mock the GetMembership call
		clientFactory.OrganizationClient.EXPECT().
			GetMembership(ctx, membershipID).
			Return(&langfuse.OrganizationMembership{
				ID:       membershipID,
				Email:    "test@example.com",
				Role:     "ADMIN",
				Status:   "ACTIVE",
				UserID:   "user-123",
				Username: "testuser",
			}, nil)

		importID := membershipID + "," + publicKey + "," + privateKey

		var importResp resource.ImportStateResponse
		importResp.State.Schema = schemaResp.Schema

		r.ImportState(ctx, resource.ImportStateRequest{ID: importID}, &importResp)

		if importResp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics from ImportState: %v", importResp.Diagnostics)
		}

		// Verify the imported state
		var stateData organizationMembershipResourceModel
		importResp.State.Get(ctx, &stateData)

		if stateData.ID.ValueString() != membershipID {
			t.Errorf("expected ID %q, got %q", membershipID, stateData.ID.ValueString())
		}
		if stateData.Email.ValueString() != "test@example.com" {
			t.Errorf("expected Email %q, got %q", "test@example.com", stateData.Email.ValueString())
		}
		if stateData.Role.ValueString() != "ADMIN" {
			t.Errorf("expected Role %q, got %q", "ADMIN", stateData.Role.ValueString())
		}
		if stateData.Status.ValueString() != "ACTIVE" {
			t.Errorf("expected Status %q, got %q", "ACTIVE", stateData.Status.ValueString())
		}
		if stateData.UserID.ValueString() != "user-123" {
			t.Errorf("expected UserID %q, got %q", "user-123", stateData.UserID.ValueString())
		}
		if stateData.Username.ValueString() != "testuser" {
			t.Errorf("expected Username %q, got %q", "testuser", stateData.Username.ValueString())
		}
		if stateData.OrganizationPublicKey.ValueString() != publicKey {
			t.Errorf("expected OrganizationPublicKey %q, got %q", publicKey, stateData.OrganizationPublicKey.ValueString())
		}
		if stateData.OrganizationPrivateKey.ValueString() != privateKey {
			t.Errorf("expected OrganizationPrivateKey %q, got %q", privateKey, stateData.OrganizationPrivateKey.ValueString())
		}
	})

	t.Run("Invalid import format - missing parts", func(t *testing.T) {
		// Only membership ID, missing public and private keys
		// This now triggers the new "Missing Organization Credentials for Import" error
		// since format "mem-123" is valid but requires provider credentials
		importID := "mem-123"

		var importResp resource.ImportStateResponse
		importResp.State.Schema = schemaResp.Schema

		r.ImportState(ctx, resource.ImportStateRequest{ID: importID}, &importResp)

		if !importResp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error for missing provider credentials")
		}

		errorFound := false
		for _, diag := range importResp.Diagnostics {
			if diag.Summary() == "Missing Organization Credentials for Import" {
				errorFound = true
				break
			}
		}
		if !errorFound {
			t.Error("expected 'Missing Organization Credentials for Import' error message")
		}
	})

	t.Run("Invalid import format - too many parts", func(t *testing.T) {
		// Too many parts
		importID := "mem-123,pk-456,sk-789,extra-part"

		var importResp resource.ImportStateResponse
		importResp.State.Schema = schemaResp.Schema

		r.ImportState(ctx, resource.ImportStateRequest{ID: importID}, &importResp)

		if !importResp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error for invalid import format")
		}

		errorFound := false
		for _, diag := range importResp.Diagnostics {
			if diag.Summary() == "Invalid import format" {
				errorFound = true
				break
			}
		}
		if !errorFound {
			t.Error("expected 'Invalid import format' error message")
		}
	})

	t.Run("Invalid import format - only two parts", func(t *testing.T) {
		// Two parts instead of three
		importID := "mem-123,pk-456"

		var importResp resource.ImportStateResponse
		importResp.State.Schema = schemaResp.Schema

		r.ImportState(ctx, resource.ImportStateRequest{ID: importID}, &importResp)

		if !importResp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error for invalid import format")
		}

		errorFound := false
		for _, diag := range importResp.Diagnostics {
			if diag.Summary() == "Invalid import format" {
				errorFound = true
				break
			}
		}
		if !errorFound {
			t.Error("expected 'Invalid import format' error message")
		}
	})

	t.Run("Import with API error", func(t *testing.T) {
		membershipID := "mem-nonexistent"
		publicKey := "pk-456"
		privateKey := "sk-789"

		// Mock API error
		clientFactory.OrganizationClient.EXPECT().
			GetMembership(ctx, membershipID).
			Return(nil, fmt.Errorf("cannot find membership"))

		importID := membershipID + "," + publicKey + "," + privateKey

		var importResp resource.ImportStateResponse
		importResp.State.Schema = schemaResp.Schema

		r.ImportState(ctx, resource.ImportStateRequest{ID: importID}, &importResp)

		if !importResp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error for API error")
		}

		errorFound := false
		for _, diag := range importResp.Diagnostics {
			if diag.Summary() == "Error importing membership" {
				errorFound = true
				break
			}
		}
		if !errorFound {
			t.Error("expected 'Error importing membership' error message")
		}
	})
}
