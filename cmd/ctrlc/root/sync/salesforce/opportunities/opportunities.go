package opportunities

import (
	"context"
	"reflect"
	"strconv"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/salesforce/common"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/k-capehart/go-salesforce/v2"
	"github.com/spf13/cobra"
)

type Opportunity struct {
	ID                             string  `json:"Id"`
	Name                           string  `json:"Name"`
	Amount                         float64 `json:"Amount"`
	StageName                      string  `json:"StageName"`
	CloseDate                      string  `json:"CloseDate"` // Salesforce returns dates as strings
	AccountID                      string  `json:"AccountId"`
	Probability                    float64 `json:"Probability"`
	IsDeleted                      bool    `json:"IsDeleted"`
	Description                    string  `json:"Description"`
	Type                           string  `json:"Type"`
	NextStep                       string  `json:"NextStep"`
	LeadSource                     string  `json:"LeadSource"`
	IsClosed                       bool    `json:"IsClosed"`
	IsWon                          bool    `json:"IsWon"`
	ForecastCategory               string  `json:"ForecastCategory"`
	ForecastCategoryName           string  `json:"ForecastCategoryName"`
	CampaignID                     string  `json:"CampaignId"`
	HasOpportunityLineItem         bool    `json:"HasOpportunityLineItem"`
	Pricebook2ID                   string  `json:"Pricebook2Id"`
	OwnerID                        string  `json:"OwnerId"`
	Territory2ID                   string  `json:"Territory2Id"`
	IsExcludedFromTerritory2Filter bool    `json:"IsExcludedFromTerritory2Filter"`
	CreatedDate                    string  `json:"CreatedDate"`
	CreatedById                    string  `json:"CreatedById"`
	LastModifiedDate               string  `json:"LastModifiedDate"`
	LastModifiedById               string  `json:"LastModifiedById"`
	SystemModstamp                 string  `json:"SystemModstamp"`
	LastActivityDate               string  `json:"LastActivityDate"`
	PushCount                      int     `json:"PushCount"`
	LastStageChangeDate            string  `json:"LastStageChangeDate"`
	ContactId                      string  `json:"ContactId"`
	LastViewedDate                 string  `json:"LastViewedDate"`
	LastReferencedDate             string  `json:"LastReferencedDate"`
	SyncedQuoteId                  string  `json:"SyncedQuoteId"`
	ContractId                     string  `json:"ContractId"`
	HasOpenActivity                bool    `json:"HasOpenActivity"`
	HasOverdueTask                 bool    `json:"HasOverdueTask"`
	LastAmountChangedHistoryId     string  `json:"LastAmountChangedHistoryId"`
	LastCloseDateChangedHistoryId  string  `json:"LastCloseDateChangedHistoryId"`

	// CustomFields holds any additional fields not defined in the struct
	CustomFields map[string]interface{} `json:"-"`
}

// GetCustomFields implements the DynamicFieldHolder interface
func (o Opportunity) GetCustomFields() map[string]interface{} {
	return o.CustomFields
}

// UnmarshalJSON implements custom unmarshalling to capture unknown fields
func (o *Opportunity) UnmarshalJSON(data []byte) error {
	type Alias Opportunity
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(o),
	}

	knownFields := common.GetKnownFieldsFromStruct(reflect.TypeOf(Opportunity{}))

	customFields, err := common.UnmarshalWithCustomFields(data, aux, knownFields)
	if err != nil {
		return err
	}

	o.CustomFields = customFields

	return nil
}

func NewSalesforceOpportunitiesCmd() *cobra.Command {
	var name string
	var domain string
	var consumerKey string
	var consumerSecret string
	var metadataMappings []string
	var limit int
	var listAllFields bool
	var whereClause string

	cmd := &cobra.Command{
		Use:   "opportunities",
		Short: "Sync Salesforce opportunities into Ctrlplane",
		Example: heredoc.Doc(`
			# Sync all Salesforce opportunities
			$ ctrlc sync salesforce opportunities
			
			# Sync opportunities with a specific filter
			$ ctrlc sync salesforce opportunities --where="IsWon = true"
			
			# Sync opportunities with custom provider name
			$ ctrlc sync salesforce opportunities --provider my-salesforce-opportunities
			
			# Sync with custom metadata mappings
			$ ctrlc sync salesforce opportunities \
			  --metadata="opportunity/stage=StageName" \
			  --metadata="opportunity/amount=Amount" \
			  --metadata="opportunity/close-date=CloseDate"
			
			# Sync with a limit on number of records
			$ ctrlc sync salesforce opportunities --limit 100
			
			# Combine filters with metadata mappings
			$ ctrlc sync salesforce opportunities \
			  --where="Amount > 50000 AND StageName != 'Closed Lost'" \
			  --metadata="opportunity/probability=Probability"
		`),
		PreRunE: common.ValidateFlags(&domain, &consumerKey, &consumerSecret),
		RunE:    runSync(&name, &domain, &consumerKey, &consumerSecret, &metadataMappings, &limit, &listAllFields, &whereClause),
	}

	// Add command flags
	cmd.Flags().StringVarP(&name, "provider", "p", "", "Name of the resource provider")
	cmd.Flags().StringVar(&domain, "domain", "", "Salesforce domain (e.g., https://my-domain.my.salesforce.com)")
	cmd.Flags().StringVar(&consumerKey, "consumer-key", "", "Salesforce consumer key")
	cmd.Flags().StringVar(&consumerSecret, "consumer-secret", "", "Salesforce consumer secret")
	cmd.Flags().StringArrayVar(&metadataMappings, "metadata", []string{}, "Custom metadata mappings (format: metadata/key=SalesforceField)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of records to sync (0 = no limit)")
	cmd.Flags().BoolVar(&listAllFields, "list-all-fields", false, "List all available Salesforce fields in the logs")
	cmd.Flags().StringVar(&whereClause, "where", "", "SOQL WHERE clause to filter records (e.g., \"IsClosed = false\")")

	return cmd
}

// runSync contains the main sync logic
func runSync(name, domain, consumerKey, consumerSecret *string, metadataMappings *[]string, limit *int, listAllFields *bool, whereClause *string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		log.Info("Syncing Salesforce opportunities into Ctrlplane", "domain", *domain)

		ctx := context.Background()

		sf, err := common.InitSalesforceClient(*domain, *consumerKey, *consumerSecret)
		if err != nil {
			return err
		}

		resources, err := processOpportunities(ctx, sf, *metadataMappings, *limit, *listAllFields, *whereClause)
		if err != nil {
			return err
		}

		if *name == "" {
			*name = "salesforce-opportunities"
		}

		return common.UpsertToCtrlplane(ctx, resources, *name)
	}
}

// processOpportunities queries and transforms opportunities
func processOpportunities(ctx context.Context, sf *salesforce.Salesforce, metadataMappings []string, limit int, listAllFields bool, whereClause string) ([]api.CreateResource, error) {
	opportunities, err := queryOpportunities(ctx, sf, limit, listAllFields, metadataMappings, whereClause)
	if err != nil {
		return nil, err
	}

	log.Info("Found Salesforce opportunities", "count", len(opportunities))

	// Transform to ctrlplane resources
	resources := []api.CreateResource{}
	for _, opp := range opportunities {
		resource := transformOpportunityToResource(opp, metadataMappings)
		resources = append(resources, resource)
	}

	return resources, nil
}

func queryOpportunities(ctx context.Context, sf *salesforce.Salesforce, limit int, listAllFields bool, metadataMappings []string, whereClause string) ([]Opportunity, error) {
	// Extract Salesforce field names from metadata mappings
	additionalFields := common.ExtractFieldsFromMetadataMappings(metadataMappings)

	var opportunities []Opportunity
	err := common.QuerySalesforceObject(ctx, sf, "Opportunity", limit, listAllFields, &opportunities, additionalFields, whereClause)
	if err != nil {
		return nil, err
	}
	return opportunities, nil
}

func transformOpportunityToResource(opportunity Opportunity, metadataMappings []string) api.CreateResource {
	// Define default metadata mappings for opportunities
	defaultMetadataMappings := map[string]string{
		"opportunity/id":                "Id",
		"ctrlplane/external-id":         "Id",
		"opportunity/account-id":        "AccountId",
		"opportunity/stage":             "StageName",
		"opportunity/amount":            "Amount",
		"opportunity/probability":       "Probability",
		"opportunity/close-date":        "CloseDate",
		"opportunity/name":              "Name",
		"opportunity/type":              "Type",
		"opportunity/owner-id":          "OwnerId",
		"opportunity/is-closed":         "IsClosed",
		"opportunity/is-won":            "IsWon",
		"opportunity/lead-source":       "LeadSource",
		"opportunity/forecast-category": "ForecastCategory",
		"opportunity/contact-id":        "ContactId",
		"opportunity/campaign-id":       "CampaignId",
		"opportunity/created-date":      "CreatedDate",
		"opportunity/last-modified":     "LastModifiedDate",
	}

	// Parse metadata mappings using common utility
	metadata := common.ParseMappings(opportunity, metadataMappings, defaultMetadataMappings)

	var closeDateFormatted string
	if opportunity.CloseDate != "" {
		if t, err := time.Parse("2006-01-02", opportunity.CloseDate); err == nil {
			closeDateFormatted = t.Format(time.RFC3339)
		} else {
			closeDateFormatted = opportunity.CloseDate
		}
	}

	// Build base config with common fields
	config := map[string]interface{}{
		// Common cross-provider fields
		"name":   opportunity.Name,
		"amount": strconv.FormatFloat(opportunity.Amount, 'f', -1, 64),
		"stage":  opportunity.StageName,
		"id":     opportunity.ID,

		// Salesforce-specific implementation details
		"salesforceOpportunity": map[string]interface{}{
			"recordId":            opportunity.ID,
			"closeDate":           closeDateFormatted,
			"accountId":           opportunity.AccountID,
			"probability":         strconv.FormatFloat(opportunity.Probability, 'f', -1, 64),
			"type":                opportunity.Type,
			"description":         opportunity.Description,
			"nextStep":            opportunity.NextStep,
			"leadSource":          opportunity.LeadSource,
			"isClosed":            opportunity.IsClosed,
			"isWon":               opportunity.IsWon,
			"forecastCategory":    opportunity.ForecastCategory,
			"ownerId":             opportunity.OwnerID,
			"contactId":           opportunity.ContactId,
			"campaignId":          opportunity.CampaignID,
			"hasLineItems":        opportunity.HasOpportunityLineItem,
			"createdDate":         opportunity.CreatedDate,
			"lastModifiedDate":    opportunity.LastModifiedDate,
			"pushCount":           opportunity.PushCount,
			"lastStageChangeDate": opportunity.LastStageChangeDate,
		},
	}

	return api.CreateResource{
		Version:    "ctrlplane.dev/crm/opportunity/v1",
		Kind:       "SalesforceOpportunity",
		Name:       opportunity.Name,
		Identifier: opportunity.ID,
		Config:     config,
		Metadata:   metadata,
	}
}
