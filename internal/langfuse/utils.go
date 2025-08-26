package langfuse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func buildBaseRequest(ctx context.Context, method, url string, body any) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

func decodeResponse(resp *http.Response, target any) error {
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status code %d, response body: %s", resp.StatusCode, string(body))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	if err = json.Unmarshal(body, &target); err != nil {
		return fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return nil
}

func buildURL(host, apiPath string) string {
	if host == "" {
		return apiPath
	}
	if host[len(host)-1] == '/' {
		return host + apiPath
	}
	return host + "/" + apiPath
}
