package opportunities

import (
	"context"
	"fmt"
	"time"

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

func NewSalesforceOpportunitiesCmd() *cobra.Command {
	var name string
	var metadataMappings map[string]string
	var limit int
	var listAllFields bool
	var whereClause string

	cmd := &cobra.Command{
		Use:   "opportunities",
		Short: "Sync Salesforce opportunities into Ctrlplane",
		Example: heredoc.Doc(`
			# Sync all Salesforce opportunities
			$ ctrlc sync salesforce opportunities \
			  --salesforce-domain="https://mycompany.my.salesforce.com" \
			  --salesforce-consumer-key="your-key" \
			  --salesforce-consumer-secret="your-secret"
			
			# Sync opportunities with a specific filter
			$ ctrlc sync salesforce opportunities --where="Amount > 100000"
			
			# Sync opportunities and list all available fields in logs
			$ ctrlc sync salesforce opportunities --list-all-fields
			
			# Sync opportunities with custom provider name
			$ ctrlc sync salesforce opportunities --provider my-salesforce-opportunities
			
			# Sync with custom metadata mappings
			$ ctrlc sync salesforce opportunities \
			  --metadata="opportunity/id=Id" \
			  --metadata="opportunity/owner-id=OwnerId" \
			  --metadata="opportunity/forecast=ForecastCategory" \
			  --metadata="opportunity/stage-custom=Custom_Stage__c"
			
			# Sync with a limit on number of records
			$ ctrlc sync salesforce opportunities --limit 500
			
			# Combine filters with metadata mappings
			$ ctrlc sync salesforce opportunities \
			  --salesforce-domain="https://mycompany.my.salesforce.com" \
			  --salesforce-consumer-key="your-key" \
			  --salesforce-consumer-secret="your-secret" \
			  --where="StageName = 'Closed Won' AND Amount > 50000" \
			  --metadata="opportunity/revenue=Amount"
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := viper.GetString("salesforce-domain")
			consumerKey := viper.GetString("salesforce-consumer-key")
			consumerSecret := viper.GetString("salesforce-consumer-secret")

			log.Info("Syncing Salesforce opportunities into Ctrlplane", "domain", domain)

			ctx := context.Background()

			sf, err := common.InitSalesforceClient(domain, consumerKey, consumerSecret)
			if err != nil {
				return err
			}

			resources, err := processOpportunities(ctx, sf, metadataMappings, limit, listAllFields, whereClause)
			if err != nil {
				return err
			}

			if name == "" {
				subdomain := common.GetSalesforceSubdomain(domain)
				name = fmt.Sprintf("%s-salesforce-opportunities", subdomain)
			}

			return common.UpsertToCtrlplane(ctx, resources, name)
		},
	}

	cmd.Flags().StringVarP(&name, "provider", "p", "", "Name of the resource provider")
	cmd.Flags().StringToStringVar(&metadataMappings, "metadata", map[string]string{}, "Custom metadata mappings (format: metadata/key=SalesforceField)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of records to sync (0 = no limit)")
	cmd.Flags().BoolVar(&listAllFields, "list-all-fields", false, "List all available Salesforce fields in the logs")
	cmd.Flags().StringVar(&whereClause, "where", "", "SOQL WHERE clause to filter records (e.g., \"Amount > 100000\")")

	return cmd
}

// processOpportunities queries and transforms opportunities
func processOpportunities(ctx context.Context, sf *salesforce.Salesforce, metadataMappings map[string]string, limit int, listAllFields bool, whereClause string) ([]api.CreateResource, error) {
	ctx, span := telemetry.StartSpan(ctx, "salesforce.opportunities.process",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("salesforce.object", "Opportunity"),
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

	var opportunities []map[string]any
	err := common.QuerySalesforceObject(ctx, sf, "Opportunity", limit, listAllFields, &opportunities, additionalFields, whereClause)
	if err != nil {
		log.Error("Failed to query Salesforce opportunities", "error", err)
		telemetry.SetSpanError(span, err)
		return nil, err
	}

	log.Info("Found Salesforce opportunities", "count", len(opportunities))
	telemetry.AddSpanAttribute(span, "salesforce.records_found", len(opportunities))

	resources := []api.CreateResource{}
	for _, opp := range opportunities {
		resource := transformOpportunityToResource(opp, metadataMappings)
		resources = append(resources, resource)
	}

	telemetry.AddSpanAttribute(span, "salesforce.records_processed", len(resources))
	telemetry.SetSpanSuccess(span)

	return resources, nil
}

func formatCloseDate(closeDate any) string {
	if closeDate == nil {
		return ""
	}

	closeDateStr := fmt.Sprintf("%v", closeDate)
	if str, ok := closeDate.(string); ok && str != "" {
		if t, err := time.Parse("2006-01-02", str); err == nil {
			return t.Format(time.RFC3339)
		}
	}
	return closeDateStr
}

func transformOpportunityToResource(opportunity map[string]any, metadataMappings map[string]string) api.CreateResource {
	metadata := map[string]string{}

	common.AddToMetadata(metadata, "opportunity/id", opportunity["Id"])
	common.AddToMetadata(metadata, "opportunity/account-id", opportunity["AccountId"])
	common.AddToMetadata(metadata, "opportunity/stage", opportunity["StageName"])
	common.AddToMetadata(metadata, "opportunity/amount", opportunity["Amount"])
	common.AddToMetadata(metadata, "opportunity/probability", opportunity["Probability"])
	common.AddToMetadata(metadata, "opportunity/name", opportunity["Name"])
	common.AddToMetadata(metadata, "opportunity/type", opportunity["Type"])
	common.AddToMetadata(metadata, "opportunity/owner-id", opportunity["OwnerId"])
	common.AddToMetadata(metadata, "opportunity/is-closed", opportunity["IsClosed"])
	common.AddToMetadata(metadata, "opportunity/is-won", opportunity["IsWon"])
	common.AddToMetadata(metadata, "opportunity/lead-source", opportunity["LeadSource"])
	common.AddToMetadata(metadata, "opportunity/forecast-category", opportunity["ForecastCategory"])
	common.AddToMetadata(metadata, "opportunity/contact-id", opportunity["ContactId"])
	common.AddToMetadata(metadata, "opportunity/campaign-id", opportunity["CampaignId"])
	common.AddToMetadata(metadata, "opportunity/created-date", opportunity["CreatedDate"])
	common.AddToMetadata(metadata, "opportunity/last-modified", opportunity["LastModifiedDate"])

	if closeDate := formatCloseDate(opportunity["CloseDate"]); closeDate != "" {
		metadata["opportunity/close-date"] = closeDate
	}

	for metadataKey, fieldName := range metadataMappings {
		if value, exists := opportunity[fieldName]; exists {
			common.AddToMetadata(metadata, metadataKey, value)
		}
	}

	fiscalQuarter := 0
	fiscalYear := 0
	if period, ok := opportunity["FiscalPeriod"].(string); ok && period != "" {
		// Period format is typically like "2024 Q1"
		if n, err := fmt.Sscanf(period, "%d Q%d", &fiscalYear, &fiscalQuarter); err != nil || n != 2 {
			log.Debug("Failed to parse fiscal period", "period", period, "error", err)
			// Leave fiscalQuarter and fiscalYear as 0
		}
	}

	config := map[string]interface{}{
		"name":        fmt.Sprintf("%v", opportunity["Name"]),
		"amount":      opportunity["Amount"],
		"stage":       fmt.Sprintf("%v", opportunity["StageName"]),
		"id":          fmt.Sprintf("%v", opportunity["Id"]),
		"probability": opportunity["Probability"],
		"isClosed":    opportunity["IsClosed"],
		"isWon":       opportunity["IsWon"],

		"salesforceOpportunity": map[string]interface{}{
			"recordId":         fmt.Sprintf("%v", opportunity["Id"]),
			"accountId":        fmt.Sprintf("%v", opportunity["AccountId"]),
			"ownerId":          fmt.Sprintf("%v", opportunity["OwnerId"]),
			"type":             fmt.Sprintf("%v", opportunity["Type"]),
			"leadSource":       fmt.Sprintf("%v", opportunity["LeadSource"]),
			"closeDate":        formatCloseDate(opportunity["CloseDate"]),
			"forecastCategory": fmt.Sprintf("%v", opportunity["ForecastCategory"]),
			"description":      fmt.Sprintf("%v", opportunity["Description"]),
			"nextStep":         fmt.Sprintf("%v", opportunity["NextStep"]),
			"hasOpenActivity":  opportunity["HasOpenActivity"],
			"createdDate":      fmt.Sprintf("%v", opportunity["CreatedDate"]),
			"lastModifiedDate": fmt.Sprintf("%v", opportunity["LastModifiedDate"]),
			"lastActivityDate": fmt.Sprintf("%v", opportunity["LastActivityDate"]),
			"fiscalQuarter":    fiscalQuarter,
			"fiscalYear":       fiscalYear,
		},
	}

	return api.CreateResource{
		Version:    "ctrlplane.dev/crm/opportunity/v1",
		Kind:       "SalesforceOpportunity",
		Name:       fmt.Sprintf("%v", opportunity["Name"]),
		Identifier: fmt.Sprintf("%v", opportunity["Id"]),
		Config:     config,
		Metadata:   metadata,
	}
}
