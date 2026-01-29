# Salesforce Sync

Sync Salesforce CRM data (Accounts, Opportunities) into Ctrlplane as resources.

## Quick Start

```bash
# Set credentials (via environment or flags)
export CTRLC_SALESFORCE_DOMAIN="https://mycompany.my.salesforce.com"
export CTRLC_SALESFORCE_CONSUMER_KEY="your-key"
export CTRLC_SALESFORCE_CONSUMER_SECRET="your-secret"

# Sync all accounts
ctrlc sync salesforce accounts

# Sync opportunities with filters
ctrlc sync salesforce opportunities --where="IsWon = true AND Amount > 50000"

# Map custom fields to metadata
ctrlc sync salesforce accounts \
  --metadata="account/tier=Tier__c" \
  --metadata="account/health=Customer_Health__c"
```

## Authentication

Requires Salesforce OAuth2 credentials from a Connected App with `api` and `refresh_token` scopes.

Credentials can be provided via:
- Environment variables: `CTRLC_SALESFORCE_DOMAIN`, `CTRLC_SALESFORCE_CONSUMER_KEY`, `CTRLC_SALESFORCE_CONSUMER_SECRET`
- Command flags: `--salesforce-domain`, `--salesforce-consumer-key`, `--salesforce-consumer-secret`

## Common Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--provider`, `-p` | Resource provider name | Auto-generated from domain |
| `--metadata` | Map Salesforce fields to metadata | Built-in defaults |
| `--where` | SOQL WHERE clause filter | None |
| `--limit` | Maximum records to sync | 0 (no limit) |
| `--list-all-fields` | Log available Salesforce fields | false |

## Metadata Mappings

Map any Salesforce field (including custom fields) to Ctrlplane metadata:

```bash
# Format: metadata-key=SalesforceFieldName
--metadata="account/tier=Tier__c"
--metadata="opportunity/stage-custom=Custom_Stage__c"
```

- Custom fields typically end with `__c`
- Use `--list-all-fields` to discover available fields
- All metadata values are stored as strings

## Resource Examples

### Account Resource
```json
{
  "version": "ctrlplane.dev/crm/account/v1",
  "kind": "SalesforceAccount",
  "name": "Acme Corporation",
  "identifier": "001XX000003DHPh",
  "config": {
    "name": "Acme Corporation",
    "type": "Customer",
    "salesforceAccount": {
      "recordId": "001XX000003DHPh",
      "ownerId": "005XX000001SvogAAC",
      // ... address, dates, etc.
    }
  },
  "metadata": {
    "account/id": "001XX000003DHPh",
    "account/type": "Customer",
    // Custom fields from --metadata
    "account/tier": "Enterprise"
  }
}
```

### Opportunity Resource
```json
{
  "version": "ctrlplane.dev/crm/opportunity/v1",
  "kind": "SalesforceOpportunity",
  "name": "Enterprise Deal",
  "identifier": "006XX000003DHPh",
  "config": {
    "amount": 250000,
    "stage": "Negotiation",
    "salesforceOpportunity": {
      "recordId": "006XX000003DHPh",
      "accountId": "001XX000003DHPh",
      // ... dates, fiscal info, etc.
    }
  },
  "metadata": {
    "opportunity/amount": "250000",
    "opportunity/stage": "Negotiation"
  }
}
```

## Advanced Usage

### Filtering with SOQL

```bash
# Complex account filters
ctrlc sync salesforce accounts \
  --where="Type = 'Customer' AND AnnualRevenue > 1000000"

# Filter opportunities by custom fields
ctrlc sync salesforce opportunities \
  --where="Custom_Field__c != null AND Stage = 'Closed Won'"
```

### Pagination

- Automatically handles large datasets with ID-based pagination
- Fetches up to 1000 records per API call
- Use `--limit` to restrict total records synced

### Default Provider Names

If no `--provider` is specified, names are auto-generated from your Salesforce subdomain:
- `https://acme.my.salesforce.com` → `acme-salesforce-accounts`

## Implementation Notes

- Uses `map[string]any` for Salesforce's dynamic schema
- Null values are omitted from resources
- Numbers and booleans are preserved in config, converted to strings in metadata
- Dates are formatted to RFC3339 where applicable