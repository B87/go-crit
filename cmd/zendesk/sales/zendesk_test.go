package zendesk_sales

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/b87/go-crit/crit"
)

// TestAuthentication tests the testAuthentication function with a mock server
func TestAuthentication(t *testing.T) {
	// Create a mock server to simulate Zendesk API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the path is correct
		if r.URL.Path != "/v2/users/me" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Check if Authorization header is present
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Successful response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"data": {"id": 123, "name": "Test User"}}`)
	}))
	defer server.Close()

	// Save original viper value and restore after test
	origToken := viper.GetString("token")
	viper.Set("token", "test-token")
	defer viper.Set("token", origToken)

	// Create a buffer to capture output
	var buf bytes.Buffer
	// Temporarily redirect output to our buffer
	originalOutput := outputWriter
	outputWriter = &buf
	defer func() {
		outputWriter = originalOutput
	}()

	// Create a standard client (no need to mock this)
	client := &http.Client{}

	// Test successful authentication
	result := testAuthentication(client, server.URL)
	assert.True(t, result, "Authentication should succeed with valid token")

	// Verify output contains expected content
	output := buf.String()
	assert.Contains(t, output, "Making test request to:")
	assert.Contains(t, output, "Zendesk API connection successful")
}

// TestFindContacts tests the testFindContacts function with a mock repository
func TestFindContacts(t *testing.T) {
	// Create a mock server to simulate Zendesk API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the path is correct
		if r.URL.Path != "/v2/contacts" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Return successful response with contacts
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
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
				},
				{
					"id": 2,
					"name": "Jane Doe",
					"first_name": "Jane",
					"last_name": "Doe",
					"email": "jane@example.com",
					"customer_status": "current",
					"created_at": "2023-02-15T14:20:00Z"
				}
			],
			"meta": {
				"total": 2
			}
		}`)
	}))
	defer server.Close()

	// Create a real client to use with the mock server
	client := &http.Client{}

	// Create a buffer to capture output
	var buf bytes.Buffer
	// Temporarily redirect output to our buffer
	originalOutput := outputWriter
	outputWriter = &buf
	defer func() {
		outputWriter = originalOutput
	}()

	// Run the testFindContacts function
	testFindContacts(client, server.URL)

	// Get the captured output
	output := buf.String()

	// Verify output contains expected content
	assert.Contains(t, output, "Successfully retrieved 2 contacts")
	assert.Contains(t, output, "First contact:")
	assert.Contains(t, output, "ID: 1")
	assert.Contains(t, output, "Name: John Smith")
	assert.Contains(t, output, "Email: john@example.com")
}

// TestCommandSetup tests the command setup and flags
func TestCommandSetup(t *testing.T) {
	// Test that the command was registered with right flags
	assert.NotNil(t, testCmd, "Test command should be defined")

	// Ensure the check-find flag is set up
	flags := testCmd.Flags()
	checkFindFlag := flags.Lookup("check-find")
	assert.NotNil(t, checkFindFlag, "check-find flag should be defined")
	assert.Equal(t, "bool", checkFindFlag.Value.Type(), "check-find should be a boolean flag")
}

// TestCommandRun tests the Run function of the test command
func TestCommandRun(t *testing.T) {
	// Save the original Run function and restore it after the test
	originalRun := testCmd.Run
	defer func() {
		testCmd.Run = originalRun
	}()

	// Create a new Run function that uses a mock client
	testCmd.Run = func(cmd *cobra.Command, args []string) {
		// Use our own client instead of calling createZendeskClient
		client := &http.Client{}

		// Get the base URL
		baseURL := viper.GetString("zendesk.base_url")
		fmt.Fprintf(outputWriter, "Testing connection to Zendesk API at %s\n", baseURL)

		// Test authentication first
		fmt.Fprintln(outputWriter, "\n[1/2] Testing authentication...")
		if !testAuthentication(client, baseURL) {
			return
		}

		// Test finding contacts if requested
		checkFind, _ := cmd.Flags().GetBool("check-find")
		if checkFind {
			fmt.Fprintln(outputWriter, "\n[2/2] Testing contact finding...")
			testFindContacts(client, baseURL)
		} else {
			fmt.Fprintln(outputWriter, "\n✅ Authentication test passed. Use --check-find to also test contact finding.")
		}
	}

	// Create a test server for the API calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Successful response for any endpoint
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if r.URL.Path == "/v2/users/me" {
			fmt.Fprintln(w, `{"data": {"id": 123, "name": "Test User"}}`)
		} else if r.URL.Path == "/v2/contacts" {
			fmt.Fprintln(w, `{
				"data": [
					{
						"id": 1,
						"name": "John Smith",
						"first_name": "John",
						"last_name": "Smith",
						"email": "john@example.com"
					}
				],
				"meta": {
					"total": 1
				}
			}`)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Set viper configuration
	viper.Set("zendesk.base_url", server.URL)
	viper.Set("token", "test-token")

	// Create a temporary command to execute
	cmd := &cobra.Command{}
	cmd.Flags().Bool("check-find", true, "")

	// Create a buffer to capture output
	var buf bytes.Buffer
	// Temporarily redirect output to our buffer
	originalOutput := outputWriter
	outputWriter = &buf
	defer func() {
		outputWriter = originalOutput
	}()

	// Execute the command's Run function
	testCmd.Run(cmd, []string{})

	// Get the captured output
	output := buf.String()

	// Verify the output
	assert.Contains(t, output, "Testing connection to Zendesk API")
	assert.Contains(t, output, "Testing authentication")
	assert.Contains(t, output, "Zendesk API connection successful")
	assert.Contains(t, output, "Testing contact finding")
}

// Writer for output - can be replaced in tests to capture output
var outputWriter io.Writer = io.Discard // Default to discarding output in tests

// testCmd represents the test command for verifying Zendesk API connectivity
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test the Zendesk API connection",
	Long: `Test the Zendesk API connection and authentication.
This command makes simple API calls to verify that your credentials
and configuration are working correctly.

Examples:
  go-crit zendesk test
  go-crit zendesk test --check-find`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get the client
		client, err := createZendeskClient()
		if err != nil {
			fmt.Fprintf(outputWriter, "Error creating Zendesk client: %v\n", err)
			return
		}

		// Get the base URL
		baseURL := viper.GetString("zendesk.base_url")
		fmt.Fprintf(outputWriter, "Testing connection to Zendesk API at %s\n", baseURL)

		// Test authentication first
		fmt.Fprintln(outputWriter, "\n[1/2] Testing authentication...")
		if !testAuthentication(client, baseURL) {
			return
		}

		// Test finding contacts if requested
		checkFind, _ := cmd.Flags().GetBool("check-find")
		if checkFind {
			fmt.Fprintln(outputWriter, "\n[2/2] Testing contact finding...")
			testFindContacts(client, baseURL)
		} else {
			fmt.Fprintln(outputWriter, "\n✅ Authentication test passed. Use --check-find to also test contact finding.")
		}
	},
}

func init() {
	zendeskCmd.AddCommand(testCmd)
	testCmd.Flags().Bool("check-find", false, "Also test the contact finding functionality")
	// Set outputWriter to stdout for normal operation
	outputWriter = os.Stdout
}

// testAuthentication tests the authentication with Zendesk API
func testAuthentication(client *http.Client, baseURL string) bool {
	// Make a simple request to verify connectivity
	testURL := fmt.Sprintf("%s/v2/users/me", baseURL)
	fmt.Fprintf(outputWriter, "Making test request to: %s\n", testURL)

	// Create request
	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		fmt.Fprintf(outputWriter, "Error creating request: %v\n", err)
		return false
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+viper.GetString("token"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "go-crit/1.0")

	// Set timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	// Execute request
	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)
	if err != nil {
		fmt.Fprintf(outputWriter, "❌ Error connecting to Zendesk API: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(outputWriter, "❌ Error reading response: %v\n", err)
		return false
	}

	// Print the response details
	fmt.Fprintf(outputWriter, "Response status: %d %s\n", resp.StatusCode, resp.Status)
	fmt.Fprintf(outputWriter, "Response time: %v\n", duration)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Fprintln(outputWriter, "✅ Zendesk API connection successful")
		fmt.Fprintln(outputWriter, "Token is valid and API is accessible")
		return true
	} else {
		fmt.Fprintln(outputWriter, "❌ Zendesk API connection failed")
		if resp.StatusCode == 401 {
			fmt.Fprintln(outputWriter, "Authentication error - please check your API token")
		} else {
			fmt.Fprintf(outputWriter, "Error response: %s\n", string(body))
		}
		return false
	}
}

// testFindContacts tests finding contacts from Zendesk
func testFindContacts(client *http.Client, baseURL string) {
	// Create a minimal criteria object
	criteria := crit.NewCriteria().
		SetPagination(1, 5) // Request only 5 items for the test

	// Include a minimal filter to test compatibility
	if viper.GetBool("test.include_filter") {
		criteria.AddFilter("updated_at", crit.OperatorGreaterThanOrEqual, time.Now().AddDate(-1, 0, 0).Format(time.RFC3339))
	}

	// Try to execute the query
	fmt.Fprintln(outputWriter, "Testing contact finding with minimal criteria...")
	contacts, total, err := queryZendeskContacts(client, baseURL, criteria)

	if err != nil {
		fmt.Fprintf(outputWriter, "❌ Error finding contacts: %v\n", err)
		fmt.Fprintln(outputWriter, "\nTroubleshooting tips:")
		fmt.Fprintln(outputWriter, "1. Check that your API token has access to contacts")
		fmt.Fprintln(outputWriter, "2. Try running with --verbose to see the full request details")
		fmt.Fprintln(outputWriter, "3. Verify the base URL is correct (default: https://api.getbase.com)")
		return
	}

	// Success!
	fmt.Fprintf(outputWriter, "✅ Successfully retrieved %d contacts (total: %d)\n", len(contacts), total)
	if len(contacts) > 0 {
		fmt.Fprintln(outputWriter, "\nFirst contact:")
		name := contacts[0].Name
		if name == "" && !contacts[0].IsOrganization {
			name = contacts[0].FirstName + " " + contacts[0].LastName
		}
		fmt.Fprintf(outputWriter, "  ID: %d\n", contacts[0].ID)
		fmt.Fprintf(outputWriter, "  Name: %s\n", name)
		fmt.Fprintf(outputWriter, "  Email: %s\n", contacts[0].Email)
	} else {
		fmt.Fprintln(outputWriter, "\nNo contacts were found. This could be normal if your account is empty.")
	}

	fmt.Fprintln(outputWriter, "\n✅ All tests passed!")
}
