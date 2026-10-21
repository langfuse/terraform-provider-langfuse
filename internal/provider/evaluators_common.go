package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
)

// Shared building blocks for the langfuse_evaluator and langfuse_evaluation_rule
// resources, both of which talk to the stable /api/public/v2 evaluators API
// with project-scoped credentials.

// promptVariableMappingSources lists the valid values of a variable mapping source.
var promptVariableMappingSources = []string{
	"input", "output", "metadata", "tool_calls", "expected_output", "experiment_item_metadata",
}

// variableMappingModel mirrors one prompt-variable mapping entry.
type variableMappingModel struct {
	Variable types.String `tfsdk:"variable"`
	Source   types.String `tfsdk:"source"`
	JSONPath types.String `tfsdk:"json_path"`
}

var variableMappingAttrTypes = map[string]attr.Type{
	"variable":  types.StringType,
	"source":    types.StringType,
	"json_path": types.StringType,
}

var variableMappingObjectType = types.ObjectType{AttrTypes: variableMappingAttrTypes}

// variableMappingsFromList converts a Terraform list of mappings into API
// mappings. A null or unknown list yields nil, which the API treats as "no
// default mapping" (evaluator) or "inherit the default mapping" (rule).
func variableMappingsFromList(ctx context.Context, list types.List) ([]langfuse.PromptVariableMapping, diag.Diagnostics) {
	if list.IsNull() || list.IsUnknown() {
		return nil, nil
	}

	var models []variableMappingModel
	diags := list.ElementsAs(ctx, &models, false)
	if diags.HasError() {
		return nil, diags
	}

	mappings := make([]langfuse.PromptVariableMapping, 0, len(models))
	for _, m := range models {
		mapping := langfuse.PromptVariableMapping{
			Variable: m.Variable.ValueString(),
			Source:   m.Source.ValueString(),
		}
		if !m.JSONPath.IsNull() && !m.JSONPath.IsUnknown() {
			v := m.JSONPath.ValueString()
			mapping.JSONPath = &v
		}
		mappings = append(mappings, mapping)
	}

	return mappings, nil
}

// variableMappingsToList converts API mappings into a Terraform list. An empty
// or nil API list becomes null so that an omitted attribute stays omitted.
func variableMappingsToList(ctx context.Context, mappings []langfuse.PromptVariableMapping) (types.List, diag.Diagnostics) {
	if len(mappings) == 0 {
		return types.ListNull(variableMappingObjectType), nil
	}

	models := make([]variableMappingModel, 0, len(mappings))
	for _, m := range mappings {
		model := variableMappingModel{
			Variable: types.StringValue(m.Variable),
			Source:   types.StringValue(m.Source),
			JSONPath: types.StringNull(),
		}
		if m.JSONPath != nil {
			model.JSONPath = types.StringValue(*m.JSONPath)
		}
		models = append(models, model)
	}

	return types.ListValueFrom(ctx, variableMappingObjectType, models)
}

// stringSliceToList converts a Go string slice into a Terraform list of strings.
// A nil slice becomes null; an empty slice becomes an empty list.
func stringSliceToList(values []string) types.List {
	if values == nil {
		return types.ListNull(types.StringType)
	}
	elems := make([]attr.Value, len(values))
	for i, v := range values {
		elems[i] = types.StringValue(v)
	}
	return types.ListValueMust(types.StringType, elems)
}

// stringListToSlice converts a Terraform list of strings into a Go slice.
// A null or unknown list yields nil.
func stringListToSlice(ctx context.Context, list types.List) ([]string, diag.Diagnostics) {
	if list.IsNull() || list.IsUnknown() {
		return nil, nil
	}
	var values []string
	diags := list.ElementsAs(ctx, &values, false)
	return values, diags
}

// optionalString returns a pointer to the string value, or nil when the value
// is null or unknown.
func optionalString(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

// stringPointerToValue converts a nullable API string into a Terraform string.
func stringPointerToValue(v *string) types.String {
	if v == nil {
		return types.StringNull()
	}
	return types.StringValue(*v)
}

// parseProjectScopedImportID splits an import ID of the form
// <project_public_key>:<project_secret_key>:<resource_id>.
func parseProjectScopedImportID(id, resourceName string) (publicKey, secretKey, resourceID string, err error) {
	parts := strings.SplitN(id, ":", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("import ID must be in the format: <project_public_key>:<project_secret_key>:<%s_id>", resourceName)
	}
	return parts[0], parts[1], parts[2], nil
}
