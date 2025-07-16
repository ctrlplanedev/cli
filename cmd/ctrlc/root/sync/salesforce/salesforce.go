package salesforce

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/salesforce/accounts"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/salesforce/opportunities"
	"github.com/spf13/cobra"
)

func NewSalesforceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "salesforce",
		Short: "Sync Salesforce resources into Ctrlplane",
		Example: heredoc.Doc(`
			# Sync all Salesforce objects
			$ ctrlc sync salesforce accounts
			$ ctrlc sync salesforce opportunities
		`),
	}

	cmd.AddCommand(accounts.NewSalesforceAccountsCmd())
	cmd.AddCommand(opportunities.NewSalesforceOpportunitiesCmd())

	return cmd
}
