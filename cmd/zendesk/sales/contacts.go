package zendesk_sales

import (
	"context"
	"net/http"
	"time"

	"github.com/b87/go-crit/crit"
)

// ZendeskContactEntity represents the raw contact data from Zendesk API
type ZendeskContactEntity struct {
	ID              int64                  `json:"id"`
	CreatorID       int64                  `json:"creator_id"`
	OrgID           int64                  `json:"organization_id,omitempty"`
	Name            string                 `json:"name"`
	FirstName       string                 `json:"first_name"`
	LastName        string                 `json:"last_name"`
	IsOrganization  bool                   `json:"is_organization"`
	ContactID       int64                  `json:"contact_id,omitempty"`
	ParentOrgID     int64                  `json:"parent_organization_id,omitempty"`
	OwnerID         int64                  `json:"owner_id"`
	Email           string                 `json:"email"`
	Phone           string                 `json:"phone"`
	Mobile          string                 `json:"mobile"`
	Fax             string                 `json:"fax"`
	Twitter         string                 `json:"twitter"`
	Facebook        string                 `json:"facebook"`
	LinkedIn        string                 `json:"linkedin"`
	Skype           string                 `json:"skype"`
	Title           string                 `json:"title"`
	Description     string                 `json:"description"`
	Industry        string                 `json:"industry"`
	Website         string                 `json:"website"`
	CustomerStatus  string                 `json:"customer_status"`
	ProspectStatus  string                 `json:"prospect_status"`
	Tags            []string               `json:"tags"`
	CustomFields    map[string]interface{} `json:"custom_fields"`
	Address         Address                `json:"address"`
	ShippingAddress Address                `json:"shipping_address"`
	BillingAddress  Address                `json:"billing_address"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// Contact is our domain model for Zendesk contacts
type Contact struct {
	ID             int64
	Name           string
	FirstName      string
	LastName       string
	IsOrganization bool
	OwnerID        int64
	Email          string
	Phone          string
	Mobile         string
	CustomerStatus string
	ProspectStatus string
	Title          string
	Description    string
	Industry       string
	Website        string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Tags           []string
	CustomFields   map[string]interface{}
	ContactID      int64
	ParentOrgID    int64
	Address        Address
}

// Address represents a physical address
type Address struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

// ZendeskContacts provides an interface to interact with Zendesk contacts
type ZendeskContacts struct {
	repo      *crit.GenericRepository[ZendeskContactEntity, Contact, crit.HTTPQuery]
	apiClient *http.Client
	baseURL   string
}

// NewZendeskContacts creates a new instance of ZendeskContacts
func NewZendeskContacts(apiToken string, baseURL string) *ZendeskContacts {
	// Create a client with Zendesk authentication
	client, err := createZendeskClient()
	if err != nil {
		panic(err)
	}

	// Create HTTP components with specific entity type
	httpBuilder := crit.NewHTTPQueryBuilder(baseURL, "/v2/contacts")
	httpExecutor := crit.NewHTTPQueryExecutor[ZendeskContactEntity](client)

	// Create a validator with allowed fields based on Zendesk API documentation
	allowedFields := []string{
		"id", "name", "first_name", "last_name", "email",
		"phone", "created_at", "updated_at", "customer_status",
		"prospect_status", "title", "description", "industry",
		"website", "is_organization", "owner_id",
	}
	httpValidator := crit.NewHTTPValidator(allowedFields, 100)

	// Create a data mapper between ZendeskContactEntity and Contact domain model
	dataMapper := crit.NewGenericHTTPMapper(
		func() Contact { return Contact{} },
		func() ZendeskContactEntity { return ZendeskContactEntity{} },
	)

	// Create a type-safe repository
	repo := crit.NewGenericRepository[ZendeskContactEntity, Contact, crit.HTTPQuery](
		httpBuilder,
		httpExecutor,
		dataMapper,
		httpValidator,
	)

	return &ZendeskContacts{
		repo:      repo,
		apiClient: client,
		baseURL:   baseURL,
	}
}

// FindCurrentCustomers retrieves all current customers
func (z *ZendeskContacts) FindCurrentCustomers(ctx context.Context) ([]Contact, int64, error) {
	criteria := crit.NewCriteria().
		AddFilter("customer_status", crit.OperatorEqual, "current").
		AddSort("updated_at", crit.OrderDesc).
		SetPagination(1, 25)

	return z.repo.Find(ctx, criteria)
}

// FindProspects retrieves contacts in a specific industry with prospect status
func (z *ZendeskContacts) FindProspects(ctx context.Context, industry string) ([]Contact, int64, error) {
	criteria := crit.NewCriteria().
		AddFilter("industry", crit.OperatorEqual, industry).
		AddFilter("prospect_status", crit.OperatorEqual, "current").
		AddSort("name", crit.OrderAsc).
		SetPagination(1, 50)

	return z.repo.Find(ctx, criteria)
}

// SearchByName searches for contacts by name (contains)
func (z *ZendeskContacts) SearchByName(ctx context.Context, nameKeyword string) ([]Contact, int64, error) {
	criteria := crit.NewCriteria().
		AddFilter("name", crit.OperatorContains, nameKeyword).
		SetPagination(1, 10)

	return z.repo.Find(ctx, criteria)
}

// FindRecentContacts finds recently created contacts within days
func (z *ZendeskContacts) FindRecentContacts(ctx context.Context, days int) ([]Contact, int64, error) {
	pastDate := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)
	criteria := crit.NewCriteria().
		AddFilter("created_at", crit.OperatorGreaterThanOrEqual, pastDate).
		AddSort("created_at", crit.OrderDesc).
		SetPagination(1, 30)

	return z.repo.Find(ctx, criteria)
}
