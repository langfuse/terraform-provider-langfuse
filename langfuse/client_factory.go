package langfuse

type clientFactoryImpl struct {
	host        string
	adminApiKey string
}

type ClientFactory interface {
	NewAdminClient() AdminClient
	NewOrganizationClient(publicKey, privateKey string) OrganizationClient
	NewLlmConnectionsClient(publicKey, privateKey string) LlmConnectionsClient
}

func NewClientFactory(host, adminApiKey string) ClientFactory {
	return &clientFactoryImpl{
		host:        host,
		adminApiKey: adminApiKey,
	}
}

func (cf *clientFactoryImpl) NewAdminClient() AdminClient {
	return NewAdminClient(cf.host, cf.adminApiKey)
}

func (cf *clientFactoryImpl) NewOrganizationClient(publicKey, privateKey string) OrganizationClient {
	return NewOrganizationClient(cf.host, publicKey, privateKey)
}

func (cf *clientFactoryImpl) NewLlmConnectionsClient(publicKey, privateKey string) LlmConnectionsClient {
	return NewLlmConnectionsClient(cf.host, publicKey, privateKey)
}
