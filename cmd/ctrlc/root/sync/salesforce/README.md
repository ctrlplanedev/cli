# Salesforce Sync

This package provides functionality to sync Salesforce CRM data into Ctrlplane as resources.

## Usage

### Prerequisites

You need Salesforce OAuth2 credentials:
- **Domain**: Your Salesforce instance URL (e.g., `https://mycompany.my.salesforce.com`)
- **Consumer Key**: From your Salesforce Connected App
- **Consumer Secret**: From your Salesforce Connected App

### Setting up Salesforce Connected App

1. Go to Setup → Apps → App Manager
2. Click "New Connected App"
3. Fill in the required fields
4. Enable OAuth Settings
5. Add OAuth Scopes:
   - `api` - Access and manage your data
   - `refresh_token` - Perform requests on your behalf at any time
6. Save and note the Consumer Key and Consumer Secret

### Authentication

The Salesforce credentials are configured at the parent `salesforce` command level and apply to all subcommands (accounts, opportunities, etc.).

You can provide credentials via environment variables:

```bash
export SALESFORCE_DOMAIN="https://mycompany.my.salesforce.com"
export SALESFORCE_CONSUMER_KEY="your-consumer-key"
export SALESFORCE_CONSUMER_SECRET="your-consumer-secret"
```

Or via command-line flags:

```bash
ctrlc sync salesforce accounts \
  --salesforce-domain "https://mycompany.my.salesforce.com" \
  --salesforce-consumer-key "your-consumer-key" \
  --salesforce-consumer-secret "your-consumer-secret"
```

### Command-Line Flags

#### Global Salesforce Flags (apply to all subcommands)

| Flag | Description | Default |
|------|-------------|---------|
| `--salesforce-domain` | Salesforce instance URL | `$SALESFORCE_DOMAIN` |
| `--salesforce-consumer-key` | OAuth2 consumer key | `$SALESFORCE_CONSUMER_KEY` |
| `--salesforce-consumer-secret` | OAuth2 consumer secret | `$SALESFORCE_CONSUMER_SECRET` |

#### Subcommand Flags

Both `accounts` and `opportunities` commands support the following flags:

| Flag | Description | Default |
|------|-------------|---------|
| `--provider`, `-p` | Resource provider name | Auto-generated from domain (e.g., `wandb-salesforce-accounts`) |
| `--metadata` | Custom metadata mappings (can be used multiple times) | Built-in defaults |
| `--where` | SOQL WHERE clause to filter records | None (syncs all records) |
| `--limit` | Maximum number of records to sync | 0 (no limit) |
| `--list-all-fields` | Log all available Salesforce fields | false |

### Syncing Accounts

```bash
# Sync all Salesforce accounts
ctrlc sync salesforce accounts

# Sync accounts with a filter (e.g., only accounts with Customer Health populated)
ctrlc sync salesforce accounts --where="Customer_Health__c != null"

# Sync accounts with complex filters
ctrlc sync salesforce accounts --where="Type = 'Customer' AND AnnualRevenue > 1000000"

# Sync accounts and list all available fields in logs
ctrlc sync salesforce accounts --list-all-fields

# Sync with custom provider name
ctrlc sync salesforce accounts --provider my-salesforce-accounts

# Limit the number of records to sync
ctrlc sync salesforce accounts --limit 500

# Combine filters with metadata mappings
ctrlc sync salesforce accounts \
  --where="Industry = 'Technology'" \
  --metadata="account/revenue=AnnualRevenue"
```

#### Custom Field Mappings

You can map any Salesforce field (including custom fields) to Ctrlplane metadata:

```bash
# Map standard and custom fields to metadata
ctrlc sync salesforce accounts \
  --metadata="account/tier=Tier__c" \
  --metadata="account/region=Region__c" \
  --metadata="account/annual-revenue=AnnualRevenue" \
  --metadata="account/health=Customer_Health__c" \
  --metadata="account/contract-value=Contract_Value__c"
```

**Key Points about Metadata Mappings**:
- Format: `metadata-key=SalesforceFieldName`
- The left side is the metadata key in Ctrlplane (e.g., `account/tier`)
- The right side is the exact Salesforce field name (e.g., `Tier__c`)
- Custom fields in Salesforce typically end with `__c`
- All values are stored as strings in metadata
- Use `--list-all-fields` to discover available field names

### Default Provider Naming

If you don't specify a `--provider` name, the system automatically generates one based on your Salesforce domain:
- `https://wandb.my.salesforce.com` → `wandb-salesforce-accounts`
- `https://acme.my.salesforce.com` → `acme-salesforce-accounts`
- `https://mycompany.my.salesforce.com` → `mycompany-salesforce-accounts`

### Syncing Opportunities

```bash
# Sync all Salesforce opportunities
ctrlc sync salesforce opportunities

# Sync only open opportunities
ctrlc sync salesforce opportunities --where="IsClosed = false"

# Sync opportunities with complex filters
ctrlc sync salesforce opportunities --where="Amount > 50000 AND StageName != 'Closed Lost'"

# Sync opportunities and list all available fields in logs
ctrlc sync salesforce opportunities --list-all-fields

# Sync with custom provider name
ctrlc sync salesforce opportunities --provider my-salesforce-opportunities

# Limit the number of records to sync
ctrlc sync salesforce opportunities --limit 500

# Combine filters with metadata mappings
ctrlc sync salesforce opportunities \
  --where="Amount > 100000" \
  --metadata="opportunity/probability=Probability"
```

#### Custom Field Mappings

Just like accounts, you can map any Salesforce opportunity field (including custom fields) to Ctrlplane metadata:

```bash
# Map standard and custom fields to metadata
ctrlc sync salesforce opportunities \
  --metadata="opportunity/type=Type__c" \
  --metadata="opportunity/expected-revenue=ExpectedRevenue" \
  --metadata="opportunity/lead-source=LeadSource" \
  --metadata="opportunity/next-step=NextStep" \
  --metadata="opportunity/use-case=Use_Case__c"
```

## Resource Schema

### Salesforce Account Resource

Resources are created with the following structure:

```json
{
  "version": "ctrlplane.dev/crm/account/v1",
  "kind": "SalesforceAccount",
  "name": "Acme Corporation",
  "identifier": "001XX000003DHPh",
  "config": {
    "name": "Acme Corporation",
    "industry": "Technology",
    "id": "001XX000003DHPh",
    "type": "Customer",
    "phone": "+1-555-0123",
    "website": "https://acme.com",
    "salesforceAccount": {
      "recordId": "001XX000003DHPh",
      "ownerId": "005XX000001SvogAAC",
      "parentId": "",
      "type": "Customer",
      "accountSource": "Web",
      "numberOfEmployees": 5000,
      "description": "Major technology customer",
      "billingAddress": {
        "street": "123 Main St",
        "city": "San Francisco",
        "state": "CA",
        "postalCode": "94105",
        "country": "USA",
        "latitude": 37.7749,
        "longitude": -122.4194
      },
      "shippingAddress": {
        "street": "123 Main St",
        "city": "San Francisco",
        "state": "CA",
        "postalCode": "94105",
        "country": "USA",
        "latitude": 37.7749,
        "longitude": -122.4194
      },
      "createdDate": "2023-01-15T10:30:00Z",
      "lastModifiedDate": "2024-01-20T15:45:00Z",
      "isDeleted": false,
      "photoUrl": "https://..."
    }
  },
  "metadata": {
    "ctrlplane/external-id": "001XX000003DHPh",
    "account/id": "001XX000003DHPh",
    "account/owner-id": "005XX000001SvogAAC",
    "account/industry": "Technology",
    "account/billing-city": "San Francisco",
    "account/billing-state": "CA",
    "account/billing-country": "USA",
    "account/website": "https://acme.com",
    "account/phone": "+1-555-0123",
    "account/type": "Customer",
    "account/source": "Web",
    "account/shipping-city": "San Francisco",
    "account/parent-id": "",
    "account/employees": "5000",
    // Custom fields added via --metadata mappings
    "account/tier": "Enterprise",
    "account/health": "Green"
  }
}
```

### Salesforce Opportunity Resource

```json
{
  "version": "ctrlplane.dev/crm/opportunity/v1", 
  "kind": "SalesforceOpportunity",
  "name": "Acme Corp - Enterprise Deal",
  "identifier": "006XX000003DHPh",
  "config": {
    "name": "Acme Corp - Enterprise Deal",
    "amount": 250000,
    "stage": "Negotiation/Review",
    "id": "006XX000003DHPh",
    "probability": 75,
    "isClosed": false,
    "isWon": false,
    "salesforceOpportunity": {
      "recordId": "006XX000003DHPh",
      "accountId": "001XX000003DHPh",
      "ownerId": "005XX000001SvogAAC",
      "type": "New Business",
      "leadSource": "Partner Referral",
      "closeDate": "2024-12-31T00:00:00Z",
      "forecastCategory": "Commit",
      "description": "Enterprise license upgrade",
      "nextStep": "Legal review",
      "hasOpenActivity": true,
      "createdDate": "2024-01-15T10:30:00Z",
      "lastModifiedDate": "2024-02-20T15:45:00Z",
      "lastActivityDate": "2024-02-19T00:00:00Z",
      "fiscalQuarter": 4,
      "fiscalYear": 2024
    }
  },
  "metadata": {
    "ctrlplane/external-id": "006XX000003DHPh",
    "opportunity/id": "006XX000003DHPh",
    "opportunity/account-id": "001XX000003DHPh",
    "opportunity/owner-id": "005XX000001SvogAAC",
    "opportunity/stage": "Negotiation/Review",
    "opportunity/amount": "250000",
    "opportunity/probability": "75",
    "opportunity/close-date": "2024-12-31",
    "opportunity/type": "New Business",
    "opportunity/lead-source": "Partner Referral",
    "opportunity/is-closed": "false",
    "opportunity/is-won": "false",
    // Custom fields added via --metadata mappings
    "opportunity/use-case": "Platform Migration",
    "opportunity/competition": "Competitor X"
  }
}
```

## Implementation Details

This integration uses the [go-salesforce](https://github.com/k-capehart/go-salesforce) library for OAuth2 authentication and SOQL queries.

### Features

- **Dynamic Field Discovery**: Automatically discovers and fetches all available fields (standard and custom) from Salesforce objects
- **Custom Field Mappings**: Map any Salesforce field to Ctrlplane metadata using `--metadata` flags
- **Flexible Filtering**: Use SOQL WHERE clauses with the `--where` flag to filter records
- **Smart Field Capture**: Automatically captures any custom Salesforce fields (ending in `__c`) not defined in the struct
- **Automatic Pagination**: Handles large datasets efficiently with ID-based pagination
- **Subdomain-based Naming**: Automatically generates provider names from your Salesforce subdomain
- **Required Field Validation**: Uses Cobra's `MarkFlagRequired` for proper validation

### Core Architecture

The sync implementation follows a clean, modular architecture:

1. **Parse Metadata Mappings**: `ParseMetadataMappings` parses the `--metadata` flags once, returning:
   - Field names to include in the SOQL query
   - A lookup map for transforming fields to metadata keys

2. **Query Salesforce**: `QuerySalesforceObject` performs the actual SOQL query with:
   - Dynamic field selection based on struct tags and metadata mappings
   - Automatic pagination handling
   - Optional field listing for discovery

3. **Transform to Resources**: Each object is transformed into a Ctrlplane resource with:
   - Standard metadata mappings
   - Custom field mappings from the `--metadata` flags
   - Proper type conversion (all metadata values are strings)

4. **Upload to Ctrlplane**: `UpsertToCtrlplane` handles the resource upload

### Handling Custom Fields

Salesforce custom fields (typically ending in `__c`) are handled through:

1. **Automatic Capture**: The `UnmarshalJSON` method captures any fields not in the struct into a `CustomFields` map
2. **Metadata Mapping**: Use `--metadata` flags to map these fields to Ctrlplane metadata
3. **Field Discovery**: Use `--list-all-fields` to see all available fields in your Salesforce instance

Example workflow:
```bash
# 1. Discover available fields
ctrlc sync salesforce accounts --list-all-fields --limit 1

# 2. Map the custom fields you need
ctrlc sync salesforce accounts \
  --metadata="account/tier=Customer_Tier__c" \
  --metadata="account/segment=Market_Segment__c" \
  --metadata="account/arr=Annual_Recurring_Revenue__c"
```

### Shared Utilities

The `common/` package provides reusable functions for all Salesforce object syncs:

- **`InitSalesforceClient`**: Sets up OAuth2 authentication
- **`ParseMetadataMappings`**: Parses `--metadata` flags into field lists and lookup maps
- **`QuerySalesforceObject`**: Generic SOQL query with pagination
- **`GetCustomFieldValue`**: Gets any field value from struct (standard or custom)
- **`UnmarshalWithCustomFields`**: Captures unknown fields from Salesforce
- **`GetKnownFieldsFromStruct`**: Extracts field names from struct tags
- **`GetSalesforceSubdomain`**: Extracts subdomain for default provider naming
- **`UpsertToCtrlplane`**: Handles resource upload to Ctrlplane

### Pagination

Records are fetched efficiently using Salesforce best practices:
- ID-based pagination (avoids OFFSET limitations)
- Configurable batch size (default: 2000 records)
- Ordered by ID for consistent results
- Use `--limit` to restrict total records synced

### Adding New Salesforce Objects

To add support for a new Salesforce object (e.g., Leads):

1. Create a new struct with JSON tags matching Salesforce field names
2. Include a `CustomFields map[string]interface{}` field
3. Implement `UnmarshalJSON` to capture custom fields
4. Create a command that:
   - Uses `ParseMetadataMappings` for field mappings
   - Calls `QuerySalesforceObject` for data retrieval
   - Transforms objects to Ctrlplane resources
   - Uses `UpsertToCtrlplane` for upload

The shared utilities handle most of the complexity, making new object support straightforward.