package apply

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewApplyCmd creates a new apply command
func NewApplyCmd() *cobra.Command {
	var filePatterns []string

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a YAML configuration file to create or update resources",
		Long:  `Apply a YAML configuration file to create or update resources in Ctrlplane.`,
		Example: heredoc.Doc(`
			# Apply a single resource file
			$ ctrlc apply -f system.yaml

			# Apply a multi-document file with systems, deployments, and environments
			$ ctrlc apply -f config.yaml

			# Apply all YAML files matching a glob pattern
			$ ctrlc apply -f "**/*.ctrlc.yaml"

			# Apply multiple patterns
			$ ctrlc apply -f infra/*.yaml -f apps/*.yaml

			# Exclude test files using ! prefix (git-style: last match wins)
			$ ctrlc apply -f "**/*.yaml" -f "!**/test*.yaml"

			# Exclude multiple patterns
			$ ctrlc apply -f "**/*.yaml" -f "!**/test*.yaml" -f "!**/staging/**"

			# Re-include a previously excluded file (last pattern wins)
			$ ctrlc apply -f "**/*.yaml" -f "!**/test*.yaml" -f "**/important-test.yaml"
		`),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(cmd.Context(), filePatterns)
		},
	}

	cmd.Flags().StringArrayVarP(&filePatterns, "file", "f", nil, "Path or glob pattern to YAML files (can be specified multiple times, prefix with ! to exclude)")
	cmd.MarkFlagRequired("file")

	return cmd
}

// expandGlob expands glob patterns to file paths, supporting ** for recursive matching
// It follows git-style pattern matching where later patterns override earlier ones
// and ! prefix negates (excludes) a pattern
func expandGlob(patterns []string) ([]string, error) {
	seen := make(map[string]bool)
	var files []string

	// Parse patterns into rules - ! prefix means exclude
	type patternRule struct {
		pattern string
		include bool // true = include, false = exclude
	}

	var rules []patternRule
	for _, p := range patterns {
		if strings.HasPrefix(p, "!") {
			rules = append(rules, patternRule{strings.TrimPrefix(p, "!"), false})
		} else {
			rules = append(rules, patternRule{p, true})
		}
	}

	// First, collect all potential files from include patterns
	candidateFiles := make(map[string]bool)
	for _, rule := range rules {
		if rule.include {
			matches, err := doublestar.FilepathGlob(rule.pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid glob pattern '%s': %w", rule.pattern, err)
			}
			for _, match := range matches {
				info, err := os.Stat(match)
				if err != nil || info.IsDir() {
					continue
				}
				candidateFiles[match] = true
			}
		}
	}

	// For each candidate file, evaluate all rules in order - last match wins
	for filePath := range candidateFiles {
		included := false
		for _, rule := range rules {
			matched, err := doublestar.PathMatch(rule.pattern, filePath)
			if err != nil {
				return nil, fmt.Errorf("invalid pattern '%s': %w", rule.pattern, err)
			}
			if matched {
				included = rule.include // last matching rule wins
			}
		}
		if included && !seen[filePath] {
			seen[filePath] = true
			files = append(files, filePath)
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files matched patterns")
	}

	return files, nil
}

func runApply(ctx context.Context, filePatterns []string) error {
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

	var documents []Document
	for _, filePath := range files {
		docs, err := ParseFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to parse file %s: %w", filePath, err)
		}
		documents = append(documents, docs...)
	}

	if len(documents) == 0 {
		log.Warn("No resources found in files")
		return nil
	}

	log.Info("Applying resources", "count", len(documents), "files", len(files))

	docCtx := NewDocContext(workspaceID.String(), client)
	docCtx.Context = ctx

	var resourceDocs []*ResourceDocument
	var otherDocs []Document
	for _, doc := range documents {
		if rd, ok := doc.(*ResourceDocument); ok {
			resourceDocs = append(resourceDocs, rd)
		} else {
			otherDocs = append(otherDocs, doc)
		}
	}

	sort.Slice(otherDocs, func(i, j int) bool {
		return otherDocs[i].Order() > otherDocs[j].Order()
	})

	var results []ApplyResult
	for _, doc := range otherDocs {
		result, err := doc.Apply(docCtx)
		if err != nil {
			log.Error("Failed to apply document", "error", err)
		}
		results = append(results, result)
	}

	if len(resourceDocs) > 0 {
		resourceResults, err := applyResourcesBatch(docCtx, resourceDocs)
		if err != nil {
			log.Error("Failed to apply resources batch", "error", err)
		}
		results = append(results, resourceResults...)
	}

	printResults(results)

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
