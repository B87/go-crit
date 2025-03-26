package crit

import (
	"errors"
)

// FilterOperator represents the type of filter operation
type FilterOperator string

// Available filter operators
const (
	OperatorEqual              FilterOperator = "eq"
	OperatorNotEqual           FilterOperator = "neq"
	OperatorGreaterThan        FilterOperator = "gt"
	OperatorGreaterThanOrEqual FilterOperator = "gte"
	OperatorLessThan           FilterOperator = "lt"
	OperatorLessThanOrEqual    FilterOperator = "lte"
	OperatorContains           FilterOperator = "contains"
	OperatorIn                 FilterOperator = "in"
	OperatorIsNull             FilterOperator = "isnull"
)

// SortOrder represents the direction of sorting
type SortOrder string

// Available sort orders
const (
	OrderAsc  SortOrder = "asc"
	OrderDesc SortOrder = "desc"
)

// Filter represents a single filter condition
type Filter struct {
	Field    string
	Operator FilterOperator
	Value    interface{}
}

// Sort represents a sort specification
type Sort struct {
	Field string
	Order SortOrder
}

// Pagination represents pagination parameters
type Pagination struct {
	Page  int
	Limit int
}

// Criteria encapsulates filter, sort, and pagination options
type Criteria struct {
	Filters    []Filter
	Sorts      []Sort
	Pagination *Pagination
}

// NewCriteria creates a new Criteria instance
func NewCriteria() *Criteria {
	return &Criteria{
		Filters: []Filter{},
		Sorts:   []Sort{},
	}
}

// ValidationConfig defines validation configuration
type ValidationConfig struct {
	AllowedFields     []string
	AllowedSortFields []string
	MaxLimit          int
	DefaultLimit      int
	DefaultSortField  string
	DefaultSortOrder  SortOrder
}

// Common errors
var (
	ErrInvalidField    = errors.New("invalid field")
	ErrInvalidOperator = errors.New("invalid operator")
	ErrInvalidSortBy   = errors.New("invalid sort field")
	ErrInvalidOrder    = errors.New("invalid sort order")
	ErrInvalidPage     = errors.New("invalid page")
	ErrInvalidLimit    = errors.New("invalid limit")
)

// AddFilter adds a filter to the criteria
func (c *Criteria) AddFilter(field string, operator FilterOperator, value interface{}) *Criteria {
	c.Filters = append(c.Filters, Filter{
		Field:    field,
		Operator: operator,
		Value:    value,
	})
	return c
}

// AddSort adds a sort to the criteria
func (c *Criteria) AddSort(field string, order SortOrder) *Criteria {
	c.Sorts = append(c.Sorts, Sort{
		Field: field,
		Order: order,
	})
	return c
}

// SetPagination sets pagination options
func (c *Criteria) SetPagination(page, limit int) *Criteria {
	c.Pagination = &Pagination{
		Page:  page,
		Limit: limit,
	}
	return c
}
