package langfuse

import "net/http"

type clientFactoryImpl struct {
	host        string
	adminApiKey string
	httpClient  *http.Client
}

type ClientFactory interface {
	NewAdminClient() AdminClient
	NewOrganizationClient(publicKey, privateKey string) OrganizationClient
	NewLlmConnectionsClient(publicKey, privateKey string) LlmConnectionsClient
}

func NewClientFactory(host, adminApiKey, tlsServerName string) ClientFactory {
	return &clientFactoryImpl{
		host:        host,
		adminApiKey: adminApiKey,
		httpClient:  newHTTPClient(tlsServerName),
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
