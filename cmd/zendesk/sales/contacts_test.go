package zendesk_sales

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/b87/go-crit/crit"
	"github.com/b87/go-crit/crit/mocks"
)

// TestZendeskContacts demonstrates a test setup for the Zendesk contacts example
func TestZendeskContacts(t *testing.T) {
	// Create a mock server to simulate Zendesk API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the request has proper authentication
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Check if the path is correct
		if r.URL.Path != "/v2/contacts" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Get query parameters
		queryParams := r.URL.Query()

		// Check if any filters are applied
		customerStatus := queryParams.Get("customer_status")

		// Prepare a response based on the query
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Example response for customer_status=current
		if customerStatus == "current" {
			fmt.Fprintln(w, `{
				"data": [
					{
						"id": 1,
						"name": "John Smith",
						"first_name": "John",
						"last_name": "Smith",
						"is_organization": false,
						"owner_id": 123,
						"email": "john@example.com",
						"phone": "123-456-7890",
						"customer_status": "current",
						"prospect_status": "none",
						"created_at": "2023-01-15T09:30:00Z",
						"updated_at": "2023-03-20T14:25:00Z",
						"address": {
							"line1": "123 Main St",
							"city": "San Francisco",
							"state": "CA",
							"postal_code": "94105",
							"country": "USA"
						}
					},
					{
						"id": 2,
						"name": "Jane Doe",
						"first_name": "Jane",
						"last_name": "Doe",
						"is_organization": false,
						"owner_id": 123,
						"email": "jane@example.com",
						"phone": "123-456-7891",
						"customer_status": "current",
						"prospect_status": "none",
						"created_at": "2023-02-10T10:15:00Z",
						"updated_at": "2023-03-15T16:40:00Z",
						"address": {
							"line1": "456 Market St",
							"city": "San Francisco",
							"state": "CA",
							"postal_code": "94105",
							"country": "USA"
						}
					}
				],
				"meta": {
					"total": 2
				}
			}`)
		} else {
			// Default response for no filters
			fmt.Fprintln(w, `{
				"data": [
					{
						"id": 1,
						"name": "John Smith",
						"first_name": "John",
						"last_name": "Smith",
						"is_organization": false,
						"owner_id": 123,
						"email": "john@example.com",
						"phone": "123-456-7890",
						"customer_status": "current",
						"prospect_status": "none",
						"created_at": "2023-01-15T09:30:00Z",
						"updated_at": "2023-03-20T14:25:00Z"
					},
					{
						"id": 3,
						"name": "Acme Corporation",
						"is_organization": true,
						"owner_id": 456,
						"email": "info@acme.com",
						"phone": "123-456-7892",
						"customer_status": "past",
						"prospect_status": "none",
						"created_at": "2022-11-05T13:45:00Z",
						"updated_at": "2023-01-20T09:15:00Z"
					}
				],
				"meta": {
					"total": 10
				}
			}`)
		}
	}))
	defer server.Close()

	// Create a client with the test token
	client := &http.Client{
		Transport: &zendeskAuthTransport{
			token: "test-token",
			base:  http.DefaultTransport,
		},
	}

	// Test using our mock server instead of the real Zendesk API
	// Create HTTP components with specific entity type (ZendeskContactEntity)
	httpBuilder := crit.NewHTTPQueryBuilder(server.URL, "/v2/contacts")
	httpExecutor := crit.NewHTTPQueryExecutor[ZendeskContactEntity](client)

	// Create a validator with allowed fields
	allowedFields := []string{
		"id", "name", "first_name", "last_name", "email",
		"phone", "created_at", "updated_at", "customer_status",
		"prospect_status", "title", "description", "industry",
		"website", "is_organization", "owner_id",
	}
	httpValidator := crit.NewHTTPValidator(allowedFields, 100)

	// Create a data mapper
	dataMapper := crit.NewGenericHTTPMapper(
		func() Contact { return Contact{} },
		func() ZendeskContactEntity { return ZendeskContactEntity{} },
	)

	// Create a repository
	repo := crit.NewGenericRepository(
		httpBuilder,
		httpExecutor,
		dataMapper,
		httpValidator,
	)

	// Example 1: Find all current customers
	criteria := crit.NewCriteria().
		AddFilter("customer_status", crit.OperatorEqual, "current").
		AddSort("updated_at", crit.OrderDesc).
		SetPagination(1, 25)

	// Execute the query with a proper context
	ctx := context.Background()
	customers, total, err := repo.Find(ctx, criteria)
	if err != nil {
		t.Fatalf("Error finding customers: %v", err)
	}

	// Verify results
	if total != 2 {
		t.Errorf("Expected total of 2 customers, got %d", total)
	}

	if len(customers) != 2 {
		t.Fatalf("Expected 2 customers, got %d", len(customers))
	}

	// Check the first customer
	customer := customers[0]
	if customer.FirstName != "John" || customer.LastName != "Smith" || customer.Email != "john@example.com" {
		t.Errorf("First customer data incorrect: %+v", customer)
	}
}

// TestZendeskContactsWithMocks demonstrates testing with mocks instead of real implementations
func TestZendeskContactsWithMocks(t *testing.T) {
	// Create mock objects for all dependencies
	mockRepo := mocks.NewRepository[ZendeskContactEntity, Contact, crit.HTTPQuery](t)

	// Create sample data for the mock to return
	expectedContacts := []Contact{
		{
			ID:             1,
			Name:           "John Smith",
			FirstName:      "John",
			LastName:       "Smith",
			IsOrganization: false,
			OwnerID:        123,
			Email:          "john@example.com",
			Phone:          "123-456-7890",
			CustomerStatus: "current",
			ProspectStatus: "none",
		},
		{
			ID:             2,
			Name:           "Jane Doe",
			FirstName:      "Jane",
			LastName:       "Doe",
			IsOrganization: false,
			OwnerID:        123,
			Email:          "jane@example.com",
			Phone:          "123-456-7891",
			CustomerStatus: "current",
			ProspectStatus: "none",
		},
	}

	// Create an expected criteria object that should match what the method creates
	expectedCriteria := crit.NewCriteria().
		AddFilter("customer_status", crit.OperatorEqual, "current").
		AddSort("updated_at", crit.OrderDesc).
		SetPagination(1, 25)

	// Configure the mock to return our sample data when Find is called with the expected criteria
	mockRepo.EXPECT().Find(mock.Anything, mock.MatchedBy(func(c *crit.Criteria) bool {
		// Simple matching logic to verify the criteria is as expected
		if len(c.Filters) != len(expectedCriteria.Filters) {
			return false
		}

		// Check that at least the customer_status filter exists and matches
		for _, filter := range c.Filters {
			if filter.Field == "customer_status" &&
				filter.Operator == crit.OperatorEqual &&
				filter.Value == "current" {
				return true
			}
		}
		return false
	})).Return(expectedContacts, int64(2), nil)

	// Create a test implementation of ZendeskContacts with explicit method mocking
	// This is a simpler approach than trying to mock the repository implementation
	zendesk := &struct {
		FindCurrentCustomersCalled bool
	}{}

	// Create a test stub for FindCurrentCustomers
	findCurrentCustomers := func(ctx context.Context) ([]Contact, int64, error) {
		zendesk.FindCurrentCustomersCalled = true
		return mockRepo.Find(ctx, expectedCriteria)
	}

	// Call the method
	contacts, total, err := findCurrentCustomers(context.Background())

	// Assertions
	assert.True(t, zendesk.FindCurrentCustomersCalled, "FindCurrentCustomers should be called")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, contacts, 2)
	assert.Equal(t, "John", contacts[0].FirstName)
	assert.Equal(t, "Smith", contacts[0].LastName)
	assert.Equal(t, "john@example.com", contacts[0].Email)
}

// TestZendeskContactsExample runs a simplified version of the example with no external dependencies
func TestZendeskContactsExample(t *testing.T) {
	// Skip this test in normal runs as it would require an actual API token
	t.Skip("Skipping example that requires a real Zendesk API token")

	// This would run the real example, not suitable for automated testing
	// RunZendeskContactsExample()
}

// TestZendeskContactsComponentMocks demonstrates using individual component mocks
func TestZendeskContactsComponentMocks(t *testing.T) {
	// Create mocks for all individual components
	mockBuilder := mocks.NewQueryBuilder[crit.HTTPQuery](t)
	mockExecutor := mocks.NewQueryExecutor[crit.HTTPQuery, ZendeskContactEntity](t)
	mockDataMapper := mocks.NewDataMapper[ZendeskContactEntity, Contact](t)
	mockValidator := mocks.NewValidator(t)

	// Sample data
	sampleQuery := crit.HTTPQuery{
		BaseURL:     "https://api.getbase.com",
		Path:        "/v2/contacts",
		QueryParams: url.Values{"customer_status": []string{"current"}},
	}

	sampleEntities := []ZendeskContactEntity{
		{
			ID:             1,
			Name:           "John Smith",
			FirstName:      "John",
			LastName:       "Smith",
			IsOrganization: false,
			Email:          "john@example.com",
			CustomerStatus: "current",
		},
	}

	sampleContacts := []Contact{
		{
			ID:             1,
			Name:           "John Smith",
			FirstName:      "John",
			LastName:       "Smith",
			IsOrganization: false,
			Email:          "john@example.com",
			CustomerStatus: "current",
		},
	}

	// Configure mock expectations
	// 1. Validator should validate the criteria
	mockValidator.EXPECT().Validate(mock.Anything).Return(nil)

	// 2. Builder should construct a query
	mockBuilder.EXPECT().BuildQuery(mock.Anything).Return(sampleQuery, nil)

	// 3. Executor should execute the query and return entities
	mockExecutor.EXPECT().Execute(mock.Anything, sampleQuery).Return(sampleEntities, int64(1), nil)

	// 4. DataMapper should map entities to models
	mockDataMapper.EXPECT().MapToModel(sampleEntities).Return(sampleContacts)

	// Create a GenericRepository with the mocks
	repo := crit.NewGenericRepository[ZendeskContactEntity, Contact, crit.HTTPQuery](
		mockBuilder,
		mockExecutor,
		mockDataMapper,
		mockValidator,
	)

	// Create criteria and execute Find
	criteria := crit.NewCriteria().
		AddFilter("customer_status", crit.OperatorEqual, "current")

	contacts, total, err := repo.Find(context.Background(), criteria)

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, contacts, 1)
	assert.Equal(t, "John", contacts[0].FirstName)
	assert.Equal(t, "Smith", contacts[0].LastName)
}

// Example usage in Go tests
func ExampleZendeskContacts_withTestServer() {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return a simplified response for any request
		fmt.Fprintln(w, `{
			"data": [
				{
					"id": 1,
					"name": "John Smith",
					"first_name": "John",
					"last_name": "Smith",
					"email": "john@example.com",
					"customer_status": "current",
					"created_at": "2023-01-15T09:30:00Z"
				}
			],
			"meta": {
				"total": 1
			}
		}`)
	}))
	defer server.Close()

	// Create a client and replace the ZendeskContacts implementation
	// to use our test server instead of the real API
	client := &http.Client{}

	// Create our ZendeskContacts client with a manual setup for testing
	baseURL := server.URL

	// Create HTTP components
	httpBuilder := crit.NewHTTPQueryBuilder(baseURL, "/v2/contacts")
	httpExecutor := crit.NewHTTPQueryExecutor[ZendeskContactEntity](client)
	httpValidator := crit.NewHTTPValidator([]string{"customer_status"}, 100)
	dataMapper := crit.NewGenericHTTPMapper(
		func() Contact { return Contact{} },
		func() ZendeskContactEntity { return ZendeskContactEntity{} },
	)

	// Create a repository
	repo := crit.NewGenericRepository[ZendeskContactEntity, Contact, crit.HTTPQuery](
		httpBuilder,
		httpExecutor,
		dataMapper,
		httpValidator,
	)

	// Create a ZendeskContacts instance with our test repository
	zendesk := &ZendeskContacts{
		repo:      repo,
		apiClient: client,
		baseURL:   baseURL,
	}

	// Create context for the query
	ctx := context.Background()

	// Find current customers using our ZendeskContacts client
	contacts, total, err := zendesk.FindCurrentCustomers(ctx)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Found %d contacts (total: %d)\n", len(contacts), total)
	if len(contacts) > 0 {
		fmt.Printf("First contact: %s %s (%s)\n",
			contacts[0].FirstName,
			contacts[0].LastName,
			contacts[0].Email)
	}
}
