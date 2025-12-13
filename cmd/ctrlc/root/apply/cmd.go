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

// NewApplyCmd creates a new apply command
func NewApplyCmd() *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a YAML configuration file to create or update resources",
		Long:  `Apply a YAML configuration file to create or update resources in Ctrlplane.`,
		Example: heredoc.Doc(`
			# Apply a single resource file
			$ ctrlc apply -f system.yaml

			# Apply a multi-document file with systems, deployments, and environments
			$ ctrlc apply -f config.yaml
		`),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(cmd.Context(), filePath)
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to the YAML configuration file (required)")
	cmd.MarkFlagRequired("file")

	return cmd
}

func runApply(ctx context.Context, filePath string) error {
	// Create API client
	apiURL := viper.GetString("url")
	apiKey := viper.GetString("api-key")
	workspace := viper.GetString("workspace")

	client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	workspaceID := client.GetWorkspaceID(ctx, workspace)

	// Parse the YAML file into Document interfaces
	documents, err := ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	if len(documents) == 0 {
		log.Warn("No resources found in file")
		return nil
	}

	log.Info("Applying resources", "count", len(documents), "file", filePath)

	// Create document context
	docCtx := NewDocContext(workspaceID.String(), client)
	docCtx.Context = ctx

	// Sort documents by Order (lower number = processed first)
	// Sort documents by Order (higher number = higher priority = processed first)
	sort.Slice(documents, func(i, j int) bool {
		return documents[i].Order() > documents[j].Order()
	})

	// Apply all documents
	var results []ApplyResult
	for _, doc := range documents {
		result, err := doc.Apply(docCtx)
		if err != nil {
			log.Error("Failed to apply document", "error", err)
		}
		results = append(results, result)
	}

	// Print summary
	printResults(results)

	// Check for errors
	for _, r := range results {
		if r.Error != nil {
			return fmt.Errorf("one or more resources failed to apply")
		}
	}

	return nil
}

func printResults(results []ApplyResult) {
	fmt.Println()

	// Color definitions
	green := color.New(color.FgGreen, color.Bold)
	red := color.New(color.FgRed, color.Bold)
	cyan := color.New(color.FgCyan)
	yellow := color.New(color.FgYellow)
	dim := color.New(color.Faint)

	// Print apply results
	for _, r := range results {
		if r.Error != nil {
			red.Print("✗ ")
			fmt.Printf("%s/%s: ", r.Type, r.Name)
			red.Printf("%v\n", r.Error)
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
	var success, failed int
	for _, r := range results {
		if r.Error != nil {
			failed++
		} else {
			success++
		}
	}

	// Summary with colors
	fmt.Printf("Applied %d resources: ", len(results))
	green.Printf("%d succeeded", success)
	fmt.Print(", ")
	if failed > 0 {
		red.Printf("%d failed\n", failed)
	} else {
		fmt.Printf("%d failed\n", failed)
	}
}
