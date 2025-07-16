package accounts

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/salesforce/common"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/k-capehart/go-salesforce/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	var metadataMappings []string
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
	cmd.Flags().StringArrayVar(&metadataMappings, "metadata", []string{}, "Custom metadata mappings (format: metadata/key=SalesforceField)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of records to sync (0 = no limit)")
	cmd.Flags().BoolVar(&listAllFields, "list-all-fields", false, "List all available Salesforce fields in the logs")
	cmd.Flags().StringVar(&whereClause, "where", "", "SOQL WHERE clause to filter records (e.g., \"Customer_Health__c != null\")")

	return cmd
}

func processAccounts(ctx context.Context, sf *salesforce.Salesforce, metadataMappings []string, limit int, listAllFields bool, whereClause string) ([]api.CreateResource, error) {
	additionalFields, mappingLookup := common.ParseMetadataMappings(metadataMappings)

	var accounts []Account
	err := common.QuerySalesforceObject(ctx, sf, "Account", limit, listAllFields, &accounts, additionalFields, whereClause)
	if err != nil {
		return nil, err
	}

	log.Info("Found Salesforce accounts", "count", len(accounts))

	resources := []api.CreateResource{}
	for _, account := range accounts {
		resource := transformAccountToResource(account, mappingLookup)
		resources = append(resources, resource)
	}

	return resources, nil
}

func transformAccountToResource(account Account, mappingLookup map[string]string) api.CreateResource {
	metadata := map[string]string{
		"ctrlplane/external-id":   account.ID,
		"account/id":              account.ID,
		"account/owner-id":        account.OwnerId,
		"account/industry":        account.Industry,
		"account/billing-city":    account.BillingCity,
		"account/billing-state":   account.BillingState,
		"account/billing-country": account.BillingCountry,
		"account/website":         account.Website,
		"account/phone":           account.Phone,
		"account/type":            account.Type,
		"account/source":          account.AccountSource,
		"account/shipping-city":   account.ShippingCity,
		"account/parent-id":       account.ParentId,
		"account/employees":       strconv.Itoa(account.NumberOfEmployees),
	}

	for fieldName, metadataKey := range mappingLookup {
		if value, found := common.GetCustomFieldValue(account, fieldName); found {
			metadata[metadataKey] = value
		}
	}

	config := map[string]interface{}{
		"name":     account.Name,
		"industry": account.Industry,
		"id":       account.ID,
		"type":     account.Type,
		"phone":    account.Phone,
		"website":  account.Website,

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
