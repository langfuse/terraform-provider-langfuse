package langfuse

import (
	"context"
	"fmt"
	"net/http"
)

type Project struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	RetentionDays int32             `json:"retentionDays"`
	Metadata      map[string]string `json:"metadata"`
}

type ProjectApiKey struct {
	ID        string `json:"id"`
	PublicKey string `json:"publicKey"`
	SecretKey string `json:"secretKey"`
}

type CreateProjectRequest struct {
	Name          string            `json:"name"`
	RetentionDays int32             `json:"retention"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type UpdateProjectRequest struct {
	Name          string            `json:"name"`
	RetentionDays int32             `json:"retention"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type listProjectsResponse struct {
	Projects []*Project `json:"projects"`
}

type listProjectApiKeysResponse struct {
	ApiKeys []ProjectApiKey `json:"apiKeys"`
}

type deleteProjectResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type deleteProjectApiKeyResponse struct {
	Success bool `json:"success"`
}

//go:generate mockgen -destination=./mocks/mock_organization_client.go -package=mocks github.com/cresta/terraform-provider-langfuse/internal/langfuse OrganizationClient

type OrganizationClient interface {
	ListProjects(ctx context.Context) ([]*Project, error)
	GetProject(ctx context.Context, projectID string) (*Project, error)
	CreateProject(ctx context.Context, request *CreateProjectRequest) (*Project, error)
	UpdateProject(ctx context.Context, projectID string, request *UpdateProjectRequest) (*Project, error)
	DeleteProject(ctx context.Context, projectID string) error
	GetProjectApiKey(ctx context.Context, projectID string, apiKeyID string) (*ProjectApiKey, error)
	CreateProjectApiKey(ctx context.Context, projectID string) (*ProjectApiKey, error)
	DeleteProjectApiKey(ctx context.Context, projectID string, apiKeyID string) error
}

type organizationClientImpl struct {
	host       string
	publicKey  string
	privateKey string
	httpClient *http.Client
}

func NewOrganizationClient(host, publicKey, privateKey string) OrganizationClient {
	return &organizationClientImpl{
		host:       host,
		publicKey:  publicKey,
		privateKey: privateKey,
		httpClient: &http.Client{},
	}
}

func (c *organizationClientImpl) ListProjects(ctx context.Context) ([]*Project, error) {
	resp, err := c.makeRequest(ctx, http.MethodGet, "api/public/organizations/projects", nil)
	if err != nil {
		return nil, err
	}

	var listProjResp listProjectsResponse
	if err := decodeResponse(resp, &listProjResp); err != nil {
		return nil, err
	}

	return listProjResp.Projects, nil
}

func (c *organizationClientImpl) GetProject(ctx context.Context, projectID string) (*Project, error) {
	// Note: this endpoint does not return `retentionDays`, so the returned value will always be 0
	resp, err := c.makeRequest(ctx, http.MethodGet, "api/public/organizations/projects", nil)
	if err != nil {
		return nil, err
	}

	var listProjResp listProjectsResponse
	if err := decodeResponse(resp, &listProjResp); err != nil {
		return nil, err
	}
	for _, proj := range listProjResp.Projects {
		if proj.ID == projectID {
			return proj, nil
		}
	}
	return nil, fmt.Errorf("cannot find project with ID %s", projectID)
}

func (c *organizationClientImpl) CreateProject(ctx context.Context, request *CreateProjectRequest) (*Project, error) {
	resp, err := c.makeRequest(ctx, http.MethodPost, "api/public/projects", request)
	if err != nil {
		return nil, err
	}

	var proj Project
	if err := decodeResponse(resp, &proj); err != nil {
		return nil, err
	}

	return &proj, nil
}

func (c *organizationClientImpl) UpdateProject(ctx context.Context, projectID string, request *UpdateProjectRequest) (*Project, error) {
	resp, err := c.makeRequest(ctx, http.MethodPut, fmt.Sprintf("api/public/projects/%s", projectID), request)
	if err != nil {
		return nil, err
	}

	var proj Project
	if err := decodeResponse(resp, &proj); err != nil {
		return nil, err
	}

	return &proj, nil
}

func (c *organizationClientImpl) DeleteProject(ctx context.Context, projectID string) error {
	resp, err := c.makeRequest(ctx, http.MethodDelete, fmt.Sprintf("api/public/projects/%s", projectID), nil)
	if err != nil {
		return err
	}

	var deleteProjResp deleteProjectResponse
	if err := decodeResponse(resp, &deleteProjResp); err != nil {
		return err
	}
	if !deleteProjResp.Success {
		return fmt.Errorf("failed to delete project with ID %s: %s", projectID, deleteProjResp.Message)
	}

	return nil
}

func (c *organizationClientImpl) GetProjectApiKey(ctx context.Context, projectID string, apiKeyID string) (*ProjectApiKey, error) {
	resp, err := c.makeRequest(ctx, http.MethodGet, fmt.Sprintf("api/public/projects/%s/apiKeys", projectID), nil)
	if err != nil {
		return nil, err
	}

	var listProjApiKeysResp listProjectApiKeysResponse
	if err := decodeResponse(resp, &listProjApiKeysResp); err != nil {
		return nil, err
	}
	for _, key := range listProjApiKeysResp.ApiKeys {
		if key.ID == apiKeyID {
			return &key, nil
		}
	}

	return nil, fmt.Errorf("cannot find API key with ID %s in project %s", apiKeyID, projectID)
}

func (c *organizationClientImpl) CreateProjectApiKey(ctx context.Context, projectID string) (*ProjectApiKey, error) {
	resp, err := c.makeRequest(ctx, http.MethodPost, fmt.Sprintf("api/public/projects/%s/apiKeys", projectID), nil)
	if err != nil {
		return nil, err
	}
	var apiKey ProjectApiKey
	if err := decodeResponse(resp, &apiKey); err != nil {
		return nil, err
	}

	return &apiKey, nil
}

func (c *organizationClientImpl) DeleteProjectApiKey(ctx context.Context, projectID string, apiKeyID string) error {
	resp, err := c.makeRequest(ctx, http.MethodDelete, fmt.Sprintf("api/public/projects/%s/apiKeys/%s", projectID, apiKeyID), nil)
	if err != nil {
		return err
	}

	var deleteProjApiKeyResp deleteProjectApiKeyResponse
	if err := decodeResponse(resp, &deleteProjApiKeyResp); err != nil {
		return err
	}
	if !deleteProjApiKeyResp.Success {
		return fmt.Errorf("failed to delete API key with ID %s in project %s", apiKeyID, projectID)
	}

	return nil
}

func (c *organizationClientImpl) makeRequest(ctx context.Context, methodType, apiPath string, body any) (*http.Response, error) {
	req, err := buildBaseRequest(ctx, methodType, buildURL(c.host, apiPath), body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.publicKey, c.privateKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	return resp, nil
}
