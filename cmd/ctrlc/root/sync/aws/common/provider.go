package common

import (
	"context"
	"fmt"
	"github.com/charmbracelet/log"
	"strings"
)

// EnsureProviderDetails generates a provider name and region string based on the provided parameters.
// The
func EnsureProviderDetails(
	ctx context.Context, prefix string, regions []string, name *string,
) {
	providerRegion := "all-regions"
	// Use regions for name if none provided
	if regions != nil && len(regions) > 0 {
		providerRegion = strings.Join(regions, "-")
	}

	// If name is not provided, try to get account ID to include in the provider name
	if name == nil {
		name = new(string)
	}
	if *name == "" {
		// Get AWS account ID for provider name using common package
		cfg, err := InitAWSConfig(ctx, regions[0])
		if err != nil {
			log.Warn("Failed to load AWS config for account ID retrieval", "error", err)
			*name = fmt.Sprintf("%s-%s", prefix, providerRegion)
		} else {
			accountID, err := GetAccountID(ctx, cfg)
			if err == nil {
				log.Info("Retrieved AWS account ID", "account_id", accountID)
				*name = fmt.Sprintf("%s-%s-%s", prefix, accountID, providerRegion)
			} else {
				log.Warn("Failed to get AWS account ID", "error", err)
				*name = fmt.Sprintf("%s-%s", prefix, providerRegion)
			}
		}
	}
}
