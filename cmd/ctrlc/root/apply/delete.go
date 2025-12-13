package apply

import (
	"context"
	"fmt"
	"sort"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// DeleteResult represents the result of deleting a document
type DeleteResult struct {
	Type   DocumentType
	Name   string
	Action string // "deleted", "not_found"
	ID     string
	Error  error
}

// NewDeleteCmd creates a new delete command
func NewDeleteCmd() *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete resources defined in a YAML configuration file",
		Long:  `Delete resources defined in a YAML configuration file from Ctrlplane.`,
		Example: heredoc.Doc(`
			# Delete resources defined in a file
			$ ctrlc delete -f system.yaml

			# Delete resources from a multi-document file
			$ ctrlc delete -f config.yaml
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd.Context(), filePath)
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to the YAML configuration file (required)")
	cmd.MarkFlagRequired("file")

	return cmd
}

func runDelete(ctx context.Context, filePath string) error {
	// Create API client
	apiURL := viper.GetString("url")
	apiKey := viper.GetString("api-key")
	workspace := viper.GetString("workspace")

	client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	workspaceID := client.GetWorkspaceID(ctx, workspace)
	if workspaceID.String() == "00000000-0000-0000-0000-000000000000" {
		return fmt.Errorf("invalid workspace: %s", workspace)
	}

	// Parse the YAML file into Document interfaces
	documents, err := ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	if len(documents) == 0 {
		log.Warn("No resources found in file")
		return nil
	}

	log.Info("Deleting resources", "count", len(documents), "file", filePath)

	// Create document context
	docCtx := NewDocContext(workspaceID.String(), client)
	docCtx.Context = ctx

	// Sort documents by reverse Order (higher number = processed first for deletion)
	// This ensures dependencies are deleted in reverse order
	sort.Slice(documents, func(i, j int) bool {
		return documents[i].Order() > documents[j].Order()
	})

	// Delete all documents
	var results []DeleteResult
	for _, doc := range documents {
		result, err := doc.Delete(docCtx)
		if err != nil {
			log.Error("Failed to delete document", "error", err)
		}
		results = append(results, result)
	}

	// Print summary
	printDeleteResults(results)

	// Check for errors
	for _, r := range results {
		if r.Error != nil {
			return fmt.Errorf("one or more resources failed to delete")
		}
	}

	return nil
}

func printDeleteResults(results []DeleteResult) {
	fmt.Println()

	// Color definitions
	green := color.New(color.FgGreen, color.Bold)
	red := color.New(color.FgRed, color.Bold)
	cyan := color.New(color.FgCyan)
	yellow := color.New(color.FgYellow)
	dim := color.New(color.Faint)

	for _, r := range results {
		if r.Error != nil {
			red.Print("✗ ")
			fmt.Printf("%s/%s: ", r.Type, r.Name)
			red.Printf("%v\n", r.Error)
		} else if r.Action == "not_found" {
			yellow.Print("- ")
			fmt.Printf("%s/", r.Type)
			cyan.Printf("%s ", r.Name)
			dim.Println("not found (skipped)")
		} else {
			green.Print("✓ ")
			fmt.Printf("%s/", r.Type)
			cyan.Printf("%s ", r.Name)
			yellow.Printf("%s ", r.Action)
			dim.Printf("(id: %s)\n", r.ID)
		}
	}

	fmt.Println()

	// Count successes and failures
	var success, failed, notFound int
	for _, r := range results {
		if r.Error != nil {
			failed++
		} else if r.Action == "not_found" {
			notFound++
		} else {
			success++
		}
	}

	fmt.Printf("Deleted %d resources: ", len(results))
	green.Printf("%d succeeded", success)
	fmt.Print(", ")
	yellow.Printf("%d not found", notFound)
	fmt.Print(", ")
	if failed > 0 {
		red.Printf("%d failed\n", failed)
	} else {
		fmt.Printf("%d failed\n", failed)
	}
}
