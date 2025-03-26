package crit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPQueryBuilder_BuildQuery_InjectionProtection(t *testing.T) {
	testCases := []struct {
		name     string
		criteria *Criteria
		expected map[string]string // Expected URL parameters
	}{
		{
			name: "Injection in field name",
			criteria: NewCriteria().AddFilter(
				"username<script>alert(1)</script>",
				OperatorEqual,
				"value",
			),
			expected: map[string]string{
				"username<script>alert(1)</script>": "value",
			},
		},
		{
			name: "Injection in string value",
			criteria: NewCriteria().AddFilter(
				"username",
				OperatorEqual,
				"<script>alert(1)</script>",
			),
			expected: map[string]string{
				"username": "<script>alert(1)</script>",
			},
		},
		{
			name: "Injection in sort field",
			criteria: NewCriteria().AddSort(
				"name<script>alert(1)</script>",
				OrderAsc,
			),
			expected: map[string]string{
				"sort": "name<script>alert(1)</script>",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := NewHTTPQueryBuilder("https://api.example.com", "/products")
			query, err := builder.BuildQuery(tc.criteria)

			require.NoError(t, err)

			// Check that the parameters are properly URL encoded
			for key, value := range tc.expected {
				assert.Equal(t, value, query.QueryParams.Get(key), "Parameter %s should have value %s", key, value)
			}
		})
	}
}

func TestHTTPQueryExecutor_Execute_InjectionProtection(t *testing.T) {
	// Define a test entity type
	type TestEntity struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	// Test cases for different injection scenarios
	testCases := []struct {
		name        string
		query       HTTPQuery
		mockHandler func(w http.ResponseWriter, r *http.Request)
		expectError bool
	}{
		{
			name: "URL path injection attempt",
			query: HTTPQuery{
				BaseURL:     "https://example.com",
				Path:        "/products/../admin",
				QueryParams: nil,
			},
			mockHandler: func(w http.ResponseWriter, r *http.Request) {
				// Ensure the path is not altered - Go's http client normalizes paths
				assert.Equal(t, "/products/../admin", r.URL.Path)

				// Return a valid response
				response := map[string]interface{}{
					"data": []TestEntity{
						{ID: 1, Name: "Test Product"},
					},
					"meta": map[string]interface{}{
						"total": 1,
					},
				}
				json.NewEncoder(w).Encode(response)
			},
			expectError: false,
		},
		{
			name: "Query parameter injection",
			query: HTTPQuery{
				BaseURL: "https://example.com",
				Path:    "/products",
				QueryParams: map[string][]string{
					"search": {"<script>alert(1)</script>"},
				},
			},
			mockHandler: func(w http.ResponseWriter, r *http.Request) {
				// Ensure the parameter is properly URL encoded
				assert.Equal(t, "<script>alert(1)</script>", r.URL.Query().Get("search"))

				// Return a valid response
				response := map[string]interface{}{
					"data": []TestEntity{},
					"meta": map[string]interface{}{
						"total": 0,
					},
				}
				json.NewEncoder(w).Encode(response)
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(tc.mockHandler))
			defer server.Close()

			// Replace the base URL with the test server URL
			tc.query.BaseURL = server.URL

			// Create the executor
			executor := NewHTTPQueryExecutor[TestEntity](http.DefaultClient)

			// Execute the query
			ctx := context.Background()
			_, _, err := executor.Execute(ctx, tc.query)

			// Verify the result
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPValidator_Validate_FieldSanitization(t *testing.T) {
	// Create a validator with allowed fields
	allowedFields := []string{"id", "name", "email", "created_at"}
	validator := NewHTTPValidator(allowedFields, 100)

	// Test validation with malicious field names
	testCases := []struct {
		name          string
		criteria      *Criteria
		expectError   bool
		errorContains string
	}{
		{
			name: "Valid fields",
			criteria: NewCriteria().
				AddFilter("id", OperatorEqual, 1).
				AddFilter("name", OperatorContains, "test").
				AddSort("created_at", OrderDesc),
			expectError: false,
		},
		{
			name: "Injection in filter field",
			criteria: NewCriteria().
				AddFilter("id", OperatorEqual, 1).
				AddFilter("<script>alert(1)</script>", OperatorEqual, "test"),
			expectError:   true,
			errorContains: "invalid field",
		},
		{
			name: "Injection in sort field",
			criteria: NewCriteria().
				AddFilter("id", OperatorEqual, 1).
				AddSort("<script>alert(1)</script>", OrderAsc),
			expectError:   true,
			errorContains: "invalid sort field",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate the criteria
			err := validator.Validate(tc.criteria)

			// Verify the result
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Integration test to verify the entire HTTP query building and execution pipeline
func TestIntegration_HTTPQueryPipeline_InjectionProtection(t *testing.T) {
	// Define a test entity and model
	type TestEntity struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	type TestModel struct {
		ID   int
		Name string
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the request is valid
		if r.URL.Path == "/products" {
			// Get query parameters
			query := r.URL.Query()

			// Check for injection attempts in parameters
			// Here we're simulating a security check
			if query.Get("name") == "<script>alert(1)</script>" {
				// In a real scenario, this might still be safe due to proper encoding
				// and output escaping in the client application
				w.WriteHeader(http.StatusOK)
				response := map[string]interface{}{
					"data": []TestEntity{},
					"meta": map[string]interface{}{
						"total": 0,
					},
				}
				json.NewEncoder(w).Encode(response)
				return
			}

			// Return success for valid queries
			w.WriteHeader(http.StatusOK)
			response := map[string]interface{}{
				"data": []TestEntity{
					{ID: 1, Name: "Test Product"},
				},
				"meta": map[string]interface{}{
					"total": 1,
				},
			}
			json.NewEncoder(w).Encode(response)
		} else {
			// Invalid path
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create the repository components
	httpBuilder := NewHTTPQueryBuilder(server.URL, "/products")
	httpExecutor := NewHTTPQueryExecutor[TestEntity](http.DefaultClient)
	httpValidator := NewHTTPValidator([]string{"id", "name", "category"}, 100)
	dataMapper := NewGenericHTTPMapper(
		func() TestModel { return TestModel{} },
		func() TestEntity { return TestEntity{} },
	)

	// Create the repository
	repo := NewGenericRepository(
		httpBuilder,
		httpExecutor,
		dataMapper,
		httpValidator,
	)

	// Test cases
	testCases := []struct {
		name          string
		criteria      *Criteria
		expectError   bool
		errorContains string
	}{
		{
			name: "Valid criteria",
			criteria: NewCriteria().
				AddFilter("name", OperatorEqual, "test_product").
				AddSort("id", OrderAsc),
			expectError: false,
		},
		{
			name: "Injection in value",
			criteria: NewCriteria().
				AddFilter("name", OperatorEqual, "<script>alert(1)</script>").
				AddSort("id", OrderAsc),
			expectError: false, // This should not error as values are properly handled
		},
		{
			name: "Injection in field",
			criteria: NewCriteria().
				AddFilter("<script>alert(1)</script>", OperatorEqual, "test"),
			expectError:   true,
			errorContains: "invalid field",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Execute the query
			ctx := context.Background()
			_, _, err := repo.Find(ctx, tc.criteria)

			// Verify the result
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
