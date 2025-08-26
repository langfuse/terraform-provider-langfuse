package mocks

import (
	langfuse "github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
	gomock "github.com/golang/mock/gomock"
)

type mockClientFactory struct {
	AdminClient        *MockAdminClient
	OrganizationClient *MockOrganizationClient
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
