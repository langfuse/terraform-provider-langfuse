package langfuse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

//go:generate mockgen -destination=./mocks/mock_evaluators_client.go -package=mocks github.com/langfuse/terraform-provider-langfuse/internal/langfuse EvaluatorsClient

// Evaluator types accepted by the stable evaluators API.
const (
	EvaluatorTypeLlmAsJudge = "llm_as_judge"
	EvaluatorTypeCode       = "code"
)

// EvaluatorChatMessage is one message of an LLM-as-a-judge evaluator prompt.
type EvaluatorChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// EvaluatorModelConfig selects the model used by an LLM-as-a-judge evaluator.
type EvaluatorModelConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// PromptVariableMapping connects one prompt variable to observation data.
//
// Source is a plain string on write. On read it is empty when the stored
// mapping is incomplete (the API returns null in that case).
type PromptVariableMapping struct {
	Variable string  `json:"variable"`
	Source   string  `json:"source"`
	JSONPath *string `json:"jsonPath,omitempty"`
}

// EvaluatorOutputDefinition is the flat structured-output definition of an
// LLM-as-a-judge evaluator. Only the fields relevant to DataType are set.
type EvaluatorOutputDefinition struct {
	DataType                   string   `json:"dataType"`
	MinValue                   *float64 `json:"minValue,omitempty"`
	MaxValue                   *float64 `json:"maxValue,omitempty"`
	Categories                 []string `json:"categories,omitempty"`
	ShouldAllowMultipleMatches *bool    `json:"shouldAllowMultipleMatches,omitempty"`
	ScoreReasoningInstructions *string  `json:"scoreReasoningInstructions,omitempty"`
	ScoreValueInstructions     *string  `json:"scoreValueInstructions,omitempty"`
}

// Evaluator is the flattened evaluator returned by the API: metadata plus the
// latest version's definition. Type-specific fields are zero for other types.
type Evaluator struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Description   *string `json:"description"`
	Type          string  `json:"type"`
	Status        string  `json:"status"`
	PausedReason  *string `json:"pausedReason"`
	PausedMessage *string `json:"pausedMessage"`
	Version       int64   `json:"version"`
	VersionID     string  `json:"versionId"`
	CreatedAt     string  `json:"createdAt"`
	UpdatedAt     string  `json:"updatedAt"`

	// llm_as_judge
	Prompt           []EvaluatorChatMessage     `json:"prompt,omitempty"`
	Variables        []string                   `json:"variables,omitempty"`
	VariableMapping  []PromptVariableMapping    `json:"variableMapping,omitempty"`
	ModelConfig      *EvaluatorModelConfig      `json:"modelConfig,omitempty"`
	OutputDefinition *EvaluatorOutputDefinition `json:"outputDefinition,omitempty"`

	// code
	SourceCode         string `json:"sourceCode,omitempty"`
	SourceCodeLanguage string `json:"sourceCodeLanguage,omitempty"`
}

// EvaluatorRequest is the flattened create request. The same shape is used
// for a full definition replacement on update, which creates a new version.
//
// Description is always sent so that null clears an existing description.
// Type-specific fields are omitted when empty so that a code evaluator never
// carries LLM fields and vice versa.
type EvaluatorRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Type        string  `json:"type"`

	// llm_as_judge
	Prompt           []EvaluatorChatMessage     `json:"prompt,omitempty"`
	ModelConfig      *EvaluatorModelConfig      `json:"modelConfig,omitempty"`
	VariableMapping  []PromptVariableMapping    `json:"variableMapping,omitempty"`
	OutputDefinition *EvaluatorOutputDefinition `json:"outputDefinition,omitempty"`

	// code
	SourceCode         string `json:"sourceCode,omitempty"`
	SourceCodeLanguage string `json:"sourceCodeLanguage,omitempty"`
}

// UpdateEvaluatorMetadataRequest renames or re-describes an evaluator without
// creating a new version.
type UpdateEvaluatorMetadataRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

// EvaluationRuleFilter is one filter condition. Value holds the JSON value
// verbatim: string, float64, bool, []any or nil depending on Type.
type EvaluationRuleFilter struct {
	Type     string `json:"type"`
	Column   string `json:"column"`
	Key      string `json:"key,omitempty"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}

// EvaluatorAssignment attaches an evaluator to a rule. A nil VariableMapping
// is serialised as null and means "inherit the evaluator's default mapping".
type EvaluatorAssignment struct {
	EvaluatorID     string                  `json:"evaluatorId"`
	VariableMapping []PromptVariableMapping `json:"variableMapping"`
}

// EvaluationRule is a live evaluation rule for incoming observations.
type EvaluationRule struct {
	ID                   string                 `json:"id"`
	Name                 string                 `json:"name"`
	Enabled              bool                   `json:"enabled"`
	Sampling             float64                `json:"sampling"`
	Filter               []EvaluationRuleFilter `json:"filter"`
	EvaluatorAssignments []EvaluatorAssignment  `json:"evaluatorAssignments"`
	CreatedAt            string                 `json:"createdAt"`
	UpdatedAt            string                 `json:"updatedAt"`
}

// EvaluationRuleRequest is used for create and for a full replacement update.
// Filter and EvaluatorAssignments must be non-nil so they serialise as [].
type EvaluationRuleRequest struct {
	Name                 string                 `json:"name"`
	Enabled              bool                   `json:"enabled"`
	Sampling             *float64               `json:"sampling,omitempty"`
	Filter               []EvaluationRuleFilter `json:"filter"`
	EvaluatorAssignments []EvaluatorAssignment  `json:"evaluatorAssignments"`
}

type deletedResourceResponse struct {
	ID string `json:"id"`
}

// APIValidationIssue is one field-level validation problem reported by the API.
type APIValidationIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    []any  `json:"path"`
}

// APIError is the structured error returned by the stable evaluators and
// evaluation-rules API.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Issues     []APIValidationIssue
	Body       string
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("request failed with status code %d, response body: %s", e.StatusCode, e.Body)
	}
	msg := fmt.Sprintf("request failed with status code %d (%s): %s", e.StatusCode, e.Code, e.Message)
	for _, issue := range e.Issues {
		path := make([]string, 0, len(issue.Path))
		for _, p := range issue.Path {
			path = append(path, fmt.Sprint(p))
		}
		msg += fmt.Sprintf("\n  - %s: %s", strings.Join(path, "."), issue.Message)
	}
	return msg
}

// IsNotFound reports whether err is an API error with HTTP status 404.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

type EvaluatorsClient interface {
	CreateEvaluator(ctx context.Context, req *EvaluatorRequest) (*Evaluator, error)
	GetEvaluator(ctx context.Context, id string) (*Evaluator, error)
	UpdateEvaluator(ctx context.Context, id string, req *EvaluatorRequest) (*Evaluator, error)
	UpdateEvaluatorMetadata(ctx context.Context, id string, req *UpdateEvaluatorMetadataRequest) (*Evaluator, error)
	DeleteEvaluator(ctx context.Context, id string) error

	CreateEvaluationRule(ctx context.Context, req *EvaluationRuleRequest) (*EvaluationRule, error)
	GetEvaluationRule(ctx context.Context, id string) (*EvaluationRule, error)
	UpdateEvaluationRule(ctx context.Context, id string, req *EvaluationRuleRequest) (*EvaluationRule, error)
	DeleteEvaluationRule(ctx context.Context, id string) error
}

type evaluatorsClientImpl struct {
	host       string
	publicKey  string
	secretKey  string
	httpClient *http.Client
}

// NewEvaluatorsClient returns a client for the stable /api/public/v2
// evaluators and evaluation-rules endpoints, authenticated with a project
// public/secret key pair.
func NewEvaluatorsClient(host, publicKey, secretKey string, httpClient *http.Client) EvaluatorsClient {
	return &evaluatorsClientImpl{
		host:       host,
		publicKey:  publicKey,
		secretKey:  secretKey,
		httpClient: httpClientOrDefault(httpClient),
	}
}

func (c *evaluatorsClientImpl) do(ctx context.Context, method, apiPath string, body, target any) error {
	req, err := buildBaseRequest(ctx, method, buildURL(c.host, apiPath), body)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.publicKey, c.secretKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}

	return decodeV2Response(resp, target)
}

// decodeV2Response decodes a success body into target and turns any non-2xx
// response into an *APIError, parsing the structured error body when present.
func decodeV2Response(resp *http.Response, target any) error {
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{StatusCode: resp.StatusCode, Body: string(body)}
		var parsed struct {
			Message string `json:"message"`
			Code    string `json:"code"`
			Details *struct {
				Issues []APIValidationIssue `json:"issues"`
			} `json:"details"`
		}
		if json.Unmarshal(body, &parsed) == nil && parsed.Code != "" {
			apiErr.Code = parsed.Code
			apiErr.Message = parsed.Message
			if parsed.Details != nil {
				apiErr.Issues = parsed.Details.Issues
			}
		}
		return apiErr
	}

	if target == nil {
		return nil
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("failed to unmarshal response body: %w", err)
	}
	return nil
}

func (c *evaluatorsClientImpl) CreateEvaluator(ctx context.Context, req *EvaluatorRequest) (*Evaluator, error) {
	var ev Evaluator
	if err := c.do(ctx, http.MethodPost, "api/public/v2/evaluators", req, &ev); err != nil {
		return nil, fmt.Errorf("failed to create evaluator: %w", err)
	}
	return &ev, nil
}

func (c *evaluatorsClientImpl) GetEvaluator(ctx context.Context, id string) (*Evaluator, error) {
	var ev Evaluator
	if err := c.do(ctx, http.MethodGet, "api/public/v2/evaluators/"+id, nil, &ev); err != nil {
		return nil, fmt.Errorf("failed to get evaluator: %w", err)
	}
	return &ev, nil
}

func (c *evaluatorsClientImpl) UpdateEvaluator(ctx context.Context, id string, req *EvaluatorRequest) (*Evaluator, error) {
	var ev Evaluator
	if err := c.do(ctx, http.MethodPatch, "api/public/v2/evaluators/"+id, req, &ev); err != nil {
		return nil, fmt.Errorf("failed to update evaluator: %w", err)
	}
	return &ev, nil
}

func (c *evaluatorsClientImpl) UpdateEvaluatorMetadata(ctx context.Context, id string, req *UpdateEvaluatorMetadataRequest) (*Evaluator, error) {
	var ev Evaluator
	if err := c.do(ctx, http.MethodPatch, "api/public/v2/evaluators/"+id, req, &ev); err != nil {
		return nil, fmt.Errorf("failed to update evaluator metadata: %w", err)
	}
	return &ev, nil
}

func (c *evaluatorsClientImpl) DeleteEvaluator(ctx context.Context, id string) error {
	var deleted deletedResourceResponse
	err := c.do(ctx, http.MethodDelete, "api/public/v2/evaluators/"+id, nil, &deleted)
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("failed to delete evaluator: %w", err)
	}
	return nil
}

func (c *evaluatorsClientImpl) CreateEvaluationRule(ctx context.Context, req *EvaluationRuleRequest) (*EvaluationRule, error) {
	var rule EvaluationRule
	if err := c.do(ctx, http.MethodPost, "api/public/v2/evaluation-rules", req, &rule); err != nil {
		return nil, fmt.Errorf("failed to create evaluation rule: %w", err)
	}
	return &rule, nil
}

func (c *evaluatorsClientImpl) GetEvaluationRule(ctx context.Context, id string) (*EvaluationRule, error) {
	var rule EvaluationRule
	if err := c.do(ctx, http.MethodGet, "api/public/v2/evaluation-rules/"+id, nil, &rule); err != nil {
		return nil, fmt.Errorf("failed to get evaluation rule: %w", err)
	}
	return &rule, nil
}

func (c *evaluatorsClientImpl) UpdateEvaluationRule(ctx context.Context, id string, req *EvaluationRuleRequest) (*EvaluationRule, error) {
	var rule EvaluationRule
	if err := c.do(ctx, http.MethodPatch, "api/public/v2/evaluation-rules/"+id, req, &rule); err != nil {
		return nil, fmt.Errorf("failed to update evaluation rule: %w", err)
	}
	return &rule, nil
}

func (c *evaluatorsClientImpl) DeleteEvaluationRule(ctx context.Context, id string) error {
	var deleted deletedResourceResponse
	err := c.do(ctx, http.MethodDelete, "api/public/v2/evaluation-rules/"+id, nil, &deleted)
	if err != nil && !IsNotFound(err) {
		return fmt.Errorf("failed to delete evaluation rule: %w", err)
	}
	return nil
}
