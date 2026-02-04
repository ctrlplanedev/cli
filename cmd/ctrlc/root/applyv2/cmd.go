package applyv2

import (
	"context"
	"fmt"
	"sort"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/internal/api/providers"
	"github.com/ctrlplanedev/cli/internal/api/resolver"
	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewApplyV2Cmd creates a new apply-v2 command
func NewApplyV2Cmd() *cobra.Command {
	var filePatterns []string

	cmd := &cobra.Command{
		Use:   "apply-v2",
		Short: "Apply a YAML configuration file using the new provider framework",
		Long:  `Apply a YAML configuration file to create or update resources in Ctrlplane.`,
		Example: heredoc.Doc(`
			# Apply a single resource file
			$ ctrlc apply-v2 -f system.yaml

			# Apply a multi-document file with systems, deployments, and environments
			$ ctrlc apply-v2 -f config.yaml

			# Apply all YAML files matching a glob pattern
			$ ctrlc apply-v2 -f "**/*.ctrlc.yaml"
		`),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApplyV2(cmd.Context(), filePatterns)
		},
	}

	cmd.Flags().StringArrayVarP(&filePatterns, "file", "f", nil, "Path or glob pattern to YAML files (can be specified multiple times, prefix with ! to exclude)")
	cmd.MarkFlagRequired("file")

	return cmd
}

func runApplyV2(ctx context.Context, filePatterns []string) error {
	files, err := expandGlob(filePatterns)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no files to apply")
	}

	apiURL := viper.GetString("url")
	apiKey := viper.GetString("api-key")
	workspace := viper.GetString("workspace")

	client, err := api.NewAPIKeyClientWithResponses(apiURL, apiKey)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	workspaceID := client.GetWorkspaceID(ctx, workspace)
	if workspaceID == uuid.Nil {
		return fmt.Errorf("workspace not found: %s", workspace)
	}

	resolver := resolver.NewAPIResolver(client, workspaceID)
	applyCtx := NewProviderContext(ctx, workspaceID.String(), client, resolver)

	var specs []providers.TypedSpec
	for _, filePath := range files {
		fileSpecs, err := ParseFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to parse file %s: %w", filePath, err)
		}
		specs = append(specs, fileSpecs...)
	}

	if len(specs) == 0 {
		log.Warn("No resources found in files")
		return nil
	}

	log.Info("Applying resources", "count", len(specs), "files", len(files))

	sortedSpecs := sortSpecsByOrder(specs)
	results := providers.DefaultProviderEngine.BatchApply(applyCtx, sortedSpecs, providers.BatchApplyOptions{})

	printResults(results)

	for _, r := range results {
		if r.Error != nil {
			return fmt.Errorf("one or more resources failed to apply")
		}
	}

	return nil
}

func sortSpecsByOrder(specs []providers.TypedSpec) []providers.TypedSpec {
	type orderedSpec struct {
		spec  providers.TypedSpec
		order int
		index int
	}

	ordered := make([]orderedSpec, 0, len(specs))
	for idx, spec := range specs {
		order := 0
		if provider, ok := providers.DefaultProviderEngine.GetProvider(spec.Type); ok {
			order = provider.Order()
		}
		ordered = append(ordered, orderedSpec{spec: spec, order: order, index: idx})
	}

	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].order == ordered[j].order {
			return ordered[i].index < ordered[j].index
		}
		return ordered[i].order > ordered[j].order
	})

	sorted := make([]providers.TypedSpec, 0, len(ordered))
	for _, item := range ordered {
		sorted = append(sorted, item.spec)
	}

	return sorted
}

func printResults(results []providers.Result) {
	fmt.Println()

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
		} else {
			green.Print("✓ ")
			fmt.Printf("%s/", r.Type)
			cyan.Printf("%s ", r.Name)
			yellow.Printf("%s ", r.Action)
			dim.Printf("(id: %s)\n", r.ID)
		}
	}

	fmt.Println()

	var success, failed int
	for _, r := range results {
		if r.Error != nil {
			failed++
		} else {
			success++
		}
	}

	fmt.Printf("Applied %d resources: ", len(results))
	green.Printf("%d succeeded", success)
	fmt.Print(", ")
	if failed > 0 {
		red.Printf("%d failed\n", failed)
	} else {
		fmt.Printf("%d failed\n", failed)
	}
}
