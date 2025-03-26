package zendesk_sales

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/b87/go-crit/crit"
	"github.com/spf13/viper"
)

// ZendeskQueryBuilder implements a custom QueryBuilder for Zendesk API
type ZendeskQueryBuilder struct {
	baseURL string
	path    string
}

// NewZendeskQueryBuilder creates a new query builder for Zendesk
func NewZendeskQueryBuilder(baseURL, path string) *ZendeskQueryBuilder {
	return &ZendeskQueryBuilder{
		baseURL: baseURL,
		path:    path,
	}
}

// BuildQuery implements the QueryBuilder interface
func (b *ZendeskQueryBuilder) BuildQuery(criteria *crit.Criteria) (crit.HTTPQuery, error) {
	params := url.Values{}

	// Add filters
	if err := b.addFilters(params, criteria.Filters); err != nil {
		return crit.HTTPQuery{}, err
	}

	// Add pagination
	b.addPagination(params, criteria.Pagination)

	// Add sorts
	b.addSorting(params, criteria.Sorts)

	// Log the parameters if verbose is enabled
	if viper.GetBool("verbose") {
		fmt.Printf("Zendesk query parameters: %v\n", params)
	}

	return crit.HTTPQuery{
		BaseURL:     b.baseURL,
		Path:        b.path,
		QueryParams: params,
	}, nil
}

// addFilters adds filters to the query parameters
func (b *ZendeskQueryBuilder) addFilters(params url.Values, filters []crit.Filter) error {
	for _, filter := range filters {
		// Handle custom fields separately
		if strings.HasPrefix(filter.Field, "custom_fields[") {
			params.Add(filter.Field, fmt.Sprintf("%v", filter.Value))
			continue
		}

		// Handle different operators
		switch filter.Operator {
		case crit.OperatorEqual:
			// Standard equals filter
			params.Add(filter.Field, formatValue(filter.Value))

		case crit.OperatorGreaterThan, crit.OperatorGreaterThanOrEqual,
			crit.OperatorLessThan, crit.OperatorLessThanOrEqual:
			// Date comparisons need special handling
			if isDateField(filter.Field) {
				var paramName string

				switch filter.Operator {
				case crit.OperatorGreaterThan:
					paramName = filter.Field + ":gt"
				case crit.OperatorGreaterThanOrEqual:
					paramName = filter.Field + ":gte"
				case crit.OperatorLessThan:
					paramName = filter.Field + ":lt"
				case crit.OperatorLessThanOrEqual:
					paramName = filter.Field + ":lte"
				}

				params.Add(paramName, formatValue(filter.Value))
			} else {
				// For non-date fields
				params.Add(filter.Field, formatValue(filter.Value))
			}

		case crit.OperatorContains:
			// Text search (may need specific syntax for Zendesk)
			params.Add(filter.Field+":contains", formatValue(filter.Value))

		case crit.OperatorIn:
			// Handle array values for IN operator
			if arr, ok := filter.Value.([]interface{}); ok {
				values := make([]string, len(arr))
				for i, v := range arr {
					values[i] = formatValue(v)
				}
				params.Add(filter.Field, strings.Join(values, ","))
			} else {
				return fmt.Errorf("IN operator requires array value for field %s", filter.Field)
			}

		case crit.OperatorIsNull:
			// Handle IS NULL - Zendesk might use a special syntax
			params.Add(filter.Field+":null", "true")

		default:
			// Log unknown operator but don't fail
			if viper.GetBool("verbose") {
				fmt.Printf("Warning: Unsupported operator %s for field %s\n",
					filter.Operator, filter.Field)
			}
		}
	}

	return nil
}

// addPagination adds pagination parameters
func (b *ZendeskQueryBuilder) addPagination(params url.Values, pagination *crit.Pagination) {
	if pagination != nil {
		// Zendesk API expects 'page' and 'per_page' parameters
		params.Add("page", strconv.Itoa(pagination.Page))
		params.Add("per_page", strconv.Itoa(pagination.Limit))
	}
}

// addSorting adds sorting parameters
func (b *ZendeskQueryBuilder) addSorting(params url.Values, sorts []crit.Sort) {
	if len(sorts) > 0 {
		// Handle multiple sorts if API supports it
		sortParams := make([]string, 0, len(sorts))

		for _, sort := range sorts {
			direction := "asc"
			if sort.Order == crit.OrderDesc {
				direction = "desc"
			}
			sortParams = append(sortParams, fmt.Sprintf("%s:%s", sort.Field, direction))
		}

		// Add as comma-separated list
		params.Add("sort_by", strings.Join(sortParams, ","))
	}
}

// isDateField returns true if the field is a date field
func isDateField(field string) bool {
	return field == "created_at" ||
		field == "updated_at" ||
		field == "last_activity_at" ||
		strings.HasSuffix(field, "_date")
}

// formatValue formats different value types for the API
func formatValue(value interface{}) string {
	switch v := value.(type) {
	case time.Time:
		// Format time values consistently
		return v.Format(time.RFC3339)
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}
