# go-crit CLI

A command-line interface for running criteria queries against APIs using the go-crit library.

## Installation

```bash
# Clone the repository
git clone https://github.com/b87/go-crit.git
cd go-crit

# Build the CLI
go build -o crit

# Optionally, install it to your PATH
go install
```

## Configuration

The CLI can be configured in several ways:

1. **Command line flags**:

  ``` bash
    # For Zendesk Sales API - token now belongs to the sales subcommand
    crit zendesk sales --token=your_api_token find contacts
  ```

2. **Environment variables**:

  ``` bash
  # For Zendesk Sales API
  export ZENDESK_SALES_TOKEN=your_api_token
  crit zendesk sales find contacts
  ```

3. **Configuration file**:

  Create a file at `~/.go-crit.yaml` or in the current directory:

  ``` yaml
  output: json
  zendesk:
    base_url: https://api.getbase.com
    sales:
      token: your_api_token
  ```

## Usage

### Global Options

```
--output string      Output format: table, json, yaml (default "table")
--verbose            Enable verbose output for debugging
```

### SQL API

The CLI supports interacting with SQL APIs, including PostgreSQL.

#### PostgreSQL∫

```bash
go run ./main.go sql find --driver postgres --dsn "postgresql://user:password@localhost:5432/database" \
  --table organizations \
  --filter "org_name:eq:Acme Inc." \
  --sort "created_at:desc" \
  --limit 20 \
  --page 1 \
  --fields "id,org_name,created_at" \
  --format json


### Zendesk API

The CLI supports interacting with Zendesk APIs, including the Zendesk Sales CRM.

#### Zendesk Sales CRM

The Zendesk Sales module requires authentication with an API token:

```bash
# Authentication can be provided via flag
crit zendesk sales --token=your_api_token find contacts

# Or via environment variable
export ZENDESK_SALES_TOKEN=your_api_token
crit zendesk sales find contacts
```

#### Find Contacts

Find contacts with the new filter syntax:

```bash
# List all contacts
crit zendesk sales find contacts

# List current customers
crit zendesk sales find contacts --filter "customer_status:eq:current"

# Limit results and sort
crit zendesk sales find contacts --limit=50 --sort-by=name --sort-order=asc

# Filter by industry and creation date
crit zendesk sales find contacts --filter "industry:eq:Technology" --filter "created_at:gte:2023-01-01"

# Search by name
crit zendesk sales find contacts --filter "name:contains:Acme"

# Filter by custom fields and organization type
crit zendesk sales find contacts --filter "is_organization:eq:true" --filter "custom_fields[region]:eq:Europe"

# Using relative dates
crit zendesk sales find contacts --filter "updated_at:gte:7 days ago"
```

## Filter Syntax

The filter syntax follows the format `field:operator:value` and provides various operators:

- `eq`, `=`, `==`: Equal to (exact match)
- `neq`, `!=`, `<>`: Not equal to
- `gt`, `>`: Greater than
- `gte`, `>=`: Greater than or equal to
- `lt`, `<`: Less than
- `lte`, `<=`: Less than or equal to
- `contains`, `like`: Contains substring (case-insensitive)
- `in`: In a list of values (comma-separated)
- `null`, `isnull`: Is null or not set

## Examples

### Find current customers in JSON format

```bash
crit zendesk sales find contacts --filter "customer_status:eq:current" --output=json
```

### Find organizations updated in the last week

```bash
crit zendesk sales find contacts --filter "is_organization:eq:true" --filter "updated_at:gte:7 days ago" --output=yaml
```

## Extending the CLI

The CLI is designed to be extensible. To add support for other APIs:

1. Create new command files in the `cmd` directory
2. Implement the necessary adapters for your API
3. Register your commands with the root command

## Troubleshooting

If you're seeing errors when using the CLI, here are some common issues and solutions:

### 400 Bad Request Errors

If you see `Error querying Zendesk contacts: unexpected status code: 400`, it could be due to:

1. **Invalid API Token**: Make sure your Zendesk API token is valid. You can test it with:

  ```bash
  crit zendesk sales --token=your_token find contacts --limit=1
  ```

2. **Incorrect Query Parameters**: Some filters might not be supported by the Zendesk API. Try with fewer filters or use the `--verbose` flag to see the actual request:

  ```bash
  crit zendesk sales find contacts --verbose
  ```

3. **Rate Limiting**: Zendesk API has rate limits. If you hit them, you'll get an error. Wait a few minutes and try again.

### Authentication Errors

For authentication errors (401), check:

1. **API Token Validity**: Ensure your token is current and hasn't expired.
2. **Token Format**: The token should be provided without any prefix (no "Bearer" text).
3. **Permission Scopes**: Make sure your token has the necessary permissions to access the requested resources.

## Command Reference

### Global Flags

``` bash
--output string     Output format: table, json, yaml (default "table")
--verbose           Enable verbose output for debugging
```

### Zendesk Commands

For more details, see the [Zendesk API Documentation](https://developer.zendesk.com/api-reference/sales-crm/requests/).