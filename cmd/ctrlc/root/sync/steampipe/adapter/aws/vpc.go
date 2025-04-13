package aws

//import (
//	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/steampipe/adapter/model"
//	"github.com/ctrlplanedev/cli/internal/api"
//)
//
//const vpcTable = "aws_vpc"
//
//var VPC model.SteampipeAdapter = &model.SteampipeAdapterStruct{
//	Table: vpcTable,
//	Translate: func(data map[string]interface{}) (api.AgentResource, bool) {
//		var entityName = vpcTable
//		var sqlRow model.SqlRow = model.SqlRow{
//			EntityName: entityName,
//			Data:       data,
//		}
//
//		var name string
//		var arn string
//		var tags model.Tags
//		var accountId string
//		var region string
//
//		var zero api.AgentResource = api.AgentResource{}
//
//		if !model.GetRequiredValue[string](sqlRow, "arn", &arn) ||
//			!model.GetRequiredValue[string](sqlRow, "vpc_id", &name) ||
//			!model.GetOptionalValue[model.Tags](sqlRow, "tags", &tags) ||
//			!model.GetRequiredValue[string](sqlRow, "account_id", &accountId) ||
//			!model.GetRequiredValue[string](sqlRow, "region", &region) {
//			return zero, false
//		}
//
//		metadata := model.BuildMetadata(map[string]string{
//			"aws/account-id": accountId,
//			"aws/arn":        arn,
//			"aws/region":     region,
//		}).AppendTags(tags)
//
//		return api.AgentResource{
//			Identifier: arn,
//			Name:       name,
//			Version:    "vpc/v1",
//			Kind:       "VPC",
//			Config: map[string]interface{}{
//				"name": name,
//			},
//			Metadata: metadata,
//		}, true
//	},
//}
