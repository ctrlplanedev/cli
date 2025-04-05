package steampipe

import (
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

func NewSyncSteampipeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "steampipe <subcommand>",
		Short: "Subcommands for integrating steampipe with Ctrlplane",
		Example: heredoc.Doc(`
			$ ctrlc sync steampipe list                  # Show which resourceGroups are available
			$ ctrlc sync steampipe send <resourceGroup>  # Send to Ctrlplane the resource info for all resourceGroups
		`),
	}

	cmd.AddCommand(NewSyncSteampipeListCmd())
	cmd.AddCommand(NewSyncSteampipeSendCmd())

	return cmd
}

func NewSyncSteampipeListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available resourceGroups",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("Listing all available resourceGroups")

			client := NewSteampipeClient()

			_, err := client.ListResourceGroups()
			if err != nil {
				return err
			}

			// for _, group := range resourceGroups {
			// 	fmt.Println(group)
			// }

			return nil
		},
	}

	return cmd
}

func NewSyncSteampipeSendCmd() *cobra.Command {
	var resourceGroup string

	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send resource info to Ctrlplane",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if resourceGroup == "" {
				return fmt.Errorf("resource-group must be provided")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("Send all available resources")

			client := NewSteampipeClient()

			resourceGroups, err := client.SendResourcesFromGroup(resourceGroup)
			if err != nil {
				return err
			}

			for _, group := range resourceGroups {
				fmt.Println(group)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&resourceGroup, "resource-group", "r", os.Getenv("STEAMPIPE_RESOURCE_GROUP"), "The resource group name")

	cmd.MarkFlagRequired("resource-group")

	return cmd
}
