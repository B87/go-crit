package crit

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/jmoiron/sqlx"
)

// SQLQuery represents a SQL query with parameters
type SQLQuery struct {
	Query  string
	Args   []interface{}
	Tables []string
}

// SQLQueryBuilder implements the QueryBuilder interface for SQL databases
type SQLQueryBuilder struct {
	baseTable      string
	parameterIndex int
	joinTables     map[string]string
}

// NewSQLQueryBuilder creates a new SQLQueryBuilder
func NewSQLQueryBuilder(baseTable string) *SQLQueryBuilder {
	return &SQLQueryBuilder{
		baseTable:  baseTable,
		joinTables: make(map[string]string),
	}
}

// AddJoin adds a join clause to the query builder
func (b *SQLQueryBuilder) AddJoin(table, joinCondition string) *SQLQueryBuilder {
	b.joinTables[table] = joinCondition
	return b
}

// BuildQuery implements the QueryBuilder interface
func (b *SQLQueryBuilder) BuildQuery(criteria *Criteria) (SQLQuery, error) {
	b.parameterIndex = 0
	query := fmt.Sprintf("SELECT * FROM %s", b.sanitizeIdentifier(b.baseTable))

	// Add joins
	if len(b.joinTables) > 0 {
		for table, condition := range b.joinTables {
			query += fmt.Sprintf(" JOIN %s ON %s", b.sanitizeIdentifier(table), condition)
		}
	}

	var whereClauses []string
	var args []interface{}

	// Add filters
	if len(criteria.Filters) > 0 {
		for _, filter := range criteria.Filters {
			whereClause, filterArgs := b.buildWhereClause(filter)
			whereClauses = append(whereClauses, whereClause)
			args = append(args, filterArgs...)
		}
	}

	// Add WHERE clause if filters exist
	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Add ORDER BY clause if sorts exist
	if len(criteria.Sorts) > 0 {
		var sortClauses []string
		for _, sort := range criteria.Sorts {
			order := "ASC"
			if sort.Order == OrderDesc {
				order = "DESC"
			}
			sortClauses = append(sortClauses, fmt.Sprintf("%s %s", b.sanitizeIdentifier(sort.Field), order))
		}
		query += " ORDER BY " + strings.Join(sortClauses, ", ")
	}

	// Add LIMIT and OFFSET for pagination
	if criteria.Pagination != nil {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d",
			b.nextParamIndex(), b.nextParamIndex())
		args = append(args, criteria.Pagination.Limit, (criteria.Pagination.Page-1)*criteria.Pagination.Limit)
	}

	return SQLQuery{
		Query:  query,
		Args:   args,
		Tables: b.getTables(),
	}, nil
}

func (b *SQLQueryBuilder) getTables() []string {
	tables := []string{b.baseTable}
	for table := range b.joinTables {
		tables = append(tables, table)
	}
	return tables
}

// nextParamIndex returns the next parameter index
func (b *SQLQueryBuilder) nextParamIndex() int {
	b.parameterIndex++
	return b.parameterIndex
}

// isJSONField checks if a field uses JSON path notation (data->'$.field' or data->>'field')
func (b *SQLQueryBuilder) isJSONField(field string) bool {
	return strings.Contains(field, "->") || strings.Contains(field, "->>")
}

// handleJSONField formats a field with proper JSON operators for PostgreSQL
// It preserves the JSON path syntax while ensuring it's properly sanitized
func (b *SQLQueryBuilder) handleJSONField(field string) string {
	// Split the field into parts (e.g., "data->name" becomes ["data", "name"])
	parts := strings.SplitN(field, "->", 2)
	if len(parts) != 2 {
		return field // Not a valid JSON path
	}

	baseField := b.sanitizeIdentifier(parts[0])

	// Check if we're using the ->> operator (return as text)
	if strings.HasPrefix(parts[1], ">") {
		jsonPath := strings.TrimPrefix(parts[1], ">")
		// If the path starts with a quote, it's likely a key name
		if strings.HasPrefix(jsonPath, "'") && strings.HasSuffix(jsonPath, "'") {
			return fmt.Sprintf("%s->>%s", baseField, jsonPath)
		}
		// Otherwise format it as needed
		return fmt.Sprintf("%s->>%s", baseField, jsonPath)
	}

	// Using the -> operator (return as JSON)
	// If the path starts with a quote, it's likely a key name
	if strings.HasPrefix(parts[1], "'") && strings.HasSuffix(parts[1], "'") {
		return fmt.Sprintf("%s->%s", baseField, parts[1])
	}
	// Otherwise format it as needed
	return fmt.Sprintf("%s->%s", baseField, parts[1])
}

// buildWhereClause builds a WHERE clause for a filter
func (b *SQLQueryBuilder) buildWhereClause(filter Filter) (string, []interface{}) {
	var whereClause string
	var args []interface{}

	// Handle JSON fields differently
	var fieldRef string
	if b.isJSONField(filter.Field) {
		fieldRef = b.handleJSONField(filter.Field)
	} else {
		fieldRef = b.sanitizeIdentifier(filter.Field)
	}

	switch filter.Operator {
	case OperatorEqual:
		whereClause = fmt.Sprintf("%s = $%d", fieldRef, b.nextParamIndex())
		args = append(args, filter.Value)
	case OperatorNotEqual:
		whereClause = fmt.Sprintf("%s != $%d", fieldRef, b.nextParamIndex())
		args = append(args, filter.Value)
	case OperatorGreaterThan:
		whereClause = fmt.Sprintf("%s > $%d", fieldRef, b.nextParamIndex())
		args = append(args, filter.Value)
	case OperatorGreaterThanOrEqual:
		whereClause = fmt.Sprintf("%s >= $%d", fieldRef, b.nextParamIndex())
		args = append(args, filter.Value)
	case OperatorLessThan:
		whereClause = fmt.Sprintf("%s < $%d", fieldRef, b.nextParamIndex())
		args = append(args, filter.Value)
	case OperatorLessThanOrEqual:
		whereClause = fmt.Sprintf("%s <= $%d", fieldRef, b.nextParamIndex())
		args = append(args, filter.Value)
	case OperatorContains:
		whereClause = fmt.Sprintf("%s LIKE $%d", fieldRef, b.nextParamIndex())
		args = append(args, "%"+filter.Value.(string)+"%")
	case OperatorIn:
		if arr, ok := filter.Value.([]interface{}); ok {
			placeholders := make([]string, len(arr))
			for i := range arr {
				placeholders[i] = fmt.Sprintf("$%d", b.nextParamIndex())
				args = append(args, arr[i])
			}
			whereClause = fmt.Sprintf("%s IN (%s)", fieldRef, strings.Join(placeholders, ", "))
		}
	case OperatorIsNull:
		if filter.Value.(bool) {
			whereClause = fmt.Sprintf("%s IS NULL", fieldRef)
		} else {
			whereClause = fmt.Sprintf("%s IS NOT NULL", fieldRef)
		}
	}

	return whereClause, args
}

// SQLQueryExecutor implements the QueryExecutor interface for SQL databases
// E is the type of entities that will be returned
type SQLQueryExecutor[E any] struct {
	db *sqlx.DB
}

// NewSQLQueryExecutor creates a new SQLQueryExecutor
func NewSQLQueryExecutor[E any](db *sqlx.DB) *SQLQueryExecutor[E] {
	return &SQLQueryExecutor[E]{db: db}
}

// Execute implements the QueryExecutor interface
func (e *SQLQueryExecutor[E]) Execute(ctx context.Context, query SQLQuery) ([]E, int64, error) {
	// Execute the query with parameters
	rows, err := e.db.QueryxContext(ctx, query.Query, query.Args...)
	if err != nil {
		return nil, 0, fmt.Errorf("error executing query: %w", err)
	}
	defer rows.Close()

	// Extract results
	var entities []E
	for rows.Next() {
		var entity E
		if err := rows.StructScan(&entity); err != nil {
			return nil, 0, fmt.Errorf("error scanning row: %w", err)
		}
		entities = append(entities, entity)
	}

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating rows: %w", err)
	}

	// Get total count
	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", query.Tables[0])
	err = e.db.GetContext(ctx, &total, countQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("error getting total count: %w", err)
	}

	return entities, total, nil
}

// SQLValidator implements the Validator interface for SQL databases
type SQLValidator struct {
	config ValidationConfig
}

// NewSQLValidator creates a new SQLValidator
func NewSQLValidator(config ValidationConfig) *SQLValidator {
	return &SQLValidator{
		config: config,
	}
}

// Validate implements the Validator interface
func (v *SQLValidator) Validate(criteria *Criteria) error {
	// Validate fields
	for _, filter := range criteria.Filters {
		if !slices.Contains(v.config.AllowedFields, filter.Field) {
			return fmt.Errorf("%w: %s", ErrInvalidField, filter.Field)
		}
	}

	// Validate sorts
	for _, sort := range criteria.Sorts {
		if !slices.Contains(v.config.AllowedSortFields, sort.Field) {
			return fmt.Errorf("%w: %s", ErrInvalidSortBy, sort.Field)
		}
	}

	// Validate pagination
	if criteria.Pagination != nil {
		if criteria.Pagination.Page < 1 {
			return ErrInvalidPage
		}
		if criteria.Pagination.Limit < 1 {
			return ErrInvalidLimit
		}
		if v.config.MaxLimit > 0 && criteria.Pagination.Limit > v.config.MaxLimit {
			criteria.Pagination.Limit = v.config.MaxLimit
		}
	} else if v.config.DefaultLimit > 0 {
		// Set default pagination if none provided
		criteria.Pagination = &Pagination{
			Page:  1,
			Limit: v.config.DefaultLimit,
		}
	}

	// Set default sort if none provided
	if len(criteria.Sorts) == 0 && v.config.DefaultSortField != "" {
		criteria.AddSort(v.config.DefaultSortField, v.config.DefaultSortOrder)
	}

	return nil
}

// EntityMapper is a non-reflection implementation of DataMapper for SQL entities
// E is the type of entities from the database
// M is the type of models for the domain
type EntityMapper[E any, M any] struct {
	modelToEntity   func(M) E
	entityToModel   func(E) M
	batchToModels   func([]E) []M
	batchToEntities func([]M) []E
}

// NewEntityMapper creates a new EntityMapper with explicitly defined mapping functions
func NewEntityMapper[E any, M any](
	modelToEntity func(M) E,
	entityToModel func(E) M,
) *EntityMapper[E, M] {
	// Create default batch mapping functions that use the single-item mappers
	batchToModels := func(entities []E) []M {
		if entities == nil {
			return nil
		}
		models := make([]M, len(entities))
		for i, entity := range entities {
			models[i] = entityToModel(entity)
		}
		return models
	}

	batchToEntities := func(models []M) []E {
		if models == nil {
			return nil
		}
		entities := make([]E, len(models))
		for i, model := range models {
			entities[i] = modelToEntity(model)
		}
		return entities
	}

	return &EntityMapper[E, M]{
		modelToEntity:   modelToEntity,
		entityToModel:   entityToModel,
		batchToModels:   batchToModels,
		batchToEntities: batchToEntities,
	}
}

// WithBatchMappers allows customizing the batch mapping functions if needed
func (m *EntityMapper[E, M]) WithBatchMappers(
	batchToModels func([]E) []M,
	batchToEntities func([]M) []E,
) *EntityMapper[E, M] {
	m.batchToModels = batchToModels
	m.batchToEntities = batchToEntities
	return m
}

// MapToModel maps database entities to domain models
func (m *EntityMapper[E, M]) MapToModel(entities []E) []M {
	return m.batchToModels(entities)
}

// MapToEntity maps domain models to database entities
func (m *EntityMapper[E, M]) MapToEntity(models []M) []E {
	return m.batchToEntities(models)
}

// MapSingleToModel maps a single database entity to a domain model
func (m *EntityMapper[E, M]) MapSingleToModel(entity E) M {
	return m.entityToModel(entity)
}

// MapSingleToEntity maps a single domain model to a database entity
func (m *EntityMapper[E, M]) MapSingleToEntity(model M) E {
	return m.modelToEntity(model)
}

// sanitizeIdentifier ensures SQL identifiers are safe by removing any potentially dangerous characters
func (b *SQLQueryBuilder) sanitizeIdentifier(identifier string) string {
	// Special case for JSON paths - preserve JSON operators
	if strings.Contains(identifier, "->") {
		return b.handleJSONField(identifier)
	}

	// Only allow alphanumeric characters, underscores, and dots (for schema.table format)
	reg := regexp.MustCompile(`[^a-zA-Z0-9_\.]`)
	return reg.ReplaceAllString(identifier, "")
}
