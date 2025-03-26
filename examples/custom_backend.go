package examples

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/b87/go-crit/crit"
)

// Example product domain models
type Product struct {
	ID          string
	Name        string
	Description string
	Price       float64
	CategoryID  string
	StockCount  int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Example SQL database entity
type ProductEntity struct {
	ID          string
	Name        string
	Description string
	Price       float64
	CategoryID  string
	StockCount  int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Example HTTP API entity
type ProductAPIEntity struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	CategoryID  string  `json:"category_id"`
	StockCount  int     `json:"stock_count"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func ExampleTypeSafeSQL() {
	// Create a new SQL query builder
	sqlBuilder := crit.NewSQLQueryBuilder("products")

	// Create a DB connection (this would typically come from your app's DB pool)
	db, _ := sql.Open("postgres", "postgres://user:password@localhost/dbname")
	dbx := sqlx.NewDb(db, "postgres")

	// Create a SQL executor with the specific entity type (ProductEntity)
	sqlExecutor := crit.NewSQLQueryExecutor[ProductEntity](dbx)

	// Create a validator with allowed fields
	validator := crit.NewSQLValidator(crit.ValidationConfig{
		AllowedFields:     []string{"name", "price", "category_id", "stock_count"},
		AllowedSortFields: []string{"name", "price", "created_at"},
		MaxLimit:          100,
		DefaultLimit:      20,
		DefaultSortField:  "created_at",
		DefaultSortOrder:  crit.OrderDesc,
	})

	// Create a data mapper between ProductEntity and Product
	dataMapper := crit.NewEntityMapper(
		// Model to Entity mapping function
		func(model Product) ProductEntity {
			return ProductEntity{
				ID:          model.ID,
				Name:        model.Name,
				Description: model.Description,
				Price:       model.Price,
				CategoryID:  model.CategoryID,
				StockCount:  model.StockCount,
				CreatedAt:   model.CreatedAt,
				UpdatedAt:   model.UpdatedAt,
			}
		},
		// Entity to Model mapping function
		func(entity ProductEntity) Product {
			return Product{
				ID:          entity.ID,
				Name:        entity.Name,
				Description: entity.Description,
				Price:       entity.Price,
				CategoryID:  entity.CategoryID,
				StockCount:  entity.StockCount,
				CreatedAt:   entity.CreatedAt,
				UpdatedAt:   entity.UpdatedAt,
			}
		},
	)

	// Create a type-safe repository
	// The repository is now parameterized with concrete types:
	// - ProductEntity for the database entity
	// - Product for the domain model
	// - SQLQuery for the query type
	repo := crit.NewGenericRepository(
		sqlBuilder,
		sqlExecutor,
		dataMapper,
		validator,
	)

	// Use the repository with type safety
	criteria := crit.NewCriteria().
		AddFilter("category_id", crit.OperatorEqual, "electronics").
		AddFilter("price", crit.OperatorGreaterThan, 100.0).
		AddSort("price", crit.OrderAsc).
		SetPagination(1, 10)

	// Find products with the criteria
	ctx := context.Background()
	products, total, err := repo.Find(ctx, criteria)
	if err != nil {
		// Handle error
		return
	}

	// Now 'products' is a []Product, not []interface{}
	fmt.Printf("Found %d products (total: %d)\n", len(products), total)

	// Use the first product with type safety (no type assertion needed)
	if len(products) > 0 {
		product := products[0]
		fmt.Printf("First product: %s - $%.2f\n", product.Name, product.Price)
	}
}

// CustomBackend example showing how to implement a custom backend
type InMemoryData struct {
	products []Product
}

// InMemoryQuery represents a query for the in-memory backend
type InMemoryQuery struct {
	Filters    []crit.Filter
	Sorts      []crit.Sort
	Pagination *crit.Pagination
}

// InMemoryQueryBuilder builds queries for the in-memory backend
type InMemoryQueryBuilder struct{}

// BuildQuery implements the QueryBuilder interface
func (b *InMemoryQueryBuilder) BuildQuery(criteria *crit.Criteria) (InMemoryQuery, error) {
	return InMemoryQuery{
		Filters:    criteria.Filters,
		Sorts:      criteria.Sorts,
		Pagination: criteria.Pagination,
	}, nil
}

// InMemoryQueryExecutor executes queries against the in-memory backend
type InMemoryQueryExecutor struct {
	data *InMemoryData
}

// Execute implements the QueryExecutor interface
func (e *InMemoryQueryExecutor) Execute(ctx context.Context, query InMemoryQuery) ([]Product, int64, error) {
	// Get total count of all products (before filtering)
	total := int64(len(e.data.products))

	// Filter products
	filtered := e.filterProducts(e.data.products, query.Filters)

	// Sort products
	sorted := e.sortProducts(filtered, query.Sorts)

	// Apply pagination
	paged := e.pageProducts(sorted, query.Pagination)

	return paged, total, nil
}

func (e *InMemoryQueryExecutor) filterProducts(products []Product, filters []crit.Filter) []Product {
	if len(filters) == 0 {
		return products
	}

	result := make([]Product, 0)
	for _, product := range products {
		match := true
		for _, filter := range filters {
			if !e.matchesFilter(product, filter) {
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

func (e *InMemoryQueryExecutor) matchesFilter(product Product, filter crit.Filter) bool {
	// Simplified filter matching logic
	switch filter.Field {
	case "name":
		if filter.Operator == crit.OperatorIn {
			values, ok := filter.Value.([]interface{})
			if !ok {
				return false
			}
			for _, v := range values {
				if str, ok := v.(string); ok && str == product.Name {
					return true
				}
			}
			return false
		}

		value, ok := filter.Value.(string)
		if !ok {
			return false
		}
		switch filter.Operator {
		case crit.OperatorEqual:
			return product.Name == value
		case crit.OperatorNotEqual:
			return product.Name != value
		case crit.OperatorContains:
			return strings.Contains(product.Name, value)
		case crit.OperatorIsNull:
			if boolVal, ok := filter.Value.(bool); ok {
				return (product.Name == "") == boolVal
			}
			return false
		}
	case "price":
		if filter.Operator == crit.OperatorIsNull {
			if boolVal, ok := filter.Value.(bool); ok {
				// For primitive types like float64, we can't truly check for null
				// so we'll consider 0 as "null" for this example
				return (product.Price == 0) == boolVal
			}
			return false
		}

		if filter.Operator == crit.OperatorIn {
			values, ok := filter.Value.([]interface{})
			if !ok {
				return false
			}
			for _, v := range values {
				// Try to handle different numeric types
				switch num := v.(type) {
				case float64:
					if product.Price == num {
						return true
					}
				case int:
					if product.Price == float64(num) {
						return true
					}
				}
			}
			return false
		}

		value, ok := filter.Value.(float64)
		if !ok {
			return false
		}
		switch filter.Operator {
		case crit.OperatorEqual:
			return product.Price == value
		case crit.OperatorNotEqual:
			return product.Price != value
		case crit.OperatorGreaterThan:
			return product.Price > value
		case crit.OperatorGreaterThanOrEqual:
			return product.Price >= value
		case crit.OperatorLessThan:
			return product.Price < value
		case crit.OperatorLessThanOrEqual:
			return product.Price <= value
		}
	case "category_id":
		if filter.Operator == crit.OperatorIsNull {
			if boolVal, ok := filter.Value.(bool); ok {
				return (product.CategoryID == "") == boolVal
			}
			return false
		} else if filter.Operator == crit.OperatorIn {
			values, ok := filter.Value.([]interface{})
			if !ok {
				return false
			}
			for _, v := range values {
				if str, ok := v.(string); ok && str == product.CategoryID {
					return true
				}
			}
			return false
		}

		value, ok := filter.Value.(string)
		if !ok {
			return false
		}
		switch filter.Operator {
		case crit.OperatorEqual:
			return product.CategoryID == value
		case crit.OperatorNotEqual:
			return product.CategoryID != value
		case crit.OperatorContains:
			return strings.Contains(product.CategoryID, value)
		}
	case "stock_count":
		if filter.Operator == crit.OperatorIsNull {
			if boolVal, ok := filter.Value.(bool); ok {
				// For primitive types like int, we can't truly check for null
				// so we'll consider 0 as "null" for this example
				return (product.StockCount == 0) == boolVal
			}
			return false
		}

		if filter.Operator == crit.OperatorIn {
			values, ok := filter.Value.([]interface{})
			if !ok {
				return false
			}
			for _, v := range values {
				// Try to handle different numeric types
				switch num := v.(type) {
				case int:
					if product.StockCount == num {
						return true
					}
				case float64:
					if product.StockCount == int(num) {
						return true
					}
				}
			}
			return false
		}

		value, ok := filter.Value.(int)
		if !ok {
			// Try to handle float64 values that might be passed
			if floatVal, ok := filter.Value.(float64); ok {
				value = int(floatVal)
			} else {
				return false
			}
		}
		switch filter.Operator {
		case crit.OperatorEqual:
			return product.StockCount == value
		case crit.OperatorNotEqual:
			return product.StockCount != value
		case crit.OperatorGreaterThan:
			return product.StockCount > value
		case crit.OperatorGreaterThanOrEqual:
			return product.StockCount >= value
		case crit.OperatorLessThan:
			return product.StockCount < value
		case crit.OperatorLessThanOrEqual:
			return product.StockCount <= value
		}
	}
	return false
}

func (e *InMemoryQueryExecutor) sortProducts(products []Product, sorts []crit.Sort) []Product {
	if len(sorts) == 0 || len(products) <= 1 {
		return products
	}

	// Create a copy to avoid modifying the original slice
	result := make([]Product, len(products))
	copy(result, products)

	// Sort by each sort field in order
	sort.SliceStable(result, func(i, j int) bool {
		for _, s := range sorts {
			var compare int

			// Compare based on the field
			switch s.Field {
			case "name":
				compare = strings.Compare(result[i].Name, result[j].Name)
			case "price":
				if result[i].Price < result[j].Price {
					compare = -1
				} else if result[i].Price > result[j].Price {
					compare = 1
				}
			case "category_id":
				compare = strings.Compare(result[i].CategoryID, result[j].CategoryID)
			case "stock_count":
				if result[i].StockCount < result[j].StockCount {
					compare = -1
				} else if result[i].StockCount > result[j].StockCount {
					compare = 1
				}
			case "id":
				compare = strings.Compare(result[i].ID, result[j].ID)
			}

			// If we have a definitive comparison (not equal), return based on sort order
			if compare != 0 {
				// For ascending order, return true if i < j
				// For descending order, return true if i > j
				return (s.Order == crit.OrderAsc && compare < 0) || (s.Order == crit.OrderDesc && compare > 0)
			}
			// If equal, continue to the next sort field
		}
		// If all sort fields are equal, maintain original order
		return i < j
	})

	return result
}

func (e *InMemoryQueryExecutor) pageProducts(products []Product, pagination *crit.Pagination) []Product {
	if pagination == nil {
		return products
	}

	start := (pagination.Page - 1) * pagination.Limit
	end := start + pagination.Limit

	if start >= len(products) {
		return []Product{}
	}

	if end > len(products) {
		end = len(products)
	}

	return products[start:end]
}

// InMemoryValidator validates criteria for the in-memory backend
type InMemoryValidator struct {
	allowedFields []string
	maxLimit      int
}

// Validate implements the Validator interface
func (v *InMemoryValidator) Validate(criteria *crit.Criteria) error {
	// Validate fields
	for _, filter := range criteria.Filters {
		if !slices.Contains(v.allowedFields, filter.Field) {
			return fmt.Errorf("%w: %s", crit.ErrInvalidField, filter.Field)
		}
	}

	// Validate sorts
	for _, sort := range criteria.Sorts {
		if !slices.Contains(v.allowedFields, sort.Field) {
			return fmt.Errorf("%w: %s", crit.ErrInvalidSortBy, sort.Field)
		}
	}

	// Validate pagination
	if criteria.Pagination != nil {
		if criteria.Pagination.Page < 1 {
			return crit.ErrInvalidPage
		}
		if criteria.Pagination.Limit < 1 {
			return crit.ErrInvalidLimit
		}
		if v.maxLimit > 0 && criteria.Pagination.Limit > v.maxLimit {
			criteria.Pagination.Limit = v.maxLimit
		}
	}

	return nil
}

// InMemoryProductMapper maps between domain models and in-memory products (identity mapper)
type InMemoryProductMapper struct{}

// MapToModel implements the DataMapper interface (identity mapping)
func (m *InMemoryProductMapper) MapToModel(entities []Product) []Product {
	return entities
}

// MapToEntity implements the DataMapper interface (identity mapping)
func (m *InMemoryProductMapper) MapToEntity(models []Product) []Product {
	return models
}

// MapSingleToModel implements the DataMapper interface (identity mapping)
func (m *InMemoryProductMapper) MapSingleToModel(entity Product) Product {
	return entity
}

// MapSingleToEntity implements the DataMapper interface (identity mapping)
func (m *InMemoryProductMapper) MapSingleToEntity(model Product) Product {
	return model
}

func ExampleTypeSafeCustomBackend() {
	// Create in-memory data store
	data := &InMemoryData{
		products: []Product{
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
		},
	}

	// Create components for in-memory backend
	queryBuilder := &InMemoryQueryBuilder{}
	queryExecutor := &InMemoryQueryExecutor{data: data}
	validator := &InMemoryValidator{
		allowedFields: []string{"name", "price", "category_id", "stock_count"},
		maxLimit:      100,
	}
	mapper := &InMemoryProductMapper{}

	// Create a type-safe repository
	// Note: The entity and model types are the same here (Product)
	repo := crit.NewGenericRepository(
		queryBuilder,
		queryExecutor,
		mapper,
		validator,
	)

	// Use the repository with type safety
	criteria := crit.NewCriteria().
		AddFilter("price", crit.OperatorGreaterThan, 1000.0).
		SetPagination(1, 10)

	// Find products with the criteria
	ctx := context.Background()
	products, total, err := repo.Find(ctx, criteria)
	if err != nil {
		// Handle error
		return
	}

	// Now 'products' is a []Product, not []interface{}
	fmt.Printf("Found %d products (total: %d)\n", len(products), total)

	// Use the first product with type safety (no type assertion needed)
	if len(products) > 0 {
		product := products[0]
		fmt.Printf("First product: %s - $%.2f\n", product.Name, product.Price)
	}
}
