package zendesk_sales

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/b87/go-crit/cmd"
)

var (
	// Generic filters that replace hardcoded filter flags
	filters []string

	// Flags for pagination
	page  int
	limit int

	// Flags for sorting
	sortBy    string
	sortOrder string
)

// zendeskCmd represents the zendesk command
var zendeskCmd = &cobra.Command{
	Use:   "zendesk",
	Short: "Interact with Zendesk API",
	Long: `Interact with the Zendesk API to query and retrieve data.
This command uses the go-crit library to build type-safe queries against the Zendesk API.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Just show help if no subcommand is provided
		cmd.Help()
	},
}

// salesCmd represents the zendesk sales command
var salesCmd = &cobra.Command{
	Use:   "sales",
	Short: "Interact with Zendesk Sales CRM API",
	Long: `Interact with the Zendesk Sales CRM API (formerly Base CRM) to query and retrieve data.
This command uses the go-crit library to build type-safe queries against the Zendesk Sales API.

Authentication:
  Authentication is done using a Zendesk API token. You can provide this using the --token flag:
  crit zendesk sales --token=YOUR_API_TOKEN ...

  You can also set the ZENDESK_SALES_TOKEN environment variable to avoid typing the token each time.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Just show help if no subcommand is provided
		cmd.Help()
	},
}

// salesFindCmd represents the "find" command for retrieving entities
var salesFindCmd = &cobra.Command{
	Use:   "find",
	Short: "Find entities from Zendesk Sales CRM",
	Long: `Find entities like contacts, deals, leads, and more from Zendesk Sales CRM.
This command allows you to retrieve specific entities or lists of entities.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Just show help if no subcommand is provided
		cmd.Help()
	},
}

// contactsCmd represents the "contacts" command for retrieving contacts
var contactsCmd = &cobra.Command{
	Use:   "contacts",
	Short: "Get contacts from Zendesk Sales CRM",
	Long: `Get contacts from Zendesk Sales CRM with various filtering options.
This command allows you to retrieve contacts with flexible criteria using the new filter syntax.

The new filter syntax replaces all the previous hardcoded filters (--customer-status, --name, etc.)
with a generic --filter flag that allows for more flexibility and follows the pattern used in the SQL module.`,
	Example: `  # Authentication (token can be provided at the sales command level)
  crit zendesk sales --token=YOUR_API_TOKEN find contacts

  # Basic usage - get all contacts (paginated)
  crit zendesk sales find contacts

  # Filter by customer status
  crit zendesk sales find contacts --filter "customer_status:eq:current"

  # Search by name (contains)
  crit zendesk sales find contacts --filter "name:contains:Acme" --limit=50

  # Multiple filters with dates
  crit zendesk sales find contacts --filter "created_at:gte:2023-01-01" --filter "industry:eq:Technology"

  # Organization filter with custom fields
  crit zendesk sales find contacts --filter "is_organization:eq:true" --filter "custom_fields[region]:eq:Europe"

  # Combine with tags and pagination
  crit zendesk sales find contacts --filter "tags:contains:premium" --page=2 --limit=25

  # Advanced date filters (supports relative dates)
  crit zendesk sales find contacts --filter "updated_at:gte:7 days ago" --sort-by="name" --sort-order="asc"`,
	Run: func(cmd *cobra.Command, args []string) {
		// Use the existing query function
		executeContactQuery(false)
	},
}

// dealsCmd represents the "deals" command for retrieving deals
var dealsCmd = &cobra.Command{
	Use:   "deals",
	Short: "Find deals from Zendesk Sales CRM",
	Long: `Find deals from Zendesk Sales CRM with various filtering options.
This command allows you to retrieve deals with flexible criteria using the new filter syntax.

The new filter syntax replaces all the previous hardcoded filters with a generic --filter flag
that allows for more flexibility and follows the pattern used in the SQL module.`,
	Example: `  # Authentication (token can be provided at the sales command level)
  crit zendesk sales --token=YOUR_API_TOKEN find deals

  # Basic usage - get all deals (paginated)
  crit zendesk sales find deals

  # Filter by deal stage
  crit zendesk sales find deals --filter "stage:eq:won"

  # Search by deal name (contains)
  crit zendesk sales find deals --filter "name:contains:Project X" --limit=50

  # Multiple filters with dates
  crit zendesk sales find deals --filter "created_at:gte:2023-01-01" --filter "value:gt:10000"

  # Filter by owner and tags
  crit zendesk sales find deals --filter "owner_id:eq:12345" --filter "tags:contains:priority"

  # Advanced filters with sorting
  crit zendesk sales find deals --filter "updated_at:gte:30 days ago" --sort-by="value" --sort-order="desc"`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Deals functionality is not yet implemented")
		fmt.Println("This is a placeholder to demonstrate command structure")
	},
}

func init() {
	// Add the commands to the command hierarchy
	cmd.RootCmd.AddCommand(zendeskCmd)
	zendeskCmd.AddCommand(salesCmd)
	salesCmd.AddCommand(salesFindCmd)
	salesFindCmd.AddCommand(contactsCmd)
	salesFindCmd.AddCommand(dealsCmd)

	// Define Zendesk-specific flags
	zendeskCmd.PersistentFlags().String("base-url", "https://api.getbase.com", "Zendesk API base URL")

	// Define sales-specific flags - add token flag to the sales command specifically
	salesCmd.PersistentFlags().String("token", "", "Zendesk API token")

	// Bind the flags to viper
	viper.BindPFlag("zendesk.base_url", zendeskCmd.PersistentFlags().Lookup("base-url"))
	viper.BindPFlag("zendesk.sales.token", salesCmd.PersistentFlags().Lookup("token"))

	// Bind environment variables
	viper.BindEnv("zendesk.sales.token", "ZENDESK_SALES_TOKEN")

	// Add common flags to the contacts command
	addCommonFlags(contactsCmd, 25) // contacts uses 25 as default limit
}

// createZendeskClient creates an HTTP client with authentication for Zendesk
func createZendeskClient() (*http.Client, error) {
	// Get the API token from the sales-specific configuration
	token := viper.GetString("zendesk.sales.token")
	if token == "" {
		return nil, fmt.Errorf("API token is required. Set it using --token flag or ZENDESK_SALES_TOKEN environment variable")
	}

	// Create an HTTP client with the token
	client := &http.Client{
		Timeout: viper.GetDuration("timeout"),
		Transport: &zendeskAuthTransport{
			token: token,
		},
	}

	return client, nil
}

// zendeskAuthTransport is a custom RoundTripper that adds authentication to requests
type zendeskAuthTransport struct {
	token string
	base  http.RoundTripper
}

// RoundTrip implements the RoundTripper interface
func (t *zendeskAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Set the authorization header with Bearer token
	req.Header.Set("Authorization", "Bearer "+t.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Zendesk requires a User-Agent header
	req.Header.Set("User-Agent", "go-crit/1.0")

	// Use the default transport if none specified
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}

	return base.RoundTrip(req)
}

// addCommonFlags adds the common flags to the given command
func addCommonFlags(cmd *cobra.Command, defaultLimit int) {
	// Generic filter flag that replaces all the hardcoded filter flags
	cmd.Flags().StringSliceVar(&filters, "filter", []string{}, `Filters in format field:operator:value
Available operators:
  eq, =, ==          Equal to (exact match)
  neq, !=, <>        Not equal to
  gt, >              Greater than
  gte, >=            Greater than or equal to
  lt, <              Less than
  lte, <=            Less than or equal to
  contains, like     Contains substring (case-insensitive)
  in                 In a list of values (comma-separated)
  null, isnull       Is null or not set

Examples:
  --filter "name:contains:Acme"
  --filter "customer_status:eq:current"
  --filter "created_at:gte:2023-01-01"
  --filter "is_organization:eq:true"
  --filter "custom_fields[region]:eq:Europe"
  --filter "tags:contains:premium"`)

	// Pagination flags
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&limit, "limit", defaultLimit, "Results per page")

	// Sorting flags
	cmd.Flags().StringVar(&sortBy, "sort-by", "updated_at", "Field to sort by")
	cmd.Flags().StringVar(&sortOrder, "sort-order", "desc", "Sort order (asc, desc)")
}
