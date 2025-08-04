package langfuse

type ClientFactory struct {
	host        string
	adminApiKey string
}

func NewClientFactory(host, adminApiKey string) *ClientFactory {
	return &ClientFactory{
		host:        host,
		adminApiKey: adminApiKey,
	}
}

func (cf *ClientFactory) NewAdminClient() AdminClient {
	return NewAdminClient(cf.host, cf.adminApiKey)
}

func (cf *ClientFactory) NewOrganizationClient(publicKey, privateKey string) OrganizationClient {
	return NewOrganizationClient(cf.host, publicKey, privateKey)
}
