package gcp

//import (
//	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/steampipe/adapter/model"
//	"github.com/ctrlplanedev/cli/internal/api"
//)
//
//const sqlTable = "gcp_sql_database_instance"
//
//var SQL model.SteampipeAdapter = &model.SteampipeAdapterStruct{
//	Table: sqlTable,
//	Translate: func(data map[string]interface{}) (api.AgentResource, bool) {
//		var entityName = sqlTable
//		var sqlRow model.SqlRow = model.SqlRow{
//			EntityName: entityName,
//			Data:       data,
//		}
//
//		var name string
//		var id string
//		var tags model.Tags
//		var project string
//		var location string
//		var sslCert string
//		var status string
//		var version string
//		var selfLink string
//
//		var zero api.AgentResource = api.AgentResource{}
//
//		if !model.GetRequiredValue[string](sqlRow, "connection_name", &id) ||
//			!model.GetRequiredValue[string](sqlRow, "name", &name) ||
//			!model.GetOptionalValue[model.Tags](sqlRow, "tags", &tags) ||
//			!model.GetRequiredValue[string](sqlRow, "project", &project) ||
//			!model.GetRequiredValue[string](sqlRow, "location", &location) ||
//			!model.GetRequiredValue[string](sqlRow, "ssl_configuration.cert", &sslCert) ||
//			!model.GetRequiredValue[string](sqlRow, "state", &status) ||
//			!model.GetRequiredValue[string](sqlRow, "database_installed_version", &version) ||
//			!model.GetRequiredValue[string](sqlRow, "self_link", &selfLink) {
//			return zero, false
//		}
//
//		metadata := model.BuildMetadata(map[string]string{
//			"ctrlplane/external-id": id,
//			"google/account-id":     project,
//			"google/id":             id,
//			"google/location":       location,
//			"google/project":        project,
//			"google/self-link":      selfLink,
//		}).AppendTags(tags)
//
//		return api.AgentResource{
//			Identifier: id,
//			Name:       name,
//			Version:    "database/v1",
//			Kind:       "Database",
//			Config: map[string]interface{}{
//				"auth": map[string]string{
//					"method":   "google/sql",
//					"location": location,
//					"project":  project,
//					"name":     name,
//				},
//				"name": name,
//				"server": map[string]string{
//					"sslCert": sslCert,
//				},
//				"status": status,
//			},
//			Metadata: metadata,
//		}, true
//	},
//}
