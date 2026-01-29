package salesforce

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/salesforce/accounts"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/salesforce/opportunities"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewSalesforceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "salesforce",
		Short: "Sync Salesforce resources into Ctrlplane",
		Example: heredoc.Doc(`
			# Sync Salesforce accounts
			$ ctrlc sync salesforce accounts \
			  --salesforce-domain="https://mycompany.my.salesforce.com" \
			  --salesforce-consumer-key="your-key" \
			  --salesforce-consumer-secret="your-secret"
			
			# Sync Salesforce opportunities
			$ ctrlc sync salesforce opportunities \
			  --salesforce-domain="https://mycompany.my.salesforce.com" \
			  --salesforce-consumer-key="your-key" \
			  --salesforce-consumer-secret="your-secret"
		`),
	}

	cmd.PersistentFlags().String("salesforce-domain", "", "Salesforce domain (e.g., https://my-domain.my.salesforce.com) (can also be set via CTRLC_SALESFORCE_DOMAIN env var)")
	cmd.PersistentFlags().String("salesforce-consumer-key", "", "Salesforce consumer key (can also be set via CTRLC_SALESFORCE_CONSUMER_KEY env var)")
	cmd.PersistentFlags().String("salesforce-consumer-secret", "", "Salesforce consumer secret (can also be set via CTRLC_SALESFORCE_CONSUMER_SECRET env var)")

	if err := viper.BindEnv("salesforce-domain", "CTRLC_SALESFORCE_DOMAIN", "SALESFORCE_DOMAIN"); err != nil {
		panic(fmt.Errorf("failed to bind CTRLC_SALESFORCE_DOMAIN env var: %w", err))
	}
	if err := viper.BindEnv("salesforce-consumer-key", "CTRLC_SALESFORCE_CONSUMER_KEY", "SALESFORCE_CONSUMER_KEY"); err != nil {
		panic(fmt.Errorf("failed to bind CTRLC_SALESFORCE_CONSUMER_KEY env var: %w", err))
	}
	if err := viper.BindEnv("salesforce-consumer-secret", "CTRLC_SALESFORCE_CONSUMER_SECRET", "SALESFORCE_CONSUMER_SECRET"); err != nil {
		panic(fmt.Errorf("failed to bind CTRLC_SALESFORCE_CONSUMER_SECRET env var: %w", err))
	}

	if err := viper.BindPFlag("salesforce-domain", cmd.PersistentFlags().Lookup("salesforce-domain")); err != nil {
		panic(fmt.Errorf("failed to bind salesforce-domain flag: %w", err))
	}
	if err := viper.BindPFlag("salesforce-consumer-key", cmd.PersistentFlags().Lookup("salesforce-consumer-key")); err != nil {
		panic(fmt.Errorf("failed to bind salesforce-consumer-key flag: %w", err))
	}
	if err := viper.BindPFlag("salesforce-consumer-secret", cmd.PersistentFlags().Lookup("salesforce-consumer-secret")); err != nil {
		panic(fmt.Errorf("failed to bind salesforce-consumer-secret flag: %w", err))
	}

	cmd.AddCommand(accounts.NewSalesforceAccountsCmd())
	cmd.AddCommand(opportunities.NewSalesforceOpportunitiesCmd())

	return cmd
}
