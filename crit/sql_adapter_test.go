package crit

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLQueryBuilder_BuildQuery_SQLInjectionProtection(t *testing.T) {
	testCases := []struct {
		name              string
		criteria          *Criteria
		expectContains    []string
		expectNotContains []string
	}{
		{
			name: "SQL injection in field name",
			criteria: NewCriteria().AddFilter(
				"username; DROP TABLE users;--",
				OperatorEqual,
				"value",
			),
			expectContains: []string{
				"usernameDROPTABLEusers", // Sanitized field name
				"$1",                     // Should use parameterized query
			},
			expectNotContains: []string{
				"DROP TABLE", // Shouldn't execute as SQL
			},
		},
		{
			name: "SQL injection in string value",
			criteria: NewCriteria().AddFilter(
				"username",
				OperatorEqual,
				"' OR '1'='1",
			),
			expectContains: []string{
				"username = $1", // Should use parameterized query
			},
			expectNotContains: []string{
				"OR '1'='1", // Shouldn't be part of the query string
			},
		},
		{
			name: "SQL injection in IN operator",
			criteria: NewCriteria().AddFilter(
				"id",
				OperatorIn,
				[]interface{}{1, "2; DROP TABLE users;--", 3},
			),
			expectContains: []string{
				"id IN ($1, $2, $3)", // Should use parameterized query
			},
			expectNotContains: []string{
				"DROP TABLE", // Shouldn't be part of the query string
			},
		},
		{
			name: "SQL injection in sort field",
			criteria: NewCriteria().AddSort(
				"name; DROP TABLE users;--",
				OrderAsc,
			),
			expectContains: []string{
				"nameDROPTABLEusers ASC", // Sanitized field name
			},
			expectNotContains: []string{
				"DROP TABLE users", // Shouldn't execute as SQL
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			builder := NewSQLQueryBuilder("users")
			query, err := builder.BuildQuery(tc.criteria)

			require.NoError(t, err)

			// Check that expected strings are in the query
			for _, s := range tc.expectContains {
				assert.Contains(t, query.Query, s)
			}

			// Check that unwanted strings are not in the query
			for _, s := range tc.expectNotContains {
				assert.NotContains(t, query.Query, s)
			}

			// For injection in values, check that they're in the args slice
			if tc.name == "SQL injection in string value" {
				assert.Equal(t, "' OR '1'='1", query.Args[0])
			}
		})
	}
}

func TestSQLQueryExecutor_Execute_SQLInjectionProtection(t *testing.T) {
	// Create a mock database connection
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	dbx := sqlx.NewDb(db, "sqlmock")

	// Define a test entity type
	type TestEntity struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	// Create a SQL executor
	executor := NewSQLQueryExecutor[TestEntity](dbx)

	// Setup test cases with SQL injection attempts
	testCases := []struct {
		name        string
		query       SQLQuery
		mockSetup   func(sqlmock.Sqlmock)
		expectError bool
	}{
		{
			name: "SQL injection in query string",
			query: SQLQuery{
				Query:  "SELECT * FROM users WHERE name = $1; DROP TABLE users;--",
				Args:   []interface{}{"test_user"},
				Tables: []string{"users"},
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				// Even though the query contains malicious SQL, the driver should
				// prevent multiple statements, and our mock should receive only the first part
				mock.ExpectQuery("SELECT \\* FROM users WHERE name = \\$1").
					WithArgs("test_user").
					WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
						AddRow(1, "test_user"))

				mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			},
			expectError: false,
		},
		{
			name: "SQL injection in parameter",
			query: SQLQuery{
				Query:  "SELECT * FROM users WHERE name = $1",
				Args:   []interface{}{"test_user' OR '1'='1"},
				Tables: []string{"users"},
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				// The SQL injection attempt should be safely passed as a parameter
				mock.ExpectQuery("SELECT \\* FROM users WHERE name = \\$1").
					WithArgs("test_user' OR '1'='1").
					WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

				mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup the mock expectations
			tc.mockSetup(mock)

			// Execute the query
			ctx := context.Background()
			_, _, err := executor.Execute(ctx, tc.query)

			// Verify the result
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSQLQueryBuilder_BuildWhereClause_Parameterization(t *testing.T) {
	builder := NewSQLQueryBuilder("users")

	// Test different filter types to ensure proper parameterization
	testCases := []struct {
		name           string
		filter         Filter
		expectedClause string
		expectedArgs   []interface{}
	}{
		{
			name: "Equal operator with string value",
			filter: Filter{
				Field:    "username",
				Operator: OperatorEqual,
				Value:    "admin' OR '1'='1",
			},
			expectedClause: "username = $1",
			expectedArgs:   []interface{}{"admin' OR '1'='1"},
		},
		{
			name: "Contains operator with SQL injection",
			filter: Filter{
				Field:    "description",
				Operator: OperatorContains,
				Value:    "test'; DROP TABLE users;--",
			},
			expectedClause: "description LIKE $1",
			expectedArgs:   []interface{}{"%test'; DROP TABLE users;--%"},
		},
		{
			name: "In operator with injection attempt",
			filter: Filter{
				Field:    "id",
				Operator: OperatorIn,
				Value:    []interface{}{1, "2; DROP TABLE users;--", 3},
			},
			expectedClause: "id IN ($1, $2, $3)",
			expectedArgs:   []interface{}{1, "2; DROP TABLE users;--", 3},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset the parameter index
			builder.parameterIndex = 0

			// Build the where clause
			clause, args := builder.buildWhereClause(tc.filter)

			// Verify the result
			assert.Equal(t, tc.expectedClause, clause)
			assert.Equal(t, tc.expectedArgs, args)
		})
	}
}

func TestSQLValidator_Validate_FieldSanitization(t *testing.T) {
	// Create a validator with allowed fields
	validator := NewSQLValidator(ValidationConfig{
		AllowedFields:     []string{"id", "name", "email", "created_at"},
		AllowedSortFields: []string{"id", "name", "created_at"},
	})

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
			name: "SQL injection in filter field",
			criteria: NewCriteria().
				AddFilter("id", OperatorEqual, 1).
				AddFilter("name; DROP TABLE users;--", OperatorEqual, "test"),
			expectError:   true,
			errorContains: "invalid field",
		},
		{
			name: "SQL injection in sort field",
			criteria: NewCriteria().
				AddFilter("id", OperatorEqual, 1).
				AddSort("name; DROP TABLE users;--", OrderAsc),
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
				assert.True(t, strings.Contains(err.Error(), tc.errorContains))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Integration test to verify the entire query building and execution pipeline
func TestIntegration_SQLQueryPipeline_SQLInjectionProtection(t *testing.T) {
	// Create a mock database connection
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	dbx := sqlx.NewDb(db, "sqlmock")

	// Define a test entity and model
	type TestEntity struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	type TestModel struct {
		ID   int
		Name string
	}

	// Create the repository components
	sqlBuilder := NewSQLQueryBuilder("users")
	sqlExecutor := NewSQLQueryExecutor[TestEntity](dbx)
	validator := NewSQLValidator(ValidationConfig{
		AllowedFields:     []string{"id", "name"},
		AllowedSortFields: []string{"id", "name"},
	})
	dataMapper := NewEntityMapper(
		// Model to Entity mapping
		func(model TestModel) TestEntity {
			return TestEntity{
				ID:   model.ID,
				Name: model.Name,
			}
		},
		// Entity to Model mapping
		func(entity TestEntity) TestModel {
			return TestModel{
				ID:   entity.ID,
				Name: entity.Name,
			}
		},
	)

	// Create the repository
	repo := NewGenericRepository(
		sqlBuilder,
		sqlExecutor,
		dataMapper,
		validator,
	)

	// Test cases
	testCases := []struct {
		name          string
		criteria      *Criteria
		mockSetup     func(sqlmock.Sqlmock)
		expectError   bool
		errorContains string
	}{
		{
			name: "Valid criteria",
			criteria: NewCriteria().
				AddFilter("name", OperatorEqual, "test_user").
				AddSort("id", OrderAsc),
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT \\* FROM users WHERE name = \\$1 ORDER BY id ASC").
					WithArgs("test_user").
					WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).
						AddRow(1, "test_user"))

				mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			},
			expectError: false,
		},
		{
			name: "SQL injection in value",
			criteria: NewCriteria().
				AddFilter("name", OperatorEqual, "' OR '1'='1").
				AddSort("id", OrderAsc),
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT \\* FROM users WHERE name = \\$1 ORDER BY id ASC").
					WithArgs("' OR '1'='1").
					WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

				mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			},
			expectError: false,
		},
		{
			name: "SQL injection in field",
			criteria: NewCriteria().
				AddFilter("name; DROP TABLE users;--", OperatorEqual, "test_user"),
			expectError:   true,
			errorContains: "invalid field",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup the mock expectations if needed
			if tc.mockSetup != nil {
				tc.mockSetup(mock)
			}

			// Execute the query
			ctx := context.Background()
			_, _, err := repo.Find(ctx, tc.criteria)

			// Verify the result
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.True(t, strings.Contains(err.Error(), tc.errorContains),
						"Expected error to contain '%s', got '%s'", tc.errorContains, err.Error())
				}
			} else {
				assert.NoError(t, err)
				// Verify all expectations were met
				assert.NoError(t, mock.ExpectationsWereMet())
			}
		})
	}
}

func TestSQLQueryBuilder_JSONFieldHandling(t *testing.T) {
	tests := []struct {
		name           string
		filter         Filter
		expectedClause string
		expectedArgs   []interface{}
	}{
		{
			name: "Simple JSON field access using ->",
			filter: Filter{
				Field:    "data->'name'",
				Operator: OperatorEqual,
				Value:    "John",
			},
			expectedClause: "data->'name' = $1",
			expectedArgs:   []interface{}{"John"},
		},
		{
			name: "JSON text extraction using ->>",
			filter: Filter{
				Field:    "data->>'age'",
				Operator: OperatorGreaterThan,
				Value:    30,
			},
			expectedClause: "data->>'age' > $1",
			expectedArgs:   []interface{}{30},
		},
		{
			name: "Nested JSON field access",
			filter: Filter{
				Field:    "metadata->'address'->>'city'",
				Operator: OperatorEqual,
				Value:    "New York",
			},
			expectedClause: "metadata->'address'->>'city' = $1",
			expectedArgs:   []interface{}{"New York"},
		},
		{
			name: "JSON field with contains operator",
			filter: Filter{
				Field:    "data->>'tags'",
				Operator: OperatorContains,
				Value:    "premium",
			},
			expectedClause: "data->>'tags' LIKE $1",
			expectedArgs:   []interface{}{"%premium%"},
		},
		{
			name: "Quoted JSON key name",
			filter: Filter{
				Field:    "data->'info'->'user data'->>'name'",
				Operator: OperatorEqual,
				Value:    "Alice",
			},
			expectedClause: "data->'info'->'user data'->>'name' = $1",
			expectedArgs:   []interface{}{"Alice"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new builder for each test to reset the parameter index
			builder := NewSQLQueryBuilder("users")
			whereClause, args := builder.buildWhereClause(tt.filter)

			if whereClause != tt.expectedClause {
				t.Errorf("buildWhereClause() clause = %v, want %v", whereClause, tt.expectedClause)
			}

			if !reflect.DeepEqual(args, tt.expectedArgs) {
				t.Errorf("buildWhereClause() args = %v, want %v", args, tt.expectedArgs)
			}
		})
	}
}
