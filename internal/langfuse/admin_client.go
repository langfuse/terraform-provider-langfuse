package langfuse

import (
	"context"
	"fmt"
	"net/http"
)

type Organization struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata"`
}

type OrganizationApiKey struct {
	ID        string `json:"id"`
	PublicKey string `json:"publicKey"`
	SecretKey string `json:"secretKey"`
}

type ListOrganizationsResponse struct {
	Organizations []*Organization `json:"organizations"`
}

type CreateOrganizationRequest struct {
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type UpdateOrganizationRequest struct {
	Name     string            `json:"name"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type deleteOrganizationResponse struct {
	Success bool `json:"success"`
}

type listOrganizationApiKeysResponse struct {
	ApiKeys []OrganizationApiKey `json:"apiKeys"`
}

type deleteOrganizationApiKeyResponse struct {
	Success bool `json:"success"`
}

//go:generate mockgen -destination=./mocks/mock_admin_client.go -package=mocks github.com/langfuse/terraform-provider-langfuse/internal/langfuse AdminClient

type AdminClient interface {
	ListOrganizations(ctx context.Context) ([]*Organization, error)
	GetOrganization(ctx context.Context, orgID string) (*Organization, error)
	CreateOrganization(ctx context.Context, request *CreateOrganizationRequest) (*Organization, error)
	UpdateOrganization(ctx context.Context, orgID string, request *UpdateOrganizationRequest) (*Organization, error)
	DeleteOrganization(ctx context.Context, orgID string) error
	GetOrganizationApiKey(ctx context.Context, orgID string, apiKeyID string) (*OrganizationApiKey, error)
	CreateOrganizationApiKey(ctx context.Context, orgID string) (*OrganizationApiKey, error)
	DeleteOrganizationApiKey(ctx context.Context, orgID string, apiKeyID string) error
}

type adminClientImpl struct {
	host       string
	apiKey     string
	httpClient *http.Client
}

func NewAdminClient(host, apiKey string) AdminClient {
	return &adminClientImpl{
		host:       host,
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

func (c *adminClientImpl) ListOrganizations(ctx context.Context) ([]*Organization, error) {
	resp, err := c.makeRequest(ctx, http.MethodGet, "api/admin/organizations", nil)
	if err != nil {
		return nil, err
	}

	var listOrgResp ListOrganizationsResponse
	if err := decodeResponse(resp, &listOrgResp); err != nil {
		return nil, err
	}

	return listOrgResp.Organizations, nil
}

func (c *adminClientImpl) GetOrganization(ctx context.Context, orgID string) (*Organization, error) {
	resp, err := c.makeRequest(ctx, http.MethodGet, fmt.Sprintf("api/admin/organizations/%s", orgID), nil)
	if err != nil {
		return nil, err
	}

	var org Organization
	if err := decodeResponse(resp, &org); err != nil {
		return nil, err
	}

	return &org, nil
}

func (c *adminClientImpl) CreateOrganization(ctx context.Context, request *CreateOrganizationRequest) (*Organization, error) {
	resp, err := c.makeRequest(ctx, http.MethodPost, "api/admin/organizations", request)
	if err != nil {
		return nil, err
	}

	var org Organization
	if err := decodeResponse(resp, &org); err != nil {
		return nil, err
	}

	return &org, nil
}

func (c *adminClientImpl) UpdateOrganization(ctx context.Context, orgID string, request *UpdateOrganizationRequest) (*Organization, error) {
	resp, err := c.makeRequest(ctx, http.MethodPut, fmt.Sprintf("api/admin/organizations/%s", orgID), request)
	if err != nil {
		return nil, err
	}

	var org Organization
	if err := decodeResponse(resp, &org); err != nil {
		return nil, err
	}

	return &org, nil
}

func (c *adminClientImpl) DeleteOrganization(ctx context.Context, orgID string) error {
	resp, err := c.makeRequest(ctx, http.MethodDelete, fmt.Sprintf("api/admin/organizations/%s", orgID), nil)
	if err != nil {
		return err
	}

	var deleteOrgResp deleteOrganizationResponse
	if err := decodeResponse(resp, &deleteOrgResp); err != nil {
		return err
	}
	if !deleteOrgResp.Success {
		return fmt.Errorf("failed to delete organization with ID %s", orgID)
	}

	return nil
}

func (c *adminClientImpl) GetOrganizationApiKey(ctx context.Context, orgID string, apiKeyID string) (*OrganizationApiKey, error) {
	resp, err := c.makeRequest(ctx, http.MethodGet, fmt.Sprintf("api/admin/organizations/%s/apiKeys", orgID), nil)
	if err != nil {
		return nil, err
	}

	var listOrgApiKeysResp listOrganizationApiKeysResponse
	if err := decodeResponse(resp, &listOrgApiKeysResp); err != nil {
		return nil, err
	}
	for _, key := range listOrgApiKeysResp.ApiKeys {
		if key.ID == apiKeyID {
			return &key, nil
		}
	}

	return nil, fmt.Errorf("cannot find API key with ID %s in organization %s", apiKeyID, orgID)
}

func (c *adminClientImpl) CreateOrganizationApiKey(ctx context.Context, orgID string) (*OrganizationApiKey, error) {
	resp, err := c.makeRequest(ctx, http.MethodPost, fmt.Sprintf("api/admin/organizations/%s/apiKeys", orgID), nil)
	if err != nil {
		return nil, err
	}

	var apiKey OrganizationApiKey
	if err := decodeResponse(resp, &apiKey); err != nil {
		return nil, err
	}

	return &apiKey, nil
}

func (c *adminClientImpl) DeleteOrganizationApiKey(ctx context.Context, orgID string, apiKeyID string) error {
	resp, err := c.makeRequest(ctx, http.MethodDelete, fmt.Sprintf("api/admin/organizations/%s/apiKeys/%s", orgID, apiKeyID), nil)
	if err != nil {
		return err
	}

	var deleteOrgApiKeyResp deleteOrganizationApiKeyResponse
	if err := decodeResponse(resp, &deleteOrgApiKeyResp); err != nil {
		return err
	}
	if !deleteOrgApiKeyResp.Success {
		return fmt.Errorf("failed to delete API key with ID %s in organization %s", apiKeyID, orgID)
	}

	return nil
}

func (c *adminClientImpl) makeRequest(ctx context.Context, method, apiPath string, body any) (*http.Response, error) {
	req, err := buildBaseRequest(ctx, method, buildURL(c.host, apiPath), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	return resp, nil
}
