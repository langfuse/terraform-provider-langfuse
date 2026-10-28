package mocks

import (
	gomock "github.com/golang/mock/gomock"
	langfuse "github.com/langfuse/terraform-provider-langfuse/internal/langfuse"
)

type mockClientFactory struct {
	AdminClient          *MockAdminClient
	OrganizationClient   *MockOrganizationClient
	LlmConnectionsClient *MockLlmConnectionsClient

	// Provider-level organization keys returned by OrganizationKeys.
	DefaultOrganizationPublicKey  string
	DefaultOrganizationPrivateKey string

	// Keys passed to the most recent NewOrganizationClient call.
	UsedOrganizationPublicKey  string
	UsedOrganizationPrivateKey string
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
	cf.UsedOrganizationPublicKey = publicKey
	cf.UsedOrganizationPrivateKey = privateKey
	return cf.OrganizationClient
}

func (cf *mockClientFactory) NewLlmConnectionsClient(publicKey, privateKey string) langfuse.LlmConnectionsClient {
	return cf.LlmConnectionsClient
}

func (cf *mockClientFactory) OrganizationKeys() (string, string) {
	return cf.DefaultOrganizationPublicKey, cf.DefaultOrganizationPrivateKey
}
