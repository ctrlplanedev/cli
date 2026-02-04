package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	contextsConfigKey       = "contexts"
	currentContextConfigKey = "current-context"
)

var contextConfigKeys = []string{
	"url",
	"api-key",
	"workspace",
}

func NewUseContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use-context <name>",
		Short: "Set the current configuration context",
		Long: heredoc.Doc(`
			Set the current configuration context, similar to kubectl config use-context.
			Contexts are defined under the "contexts" key in your config file.
		`),
		Example: heredoc.Doc(`
			# Switch to the "wandb" context
			$ ctrlc config use-context wandb
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			contextName := strings.TrimSpace(args[0])
			if contextName == "" {
				return fmt.Errorf("context name cannot be empty")
			}

			contextValues, err := loadContextValues(contextName)
			if err != nil {
				return err
			}

			missingKeys := requiredContextKeysMissing(contextValues, contextConfigKeys)
			if len(missingKeys) > 0 {
				return fmt.Errorf("context %q is missing required keys: %s", contextName, strings.Join(missingKeys, ", "))
			}

			for _, key := range contextConfigKeys {
				viper.Set(key, contextValues[key])
			}
			viper.Set(currentContextConfigKey, contextName)

			if err := viper.WriteConfig(); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}

			log.Info("Switched context", "context", contextName)
			return nil
		},
	}

	return cmd
}

func loadContextValues(contextName string) (map[string]string, error) {
	contexts := viper.GetStringMap(contextsConfigKey)
	if len(contexts) == 0 {
		return nil, fmt.Errorf("no contexts are defined under %q in the config file", contextsConfigKey)
	}

	contextRaw, ok := contexts[contextName]
	if !ok {
		available := contextNames(contexts)
		if len(available) == 0 {
			return nil, fmt.Errorf("context %q not found", contextName)
		}
		return nil, fmt.Errorf("context %q not found. Available contexts: %s", contextName, strings.Join(available, ", "))
	}

	contextMap, ok := normalizeStringMap(contextRaw)
	if !ok {
		return nil, fmt.Errorf("context %q has an invalid format", contextName)
	}

	values := make(map[string]string, len(contextMap))
	for _, key := range contextConfigKeys {
		rawValue, ok := contextMap[key]
		if !ok {
			continue
		}
		stringValue, ok := rawValue.(string)
		if !ok {
			return nil, fmt.Errorf("context %q key %q must be a string", contextName, key)
		}
		values[key] = strings.TrimSpace(stringValue)
	}

	return values, nil
}

func requiredContextKeysMissing(values map[string]string, keys []string) []string {
	var missing []string
	for _, key := range keys {
		if strings.TrimSpace(values[key]) == "" {
			missing = append(missing, key)
		}
	}
	return missing
}

func contextNames(contexts map[string]interface{}) []string {
	names := make([]string, 0, len(contexts))
	for name := range contexts {
		if strings.TrimSpace(name) == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func normalizeStringMap(input interface{}) (map[string]interface{}, bool) {
	switch typed := input.(type) {
	case map[string]interface{}:
		return typed, true
	case map[interface{}]interface{}:
		normalized := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			keyString, ok := key.(string)
			if !ok {
				return nil, false
			}
			normalized[keyString] = value
		}
		return normalized, true
	default:
		return nil, false
	}
}
