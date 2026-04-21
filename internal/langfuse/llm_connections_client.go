package langfuse

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

//go:generate mockgen -destination=./mocks/mock_llm_connections_client.go -package=mocks github.com/langfuse/terraform-provider-langfuse/internal/langfuse LlmConnectionsClient

type LlmConnection struct {
	ID                string         `json:"id"`
	Provider          string         `json:"provider"`
	Adapter           string         `json:"adapter"`
	DisplaySecretKey  string         `json:"displaySecretKey"`
	BaseURL           string         `json:"baseURL,omitempty"`
	CustomModels      []string       `json:"customModels,omitempty"`
	WithDefaultModels bool           `json:"withDefaultModels"`
	ExtraHeaderKeys   []string       `json:"extraHeaderKeys,omitempty"`
	Config            map[string]any `json:"config,omitempty"`
	CreatedAt         string         `json:"createdAt"`
	UpdatedAt         string         `json:"updatedAt"`
}

type UpsertLlmConnectionRequest struct {
	Adapter           string            `json:"adapter"`
	Provider          string            `json:"provider"`
	SecretKey         string            `json:"secretKey"`
	BaseURL           string            `json:"baseURL,omitempty"`
	Config            map[string]any    `json:"config,omitempty"`
	CustomModels      []string          `json:"customModels,omitempty"`
	ExtraHeaders      map[string]string `json:"extraHeaders,omitempty"`
	WithDefaultModels *bool             `json:"withDefaultModels,omitempty"`
}

type ListLlmConnectionsResponse struct {
	Data []LlmConnection `json:"data"`
	Meta PaginationMeta  `json:"meta"`
}

type PaginationMeta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}

type deleteLlmConnectionResponse struct {
	Message string `json:"message"`
}

type LlmConnectionsClient interface {
	ListLlmConnections(ctx context.Context, page, limit *int) (*ListLlmConnectionsResponse, error)
	UpsertLlmConnection(ctx context.Context, req *UpsertLlmConnectionRequest) (*LlmConnection, error)
	DeleteLlmConnection(ctx context.Context, id string) error
}

type llmConnectionsClientImpl struct {
	host       string
	publicKey  string
	privateKey string
	httpClient *http.Client
}

func NewLlmConnectionsClient(host, publicKey, privateKey string) LlmConnectionsClient {
	return &llmConnectionsClientImpl{
		host:       host,
		publicKey:  publicKey,
		privateKey: privateKey,
		httpClient: &http.Client{},
	}
}

func (c *llmConnectionsClientImpl) makeRequest(ctx context.Context, methodType, apiPath string, body any) (*http.Response, error) {
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

func (c *llmConnectionsClientImpl) ListLlmConnections(ctx context.Context, page, limit *int) (*ListLlmConnectionsResponse, error) {
	baseURL := buildURL(c.host, "api/public/llm-connections")

	if page != nil || limit != nil {
		parsed, err := url.Parse(baseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse llm connections URL: %w", err)
		}
		q := url.Values{}
		if page != nil {
			q.Set("page", fmt.Sprintf("%d", *page))
		}
		if limit != nil {
			q.Set("limit", fmt.Sprintf("%d", *limit))
		}
		parsed.RawQuery = q.Encode()
		baseURL = parsed.String()
	}

	req, err := buildBaseRequest(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build list llm connections request: %w", err)
	}
	req.SetBasicAuth(c.publicKey, c.privateKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make list llm connections request: %w", err)
	}

	var listResp ListLlmConnectionsResponse
	if err := decodeResponse(resp, &listResp); err != nil {
		return nil, err
	}

	return &listResp, nil
}

func (c *llmConnectionsClientImpl) UpsertLlmConnection(ctx context.Context, req *UpsertLlmConnectionRequest) (*LlmConnection, error) {
	resp, err := c.makeRequest(ctx, http.MethodPut, "api/public/llm-connections", req)
	if err != nil {
		return nil, fmt.Errorf("failed to make upsert llm connection request: %w", err)
	}

	var connection LlmConnection
	if err := decodeResponse(resp, &connection); err != nil {
		return nil, err
	}

	return &connection, nil
}

func (c *llmConnectionsClientImpl) DeleteLlmConnection(ctx context.Context, id string) error {
	resp, err := c.makeRequest(ctx, http.MethodDelete, fmt.Sprintf("api/public/llm-connections/%s", id), nil)
	if err != nil {
		return err
	}

	var deleteResp deleteLlmConnectionResponse
	if err := decodeResponse(resp, &deleteResp); err != nil {
		return err
	}

	return nil
}
