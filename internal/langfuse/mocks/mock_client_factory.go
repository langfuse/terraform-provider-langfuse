package mocks

import (
	gomock "github.com/golang/mock/gomock"
	langfuse "github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
)

type mockClientFactory struct {
	AdminClient          *MockAdminClient
	OrganizationClient   *MockOrganizationClient
	LlmConnectionsClient *MockLlmConnectionsClient
}

func NewMockClientFactory(ctrl *gomock.Controller) *mockClientFactory {
	return &mockClientFactory{
		AdminClient:          NewMockAdminClient(ctrl),
		OrganizationClient:   NewMockOrganizationClient(ctrl),
		LlmConnectionsClient: NewMockLlmConnectionsClient(ctrl),
	}
}

func (cf *mockClientFactory) NewAdminClient() langfuse.AdminClient {
	return cf.AdminClient
}

func (cf *mockClientFactory) NewOrganizationClient(publicKey, privateKey string) langfuse.OrganizationClient {
	return cf.OrganizationClient
}

func (cf *mockClientFactory) NewLlmConnectionsClient(publicKey, privateKey string) langfuse.LlmConnectionsClient {
	return cf.LlmConnectionsClient
}
