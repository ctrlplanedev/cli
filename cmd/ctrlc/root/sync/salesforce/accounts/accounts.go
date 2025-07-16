package accounts

import (
	"context"
	"reflect"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/salesforce/common"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/k-capehart/go-salesforce/v2"
	"github.com/spf13/cobra"
)

type Account struct {
	ID                      string      `json:"Id"` // this is the globally unique identifier for the account
	IsDeleted               bool        `json:"IsDeleted"`
	MasterRecordId          string      `json:"MasterRecordId"`
	Name                    string      `json:"Name"`
	Type                    string      `json:"Type"`
	ParentId                string      `json:"ParentId"`
	BillingStreet           string      `json:"BillingStreet"`
	BillingCity             string      `json:"BillingCity"`
	BillingState            string      `json:"BillingState"`
	BillingPostalCode       string      `json:"BillingPostalCode"`
	BillingCountry          string      `json:"BillingCountry"`
	BillingLatitude         float64     `json:"BillingLatitude"`
	BillingLongitude        float64     `json:"BillingLongitude"`
	BillingGeocodeAccuracy  string      `json:"BillingGeocodeAccuracy"`
	BillingAddress          interface{} `json:"BillingAddress"`
	ShippingStreet          string      `json:"ShippingStreet"`
	ShippingCity            string      `json:"ShippingCity"`
	ShippingState           string      `json:"ShippingState"`
	ShippingPostalCode      string      `json:"ShippingPostalCode"`
	ShippingCountry         string      `json:"ShippingCountry"`
	ShippingLatitude        float64     `json:"ShippingLatitude"`
	ShippingLongitude       float64     `json:"ShippingLongitude"`
	ShippingGeocodeAccuracy string      `json:"ShippingGeocodeAccuracy"`
	ShippingAddress         interface{} `json:"ShippingAddress"`
	Phone                   string      `json:"Phone"`
	Website                 string      `json:"Website"`
	PhotoUrl                string      `json:"PhotoUrl"`
	Industry                string      `json:"Industry"`
	NumberOfEmployees       int         `json:"NumberOfEmployees"`
	Description             string      `json:"Description"`
	OwnerId                 string      `json:"OwnerId"`
	CreatedDate             string      `json:"CreatedDate"`
	CreatedById             string      `json:"CreatedById"`
	LastModifiedDate        string      `json:"LastModifiedDate"`
	LastModifiedById        string      `json:"LastModifiedById"`
	SystemModstamp          string      `json:"SystemModstamp"`
	LastActivityDate        string      `json:"LastActivityDate"`
	LastViewedDate          string      `json:"LastViewedDate"`
	LastReferencedDate      string      `json:"LastReferencedDate"`
	Jigsaw                  string      `json:"Jigsaw"`
	JigsawCompanyId         string      `json:"JigsawCompanyId"`
	AccountSource           string      `json:"AccountSource"`
	SicDesc                 string      `json:"SicDesc"`
	IsPriorityRecord        bool        `json:"IsPriorityRecord"`

	// CustomFields holds any additional fields not defined in the struct
	// This allows handling of custom Salesforce fields like Tier__c with
	// the --metadata flag.
	CustomFields map[string]interface{} `json:"-"`
}

// GetCustomFields implements the DynamicFieldHolder interface
func (a Account) GetCustomFields() map[string]interface{} {
	return a.CustomFields
}

func (a *Account) UnmarshalJSON(data []byte) error {
	type Alias Account
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(a),
	}

	knownFields := common.GetKnownFieldsFromStruct(reflect.TypeOf(Account{}))

	customFields, err := common.UnmarshalWithCustomFields(data, aux, knownFields)
	if err != nil {
		return err
	}

	a.CustomFields = customFields

	return nil
}

func NewSalesforceAccountsCmd() *cobra.Command {
	var name string
	var domain string
	var consumerKey string
	var consumerSecret string
	var metadataMappings []string
	var limit int
	var listAllFields bool
	var whereClause string

	cmd := &cobra.Command{
		Use:   "accounts",
		Short: "Sync Salesforce accounts into Ctrlplane",
		Example: heredoc.Doc(`
			# Make sure Salesforce credentials are configured via environment variables
			
			# Sync all Salesforce accounts
			$ ctrlc sync salesforce accounts
			
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
			  --where="Type = 'Customer' AND AnnualRevenue > 1000000" \
			  --metadata="account/revenue=AnnualRevenue"
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
	cmd.Flags().StringVar(&whereClause, "where", "", "SOQL WHERE clause to filter records (e.g., \"Customer_Health__c != null\")")

	return cmd
}

// runSync contains the main sync logic
func runSync(name, domain, consumerKey, consumerSecret *string, metadataMappings *[]string, limit *int, listAllFields *bool, whereClause *string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		log.Info("Syncing Salesforce accounts into Ctrlplane", "domain", *domain)

		ctx := context.Background()

		sf, err := common.InitSalesforceClient(*domain, *consumerKey, *consumerSecret)
		if err != nil {
			return err
		}

		resources, err := processAccounts(ctx, sf, *metadataMappings, *limit, *listAllFields, *whereClause)
		if err != nil {
			return err
		}

		if *name == "" {
			*name = "salesforce-accounts"
		}

		return common.UpsertToCtrlplane(ctx, resources, *name)
	}
}

// processAccounts queries and transforms accounts
func processAccounts(ctx context.Context, sf *salesforce.Salesforce, metadataMappings []string, limit int, listAllFields bool, whereClause string) ([]api.CreateResource, error) {

	accounts, err := queryAccounts(ctx, sf, limit, listAllFields, metadataMappings, whereClause)
	if err != nil {
		return nil, err
	}

	log.Info("Found Salesforce accounts", "count", len(accounts))

	resources := []api.CreateResource{}
	for _, account := range accounts {
		resource := transformAccountToResource(account, metadataMappings)
		resources = append(resources, resource)
	}

	return resources, nil
}

func queryAccounts(ctx context.Context, sf *salesforce.Salesforce, limit int, listAllFields bool, metadataMappings []string, whereClause string) ([]Account, error) {
	additionalFields := common.ExtractFieldsFromMetadataMappings(metadataMappings)

	var accounts []Account
	err := common.QuerySalesforceObject(ctx, sf, "Account", limit, listAllFields, &accounts, additionalFields, whereClause)
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

func transformAccountToResource(account Account, metadataMappings []string) api.CreateResource {
	defaultMetadataMappings := map[string]string{
		"ctrlplane/external-id":   "Id",
		"account/id":              "Id",
		"account/owner-id":        "OwnerId",
		"account/industry":        "Industry",
		"account/billing-city":    "BillingCity",
		"account/billing-state":   "BillingState",
		"account/billing-country": "BillingCountry",
		"account/website":         "Website",
		"account/phone":           "Phone",
		"account/type":            "Type",
		"account/source":          "AccountSource",
		"account/shipping-city":   "ShippingCity",
		"account/parent-id":       "ParentId",
		"account/employees":       "NumberOfEmployees",
		"account/region":          "Region__c",
		"account/annual-revenue":  "Annual_Revenue__c",
		"account/tier":            "Tier__c",
		"account/health":          "Customer_Health__c",
	}

	// Parse metadata mappings using common utility
	metadata := common.ParseMappings(account, metadataMappings, defaultMetadataMappings)

	// Build base config with common fields
	config := map[string]interface{}{
		// Common cross-provider fields
		"name":     account.Name,
		"industry": account.Industry,
		"id":       account.ID,
		"type":     account.Type,
		"phone":    account.Phone,
		"website":  account.Website,

		// Salesforce-specific implementation details
		"salesforceAccount": map[string]interface{}{
			"recordId":          account.ID,
			"ownerId":           account.OwnerId,
			"parentId":          account.ParentId,
			"type":              account.Type,
			"accountSource":     account.AccountSource,
			"numberOfEmployees": account.NumberOfEmployees,
			"description":       account.Description,
			"billingAddress": map[string]interface{}{
				"street":     account.BillingStreet,
				"city":       account.BillingCity,
				"state":      account.BillingState,
				"postalCode": account.BillingPostalCode,
				"country":    account.BillingCountry,
				"latitude":   account.BillingLatitude,
				"longitude":  account.BillingLongitude,
			},
			"shippingAddress": map[string]interface{}{
				"street":     account.ShippingStreet,
				"city":       account.ShippingCity,
				"state":      account.ShippingState,
				"postalCode": account.ShippingPostalCode,
				"country":    account.ShippingCountry,
				"latitude":   account.ShippingLatitude,
				"longitude":  account.ShippingLongitude,
			},
			"createdDate":      account.CreatedDate,
			"lastModifiedDate": account.LastModifiedDate,
			"isDeleted":        account.IsDeleted,
			"photoUrl":         account.PhotoUrl,
		},
	}

	return api.CreateResource{
		Version:    "ctrlplane.dev/crm/account/v1",
		Kind:       "SalesforceAccount",
		Name:       account.Name,
		Identifier: account.ID,
		Config:     config,
		Metadata:   metadata,
	}
}
