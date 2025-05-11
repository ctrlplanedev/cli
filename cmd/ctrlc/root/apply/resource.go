package apply

import (
	"context"
	"encoding/json"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
)

func processResourceProvider(ctx context.Context, client *api.ClientWithResponses, workspaceID string, provider ResourceProvider) {
	if provider.Name == "" {
		log.Info("Resource provider not provided, skipping")
		return
	}

	rp, err := api.NewResourceProvider(client, workspaceID, provider.Name)
	if err != nil {
		log.Error("Failed to create resource provider", "name", provider.Name, "error", err)
		return
	}

	resources := make([]api.CreateResource, 0)
	for _, resource := range provider.Resources {

		vars := make([]api.Variable, 0)
		if resource.Variables != nil {
			for _, variable := range *resource.Variables {
				if variable.Reference != nil {
					if variable.Path == nil || len(*variable.Path) == 0 {
						log.Error(
							"Reference variable must have a path",
							"name", resource.Name,
							"key", variable.Key,
							"reference", *variable.Reference,
						)
						continue
					}
					pathValue := []string{}
					if variable.Path != nil {
						pathValue = *variable.Path
					}
					refVar := api.ReferenceVariable{
						Key:          variable.Key,
						Reference:    *variable.Reference,
						Path:         pathValue,
						DefaultValue: nil,
					}

					if variable.DefaultValue != nil {
						refVar.DefaultValue = &api.ReferenceVariable_DefaultValue{}
						valueData, _ := json.Marshal(*variable.DefaultValue)
						refVar.DefaultValue.UnmarshalJSON(valueData)
					}

					var varRef api.Variable
					varRef.FromReferenceVariable(refVar)
					vars = append(vars, varRef)
				}

				if variable.Value != nil {
					directVar := api.DirectVariable{
						Key:   variable.Key,
						Value: api.DirectVariable_Value{},
					}

					// Set the value based on type
					valueData, _ := json.Marshal(*variable.Value)
					directVar.Value.UnmarshalJSON(valueData)

					var varDirect api.Variable
					varDirect.FromDirectVariable(directVar)
					vars = append(vars, varDirect)
				}
			}
		}

		resources = append(resources, api.CreateResource{
			Identifier: resource.Identifier,
			Name:       resource.Name,
			Version:    resource.Version,
			Kind:       resource.Kind,
			Config:     resource.Config,
			Metadata:   resource.Metadata,
			Variables:  &vars,
		})
	}

	upsertResp, err := rp.UpsertResource(ctx, resources)
	if err != nil {
		log.Error("Failed to upsert resources", "name", provider.Name, "error", err)
		return
	}

	log.Info("Response from upserting resources", "status", upsertResp.Status)
}
