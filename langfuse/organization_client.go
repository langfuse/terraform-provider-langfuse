package langfuse

import (
	"context"
)

type Project struct {
	ID        string
	Name      string
	Retention int32
}

type ProjectApiKey struct {
	ID        string
	ProjectID string
	PublicKey string
	SecretKey string
}

type UpdateProjectRequest struct {
	Name      string            `json:"name"`
	Retention int32             `json:"retention"`
	Metadata  map[string]string `json:"metadata"`
}

//go:generate mockgen -destination=./mocks/mock_organization_client.go -package=mocks github.com/cresta/terraform-provider-langfuse/langfuse OrganizationClient

type OrganizationClient interface {
	GetProject(ctx context.Context, projectID string) (*Project, error)
	CreateProject(ctx context.Context, project Project) (*Project, error)
	UpdateProject(ctx context.Context, projectID string, request *UpdateProjectRequest) (*Project, error)
	DeleteProject(ctx context.Context, projectID string) error
	GetProjectApiKey(ctx context.Context, projectID string, apiKeyID string) (*ProjectApiKey, error)
	CreateProjectApiKey(ctx context.Context, projectID string) (*ProjectApiKey, error)
	DeleteProjectApiKey(ctx context.Context, projectID string) error
}

type organizationClientImpl struct {
	host       string
	publicKey  string
	privateKey string
}

func NewOrganizationClient(host, publicKey, privateKey string) OrganizationClient {
	return &organizationClientImpl{
		host:       host,
		publicKey:  publicKey,
		privateKey: privateKey,
	}
}

// TODO: Implement
func (c *organizationClientImpl) GetProject(ctx context.Context, projectId string) (*Project, error) {
	return nil, nil
}

func (c *organizationClientImpl) CreateProject(ctx context.Context, project Project) (*Project, error) {
	return nil, nil
}

func (c *organizationClientImpl) UpdateProject(ctx context.Context, projectID string, request *UpdateProjectRequest) (*Project, error) {
	return nil, nil
}

func (c *organizationClientImpl) DeleteProject(ctx context.Context, projectId string) error {
	return nil
}

func (c *organizationClientImpl) GetProjectApiKey(ctx context.Context, projectId string, apiKeyID string) (*ProjectApiKey, error) {
	return nil, nil
}

func (c *organizationClientImpl) CreateProjectApiKey(ctx context.Context, projectId string) (*ProjectApiKey, error) {
	return nil, nil
}

func (c *organizationClientImpl) DeleteProjectApiKey(ctx context.Context, projectId string) error {
	return nil
}
