package sql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/b87/go-crit/crit"
	"github.com/b87/go-crit/crit/mocks"
)

func TestParseOperator(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Equal1", "eq", "eq"},
		{"Equal2", "=", "eq"},
		{"Equal3", "==", "eq"},
		{"NotEqual1", "neq", "neq"},
		{"NotEqual2", "!=", "neq"},
		{"NotEqual3", "<>", "neq"},
		{"GreaterThan1", "gt", "gt"},
		{"GreaterThan2", ">", "gt"},
		{"GreaterThanOrEqual1", "gte", "gte"},
		{"GreaterThanOrEqual2", ">=", "gte"},
		{"LessThan1", "lt", "lt"},
		{"LessThan2", "<", "lt"},
		{"LessThanOrEqual1", "lte", "lte"},
		{"LessThanOrEqual2", "<=", "lte"},
		{"Contains1", "contains", "contains"},
		{"Contains2", "like", "contains"},
		{"In", "in", "in"},
		{"IsNull1", "null", "isnull"},
		{"IsNull2", "isnull", "isnull"},
		{"Default", "unknown", "eq"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseOperator(tt.input)
			assert.Equal(t, tt.expected, string(result), "Operator parsing failed for %s", tt.input)
		})
	}
}

func TestCommandStructure(t *testing.T) {
	// Test that all expected commands are registered
	assert.NotNil(t, sqlCmd, "SQL command should be defined")
	assert.NotNil(t, sqlConnectCmd, "SQL connect command should be defined")
	assert.NotNil(t, sqlFindCmd, "SQL find command should be defined")

	// Test command hierarchy
	var foundConnect, foundFind bool
	for _, cmd := range sqlCmd.Commands() {
		switch cmd.Name() {
		case "test":
			foundConnect = true
		case "find":
			foundFind = true
		}
	}

	assert.True(t, foundConnect, "connect command should be registered as a subcommand of sql")
	assert.True(t, foundFind, "find command should be registered as a subcommand of sql")

	// Test that required flags are defined
	findFlags := sqlFindCmd.Flags()
	assert.NotNil(t, findFlags.Lookup("table"), "table flag should be defined")
	assert.NotNil(t, findFlags.Lookup("limit"), "limit flag should be defined")
	assert.NotNil(t, findFlags.Lookup("page"), "page flag should be defined")
	assert.NotNil(t, findFlags.Lookup("fields"), "fields flag should be defined")
	assert.NotNil(t, findFlags.Lookup("filter"), "filter flag should be defined")
	assert.NotNil(t, findFlags.Lookup("sort"), "sort flag should be defined")

	// Test global flags
	globalFlags := sqlCmd.PersistentFlags()
	assert.NotNil(t, globalFlags.Lookup("driver"), "driver flag should be defined")
	assert.NotNil(t, globalFlags.Lookup("dsn"), "dsn flag should be defined")
	assert.NotNil(t, globalFlags.Lookup("format"), "format flag should be defined")
	assert.NotNil(t, globalFlags.Lookup("timeout"), "timeout flag should be defined")
}

// TestCriteriaFilterMock demonstrates how to use the CriteriaFilter mock
func TestCriteriaFilterMock(t *testing.T) {
	// Create a mock CriteriaFilter
	mockFilter := mocks.NewCriteriaFilter[struct{ Name string }](t)

	// Set up expectations
	testObj := struct{ Name string }{"test"}
	expectedFilters := []crit.Filter{
		{Field: "name", Operator: crit.OperatorEqual, Value: "test"},
	}

	// Configure the mock to return our expected filters
	mockFilter.EXPECT().ToFilters(testObj).Return(expectedFilters)

	// Use the mock
	filters := mockFilter.ToFilters(testObj)

	// Assert results
	assert.Equal(t, 1, len(filters))
	assert.Equal(t, "name", filters[0].Field)
	assert.Equal(t, crit.OperatorEqual, filters[0].Operator)
	assert.Equal(t, "test", filters[0].Value)
}

// TestQueryExecutorMock demonstrates how to use the QueryExecutor mock
func TestQueryExecutorMock(t *testing.T) {
	// Create a mock QueryExecutor with appropriate types
	mockExecutor := mocks.NewQueryExecutor[string, map[string]interface{}](t)

	// Set up sample data
	sampleQuery := "SELECT * FROM users WHERE name = 'test'"
	sampleResults := []map[string]interface{}{
		{"id": 1, "name": "test", "age": 20},
	}

	// Create a context for the test
	ctx := context.Background()

	// Configure the mock
	mockExecutor.EXPECT().Execute(mock.MatchedBy(func(c context.Context) bool {
		return true // Accept any context
	}), sampleQuery).Return(sampleResults, int64(1), nil)

	// Use the mock
	results, count, err := mockExecutor.Execute(ctx, sampleQuery)

	// Assert results
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, 1, results[0]["id"])
	assert.Equal(t, "test", results[0]["name"])
	assert.Equal(t, 20, results[0]["age"])
}
