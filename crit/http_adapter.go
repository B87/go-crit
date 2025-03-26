package crit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// HTTPQuery represents an HTTP API query with URL parameters
type HTTPQuery struct {
	BaseURL     string
	Path        string
	QueryParams url.Values
}

// HTTPQueryBuilder implements the QueryBuilder interface for HTTP APIs
type HTTPQueryBuilder struct {
	baseURL string
	path    string
}

// NewHTTPQueryBuilder creates a new HTTPQueryBuilder
func NewHTTPQueryBuilder(baseURL, path string) *HTTPQueryBuilder {
	return &HTTPQueryBuilder{
		baseURL: baseURL,
		path:    path,
	}
}

// BuildQuery implements the QueryBuilder interface
func (b *HTTPQueryBuilder) BuildQuery(criteria *Criteria) (HTTPQuery, error) {
	params := url.Values{}

	// Add filters
	for _, filter := range criteria.Filters {
		switch filter.Operator {
		case OperatorEqual:
			params.Add(filter.Field, fmt.Sprintf("%v", filter.Value))
		case OperatorGreaterThan:
			params.Add(filter.Field+"_gt", fmt.Sprintf("%v", filter.Value))
		case OperatorGreaterThanOrEqual:
			params.Add(filter.Field+"_gte", fmt.Sprintf("%v", filter.Value))
		case OperatorLessThan:
			params.Add(filter.Field+"_lt", fmt.Sprintf("%v", filter.Value))
		case OperatorLessThanOrEqual:
			params.Add(filter.Field+"_lte", fmt.Sprintf("%v", filter.Value))
		case OperatorContains:
			params.Add(filter.Field+"_contains", fmt.Sprintf("%v", filter.Value))
		case OperatorIn:
			if arr, ok := filter.Value.([]interface{}); ok {
				values := make([]string, len(arr))
				for i, v := range arr {
					values[i] = fmt.Sprintf("%v", v)
				}
				params.Add(filter.Field+"_in", strings.Join(values, ","))
			}
		case OperatorIsNull:
			params.Add(filter.Field+"_isnull", "true")
		}
	}

	// Add sorts
	if len(criteria.Sorts) > 0 {
		sortFields := make([]string, len(criteria.Sorts))
		for i, sort := range criteria.Sorts {
			if sort.Order == OrderDesc {
				sortFields[i] = "-" + sort.Field
			} else {
				sortFields[i] = sort.Field
			}
		}
		params.Add("sort", strings.Join(sortFields, ","))
	}

	// Add pagination
	if criteria.Pagination != nil {
		params.Add("page", strconv.Itoa(criteria.Pagination.Page))
		params.Add("limit", strconv.Itoa(criteria.Pagination.Limit))
	}

	return HTTPQuery{
		BaseURL:     b.baseURL,
		Path:        b.path,
		QueryParams: params,
	}, nil
}

// HTTPResponse represents a generic HTTP response
type HTTPResponse[E any] struct {
	Data []E `json:"data"`
	Meta struct {
		Total int64 `json:"total"`
	} `json:"meta"`
}

// HTTPQueryExecutor implements the QueryExecutor interface for HTTP APIs
// E is the type of entities that will be returned
type HTTPQueryExecutor[E any] struct {
	client *http.Client
}

// NewHTTPQueryExecutor creates a new HTTPQueryExecutor
func NewHTTPQueryExecutor[E any](client *http.Client) *HTTPQueryExecutor[E] {
	if client == nil {
		client = http.DefaultClient
	}
	return &HTTPQueryExecutor[E]{client: client}
}

// Execute implements the QueryExecutor interface
func (e *HTTPQueryExecutor[E]) Execute(ctx context.Context, query HTTPQuery) ([]E, int64, error) {
	// Build full URL
	fullURL := query.BaseURL
	if !strings.HasSuffix(fullURL, "/") && !strings.HasPrefix(query.Path, "/") {
		fullURL += "/"
	}
	fullURL += query.Path
	if len(query.QueryParams) > 0 {
		fullURL += "?" + query.QueryParams.Encode()
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("error executing request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("error reading response body: %w", err)
	}

	// Parse response
	var httpResp HTTPResponse[E]
	if err := json.Unmarshal(body, &httpResp); err != nil {
		return nil, 0, fmt.Errorf("error parsing response body: %w", err)
	}

	return httpResp.Data, httpResp.Meta.Total, nil
}

// HTTPValidator implements the Validator interface for HTTP APIs
type HTTPValidator struct {
	allowedFields []string
	maxLimit      int
}

// NewHTTPValidator creates a new HTTPValidator
func NewHTTPValidator(allowedFields []string, maxLimit int) *HTTPValidator {
	return &HTTPValidator{
		allowedFields: allowedFields,
		maxLimit:      maxLimit,
	}
}

// Validate implements the Validator interface
func (v *HTTPValidator) Validate(criteria *Criteria) error {
	// Validate fields
	for _, filter := range criteria.Filters {
		if !slices.Contains(v.allowedFields, filter.Field) {
			return fmt.Errorf("%w: %s", ErrInvalidField, filter.Field)
		}
	}

	// Validate sorts
	for _, sort := range criteria.Sorts {
		if !slices.Contains(v.allowedFields, sort.Field) {
			return fmt.Errorf("%w: %s", ErrInvalidSortBy, sort.Field)
		}
	}

	// Validate pagination
	if criteria.Pagination != nil {
		if criteria.Pagination.Page < 1 {
			return fmt.Errorf("%w: %s", ErrInvalidPage, strconv.Itoa(criteria.Pagination.Page))
		}
		if criteria.Pagination.Limit < 1 {
			return fmt.Errorf("%w: %s", ErrInvalidLimit, strconv.Itoa(criteria.Pagination.Limit))
		}
		if v.maxLimit > 0 && criteria.Pagination.Limit > v.maxLimit {
			criteria.Pagination.Limit = v.maxLimit
		}
	}

	return nil
}

// GenericHTTPMapper is a generic implementation of DataMapper for HTTP entities
// E is the type of entities from the API response
// M is the type of models for the domain
type GenericHTTPMapper[E any, M any] struct {
	modelConstructor  func() M
	entityConstructor func() E
}

// NewGenericHTTPMapper creates a new GenericHTTPMapper
func NewGenericHTTPMapper[E any, M any](
	modelConstructor func() M,
	entityConstructor func() E,
) *GenericHTTPMapper[E, M] {
	return &GenericHTTPMapper[E, M]{
		modelConstructor:  modelConstructor,
		entityConstructor: entityConstructor,
	}
}

// MapToModel maps API entities to domain models
func (m *GenericHTTPMapper[E, M]) MapToModel(entities []E) []M {
	if entities == nil {
		return nil
	}

	models := make([]M, len(entities))
	for i, entity := range entities {
		models[i] = m.MapSingleToModel(entity)
	}
	return models
}

// MapToEntity maps domain models to API entities
func (m *GenericHTTPMapper[E, M]) MapToEntity(models []M) []E {
	if models == nil {
		return nil
	}

	entities := make([]E, len(models))
	for i, model := range models {
		entities[i] = m.MapSingleToEntity(model)
	}
	return entities
}

// MapSingleToModel maps a single API entity to a domain model
func (m *GenericHTTPMapper[E, M]) MapSingleToModel(entity E) M {
	model := m.modelConstructor()

	// Use reflection to copy matching fields
	srcVal := reflect.ValueOf(entity)
	dstVal := reflect.ValueOf(&model).Elem()

	if srcVal.Kind() == reflect.Struct && dstVal.Kind() == reflect.Struct {
		for i := 0; i < srcVal.NumField(); i++ {
			srcField := srcVal.Type().Field(i)
			if _, found := dstVal.Type().FieldByName(srcField.Name); found {
				srcValue := srcVal.Field(i)
				dstValue := dstVal.FieldByName(srcField.Name)

				if dstValue.IsValid() && dstValue.CanSet() && srcValue.Type().AssignableTo(dstValue.Type()) {
					dstValue.Set(srcValue)
				}
			}
		}
	}

	return model
}

// MapSingleToEntity maps a single domain model to an API entity
func (m *GenericHTTPMapper[E, M]) MapSingleToEntity(model M) E {
	entity := m.entityConstructor()

	// Use reflection to copy matching fields
	srcVal := reflect.ValueOf(model)
	dstVal := reflect.ValueOf(&entity).Elem()

	if srcVal.Kind() == reflect.Struct && dstVal.Kind() == reflect.Struct {
		for i := 0; i < srcVal.NumField(); i++ {
			srcField := srcVal.Type().Field(i)
			if _, found := dstVal.Type().FieldByName(srcField.Name); found {
				srcValue := srcVal.Field(i)
				dstValue := dstVal.FieldByName(srcField.Name)

				if dstValue.IsValid() && dstValue.CanSet() && srcValue.Type().AssignableTo(dstValue.Type()) {
					dstValue.Set(srcValue)
				}
			}
		}
	}

	return entity
}
