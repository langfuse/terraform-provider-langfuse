package mocks

import (
	langfuse "github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
	gomock "github.com/golang/mock/gomock"
)

type mockClientFactory struct {
	AdminClient        *MockAdminClient
	OrganizationClient *MockOrganizationClient
	orgPublicKey       string
	orgPrivateKey      string
}

func NewMockClientFactory(ctrl *gomock.Controller) *mockClientFactory {
	return &mockClientFactory{
		AdminClient:        NewMockAdminClient(ctrl),
		OrganizationClient: NewMockOrganizationClient(ctrl),
	}
}

func (cf *mockClientFactory) NewAdminClient() langfuse.AdminClient {
	return cf.AdminClient
}

func (cf *mockClientFactory) NewOrganizationClient(publicKey, privateKey string) langfuse.OrganizationClient {
	return cf.OrganizationClient
}

func (cf *mockClientFactory) GetDefaultOrgPublicKey() string {
	return cf.orgPublicKey
}

func (cf *mockClientFactory) GetDefaultOrgPrivateKey() string {
	return cf.orgPrivateKey
}

func (cf *mockClientFactory) HasDefaultOrgCredentials() bool {
	return cf.orgPublicKey != "" && cf.orgPrivateKey != ""
}

// SetDefaultOrgCredentials is a helper method for tests to set provider-level credentials
func (cf *mockClientFactory) SetDefaultOrgCredentials(publicKey, privateKey string) {
	cf.orgPublicKey = publicKey
	cf.orgPrivateKey = privateKey
}
