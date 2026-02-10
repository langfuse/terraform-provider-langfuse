package langfuse

type clientFactoryImpl struct {
	host           string
	adminApiKey    string
	orgPublicKey   string
	orgPrivateKey  string
}

type ClientFactory interface {
	NewAdminClient() AdminClient
	NewOrganizationClient(publicKey, privateKey string) OrganizationClient
	GetDefaultOrgPublicKey() string
	GetDefaultOrgPrivateKey() string
	HasDefaultOrgCredentials() bool
}

func NewClientFactory(host, adminApiKey, orgPublicKey, orgPrivateKey string) ClientFactory {
	return &clientFactoryImpl{
		host:          host,
		adminApiKey:   adminApiKey,
		orgPublicKey:  orgPublicKey,
		orgPrivateKey: orgPrivateKey,
	}
}

func (cf *clientFactoryImpl) NewAdminClient() AdminClient {
	return NewAdminClient(cf.host, cf.adminApiKey)
}

func (cf *clientFactoryImpl) NewOrganizationClient(publicKey, privateKey string) OrganizationClient {
	return NewOrganizationClient(cf.host, publicKey, privateKey)
}

func (cf *clientFactoryImpl) GetDefaultOrgPublicKey() string {
	return cf.orgPublicKey
}

func (cf *clientFactoryImpl) GetDefaultOrgPrivateKey() string {
	return cf.orgPrivateKey
}

func (cf *clientFactoryImpl) HasDefaultOrgCredentials() bool {
	return cf.orgPublicKey != "" && cf.orgPrivateKey != ""
}
