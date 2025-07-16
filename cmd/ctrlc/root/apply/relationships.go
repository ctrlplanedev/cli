package apply

import (
	"context"
	"fmt"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
)

func processResourceRelationships(
	ctx context.Context,
	client *api.ClientWithResponses,
	workspaceID string,
	relationships []ResourceRelationship,
) {
	log.Info("Processing relationships", "count", len(relationships))
	for _, relationship := range relationships {
		log.Info("Processing relationship", "relationship", relationship)
		rule := createRelationshipRequestBody(workspaceID, relationship)
		res, err := client.CreateResourceRelationshipRuleWithResponse(ctx, rule)
		if err != nil {
			log.Error("Failed to create relationship", "error", err, "reference", relationship.Reference)
		}

		if res == nil {
			log.Error("Empty response when creating relationship", "reference", relationship.Reference)
			continue
		}

		if res.StatusCode() == http.StatusConflict {
			log.Info("Relationship already exists, skipping", "reference", relationship.Reference)
			continue
		}

		if res.StatusCode() != http.StatusOK {
			log.Error("Failed to create relationship", "status", res.StatusCode(), "error", string(res.Body))
		}
	}
}

func createMetadataKeysMatch(match MetadataKeysMatch) (*struct {
	SourceKey string `json:"sourceKey"`
	TargetKey string `json:"targetKey"`
}, error) {
	if match.Key != nil {
		return &struct {
			SourceKey string `json:"sourceKey"`
			TargetKey string `json:"targetKey"`
		}{
			SourceKey: *match.Key,
			TargetKey: *match.Key,
		}, nil
	}

	if match.SourceKey == nil || match.TargetKey == nil {
		return nil, fmt.Errorf("sourceKey and targetKey must be provided")
	}

	return &struct {
		SourceKey string `json:"sourceKey"`
		TargetKey string `json:"targetKey"`
	}{
		SourceKey: *match.SourceKey,
		TargetKey: *match.TargetKey,
	}, nil
}

func createRelationshipRequestBody(workspaceId string, relationship ResourceRelationship) api.CreateResourceRelationshipRule {
	config := api.CreateResourceRelationshipRule{
		WorkspaceId:          workspaceId,
		Reference:            relationship.Reference,
		DependencyType:       api.ResourceRelationshipRuleDependencyType(relationship.DependencyType),
		MetadataKeysMatches:  &[]api.MetadataKeyMatchConstraint{},
		TargetMetadataEquals: &[]api.MetadataEqualsConstraint{},
		SourceMetadataEquals: &[]api.MetadataEqualsConstraint{},
	}

	if relationship.Target != nil {
		config.TargetKind = relationship.Target.Kind
		config.TargetVersion = relationship.Target.Version

		if relationship.Target.MetadataEquals != nil {
			targetMetadataEquals := []api.MetadataEqualsConstraint{}

			for key, value := range relationship.Target.MetadataEquals {
				targetMetadataEquals = append(targetMetadataEquals, api.MetadataEqualsConstraint{
					Key:   &key,
					Value: &value,
				})
			}

			config.TargetMetadataEquals = &targetMetadataEquals
		}
	}

	if relationship.Source != nil {
		config.SourceKind = relationship.Source.Kind
		config.SourceVersion = relationship.Source.Version

		if relationship.Source.MetadataEquals != nil {
			sourceMetadataEquals := []api.MetadataEqualsConstraint{}

			for key, value := range relationship.Source.MetadataEquals {
				sourceMetadataEquals = append(sourceMetadataEquals, api.MetadataEqualsConstraint{
					Key:   &key,
					Value: &value,
				})
			}

			config.SourceMetadataEquals = &sourceMetadataEquals
		}
	}

	if relationship.MetadataKeysMatch != nil {
		metadataKeysMatches := []api.MetadataKeyMatchConstraint{}

		for _, match := range relationship.MetadataKeysMatch {
			metadataKeysMatch, err := createMetadataKeysMatch(match)
			if err != nil {
				log.Error("Failed to create metadata keys match", "error", err, "match", match)
				continue
			}

			metadataKeysMatches = append(metadataKeysMatches, *metadataKeysMatch)
		}

		config.MetadataKeysMatches = &metadataKeysMatches
	}

	// Log the MetadataTargetKeysEquals for debugging
	if config.TargetMetadataEquals != nil && len(*config.TargetMetadataEquals) > 0 {
		fmt.Println("MetadataTargetEquals:")
		for _, kv := range *config.TargetMetadataEquals {
			fmt.Printf("  Key: %s, Value: %s\n", *kv.Key, *kv.Value)
		}
	}

	return config
}
