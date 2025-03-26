# Generic Criteria Pattern

This package provides a flexible, reusable criteria pattern implementation for querying data from various backends (SQL databases, HTTP APIs, in-memory datastores, etc.). It allows for building type-safe, composable queries with filtering, sorting, and pagination functionality.

## Features

- **Type-safe implementation using Go generics** - All interfaces are parameterized to ensure compile-time type checking
- **Flexible filtering** - Filter by field, operator, and value with multiple filter conditions
- **Sorting** - Sort by multiple fields in ascending or descending order
- **Pagination** - Paginate results with page and limit parameters
- **Multi-backend support** - Works with SQL databases, HTTP APIs, in-memory data, and custom backends
- **Interface-based design** - Easily extend for different backend systems
- **SQL generation** - Automatic generation of SQL queries with parameterized queries
- **Validation** - Validate criteria against allowed fields and operations
- **SQL injection protection** - Parameterized queries prevent SQL injection attacks

## Type Safety with Generics

With Go 1.18+ generics support, the criteria pattern is implemented using type parameters to ensure type safety throughout the query building, execution, and mapping process. This eliminates the need for type assertions and reduces runtime errors.

### Key Generic Interfaces

- `QueryBuilder[Q any]` - Builds a query of type Q from criteria
- `QueryExecutor[Q any, E any]` - Executes a query of type Q and returns entities of type E
- `DataMapper[E any, M any]` - Maps between entity types E and model types M
- `Repository[E any, M any, Q any]` - Generic repository for querying entities
- `CriteriaFilter[T any]` - Converts an object to filters
- `CriteriaSort[T any]` - Converts an object to sorts

## Using with SQL Database


## Using with HTTP API


## Creating a Custom Backend

To implement a custom backend, you need to create implementations of the following interfaces:

1. A query type specific to your backend
2. A `QueryBuilder` implementation that builds your query type
3. A `QueryExecutor` implementation that executes your query type
4. A `DataMapper` implementation that maps between your entities and domain models
5. A `Validator` implementation that validates criteria for your backend

See the `example_type_safe.go` file for a complete in-memory backend implementation.

## Available Operators

The following operators are available for filtering:

- `OperatorEqual` - Equality comparison
- `OperatorNotEqual` - Inequality comparison
- `OperatorGreaterThan` - Greater than comparison
- `OperatorGreaterThanOrEqual` - Greater than or equal comparison
- `OperatorLessThan` - Less than comparison
- `OperatorLessThanOrEqual` - Less than or equal comparison
- `OperatorContains` - Contains substring (for string fields)
- `OperatorIn` - Value is in a list of values
- `OperatorIsNull` - Field is null

## Sort Order

The following sort orders are available:

- `OrderAsc` - Ascending order
- `OrderDesc` - Descending order


## Best Practices

- Define your domain models and database entities as separate types
- Use the `ValidationConfig` to restrict which fields can be filtered and sorted
- Set sensible default limits for pagination
- Use the type-safe generics to ensure compile-time checking
- Create repositories for specific entity types to simplify usage

## Extending the Pattern

This criteria pattern can be extended in several ways:

- Add support for more complex filter combinations (AND/OR logic)
- Implement custom operators specific to your domain
- Add support for aggregations or group-by operations
- Implement caching mechanisms for query results
- Add support for transactions
