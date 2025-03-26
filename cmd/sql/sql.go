package sql

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"           // PostgreSQL driver
	_ "github.com/mattn/go-sqlite3" // SQLite driver
	"github.com/spf13/cobra"

	"github.com/b87/go-crit/cmd"
	"github.com/b87/go-crit/crit"
)

// Variables to store flags
var (
	dbDriver     string
	dbDSN        string
	dbTable      string
	queryLimit   int
	queryPage    int
	outputFormat string
	timeout      time.Duration
	fields       []string
	filters      []string
	sortFields   []string
)

func init() {
	// Add commands to the SQL command
	sqlCmd.AddCommand(sqlConnectCmd)
	sqlCmd.AddCommand(sqlFindCmd)

	// Add the SQL command to the root command
	cmd.RootCmd.AddCommand(sqlCmd)

	// Global flags for SQL commands
	sqlCmd.PersistentFlags().StringVar(&dbDriver, "driver", "postgres", "Database driver (mysql, postgres, sqlite3)")
	sqlCmd.PersistentFlags().StringVar(&dbDSN, "dsn", "", "Database connection string")
	sqlCmd.PersistentFlags().StringVar(&outputFormat, "format", "table", "Output format (table, json, csv)")
	sqlCmd.PersistentFlags().DurationVar(&timeout, "timeout", 30*time.Second, "Query timeout")

	// Flags for the find command
	sqlFindCmd.Flags().StringVar(&dbTable, "table", "", "Table name")
	sqlFindCmd.Flags().IntVar(&queryLimit, "limit", 10, "Maximum number of records to return")
	sqlFindCmd.Flags().IntVar(&queryPage, "page", 1, "Page number for pagination")
	sqlFindCmd.Flags().StringSliceVar(&fields, "fields", []string{}, "Allowed fields to query and sort")
	sqlFindCmd.Flags().StringSliceVar(&filters, "filter", []string{}, "Filters in format field:operator:value")
	sqlFindCmd.Flags().StringSliceVar(&sortFields, "sort", []string{}, "Sort fields in format field:order (asc or desc)")
}

var sqlCmd = &cobra.Command{
	Use:   "sql",
	Short: "Interact with SQL databases",
	Long:  `Interact with SQL databases using the go-crit library.`,
	Example: `
crit sql test --driver mysql --dsn "user:pass@tcp(localhost:3306)/dbname"
crit sql find --table users --filter "status:eq:active" --sort "created_at:desc" --limit 20 --page 1
`,
	Run: func(cmd *cobra.Command, args []string) {
		// Just show help if no subcommand is provided
		cmd.Help()
	},
}

var sqlConnectCmd = &cobra.Command{
	Use:   "test",
	Short: "Test connection to a SQL database",
	Long:  `Test connection to a SQL database using the provided driver and DSN.`,
	Example: `  crit sql test --driver mysql --dsn "user:pass@tcp(localhost:3306)/dbname"
  crit sql test --driver postgres --dsn "postgres://user:pass@localhost:5432/dbname?sslmode=disable"
  crit sql test --driver sqlite3 --dsn "./local.db"`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		db, err := sqlx.ConnectContext(ctx, dbDriver, dbDSN)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()

		fmt.Println("Successfully connected to database")

		// Test the connection with a ping
		if err := db.PingContext(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error pinging database: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Database connection is valid")
	},
}

var sqlFindCmd = &cobra.Command{
	Use:   "find",
	Short: "Find records in a SQL database",
	Long:  `Find records in a SQL database using criteria filters, sorting, and pagination.`,
	Example: `  crit sql find --table users --filter "status:eq:active" --filter "created_at:gt:2020-01-01"
  crit sql find --table orders --sort "created_at:desc" --limit 50 --page 2
  crit sql find --table products --filter "price:gt:100" --filter "category:in:electronics,books" --fields "id,name,price,category"

  # PostgreSQL JSON field filtering examples
  crit sql find --table users --filter "data->>'name':eq:John" --driver postgres
  crit sql find --table products --filter "metadata->>'price':gt:100" --driver postgres
  crit sql find --table events --filter "attributes->'tags'->>'color':eq:blue" --driver postgres`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		if dbTable == "" {
			fmt.Fprintf(os.Stderr, "Error: table name is required\n")
			os.Exit(1)
		}

		db, err := sqlx.ConnectContext(ctx, dbDriver, dbDSN)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()

		// Create the criteria
		criteria := crit.NewCriteria()

		// Add filters
		for _, filter := range filters {
			parts := strings.SplitN(filter, ":", 3)
			if len(parts) != 3 {
				fmt.Fprintf(os.Stderr, "Error: invalid filter format: %s. Use field:operator:value\n", filter)
				continue
			}

			field := parts[0]
			operator := parseOperator(parts[1])
			value := parts[2]

			criteria.AddFilter(field, operator, value)
		}

		// Add sort fields
		for _, sort := range sortFields {
			parts := strings.SplitN(sort, ":", 2)
			field := parts[0]
			order := crit.OrderAsc
			if len(parts) > 1 && strings.ToLower(parts[1]) == "desc" {
				order = crit.OrderDesc
			}
			criteria.AddSort(field, order)
		}

		// Add pagination
		if queryLimit > 0 {
			criteria.SetPagination(queryPage, queryLimit)
		}

		// Create validator with allowed fields
		validator := crit.NewSQLValidator(crit.ValidationConfig{
			AllowedFields:     fields,
			AllowedSortFields: fields,
			DefaultLimit:      100,
			MaxLimit:          1000,
		})

		// Validate criteria
		if err := validator.Validate(criteria); err != nil {
			fmt.Fprintf(os.Stderr, "Error in criteria: %v\n", err)
			os.Exit(1)
		}

		// Create query builder
		queryBuilder := crit.NewSQLQueryBuilder(dbTable)

		// Build the query
		query, err := queryBuilder.BuildQuery(criteria)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error building query: %v\n", err)
			os.Exit(1)
		}

		// Execute the query and get the results as maps
		results, err := executeAndGetMaps(ctx, db, query.Query, query.Args...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error executing query: %v\n", err)
			os.Exit(1)
		}

		// Get column names from the first result
		var columns []string
		if len(results) > 0 {
			for col := range results[0] {
				columns = append(columns, col)
			}
		}

		// Output results
		outputResults(results, columns)
	},
}

// Parse operator string to FilterOperator
func parseOperator(op string) crit.FilterOperator {
	switch strings.ToLower(op) {
	case "eq", "=", "==":
		return crit.OperatorEqual
	case "neq", "!=", "<>":
		return crit.OperatorNotEqual
	case "gt", ">":
		return crit.OperatorGreaterThan
	case "gte", ">=":
		return crit.OperatorGreaterThanOrEqual
	case "lt", "<":
		return crit.OperatorLessThan
	case "lte", "<=":
		return crit.OperatorLessThanOrEqual
	case "contains", "like":
		return crit.OperatorContains
	case "in":
		return crit.OperatorIn
	case "null", "isnull":
		return crit.OperatorIsNull
	default:
		return crit.OperatorEqual
	}
}

// Execute a query and return results as maps
func executeAndGetMaps(ctx context.Context, db *sqlx.DB, query string, args ...interface{}) ([]map[string]interface{}, error) {
	rows, err := db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for rows.Next() {
		row := make(map[string]interface{})
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}

		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// Output results in the specified format
func outputResults(results []map[string]interface{}, columns []string) {
	if len(results) == 0 {
		fmt.Println("No results found")
		return
	}

	switch strings.ToLower(outputFormat) {
	case "json":
		jsonOutput, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling to JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonOutput))

	case "csv":
		// Print header
		fmt.Println(strings.Join(columns, ","))

		// Print rows
		for _, row := range results {
			var values []string
			for _, col := range columns {
				var value string
				if row[col] == nil {
					value = "NULL"
				} else {
					value = fmt.Sprintf("%v", row[col])
					// Escape commas and quotes for CSV
					if strings.Contains(value, ",") || strings.Contains(value, "\"") {
						value = strings.ReplaceAll(value, "\"", "\"\"")
						value = "\"" + value + "\""
					}
				}
				values = append(values, value)
			}
			fmt.Println(strings.Join(values, ","))
		}

	default: // table format
		// Find the max width for each column
		colWidths := make(map[string]int)
		for _, col := range columns {
			colWidths[col] = len(col)
		}

		for _, row := range results {
			for _, col := range columns {
				var value string
				if row[col] == nil {
					value = "NULL"
				} else {
					value = fmt.Sprintf("%v", row[col])
				}
				if len(value) > colWidths[col] {
					colWidths[col] = len(value)
				}
			}
		}

		// Print the header
		var sb strings.Builder
		for i, col := range columns {
			if i > 0 {
				sb.WriteString("  ")
			}
			format := fmt.Sprintf("%%-%ds", colWidths[col])
			sb.WriteString(fmt.Sprintf(format, col))
		}
		fmt.Println(sb.String())

		// Print the separator
		sb.Reset()
		for i, col := range columns {
			if i > 0 {
				sb.WriteString("  ")
			}
			sb.WriteString(strings.Repeat("-", colWidths[col]))
		}
		fmt.Println(sb.String())

		// Print the rows
		for _, row := range results {
			sb.Reset()
			for i, col := range columns {
				if i > 0 {
					sb.WriteString("  ")
				}
				var value string
				if row[col] == nil {
					value = "NULL"
				} else {
					value = fmt.Sprintf("%v", row[col])
				}
				format := fmt.Sprintf("%%-%ds", colWidths[col])
				sb.WriteString(fmt.Sprintf(format, value))
			}
			fmt.Println(sb.String())
		}
	}
}
