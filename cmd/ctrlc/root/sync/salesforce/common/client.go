package common

import (
	"fmt"

	"github.com/k-capehart/go-salesforce/v2"
)

func InitSalesforceClient(domain, consumerKey, consumerSecret string) (*salesforce.Salesforce, error) {
	sf, err := salesforce.Init(salesforce.Creds{
		Domain:         domain,
		ConsumerKey:    consumerKey,
		ConsumerSecret: consumerSecret,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Salesforce client: %w", err)
	}
	return sf, nil
}
