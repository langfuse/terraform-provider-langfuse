package langfuse

import (
	"context"
)

type Organization struct {
	ID   string
	Name string
}

type OrganizationApiKey struct {
	ID             string
	OrganizationID string
	PublicKey      string
	SecretKey      string
}

type UpdateOrganizationRequest struct {
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata"`
}

//go:generate mockgen -destination=./mocks/mock_admin_client.go -package=mocks github.com/cresta/terraform-provider-langfuse/langfuse AdminClient

type AdminClient interface {
	GetOrganization(ctx context.Context, orgID string) (*Organization, error)
	CreateOrganization(ctx context.Context, name string) (*Organization, error)
	UpdateOrganization(ctx context.Context, orgID string, request *UpdateOrganizationRequest) (*Organization, error)
	DeleteOrganization(ctx context.Context, orgID string) error
	GetOrganizationApiKey(ctx context.Context, orgID string, apiKeyID string) (*OrganizationApiKey, error)
	CreateOrganizationApiKey(ctx context.Context, orgID string) (*OrganizationApiKey, error)
	DeleteOrganizationApiKey(ctx context.Context, orgID string, apiKeyID string) error
}

type adminClientImpl struct {
	host   string
	apiKey string
}

func NewAdminClient(host, apiKey string) AdminClient {
	return &adminClientImpl{
		host:   host,
		apiKey: apiKey,
	}
}

// TODO: Implement
func (c *adminClientImpl) GetOrganization(ctx context.Context, orgID string) (*Organization, error) {
	return nil, nil
}

func (c *adminClientImpl) CreateOrganization(ctx context.Context, name string) (*Organization, error) {
	return nil, nil
}

func (c *adminClientImpl) UpdateOrganization(ctx context.Context, orgID string, request *UpdateOrganizationRequest) (*Organization, error) {
	return nil, nil
}

func (c *adminClientImpl) DeleteOrganization(ctx context.Context, orgID string) error {
	return nil
}

func (c *adminClientImpl) GetOrganizationApiKey(ctx context.Context, orgID string, apiKeyID string) (*OrganizationApiKey, error) {
	return nil, nil
}

func (c *adminClientImpl) CreateOrganizationApiKey(ctx context.Context, orgID string) (*OrganizationApiKey, error) {
	return nil, nil
}

func (c *adminClientImpl) DeleteOrganizationApiKey(ctx context.Context, orgID string, apiKeyID string) error {
	return nil
}
