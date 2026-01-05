package langfuse

import (
	"context"
	"fmt"
	"net/http"
	"strings"
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

type OrganizationMembership struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Status   string `json:"status"`
	UserID   string `json:"userId"`
	Username string `json:"username"`
}

type SCIMUserRequest struct {
	UserName string `json:"userName"`
	Emails   []struct {
		Value   string `json:"value"`
		Primary bool   `json:"primary"`
	} `json:"emails"`
	Password string `json:"password,omitempty"`
	Active   bool   `json:"active"`
}

type SCIMUserResponse struct {
	ID       string `json:"id"`
	UserName string `json:"userName"`
	Emails   []struct {
		Value   string `json:"value"`
		Primary bool   `json:"primary"`
	} `json:"emails"`
	Active bool `json:"active"`
}

type UpdateMembershipRequest struct {
	UserID string `json:"userId,omitempty"` // User ID from SCIM
	Email  string `json:"email,omitempty"`  // Or email
	Role   string `json:"role"`
}

type listMembershipsResponse struct {
	Memberships []OrganizationMembership `json:"memberships"`
}

type removeMemberResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

//go:generate mockgen -destination=./mocks/mock_organization_client.go -package=mocks github.com/langfuse/terraform-provider-langfuse/internal/langfuse OrganizationClient

type OrganizationClient interface {
	ListProjects(ctx context.Context) ([]*Project, error)
	GetProject(ctx context.Context, projectID string) (*Project, error)
	CreateProject(ctx context.Context, request *CreateProjectRequest) (*Project, error)
	UpdateProject(ctx context.Context, projectID string, request *UpdateProjectRequest) (*Project, error)
	DeleteProject(ctx context.Context, projectID string) error
	GetProjectApiKey(ctx context.Context, projectID string, apiKeyID string) (*ProjectApiKey, error)
	CreateProjectApiKey(ctx context.Context, projectID string) (*ProjectApiKey, error)
	DeleteProjectApiKey(ctx context.Context, projectID string, apiKeyID string) error
	ListMemberships(ctx context.Context) ([]OrganizationMembership, error)
	GetMembership(ctx context.Context, membershipID string) (*OrganizationMembership, error)
	UpdateMembership(ctx context.Context, membershipID string, request *UpdateMembershipRequest) (*OrganizationMembership, error)
	RemoveMember(ctx context.Context, membershipID string) error
	CreateSCIMUser(ctx context.Context, request *SCIMUserRequest) (*SCIMUserResponse, error)
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

func (c *organizationClientImpl) ListMemberships(ctx context.Context) ([]OrganizationMembership, error) {
	resp, err := c.makeRequest(ctx, http.MethodGet, "api/public/organizations/memberships", nil)
	if err != nil {
		return nil, err
	}

	var listMembershipsResp listMembershipsResponse
	if err := decodeResponse(resp, &listMembershipsResp); err != nil {
		return nil, err
	}

	return listMembershipsResp.Memberships, nil
}

func (c *organizationClientImpl) GetMembership(ctx context.Context, membershipID string) (*OrganizationMembership, error) {
	memberships, err := c.ListMemberships(ctx)
	if err != nil {
		return nil, err
	}

	for _, membership := range memberships {
		// The API may not return the membership ID field, so check both ID and UserID
		if membership.ID == membershipID || membership.UserID == membershipID {
			return &membership, nil
		}
	}

	return nil, fmt.Errorf("cannot find membership with ID %s", membershipID)
}

func (c *organizationClientImpl) UpdateMembership(ctx context.Context, membershipID string, request *UpdateMembershipRequest) (*OrganizationMembership, error) {
	// Retrieve current membership to get the user ID
	currentMembership, err := c.GetMembership(ctx, membershipID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current membership: %w", err)
	}

	// Prepare update request with user ID
	userIDToUpdate := request.UserID
	if userIDToUpdate == "" {
		userIDToUpdate = currentMembership.UserID
	}
	
	updateRequest := UpdateMembershipRequest{
		UserID: userIDToUpdate,
		Role:   request.Role,
	}

	resp, err := c.makeRequest(ctx, http.MethodPut, "api/public/organizations/memberships", updateRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to update membership: %w", err)
	}

	var updatedMembership OrganizationMembership
	if err := decodeResponse(resp, &updatedMembership); err != nil {
		return nil, fmt.Errorf("failed to decode membership response: %w", err)
	}

	// The PUT response may not include the membership ID, so preserve it from the original request
	if updatedMembership.ID == "" {
		updatedMembership.ID = membershipID
	}

	return &updatedMembership, nil
}

func (c *organizationClientImpl) RemoveMember(ctx context.Context, membershipID string) error {
	// DELETE endpoint requires userId in the request body
	deleteRequest := struct {
		UserID string `json:"userId"`
	}{
		UserID: membershipID,
	}
	
	resp, err := c.makeRequest(ctx, http.MethodDelete, "api/public/organizations/memberships", deleteRequest)
	if err != nil {
		return err
	}

	var removeMemberResp removeMemberResponse
	if err := decodeResponse(resp, &removeMemberResp); err != nil {
		return err
	}
	
	// API returns success: false but with a success message, so we check the message too
	if !removeMemberResp.Success && !strings.Contains(strings.ToLower(removeMemberResp.Message), "deleted") && !strings.Contains(strings.ToLower(removeMemberResp.Message), "removed") {
		return fmt.Errorf("failed to remove member with ID %s: %s", membershipID, removeMemberResp.Message)
	}

	return nil
}

func (c *organizationClientImpl) CreateSCIMUser(ctx context.Context, request *SCIMUserRequest) (*SCIMUserResponse, error) {
	// Ensure Active is true if not explicitly set
	if !request.Active {
		request.Active = true
	}

	resp, err := c.makeRequest(ctx, http.MethodPost, "api/public/scim/Users", request)
	if err != nil {
		return nil, fmt.Errorf("failed to create SCIM user: %w", err)
	}

	var scimUser SCIMUserResponse
	if err := decodeResponse(resp, &scimUser); err != nil {
		return nil, fmt.Errorf("failed to decode SCIM user response: %w", err)
	}

	return &scimUser, nil
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
