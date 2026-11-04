package provider

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/langfuse/terraform-provider-langfuse/internal/langfuse/mocks"
)

func TestNewOrganizationClient(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		providerPublic  string
		providerPrivate string
		resourcePublic  types.String
		resourcePrivate types.String
		wantPublic      string
		wantPrivate     string
		wantError       bool
	}{
		{
			name:            "resource keys take precedence",
			providerPublic:  "pk-provider",
			providerPrivate: "sk-provider",
			resourcePublic:  types.StringValue("pk-resource"),
			resourcePrivate: types.StringValue("sk-resource"),
			wantPublic:      "pk-resource",
			wantPrivate:     "sk-resource",
		},
		{
			name:            "falls back to provider keys when resource keys are null",
			providerPublic:  "pk-provider",
			providerPrivate: "sk-provider",
			resourcePublic:  types.StringNull(),
			resourcePrivate: types.StringNull(),
			wantPublic:      "pk-provider",
			wantPrivate:     "sk-provider",
		},
		{
			name:            "no keys anywhere",
			resourcePublic:  types.StringNull(),
			resourcePrivate: types.StringNull(),
			wantError:       true,
		},
		{
			name:            "only one resource key is set",
			providerPublic:  "pk-provider",
			providerPrivate: "sk-provider",
			resourcePublic:  types.StringValue("pk-resource"),
			resourcePrivate: types.StringNull(),
			wantError:       true,
		},
		{
			name:            "only one provider key is set",
			providerPublic:  "pk-provider",
			resourcePublic:  types.StringNull(),
			resourcePrivate: types.StringNull(),
			wantError:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			clientFactory := mocks.NewMockClientFactory(ctrl)
			clientFactory.DefaultOrganizationPublicKey = tc.providerPublic
			clientFactory.DefaultOrganizationPrivateKey = tc.providerPrivate

			client, diags := newOrganizationClient(clientFactory, tc.resourcePublic, tc.resourcePrivate)

			if tc.wantError {
				if !diags.HasError() || diags.Errors()[0].Summary() != "Missing organization credentials" {
					t.Fatalf("expected 'Missing organization credentials' error, got: %v", diags)
				}
				if client != nil {
					t.Errorf("expected no client on error")
				}
				return
			}

			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if clientFactory.UsedOrganizationPublicKey != tc.wantPublic || clientFactory.UsedOrganizationPrivateKey != tc.wantPrivate {
				t.Errorf("unexpected keys used: got %q/%q, want %q/%q",
					clientFactory.UsedOrganizationPublicKey, clientFactory.UsedOrganizationPrivateKey, tc.wantPublic, tc.wantPrivate)
			}
		})
	}
}
