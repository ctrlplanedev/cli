package opportunities

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/salesforce/common"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/k-capehart/go-salesforce/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

func processOpportunities(ctx context.Context, sf *salesforce.Salesforce, metadataMappings map[string]string, limit int, listAllFields bool, whereClause string) ([]api.CreateResource, error) {
	additionalFields := make([]string, 0, len(metadataMappings))
	for _, fieldName := range metadataMappings {
		additionalFields = append(additionalFields, fieldName)
	}

	var opportunities []Opportunity
	err := common.QuerySalesforceObject(ctx, sf, "Opportunity", limit, listAllFields, &opportunities, additionalFields, whereClause)
	if err != nil {
		return nil, err
	}

	log.Info("Found Salesforce opportunities", "count", len(opportunities))

	resources := []api.CreateResource{}
	for _, opp := range opportunities {
		resource := transformOpportunityToResource(opp, metadataMappings)
		resources = append(resources, resource)
	}

	return resources, nil
}

func transformOpportunityToResource(opportunity Opportunity, metadataMappings map[string]string) api.CreateResource {
	var closeDateFormatted string
	if opportunity.CloseDate != "" {
		if t, err := time.Parse("2006-01-02", opportunity.CloseDate); err == nil {
			closeDateFormatted = t.Format(time.RFC3339)
		} else {
			closeDateFormatted = opportunity.CloseDate
		}
	}

	metadata := map[string]string{
		"opportunity/id":                opportunity.ID,
		"ctrlplane/external-id":         opportunity.ID,
		"opportunity/account-id":        opportunity.AccountID,
		"opportunity/stage":             opportunity.StageName,
		"opportunity/amount":            strconv.FormatFloat(opportunity.Amount, 'f', -1, 64),
		"opportunity/probability":       strconv.FormatFloat(opportunity.Probability, 'f', -1, 64),
		"opportunity/close-date":        closeDateFormatted,
		"opportunity/name":              opportunity.Name,
		"opportunity/type":              opportunity.Type,
		"opportunity/owner-id":          opportunity.OwnerID,
		"opportunity/is-closed":         strconv.FormatBool(opportunity.IsClosed),
		"opportunity/is-won":            strconv.FormatBool(opportunity.IsWon),
		"opportunity/lead-source":       opportunity.LeadSource,
		"opportunity/forecast-category": opportunity.ForecastCategory,
		"opportunity/contact-id":        opportunity.ContactId,
		"opportunity/campaign-id":       opportunity.CampaignID,
		"opportunity/created-date":      opportunity.CreatedDate,
		"opportunity/last-modified":     opportunity.LastModifiedDate,
	}

	for metadataKey, fieldName := range metadataMappings {
		if value, found := common.GetCustomFieldValue(opportunity, fieldName); found {
			metadata[metadataKey] = value
		}
	}

	config := map[string]interface{}{
		"name":   opportunity.Name,
		"amount": strconv.FormatFloat(opportunity.Amount, 'f', -1, 64),
		"stage":  opportunity.StageName,
		"id":     opportunity.ID,

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
