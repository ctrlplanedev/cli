package gcp

//import (
//	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/steampipe/adapter/model"
//	"github.com/ctrlplanedev/cli/internal/api"
//	"strconv"
//)
//
//const computeTable = "gcp_compute_instance"
//
//var Compute model.SteampipeAdapter = &model.SteampipeAdapterStruct{
//	Table: computeTable,
//	Translate: func(data map[string]interface{}) (api.AgentResource, bool) {
//		var entityName = computeTable
//		var sqlRow model.SqlRow = model.SqlRow{
//			EntityName: entityName,
//			Data:       data,
//		}
//
//		var name string
//		var id int64
//		var tags model.Tags
//		var project string
//		var location string
//		var status string
//		var selfLink string
//
//		var zero api.AgentResource = api.AgentResource{}
//
//		if !model.GetRequiredValue[int64](sqlRow, "id", &id) ||
//			!model.GetRequiredValue[string](sqlRow, "name", &name) ||
//			!model.GetOptionalValue[model.Tags](sqlRow, "tags", &tags) ||
//			!model.GetRequiredValue[string](sqlRow, "project", &project) ||
//			!model.GetRequiredValue[string](sqlRow, "location", &location) ||
//			!model.GetRequiredValue[string](sqlRow, "status", &status) ||
//			!model.GetRequiredValue[string](sqlRow, "self_link", &selfLink) {
//			return zero, false
//		}
//
//		metadata := model.BuildMetadata(map[string]string{
//			"ctrlplane/external-id": strconv.FormatInt(id, 10),
//			"google/account-id":     project,
//			"google/id":             strconv.FormatInt(id, 10),
//			"google/location":       location,
//			"google/project":        project,
//			"google/self-link":      selfLink,
//		}).AppendTags(tags)
//
//		return api.AgentResource{
//			Identifier: strconv.FormatInt(id, 10),
//			Name:       name,
//			Version:    "compute/v1",
//			Kind:       "Compute",
//			Config: map[string]interface{}{
//				"auth": map[string]string{
//					"method":   "google/compute",
//					"location": location,
//					"project":  project,
//					"name":     name,
//				},
//				"name":   name,
//				"server": map[string]string{},
//				"status": status,
//			},
//			Metadata: metadata,
//		}, true
//	},
//}
