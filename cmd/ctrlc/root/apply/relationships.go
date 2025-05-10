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

func createRelationshipRequestBody(workspaceId string, relationship ResourceRelationship) api.CreateResourceRelationshipRule {
	config := api.CreateResourceRelationshipRule{
		WorkspaceId:    workspaceId,
		Reference:      relationship.Reference,
		DependencyType: api.ResourceRelationshipRuleDependencyType(relationship.DependencyType),
		MetadataKeysMatches: &[]string{},
		TargetMetadataEquals: &[]struct{
			Key   string `json:"key"`
			Value string `json:"value"`
		}{},
	}

	if relationship.Target != nil {
		config.TargetKind = relationship.Target.Kind
		config.TargetVersion = relationship.Target.Version


		if relationship.Target.MetadataEquals != nil {
			targetMetadataEquals := []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			}{}

			for key, value := range relationship.Target.MetadataEquals {
				targetMetadataEquals = append(targetMetadataEquals, struct {
					Key   string `json:"key"`
					Value string `json:"value"`
				}{
					Key:   key,
					Value: value,
				})
			}

			config.TargetMetadataEquals = &targetMetadataEquals
		}
	}

	if relationship.Source != nil {
		config.SourceKind = relationship.Source.Kind
		config.SourceVersion = relationship.Source.Version
	}

	if relationship.MetadataKeysMatch != nil {
		config.MetadataKeysMatches = &relationship.MetadataKeysMatch
	}

	// Log the MetadataTargetKeysEquals for debugging
	if config.TargetMetadataEquals != nil && len(*config.TargetMetadataEquals) > 0 {
		fmt.Println("MetadataTargetEquals:")
		for _, kv := range *config.TargetMetadataEquals {
			fmt.Printf("  Key: %s, Value: %s\n", kv.Key, kv.Value)
		}
	}

	return config
}
