package salesforce

import (
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

	cmd.PersistentFlags().String("salesforce-domain", "", "Salesforce domain (e.g., https://my-domain.my.salesforce.com)")
	cmd.PersistentFlags().String("salesforce-consumer-key", "", "Salesforce consumer key")
	cmd.PersistentFlags().String("salesforce-consumer-secret", "", "Salesforce consumer secret")

	viper.BindPFlag("salesforce-domain", cmd.PersistentFlags().Lookup("salesforce-domain"))
	viper.BindPFlag("salesforce-consumer-key", cmd.PersistentFlags().Lookup("salesforce-consumer-key"))
	viper.BindPFlag("salesforce-consumer-secret", cmd.PersistentFlags().Lookup("salesforce-consumer-secret"))

	cmd.MarkPersistentFlagRequired("salesforce-domain")
	cmd.MarkPersistentFlagRequired("salesforce-consumer-key")
	cmd.MarkPersistentFlagRequired("salesforce-consumer-secret")

	cmd.AddCommand(accounts.NewSalesforceAccountsCmd())
	cmd.AddCommand(opportunities.NewSalesforceOpportunitiesCmd())

	return cmd
}
