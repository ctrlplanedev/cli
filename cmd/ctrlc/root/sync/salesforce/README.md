# Salesforce Sync

This package provides functionality to sync Salesforce CRM data into Ctrlplane as resources.

## Usage

### Prerequisites

You need Salesforce OAuth2 credentials:
- **Domain**: Your Salesforce instance URL (e.g., `https://my-domain.my.salesforce.com`)
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

You can provide credentials via environment variables:

```bash
export SALESFORCE_DOMAIN="https://my-domain.my.salesforce.com"
export SALESFORCE_CONSUMER_KEY="your-consumer-key"
export SALESFORCE_CONSUMER_SECRET="your-consumer-secret"
```

Or via command-line flags:

```bash
ctrlc sync salesforce accounts \
  --domain "https://my-domain.my.salesforce.com" \
  --consumer-key "your-consumer-key" \
  --consumer-secret "your-consumer-secret"
```

### Command-Line Flags

Both `accounts` and `opportunities` commands support the following flags:

| Flag | Description | Default |
|------|-------------|---------|
| `--domain` | Salesforce instance URL | `$SALESFORCE_DOMAIN` |
| `--consumer-key` | OAuth2 consumer key | `$SALESFORCE_CONSUMER_KEY` |
| `--consumer-secret` | OAuth2 consumer secret | `$SALESFORCE_CONSUMER_SECRET` |
| `--provider`, `-p` | Resource provider name | `salesforce-accounts` or `salesforce-opportunities` |
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
  --metadata="account/id=Id" \
  --metadata="ctrlplane/external-id=MasterRecordId" \
  --metadata="account/owner-id=OwnerId" \
  --metadata="account/tier=Tier__c" \
  --metadata="account/region=Region__c" \
  --metadata="account/annual-revenue=Annual_Revenue__c" \
  --metadata="account/health=Customer_Health__c"
```

**Note**: 
- Metadata values are always stored as strings, so all field values are automatically converted
- The format is `prefix/key=SalesforceField` for metadata mappings where:
  - `ctrlplane/` prefix is for system fields (e.g., `ctrlplane/external-id`)
  - `account/` prefix is for account-specific metadata
  - Use custom prefixes as needed for your organization
- The sync automatically includes common fields with default mappings that can be overridden
- The `Id` is mapped to `ctrlplane/external-id` by default for accounts
- All fields are fetched from Salesforce, so custom fields are always available for mapping

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
  --metadata="opportunity/id=Id" \
  --metadata="opportunity/account-id=AccountId" \
  --metadata="opportunity/type=Type__c" \
  --metadata="opportunity/expected-revenue=ExpectedRevenue" \
  --metadata="opportunity/lead-source=LeadSource"
```

**Note**: 
- Metadata values are always stored as strings, so all field values are automatically converted
- The format is `prefix/key=SalesforceField` for metadata mappings where:
  - `ctrlplane/` prefix is for system fields (e.g., `ctrlplane/external-id`)
  - `opportunity/` prefix is for opportunity-specific metadata
  - Use custom prefixes as needed for your organization
- The sync automatically includes common fields with default mappings that can be overridden

## Resource Schema

### Salesforce Account Resource

Resources are created with the following structure:

```json
{
  "version": "ctrlplane.dev/crm/account/v1",
  "kind": "SalesforceAccount",
  "name": "Account Name",
  "identifier": "001XX000003DHPh",
  "config": {
    "name": "Account Name",
    "industry": "Technology",
    "id": "001XX000003DHPh",
    "salesforceAccount": {
      "recordId": "001XX000003DHPh",
      "ownerId": "005XX000001SvogAAC",
      "billingCity": "San Francisco",
      "website": "https://example.com"
    }
  },
  "metadata": {
    "salesforce.account.id": "001XX000003DHPh",
    "salesforce.account.owner_id": "005XX000001SvogAAC",
    "salesforce.account.industry": "Technology",
    "salesforce.account.billing_city": "San Francisco",
    "salesforce.account.website": "https://example.com"
  }
}
```

### Salesforce Opportunity Resource

```json
{
  "version": "ctrlplane.dev/crm/opportunity/v1", 
  "kind": "SalesforceOpportunity",
  "name": "Opportunity Name",
  "identifier": "006XX000003DHPh",
  "config": {
    "name": "Opportunity Name",
    "amount": "50000.00",
    "stage": "Qualification",
    "id": "006XX000003DHPh",
    "salesforceOpportunity": {
      "recordId": "006XX000003DHPh",
      "closeDate": "2024-12-31T00:00:00Z",
      "accountId": "001XX000003DHPh",
      "probability": "10"
    }
  },
  "metadata": {
    "salesforce.opportunity.id": "006XX000003DHPh",
    "salesforce.opportunity.account_id": "001XX000003DHPh",
    "salesforce.opportunity.stage": "Qualification",
    "salesforce.opportunity.amount": "50000.00",
    "salesforce.opportunity.probability": "10",
    "salesforce.opportunity.close_date": "2024-12-31"
  }
}
```

## Implementation Details

This integration uses the [go-salesforce](https://github.com/k-capehart/go-salesforce) library for OAuth2 authentication and SOQL queries.

### Features

- **Dynamic Field Discovery**: Automatically discovers and fetches all available fields (standard and custom) from Salesforce objects
- **Custom Field Mappings**: Map any Salesforce field to Ctrlplane metadata using command-line flags
- **Flexible Filtering**: Use SOQL WHERE clauses with the `--where` flag to filter records
- **Flexible Transformations**: Use reflection-based utilities to handle field mappings dynamically
- **Extensible Architecture**: Shared utilities in `common/util.go` make it easy to add support for new Salesforce objects
- **Automatic Pagination**: Fetches all records by default, with support for limiting the number of records
- **Smart Field Capture**: Automatically captures any custom Salesforce fields not defined in the struct
- **Optional Field Listing**: Use `--list-all-fields` to see all available fields in the logs

### Currently Syncs

- **Accounts**: Complete account information including:
  - Basic fields (name, industry, website, phone)
  - Address information (billing and shipping)
  - Hierarchy (parent/child relationships)
  - Custom fields (any field ending in `__c`)
  
- **Opportunities**: Deal information including:
  - Basic fields (name, amount, stage, close date)
  - Relationships (account associations)
  - Probability and forecast data
  - Custom fields (any field ending in `__c`)

### Pagination

By default, the sync will retrieve all records from Salesforce using pagination:
- Records are fetched in batches of 2000 (Salesforce's default)
- Uses ID-based pagination to handle large datasets (avoids Salesforce's OFFSET limitation)
- Use the `--limit` flag to restrict the number of records synced
- Records are ordered by ID for consistent pagination

### Shared Utilities

The `common/util.go` file provides reusable functions for all Salesforce object syncs:
- `QuerySalesforceObject`: Generic function to query any Salesforce object with pagination support
- `UnmarshalWithCustomFields`: Captures any fields from Salesforce that aren't defined in the Go struct
- `GetKnownFieldsFromStruct`: Automatically extracts field names from struct tags
- `ParseMappings`: Handles custom metadata field mappings

These utilities make it easy to add support for new Salesforce objects (Leads, Contacts, etc.) with minimal code duplication.