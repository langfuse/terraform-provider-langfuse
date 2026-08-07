package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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
