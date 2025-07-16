package common

import (
	"fmt"
	"os"

	"github.com/k-capehart/go-salesforce/v2"
	"github.com/spf13/cobra"
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

func ValidateFlags(domain, consumerKey, consumerSecret *string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if *domain == "" {
			*domain = os.Getenv("SALESFORCE_DOMAIN")
		}
		if *consumerKey == "" {
			*consumerKey = os.Getenv("SALESFORCE_CONSUMER_KEY")
		}
		if *consumerSecret == "" {
			*consumerSecret = os.Getenv("SALESFORCE_CONSUMER_SECRET")
		}

		if *domain == "" || *consumerKey == "" || *consumerSecret == "" {
			return fmt.Errorf("Salesforce credentials are required. Set SALESFORCE_DOMAIN, SALESFORCE_CONSUMER_KEY, and SALESFORCE_CONSUMER_SECRET environment variables or use flags")
		}

		return nil
	}
}
