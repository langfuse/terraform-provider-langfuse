package langfuse

import "net/http"

type clientFactoryImpl struct {
	host                   string
	adminApiKey            string
	organizationPublicKey  string
	organizationPrivateKey string
	httpClient             *http.Client
}

type ClientFactory interface {
	NewAdminClient() AdminClient
	NewOrganizationClient(publicKey, privateKey string) OrganizationClient
	NewLlmConnectionsClient(publicKey, privateKey string) LlmConnectionsClient
	// OrganizationKeys returns the provider-level organization API keys, empty when not configured.
	OrganizationKeys() (publicKey, privateKey string)
}

func NewClientFactory(host, adminApiKey, organizationPublicKey, organizationPrivateKey, tlsServerName string) ClientFactory {
	return &clientFactoryImpl{
		host:                   host,
		adminApiKey:            adminApiKey,
		organizationPublicKey:  organizationPublicKey,
		organizationPrivateKey: organizationPrivateKey,
		httpClient:             newHTTPClient(tlsServerName),
	}
}

func (cf *clientFactoryImpl) NewAdminClient() AdminClient {
	return NewAdminClient(cf.host, cf.adminApiKey, cf.httpClient)
}

func (cf *clientFactoryImpl) NewOrganizationClient(publicKey, privateKey string) OrganizationClient {
	return NewOrganizationClient(cf.host, publicKey, privateKey, cf.httpClient)
}

func (cf *clientFactoryImpl) NewLlmConnectionsClient(publicKey, privateKey string) LlmConnectionsClient {
	return NewLlmConnectionsClient(cf.host, publicKey, privateKey, cf.httpClient)
}

func (cf *clientFactoryImpl) OrganizationKeys() (string, string) {
	return cf.organizationPublicKey, cf.organizationPrivateKey
}
