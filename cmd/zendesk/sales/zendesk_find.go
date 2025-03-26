package zendesk_sales

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/b87/go-crit/crit"
)

// This file contains functions for finding contacts
// The command registration has been moved to zendesk.go in the new command hierarchy
// Flag variables are now declared in zendesk.go

// executeContactQuery builds criteria from flags and executes the query
func executeContactQuery(isSearch bool) {
	// Create a criteria object
	criteria := buildCriteriaFromFlags()

	// Get the client
	client, err := createZendeskClient()
	if err != nil {
		fmt.Println(err)
		return
	}

	// Get the base URL
	baseURL := viper.GetString("zendesk.base_url")

	// Execute the query
	contacts, total, err := queryZendeskContacts(client, baseURL, criteria)
	if err != nil {
		// Use our improved error handling
		fmt.Println(err)
		return
	}

	// Output the results based on the format
	outputResults(contacts, total)
}

// buildCriteriaFromFlags creates a criteria object from the command flags
func buildCriteriaFromFlags() *crit.Criteria {
	criteria := crit.NewCriteria()

	// Process all filter flags
	for _, filter := range filters {
		parts := strings.SplitN(filter, ":", 3)
		if len(parts) != 3 {
			fmt.Printf("Invalid filter format: %s. Use field:operator:value\n", filter)
			continue
		}

		field := parts[0]
		operator := parseOperator(parts[1])
		value := parts[2]

		// Handle special cases for certain fields
		if field == "is_organization" && (value == "true" || value == "false") {
			// Convert string boolean to actual boolean
			boolValue := value == "true"
			criteria.AddFilter(field, operator, boolValue)
		} else if strings.HasPrefix(field, "created_at") || strings.HasPrefix(field, "updated_at") {
			// Parse date values
			if t, err := parseDate(value); err == nil {
				criteria.AddFilter(field, operator, t.Format(time.RFC3339))
			} else {
				fmt.Printf("Invalid date format for %s: %v\n", field, err)
			}
		} else if strings.HasPrefix(field, "custom_fields[") && strings.HasSuffix(field, "]") {
			// Handle custom fields directly (already in the right format)
			criteria.AddFilter(field, operator, value)
		} else if field == "custom_field" {
			// Special case for backward compatibility with the old format
			// Expected format: custom_field:eq:key=value
			parts := strings.SplitN(value, "=", 2)
			if len(parts) != 2 {
				fmt.Printf("Invalid custom field format: %s. Use format 'custom_field:eq:key=value'\n", value)
				continue
			}

			fieldName := fmt.Sprintf("custom_fields[%s]", parts[0])
			criteria.AddFilter(fieldName, crit.OperatorEqual, parts[1])
		} else if field == "tags" && operator == crit.OperatorContains {
			// Handle tags - can be comma-separated for backward compatibility
			tagValues := strings.Split(value, ",")
			for _, tag := range tagValues {
				criteria.AddFilter("tags", crit.OperatorContains, strings.TrimSpace(tag))
			}
		} else {
			// Normal filter
			criteria.AddFilter(field, operator, value)
		}
	}

	// Add sorting if provided
	if sortBy != "" {
		// Default to ascending order if not specified
		order := crit.OrderAsc
		if strings.ToLower(sortOrder) == "desc" {
			order = crit.OrderDesc
		}
		criteria.AddSort(sortBy, order)
	}

	// Add pagination
	criteria.SetPagination(page, limit)

	return criteria
}

// parseOperator converts a string operator to a FilterOperator
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
		fmt.Printf("Warning: Unknown operator '%s', defaulting to 'eq'\n", op)
		return crit.OperatorEqual
	}
}

// parseDate parses a date string in various formats
func parseDate(dateStr string) (time.Time, error) {
	// Support multiple date formats in order from most specific to least specific
	formats := []string{
		time.RFC3339,          // "2006-01-02T15:04:05Z07:00"
		"2006-01-02T15:04:05", // "2006-01-02T15:04:05"
		"2006-01-02 15:04:05", // "2006-01-02 15:04:05"
		"2006-01-02",          // "2006-01-02"
		"01/02/2006",          // "MM/DD/YYYY"
		"02/01/2006",          // "DD/MM/YYYY"
		"Jan 2, 2006",         // "Mon D, YYYY"
		"January 2, 2006",     // "Month D, YYYY"
	}

	// Try each format
	var err error
	var t time.Time

	for _, format := range formats {
		t, err = time.Parse(format, dateStr)
		if err == nil {
			return t, nil
		}
	}

	// If relative date (e.g., "today", "yesterday", "1 week ago")
	// Use simple parsing for common relative dates
	switch strings.ToLower(dateStr) {
	case "today":
		return time.Now().Truncate(24 * time.Hour), nil
	case "yesterday":
		return time.Now().AddDate(0, 0, -1).Truncate(24 * time.Hour), nil
	case "last week", "1 week ago":
		return time.Now().AddDate(0, 0, -7).Truncate(24 * time.Hour), nil
	case "last month", "1 month ago":
		return time.Now().AddDate(0, -1, 0).Truncate(24 * time.Hour), nil
	}

	// Try to parse a relative date in the format "X days ago", "X months ago", "X years ago"
	relativeRegex := regexp.MustCompile(`^(\d+)\s*(day|days|month|months|year|years)\s*ago$`)
	if matches := relativeRegex.FindStringSubmatch(strings.ToLower(dateStr)); len(matches) == 3 {
		amount, _ := strconv.Atoi(matches[1]) // Safe to ignore error since regex ensures it's a number
		unit := matches[2]

		now := time.Now()
		switch {
		case strings.HasPrefix(unit, "day"):
			return now.AddDate(0, 0, -amount).Truncate(24 * time.Hour), nil
		case strings.HasPrefix(unit, "month"):
			return now.AddDate(0, -amount, 0).Truncate(24 * time.Hour), nil
		case strings.HasPrefix(unit, "year"):
			return now.AddDate(-amount, 0, 0).Truncate(24 * time.Hour), nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized date format: %s. Use YYYY-MM-DD or other common formats", dateStr)
}

// queryZendeskContacts queries Zendesk contacts using the criteria library
func queryZendeskContacts(client *http.Client, baseURL string, criteria *crit.Criteria) ([]Contact, int64, error) {
	// Create HTTP components with specific entity type (ZendeskContactEntity)
	// Use our custom Zendesk query builder instead of the generic one
	httpBuilder := NewZendeskQueryBuilder(baseURL, "/v2/contacts")

	// Use our custom executor
	httpExecutor := NewZendeskHTTPExecutor(client)

	// Create a validator with allowed fields based on Zendesk API documentation
	allowedFields := []string{
		"id", "name", "first_name", "last_name", "email",
		"phone", "created_at", "updated_at", "customer_status",
		"prospect_status", "title", "description", "industry",
		"website", "is_organization", "owner_id", "custom_fields",
	}
	httpValidator := crit.NewHTTPValidator(allowedFields, 100)

	// Create a data mapper between ZendeskContactEntity and Contact domain model
	dataMapper := crit.NewGenericHTTPMapper(
		func() Contact { return Contact{} },
		func() ZendeskContactEntity { return ZendeskContactEntity{} },
	)

	// Create a type-safe repository
	repo := crit.NewGenericRepository(
		httpBuilder,
		httpExecutor,
		dataMapper,
		httpValidator,
	)

	// Enable debug output if verbose mode is on
	debug := viper.GetBool("verbose")
	if debug {
		fmt.Printf("Making request to: %s/v2/contacts\n", baseURL)

		// Print the criteria details in a more user-friendly way
		fmt.Println("Using criteria:")

		// Print filters
		if len(criteria.Filters) > 0 {
			fmt.Println("  Filters:")
			for _, filter := range criteria.Filters {
				// Format the operator in a readable way
				op := ""
				switch filter.Operator {
				case crit.OperatorEqual:
					op = "eq"
				case crit.OperatorNotEqual:
					op = "neq"
				case crit.OperatorGreaterThan:
					op = "gt"
				case crit.OperatorGreaterThanOrEqual:
					op = "gte"
				case crit.OperatorLessThan:
					op = "lt"
				case crit.OperatorLessThanOrEqual:
					op = "lte"
				case crit.OperatorContains:
					op = "contains"
				case crit.OperatorIn:
					op = "in"
				case crit.OperatorIsNull:
					op = "isnull"
				default:
					op = string(filter.Operator)
				}

				// Print the filter
				fmt.Printf("    %s:%s:%v\n", filter.Field, op, filter.Value)
			}
		}

		// Print sorting
		if len(criteria.Sorts) > 0 {
			fmt.Println("  Sort:")
			for _, sort := range criteria.Sorts {
				order := "asc"
				if sort.Order == crit.OrderDesc {
					order = "desc"
				}
				fmt.Printf("    %s:%s\n", sort.Field, order)
			}
		}

		// Print pagination
		if criteria.Pagination != nil {
			fmt.Printf("  Page: %d, Limit: %d\n", criteria.Pagination.Page, criteria.Pagination.Limit)
		}
	}

	// Execute the query
	ctx := context.Background()
	return repo.Find(ctx, criteria)
}

// outputResults outputs the results in the specified format
func outputResults(contacts []Contact, total int64) {
	format := viper.GetString("output")

	switch strings.ToLower(format) {
	case "json":
		// Output as JSON
		data, err := json.MarshalIndent(map[string]interface{}{
			"contacts": contacts,
			"total":    total,
		}, "", "  ")
		if err != nil {
			fmt.Printf("Error formatting output: %v\n", err)
			return
		}
		fmt.Println(string(data))

	case "yaml":
		// Output as YAML
		data, err := yaml.Marshal(map[string]interface{}{
			"contacts": contacts,
			"total":    total,
		})
		if err != nil {
			fmt.Printf("Error formatting output: %v\n", err)
			return
		}
		fmt.Println(string(data))

	default:
		// Output as a table by default
		if len(contacts) == 0 {
			fmt.Println("No contacts found")
			return
		}

		// Create a tabwriter
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "ID\tName\tEmail\tCustomer Status\tIndustry\tUpdated At\n")
		for _, contact := range contacts {
			fmt.Fprintf(w, "%v\t%s\t%s\t%s\t%s\t%s\n",
				contact.ID,
				contact.Name,
				contact.Email,
				contact.CustomerStatus,
				contact.Industry,
				contact.UpdatedAt.Format("2006-01-02 15:04:05"),
			)
		}
		w.Flush()

		// Print total
		fmt.Printf("\nTotal: %d\n", total)
	}
}
