package zendesk_sales

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/viper"

	"github.com/b87/go-crit/crit"
)

// ZendeskHTTPResponse represents the response format from Zendesk API
type ZendeskHTTPResponse struct {
	Data []ZendeskContactEntity `json:"data"`
	Meta struct {
		Total int64 `json:"total"`
	} `json:"meta"`
}

// ZendeskHTTPExecutor is a custom executor for Zendesk API
type ZendeskHTTPExecutor struct {
	client *http.Client
	mapper *ZendeskResponseMapper
}

// NewZendeskHTTPExecutor creates a new executor for Zendesk
func NewZendeskHTTPExecutor(client *http.Client) *ZendeskHTTPExecutor {
	if client == nil {
		client = http.DefaultClient
	}
	return &ZendeskHTTPExecutor{
		client: client,
		mapper: &ZendeskResponseMapper{},
	}
}

// Execute performs the HTTP request and handles Zendesk-specific response parsing
func (e *ZendeskHTTPExecutor) Execute(ctx context.Context, query crit.HTTPQuery) ([]ZendeskContactEntity, int64, error) {
	// Build full URL
	fullURL := buildFullURL(query)

	// Log request details in verbose mode
	logRequestDetails(fullURL, query)

	// Create and execute request
	resp, err := e.executeRequest(ctx, fullURL)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("error processing response: %v", resp)
	}

	// Read and parse response body
	contacts, total, err := e.parseResponseBody(resp)
	if err != nil {
		return nil, 0, fmt.Errorf("error processing response: %v", err)
	}

	return contacts, total, nil
}

// buildFullURL combines the base URL and path with query parameters
func buildFullURL(query crit.HTTPQuery) string {
	// Ensure there's a slash between baseURL and path if needed
	fullURL := query.BaseURL
	if !strings.HasSuffix(fullURL, "/") && !strings.HasPrefix(query.Path, "/") {
		fullURL += "/"
	}
	fullURL += query.Path

	// Add query parameters if present
	if len(query.QueryParams) > 0 {
		fullURL += "?" + query.QueryParams.Encode()
	}

	return fullURL
}

// logRequestDetails logs details about the request in verbose mode
func logRequestDetails(fullURL string, query crit.HTTPQuery) {
	if viper.GetBool("verbose") {
		fmt.Printf("Making request to: %s\n", fullURL)
		if len(query.QueryParams) > 0 {
			fmt.Printf("Query parameters: %v\n", query.QueryParams)
		}
	}
}

// executeRequest creates and executes an HTTP request
func (e *ZendeskHTTPExecutor) executeRequest(ctx context.Context, fullURL string) (*http.Response, error) {
	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Execute request
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing request: %w", err)
	}

	return resp, nil
}

// parseResponseBody reads and parses the response body
func (e *ZendeskHTTPExecutor) parseResponseBody(resp *http.Response) ([]ZendeskContactEntity, int64, error) {
	// Read the entire response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("error reading response body: %w", err)
	}

	// Log response in verbose mode
	if viper.GetBool("verbose") {
		fmt.Printf("Response status: %d\n", resp.StatusCode)
		fmt.Printf("Response body: %s\n", string(body))
	}

	// Use our response mapper to parse the body
	return e.mapper.MapResponse(body)
}
