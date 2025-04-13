package aws

//import (
//	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/steampipe/adapter/model"
//	"github.com/ctrlplanedev/cli/internal/api"
//)
//
//const rdsTable = "aws_rds_db_cluster"
//
//var RDS model.SteampipeAdapter = &model.SteampipeAdapterStruct{
//	Table: rdsTable,
//	Translate: func(data map[string]interface{}) (api.AgentResource, bool) {
//		var entityName = rdsTable
//		var sqlRow model.SqlRow = model.SqlRow{
//			EntityName: entityName,
//			Data:       data,
//		}
//
//		var name string
//		var dbName string
//		var arn string
//		var tags model.Tags
//		var accountId string
//		var region string
//		var status string
//		var version string
//		var endpoint string
//
//		var zero api.AgentResource = api.AgentResource{}
//
//		if !model.GetRequiredValue[string](sqlRow, "arn", &arn) ||
//			!model.GetRequiredValue[string](sqlRow, "db_cluster_identifier", &name) ||
//			!model.GetRequiredValue[string](sqlRow, "database_name", &dbName) ||
//			!model.GetOptionalValue[model.Tags](sqlRow, "tags", &tags) ||
//			!model.GetRequiredValue[string](sqlRow, "account_id", &accountId) ||
//			!model.GetRequiredValue[string](sqlRow, "region", &region) ||
//			!model.GetRequiredValue[string](sqlRow, "status", &status) ||
//			!model.GetRequiredValue[string](sqlRow, "engine_version", &version) ||
//			!model.GetRequiredValue[string](sqlRow, "endpoint", &endpoint) {
//			return zero, false
//		}
//
//		metadata := model.BuildMetadata(map[string]string{
//			"aws/account-id":     accountId,
//			"aws/arn":            arn,
//			"aws/region":         region,
//			"aws/engine-version": version,
//		}).AppendTags(tags)
//
//		return api.AgentResource{
//			Identifier: arn,
//			Name:       name,
//			Version:    "database/v1",
//			Kind:       "Database",
//			Config: map[string]interface{}{
//				"auth": map[string]string{
//					"method":      "aws/rds",
//					"region":      region,
//					"accountId":   accountId,
//					"clusterName": name,
//				},
//				"name": name,
//				"server": map[string]string{
//					"endpoint": endpoint,
//				},
//				"status": status,
//			},
//			Metadata: metadata,
//		}, true
//	},
//}
