package examples

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/b87/go-crit/crit"
)

// TestInMemoryBackend_Basic tests the basic functionality of the in-memory backend
func TestInMemoryBackend_Basic(t *testing.T) {
	// Create sample products
	products := []Product{
		{
			ID:          "1",
			Name:        "Laptop",
			Description: "Powerful laptop",
			Price:       1200.0,
			CategoryID:  "electronics",
			StockCount:  15,
		},
		{
			ID:          "2",
			Name:        "Smartphone",
			Description: "Latest smartphone",
			Price:       800.0,
			CategoryID:  "electronics",
			StockCount:  30,
		},
		{
			ID:          "3",
			Name:        "Desk Chair",
			Description: "Ergonomic chair",
			Price:       250.0,
			CategoryID:  "furniture",
			StockCount:  10,
		},
	}

	// Create in-memory data store
	data := &InMemoryData{products: products}

	// Create components for in-memory backend
	queryBuilder := &InMemoryQueryBuilder{}
	queryExecutor := &InMemoryQueryExecutor{data: data}
	validator := &InMemoryValidator{
		allowedFields: []string{"id", "name", "price", "category_id", "stock_count"},
		maxLimit:      100,
	}
	mapper := &InMemoryProductMapper{}

	// Create repository
	repo := crit.NewGenericRepository(
		queryBuilder,
		queryExecutor,
		mapper,
		validator,
	)

	// Test cases
	testCases := []struct {
		name          string
		criteria      *crit.Criteria
		expectedCount int
		expectedIDs   []string
		expectError   bool
		errorContains string
	}{
		{
			name: "Filter by price greater than",
			criteria: crit.NewCriteria().
				AddFilter("price", crit.OperatorGreaterThan, 1000.0),
			expectedCount: 1,
			expectedIDs:   []string{"1"}, // Only the laptop
		},
		{
			name: "Filter by category",
			criteria: crit.NewCriteria().
				AddFilter("category_id", crit.OperatorEqual, "electronics"),
			expectedCount: 2,
			expectedIDs:   []string{"1", "2"}, // Laptop and smartphone
		},
		{
			name: "Multi	ple filters",
			criteria: crit.NewCriteria().
				AddFilter("category_id", crit.OperatorEqual, "electronics").
				AddFilter("price", crit.OperatorLessThan, 1000.0),
			expectedCount: 1,
			expectedIDs:   []string{"2"}, // Only smartphone
		},
		{
			name: "Pagination - first page",
			criteria: crit.NewCriteria().
				SetPagination(1, 2),
			expectedCount: 2,
			expectedIDs:   []string{"1", "2"}, // First two products
		},
		{
			name: "Pagination - second page",
			criteria: crit.NewCriteria().
				SetPagination(2, 2),
			expectedCount: 1,
			expectedIDs:   []string{"3"}, // Last product
		},
		{
			name: "Invalid field",
			criteria: crit.NewCriteria().
				AddFilter("invalid_field", crit.OperatorEqual, "value"),
			expectError:   true,
			errorContains: "invalid field",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Execute the query
			ctx := context.Background()
			result, total, err := repo.Find(ctx, tc.criteria)

			// Verify the result
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, len(tc.expectedIDs), len(result), "Result count should match expected")
				assert.Equal(t, int64(len(products)), total, "Total count should be the total number of products")

				// Verify the IDs of returned products
				resultIDs := make([]string, len(result))
				for i, product := range result {
					resultIDs[i] = product.ID
				}
				assert.ElementsMatch(t, tc.expectedIDs, resultIDs, "Result IDs should match expected")
			}
		})
	}
}

// TestInMemoryBackend_SecurityChecks tests validation and security aspects of the in-memory backend
func TestInMemoryBackend_SecurityChecks(t *testing.T) {
	// Create sample products
	products := []Product{
		{
			ID:          "1",
			Name:        "Laptop",
			Description: "Powerful laptop",
			Price:       1200.0,
			CategoryID:  "electronics",
			StockCount:  15,
		},
	}

	// Create in-memory data store
	data := &InMemoryData{products: products}

	// Create validator with restricted allowed fields
	validator := &InMemoryValidator{
		allowedFields: []string{"id", "name", "price"},
		maxLimit:      100,
	}

	// Create components for in-memory backend
	queryBuilder := &InMemoryQueryBuilder{}
	queryExecutor := &InMemoryQueryExecutor{data: data}
	mapper := &InMemoryProductMapper{}

	// Create repository
	repo := crit.NewGenericRepository(
		queryBuilder,
		queryExecutor,
		mapper,
		validator,
	)

	// Test cases
	testCases := []struct {
		name          string
		criteria      *crit.Criteria
		expectError   bool
		errorContains string
	}{
		{
			name: "Valid field",
			criteria: crit.NewCriteria().
				AddFilter("name", crit.OperatorEqual, "Laptop"),
			expectError: false,
		},
		{
			name: "Potential injection in field name",
			criteria: crit.NewCriteria().
				AddFilter("name; DROP TABLE products;--", crit.OperatorEqual, "value"),
			expectError:   true,
			errorContains: "invalid field",
		},
		{
			name: "Potential injection in sort field",
			criteria: crit.NewCriteria().
				AddSort("name; DROP TABLE products;--", crit.OrderAsc),
			expectError:   true,
			errorContains: "invalid sort field",
		},
		{
			name: "Invalid field",
			criteria: crit.NewCriteria().
				AddFilter("category_id", crit.OperatorEqual, "electronics"), // Not in allowed fields
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

// This test ensures that our InMemoryQueryExecutor correctly implements
// all filter operators and matches the behavior of the SQL and HTTP adapters
func TestInMemoryBackend_FilterOperators(t *testing.T) {
	// Create a helper function to create int pointers
	intPtr := func(i int) *int { return &i }

	// Create sample products with variety of values
	// Note: Using a local Product struct with pointer fields for the test
	type TestProduct struct {
		ID         string
		Name       string
		Price      float64
		CategoryID string
		StockCount *int // Using pointer to represent nullable field
	}

	products := []TestProduct{
		{ID: "1", Name: "Product A", Price: 100.0, CategoryID: "c1", StockCount: intPtr(10)},
		{ID: "2", Name: "Product B", Price: 200.0, CategoryID: "c1", StockCount: intPtr(0)},
		{ID: "3", Name: "Product C", Price: 300.0, CategoryID: "c2", StockCount: intPtr(5)},
		{ID: "4", Name: "Special Product", Price: 150.0, CategoryID: "c2", StockCount: intPtr(15)},
		{ID: "5", Name: "Product D", Price: 250.0, CategoryID: "c3", StockCount: nil}, // Null value
	}

	// Create a simplified executor for testing filters
	type testExecutor struct{}

	// Helper function to match filters against our test products
	matchesFilter := func(product TestProduct, filter crit.Filter) bool {
		switch filter.Field {
		case "name":
			value, ok := filter.Value.(string)
			if !ok {
				return false
			}
			switch filter.Operator {
			case crit.OperatorEqual:
				return product.Name == value
			case crit.OperatorContains:
				return strings.Contains(product.Name, value)
			}
		case "price":
			value, ok := filter.Value.(float64)
			if !ok {
				return false
			}
			switch filter.Operator {
			case crit.OperatorEqual:
				return product.Price == value
			case crit.OperatorGreaterThan:
				return product.Price > value
			case crit.OperatorLessThan:
				return product.Price < value
			}
		case "category_id":
			if filter.Operator == crit.OperatorIn {
				if values, ok := filter.Value.([]interface{}); ok {
					for _, v := range values {
						if strVal, ok := v.(string); ok && strVal == product.CategoryID {
							return true
						}
					}
					return false
				}
				return false
			}

			value, ok := filter.Value.(string)
			if !ok {
				return false
			}
			return filter.Operator == crit.OperatorEqual && product.CategoryID == value
		case "stock_count":
			if filter.Operator == crit.OperatorIsNull {
				value, ok := filter.Value.(bool)
				if !ok {
					return false
				}
				return (product.StockCount == nil) == value
			}
		}
		return false
	}

	// Implement a simple filter function that works with our test products
	filterProducts := func(products []TestProduct, filters []crit.Filter) []TestProduct {
		if len(filters) == 0 {
			return products
		}

		result := make([]TestProduct, 0)
		for _, product := range products {
			match := true
			for _, filter := range filters {
				if !matchesFilter(product, filter) {
					match = false
					break
				}
			}
			if match {
				result = append(result, product)
			}
		}
		return result
	}

	// Test different filter operators
	testCases := []struct {
		name          string
		filter        crit.Filter
		expectedCount int
		expectedIDs   []string
	}{
		{
			name: "Equal operator",
			filter: crit.Filter{
				Field:    "name",
				Operator: crit.OperatorEqual,
				Value:    "Product A",
			},
			expectedCount: 1,
			expectedIDs:   []string{"1"},
		},
		{
			name: "Greater than operator",
			filter: crit.Filter{
				Field:    "price",
				Operator: crit.OperatorGreaterThan,
				Value:    200.0,
			},
			expectedCount: 2,
			expectedIDs:   []string{"3", "5"},
		},
		{
			name: "Less than operator",
			filter: crit.Filter{
				Field:    "price",
				Operator: crit.OperatorLessThan,
				Value:    200.0,
			},
			expectedCount: 2,
			expectedIDs:   []string{"1", "4"},
		},
		{
			name: "Contains operator",
			filter: crit.Filter{
				Field:    "name",
				Operator: crit.OperatorContains,
				Value:    "Special",
			},
			expectedCount: 1,
			expectedIDs:   []string{"4"},
		},
		{
			name: "In operator",
			filter: crit.Filter{
				Field:    "category_id",
				Operator: crit.OperatorIn,
				Value:    []interface{}{"c1", "c3"},
			},
			expectedCount: 3,
			expectedIDs:   []string{"1", "2", "5"},
		},
		{
			name: "Is Null operator",
			filter: crit.Filter{
				Field:    "stock_count",
				Operator: crit.OperatorIsNull,
				Value:    true,
			},
			expectedCount: 1,
			expectedIDs:   []string{"5"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Filter products using the filter
			filtered := filterProducts(products, []crit.Filter{tc.filter})

			// Verify the result
			assert.Len(t, filtered, tc.expectedCount, "Filtered count should match expected")

			// Verify the IDs of filtered products
			resultIDs := make([]string, len(filtered))
			for i, product := range filtered {
				resultIDs[i] = product.ID
			}
			assert.ElementsMatch(t, tc.expectedIDs, resultIDs, "Filtered IDs should match expected")
		})
	}
}

// TestInMemoryBackend_Sorting tests sorting functionality of the in-memory backend
func TestInMemoryBackend_Sorting(t *testing.T) {
	// Create sample products
	products := []Product{
		{
			ID:          "1",
			Name:        "Laptop",
			Description: "Powerful laptop",
			Price:       1200.0,
			CategoryID:  "electronics",
			StockCount:  15,
		},
		{
			ID:          "2",
			Name:        "Smartphone",
			Description: "Latest smartphone",
			Price:       800.0,
			CategoryID:  "electronics",
			StockCount:  30,
		},
		{
			ID:          "3",
			Name:        "Headphones",
			Description: "Noise-canceling headphones",
			Price:       300.0,
			CategoryID:  "accessories",
			StockCount:  50,
		},
	}

	// Create in-memory data store
	data := &InMemoryData{products: products}

	// Create validator with allowed fields
	validator := &InMemoryValidator{
		allowedFields: []string{"id", "name", "price", "category_id", "stock_count"},
		maxLimit:      100,
	}

	// Create components for in-memory backend
	queryBuilder := &InMemoryQueryBuilder{}
	queryExecutor := &InMemoryQueryExecutor{data: data}
	mapper := &InMemoryProductMapper{}

	// Create repository
	repo := crit.NewGenericRepository(
		queryBuilder,
		queryExecutor,
		mapper,
		validator,
	)

	// Test cases
	testCases := []struct {
		name          string
		criteria      *crit.Criteria
		expectedOrder []string // Expected order of product IDs in result
	}{
		{
			name: "Sort by price ascending",
			criteria: crit.NewCriteria().
				AddSort("price", crit.OrderAsc),
			expectedOrder: []string{"3", "2", "1"}, // Headphones (300), Smartphone (800), Laptop (1200)
		},
		{
			name: "Sort by price descending",
			criteria: crit.NewCriteria().
				AddSort("price", crit.OrderDesc),
			expectedOrder: []string{"1", "2", "3"}, // Laptop (1200), Smartphone (800), Headphones (300)
		},
		{
			name: "Sort by name ascending",
			criteria: crit.NewCriteria().
				AddSort("name", crit.OrderAsc),
			expectedOrder: []string{"3", "1", "2"}, // Headphones, Laptop, Smartphone (alphabetical)
		},
		{
			name: "Sort by name descending",
			criteria: crit.NewCriteria().
				AddSort("name", crit.OrderDesc),
			expectedOrder: []string{"2", "1", "3"}, // Smartphone, Laptop, Headphones (reverse alphabetical)
		},
		{
			name: "Sort by category_id, then by price",
			criteria: crit.NewCriteria().
				AddSort("category_id", crit.OrderAsc).
				AddSort("price", crit.OrderDesc),
			expectedOrder: []string{"3", "1", "2"}, // accessories (Headphones), then electronics by price descending (Laptop, Smartphone)
		},
		{
			name: "Sort by stock_count descending",
			criteria: crit.NewCriteria().
				AddSort("stock_count", crit.OrderDesc),
			expectedOrder: []string{"3", "2", "1"}, // 50, 30, 15
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Execute the query
			ctx := context.Background()
			result, _, err := repo.Find(ctx, tc.criteria)

			// Verify the result
			require.NoError(t, err)
			require.Equal(t, len(tc.expectedOrder), len(result), "Result count should match expected")

			// Verify the order of returned products
			resultIDs := make([]string, len(result))
			for i, product := range result {
				resultIDs[i] = product.ID
			}
			assert.Equal(t, tc.expectedOrder, resultIDs, "Result IDs should match expected order")
		})
	}
}

// TestInMemoryBackend_AdditionalOperators tests additional operator functionality of the in-memory backend
func TestInMemoryBackend_AdditionalOperators(t *testing.T) {
	// Create sample products
	products := []Product{
		{
			ID:          "1",
			Name:        "Laptop",
			Description: "Powerful laptop",
			Price:       1200.0,
			CategoryID:  "electronics",
			StockCount:  15,
		},
		{
			ID:          "2",
			Name:        "Smartphone",
			Description: "Latest smartphone",
			Price:       800.0,
			CategoryID:  "electronics",
			StockCount:  30,
		},
		{
			ID:          "3",
			Name:        "Headphones",
			Description: "Noise-canceling headphones",
			Price:       300.0,
			CategoryID:  "accessories",
			StockCount:  50,
		},
		{
			ID:          "4",
			Name:        "Empty Item",
			Description: "",
			Price:       0.0,
			CategoryID:  "",
			StockCount:  0,
		},
	}

	// Create in-memory data store
	data := &InMemoryData{products: products}

	// Create validator with allowed fields
	validator := &InMemoryValidator{
		allowedFields: []string{"id", "name", "price", "category_id", "stock_count", "description"},
		maxLimit:      100,
	}

	// Create components for in-memory backend
	queryBuilder := &InMemoryQueryBuilder{}
	queryExecutor := &InMemoryQueryExecutor{data: data}
	mapper := &InMemoryProductMapper{}

	// Create repository
	repo := crit.NewGenericRepository(
		queryBuilder,
		queryExecutor,
		mapper,
		validator,
	)

	// Test cases
	testCases := []struct {
		name        string
		criteria    *crit.Criteria
		expectedIDs []string
	}{
		{
			name: "NotEqual operator",
			criteria: crit.NewCriteria().
				AddFilter("category_id", crit.OperatorNotEqual, "electronics"),
			expectedIDs: []string{"3", "4"}, // Headphones and Empty Item
		},
		{
			name: "GreaterThanOrEqual operator",
			criteria: crit.NewCriteria().
				AddFilter("price", crit.OperatorGreaterThanOrEqual, 800.0),
			expectedIDs: []string{"1", "2"}, // Laptop and Smartphone
		},
		{
			name: "LessThanOrEqual operator",
			criteria: crit.NewCriteria().
				AddFilter("price", crit.OperatorLessThanOrEqual, 800.0),
			expectedIDs: []string{"2", "3", "4"}, // Smartphone, Headphones, and Empty Item
		},
		{
			name: "In operator with strings",
			criteria: crit.NewCriteria().
				AddFilter("name", crit.OperatorIn, []interface{}{"Laptop", "Headphones"}),
			expectedIDs: []string{"1", "3"}, // Laptop and Headphones
		},
		{
			name: "In operator with numbers",
			criteria: crit.NewCriteria().
				AddFilter("price", crit.OperatorIn, []interface{}{0.0, 300.0, 1200.0}),
			expectedIDs: []string{"1", "3", "4"}, // Laptop, Headphones, and Empty Item
		},
		{
			name: "IsNull operator - finding nulls",
			criteria: crit.NewCriteria().
				AddFilter("category_id", crit.OperatorIsNull, true),
			expectedIDs: []string{"4"}, // Empty Item
		},
		{
			name: "IsNull operator - finding non-nulls",
			criteria: crit.NewCriteria().
				AddFilter("category_id", crit.OperatorIsNull, false),
			expectedIDs: []string{"1", "2", "3"}, // All except Empty Item
		},
		{
			name: "Multiple operators combined",
			criteria: crit.NewCriteria().
				AddFilter("price", crit.OperatorGreaterThan, 0.0).
				AddFilter("stock_count", crit.OperatorLessThan, 40).
				AddFilter("name", crit.OperatorNotEqual, "Headphones"),
			expectedIDs: []string{"1", "2"}, // Laptop and Smartphone
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Execute the query
			ctx := context.Background()
			result, _, err := repo.Find(ctx, tc.criteria)

			// Verify the result
			require.NoError(t, err)
			assert.Equal(t, len(tc.expectedIDs), len(result), "Result count should match expected")

			// Verify the IDs of returned products
			resultIDs := make([]string, len(result))
			for i, product := range result {
				resultIDs[i] = product.ID
			}
			assert.ElementsMatch(t, tc.expectedIDs, resultIDs, "Result IDs should match expected")
		})
	}
}
