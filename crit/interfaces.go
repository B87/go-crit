package crit

import "context"

// QueryBuilder builds a query of type Q from a criteria
type QueryBuilder[Q any] interface {
	BuildQuery(criteria *Criteria) (Q, error)
}

// QueryExecutor executes a query of type Q and returns entities of type E
type QueryExecutor[Q any, E any] interface {
	Execute(ctx context.Context, query Q) ([]E, int64, error)
}

// DataMapper maps between entity types E and model types M
type DataMapper[E any, M any] interface {
	MapToModel(entities []E) []M
	MapToEntity(models []M) []E
	MapSingleToModel(entity E) M
	MapSingleToEntity(model M) E
}

// Validator validates criteria
type Validator interface {
	Validate(criteria *Criteria) error
}

// CriteriaFilter converts an object to a set of filters
type CriteriaFilter[T any] interface {
	ToFilters(obj T) []Filter
}

// CriteriaSort converts an object to a set of sorts
type CriteriaSort[T any] interface {
	ToSorts(obj T) []Sort
}

// Repository is a generic repository for querying entities
type Repository[E any, M any, Q any] interface {
	Find(ctx context.Context, criteria *Criteria) ([]M, int64, error)
}

// GenericRepository is a type-safe implementation of Repository
type GenericRepository[E any, M any, Q any] struct {
	queryBuilder  QueryBuilder[Q]
	queryExecutor QueryExecutor[Q, E]
	dataMapper    DataMapper[E, M]
	validator     Validator
}

// NewGenericRepository creates a new GenericRepository
func NewGenericRepository[E any, M any, Q any](
	queryBuilder QueryBuilder[Q],
	queryExecutor QueryExecutor[Q, E],
	dataMapper DataMapper[E, M],
	validator Validator,
) *GenericRepository[E, M, Q] {
	return &GenericRepository[E, M, Q]{
		queryBuilder:  queryBuilder,
		queryExecutor: queryExecutor,
		dataMapper:    dataMapper,
		validator:     validator,
	}
}

// Find implements the Repository interface
func (r *GenericRepository[E, M, Q]) Find(ctx context.Context, criteria *Criteria) ([]M, int64, error) {
	// Validate criteria
	if r.validator != nil {
		if err := r.validator.Validate(criteria); err != nil {
			return nil, 0, err
		}
	}

	// Build query
	query, err := r.queryBuilder.BuildQuery(criteria)
	if err != nil {
		return nil, 0, err
	}

	// Execute query
	entities, total, err := r.queryExecutor.Execute(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	// Map entities to models
	models := r.dataMapper.MapToModel(entities)

	return models, total, nil
}
