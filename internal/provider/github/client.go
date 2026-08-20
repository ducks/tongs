package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ducks/tongs/internal/config"
	"github.com/ducks/tongs/internal/provider"
)

type Client struct {
	host       string
	token      string
	apiURL     string
	graphqlURL string
	http       *http.Client
}

func New(host string) (*Client, error) {
	if host == "" {
		host = "github.com"
	}
	token, err := config.Token("github", host)
	if err != nil {
		return nil, &provider.Error{Code: "authentication_missing", Message: err.Error()}
	}

	apiURL := "https://api.github.com"
	graphqlURL := apiURL + "/graphql"
	if host != "github.com" {
		apiURL = "https://" + host + "/api/v3"
		graphqlURL = "https://" + host + "/api/graphql"
	}

	return &Client{
		host:       host,
		token:      token,
		apiURL:     apiURL,
		graphqlURL: graphqlURL,
		http:       &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *Client) Name() string { return "github" }

func (c *Client) rest(ctx context.Context, method, requestPath string, input, output interface{}) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode GitHub request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, c.apiURL+requestPath, body)
	if err != nil {
		return fmt.Errorf("create GitHub request: %w", err)
	}
	c.setHeaders(request)

	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("GitHub request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return decodeAPIError(response)
	}
	if output == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(output); err != nil {
		return fmt.Errorf("decode GitHub response: %w", err)
	}
	return nil
}

func (c *Client) graphql(ctx context.Context, query string, variables, output interface{}) error {
	payload := map[string]interface{}{"query": query, "variables": variables}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode GitHub GraphQL request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.graphqlURL, bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("create GitHub GraphQL request: %w", err)
	}
	c.setHeaders(request)

	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("GitHub GraphQL request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return decodeAPIError(response)
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string        `json:"message"`
			Path    []interface{} `json:"path"`
			Type    string        `json:"type"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode GitHub GraphQL response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		return &provider.Error{
			Code:    "github_graphql_error",
			Message: envelope.Errors[0].Message,
			Details: envelope.Errors,
		}
	}
	if err := json.Unmarshal(envelope.Data, output); err != nil {
		return fmt.Errorf("decode GitHub GraphQL data: %w", err)
	}
	return nil
}

func (c *Client) setHeaders(request *http.Request) {
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "tongs/0.1")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

func decodeAPIError(response *http.Response) error {
	var payload struct {
		Message          string      `json:"message"`
		Errors           interface{} `json:"errors"`
		DocumentationURL string      `json:"documentation_url"`
	}
	_ = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload)
	message := payload.Message
	if message == "" {
		message = response.Status
	}
	return &provider.Error{
		Code:       githubErrorCode(response.StatusCode),
		Message:    message,
		StatusCode: response.StatusCode,
		Details: map[string]interface{}{
			"errors":            payload.Errors,
			"documentation_url": payload.DocumentationURL,
		},
	}
}

func githubErrorCode(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "authentication_failed"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusUnprocessableEntity:
		return "validation_failed"
	case http.StatusTooManyRequests:
		return "rate_limited"
	default:
		return "github_api_error"
	}
}

func repoPath(repo provider.Repository) string {
	return "/repos/" + url.PathEscape(repo.Owner) + "/" + url.PathEscape(repo.Name)
}

func parseTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return &parsed
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func normalizeState(value string) string { return strings.ToLower(value) }
