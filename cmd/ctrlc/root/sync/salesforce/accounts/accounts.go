package accounts

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/salesforce/common"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/telemetry"
	"github.com/k-capehart/go-salesforce/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func NewSalesforceAccountsCmd() *cobra.Command {
	var name string
	var metadataMappings map[string]string
	var limit int
	var listAllFields bool
	var whereClause string

	cmd := &cobra.Command{
		Use:   "accounts",
		Short: "Sync Salesforce accounts into Ctrlplane",
		Example: heredoc.Doc(`
			# Sync all Salesforce accounts
			$ ctrlc sync salesforce accounts \
			  --salesforce-domain="https://mycompany.my.salesforce.com" \
			  --salesforce-consumer-key="your-key" \
			  --salesforce-consumer-secret="your-secret"
			
			# Sync accounts with a specific filter
			$ ctrlc sync salesforce accounts --where="Customer_Health__c != null"
			
			# Sync accounts and list all available fields in logs
			$ ctrlc sync salesforce accounts --list-all-fields
			
			# Sync accounts with custom provider name
			$ ctrlc sync salesforce accounts --provider my-salesforce
			
			# Sync with custom metadata mappings
			$ ctrlc sync salesforce accounts \
			  --metadata="account/id=Id" \
			  --metadata="account/owner-id=OwnerId" \
			  --metadata="account/tier=Tier__c" \
			  --metadata="account/region=Region__c" \
			  --metadata="account/annual-revenue=Annual_Revenue__c" \
			  --metadata="account/health=Customer_Health__c"
			
			# Sync with a limit on number of records
			$ ctrlc sync salesforce accounts --limit 500
			
			# Combine filters with metadata mappings
			$ ctrlc sync salesforce accounts \
			  --salesforce-domain="https://mycompany.my.salesforce.com" \
			  --salesforce-consumer-key="your-key" \
			  --salesforce-consumer-secret="your-secret" \
			  --where="Type = 'Customer' AND AnnualRevenue > 1000000" \
			  --metadata="account/revenue=AnnualRevenue"
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := viper.GetString("salesforce-domain")
			consumerKey := viper.GetString("salesforce-consumer-key")
			consumerSecret := viper.GetString("salesforce-consumer-secret")

			log.Info("Syncing Salesforce accounts into Ctrlplane", "domain", domain)

			ctx := context.Background()

			sf, err := common.InitSalesforceClient(domain, consumerKey, consumerSecret)
			if err != nil {
				return err
			}

			resources, err := processAccounts(ctx, sf, metadataMappings, limit, listAllFields, whereClause)
			if err != nil {
				return err
			}

			if name == "" {
				subdomain := common.GetSalesforceSubdomain(domain)
				name = fmt.Sprintf("%s-salesforce-accounts", subdomain)
			}

			return common.UpsertToCtrlplane(ctx, resources, name)
		},
	}

	cmd.Flags().StringVarP(&name, "provider", "p", "", "Name of the resource provider")
	cmd.Flags().StringToStringVar(&metadataMappings, "metadata", map[string]string{}, "Custom metadata mappings (format: metadata/key=SalesforceField)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of records to sync (0 = no limit)")
	cmd.Flags().BoolVar(&listAllFields, "list-all-fields", false, "List all available Salesforce fields in the logs")
	cmd.Flags().StringVar(&whereClause, "where", "", "SOQL WHERE clause to filter records (e.g., \"Customer_Health__c != null\")")

	return cmd
}

func processAccounts(ctx context.Context, sf *salesforce.Salesforce, metadataMappings map[string]string, limit int, listAllFields bool, whereClause string) ([]api.CreateResource, error) {
	ctx, span := telemetry.StartSpan(ctx, "salesforce.accounts.process",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("salesforce.object", "Account"),
			attribute.Int("salesforce.limit", limit),
			attribute.Bool("salesforce.list_all_fields", listAllFields),
		),
	)
	defer span.End()

	if whereClause != "" {
		telemetry.AddSpanAttribute(span, "salesforce.where_clause", whereClause)
	}

	additionalFields := make([]string, 0, len(metadataMappings))
	for _, fieldName := range metadataMappings {
		additionalFields = append(additionalFields, fieldName)
	}

	var accounts []map[string]any
	err := common.QuerySalesforceObject(ctx, sf, "Account", limit, listAllFields, &accounts, additionalFields, whereClause)
	if err != nil {
		log.Error("Failed to query Salesforce accounts", "error", err)
		telemetry.SetSpanError(span, err)
		return nil, err
	}

	log.Info("Found Salesforce accounts", "count", len(accounts))
	telemetry.AddSpanAttribute(span, "salesforce.records_found", len(accounts))

	resources := []api.CreateResource{}
	for _, account := range accounts {
		resource := transformAccountToResource(account, metadataMappings)
		resources = append(resources, resource)
	}

	telemetry.AddSpanAttribute(span, "salesforce.records_processed", len(resources))
	telemetry.SetSpanSuccess(span)

	return resources, nil
}

func transformAccountToResource(account map[string]any, metadataMappings map[string]string) api.CreateResource {
	metadata := map[string]string{}
	common.AddToMetadata(metadata, "account/id", account["Id"])
	common.AddToMetadata(metadata, "account/owner-id", account["OwnerId"])
	common.AddToMetadata(metadata, "account/industry", account["Industry"])
	common.AddToMetadata(metadata, "account/billing-city", account["BillingCity"])
	common.AddToMetadata(metadata, "account/billing-state", account["BillingState"])
	common.AddToMetadata(metadata, "account/billing-country", account["BillingCountry"])
	common.AddToMetadata(metadata, "account/website", account["Website"])
	common.AddToMetadata(metadata, "account/phone", account["Phone"])
	common.AddToMetadata(metadata, "account/type", account["Type"])
	common.AddToMetadata(metadata, "account/source", account["AccountSource"])
	common.AddToMetadata(metadata, "account/shipping-city", account["ShippingCity"])
	common.AddToMetadata(metadata, "account/parent-id", account["ParentId"])
	common.AddToMetadata(metadata, "account/employees", account["NumberOfEmployees"])

	for metadataKey, fieldName := range metadataMappings {
		if value, exists := account[fieldName]; exists {
			common.AddToMetadata(metadata, metadataKey, value)
		}
	}

	config := map[string]interface{}{
		"name":     fmt.Sprintf("%v", account["Name"]),
		"industry": fmt.Sprintf("%v", account["Industry"]),
		"id":       fmt.Sprintf("%v", account["Id"]),
		"type":     fmt.Sprintf("%v", account["Type"]),
		"phone":    fmt.Sprintf("%v", account["Phone"]),
		"website":  fmt.Sprintf("%v", account["Website"]),

		"salesforceAccount": map[string]interface{}{
			"recordId":          fmt.Sprintf("%v", account["Id"]),
			"ownerId":           fmt.Sprintf("%v", account["OwnerId"]),
			"parentId":          fmt.Sprintf("%v", account["ParentId"]),
			"type":              fmt.Sprintf("%v", account["Type"]),
			"accountSource":     fmt.Sprintf("%v", account["AccountSource"]),
			"numberOfEmployees": account["NumberOfEmployees"],
			"description":       fmt.Sprintf("%v", account["Description"]),
			"billingAddress": map[string]interface{}{
				"street":     fmt.Sprintf("%v", account["BillingStreet"]),
				"city":       fmt.Sprintf("%v", account["BillingCity"]),
				"state":      fmt.Sprintf("%v", account["BillingState"]),
				"postalCode": fmt.Sprintf("%v", account["BillingPostalCode"]),
				"country":    fmt.Sprintf("%v", account["BillingCountry"]),
				"latitude":   account["BillingLatitude"],
				"longitude":  account["BillingLongitude"],
			},
			"shippingAddress": map[string]interface{}{
				"street":     fmt.Sprintf("%v", account["ShippingStreet"]),
				"city":       fmt.Sprintf("%v", account["ShippingCity"]),
				"state":      fmt.Sprintf("%v", account["ShippingState"]),
				"postalCode": fmt.Sprintf("%v", account["ShippingPostalCode"]),
				"country":    fmt.Sprintf("%v", account["ShippingCountry"]),
				"latitude":   account["ShippingLatitude"],
				"longitude":  account["ShippingLongitude"],
			},
			"createdDate":      fmt.Sprintf("%v", account["CreatedDate"]),
			"lastModifiedDate": fmt.Sprintf("%v", account["LastModifiedDate"]),
			"isDeleted":        account["IsDeleted"],
			"photoUrl":         fmt.Sprintf("%v", account["PhotoUrl"]),
		},
	}

	return api.CreateResource{
		Version:    "ctrlplane.dev/crm/account/v1",
		Kind:       "SalesforceAccount",
		Name:       fmt.Sprintf("%v", account["Name"]),
		Identifier: fmt.Sprintf("%v", account["Id"]),
		Config:     config,
		Metadata:   metadata,
	}
}
