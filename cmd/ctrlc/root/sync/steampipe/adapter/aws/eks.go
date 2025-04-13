package aws

//import (
//	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/steampipe/adapter/model"
//	"github.com/ctrlplanedev/cli/internal/api"
//)
//
//const eksTable = "aws_eks_cluster"
//
//var EKS model.SteampipeAdapter = &model.SteampipeAdapterStruct{
//	Table: eksTable,
//	Translate: func(data map[string]interface{}) (api.AgentResource, bool) {
//		var entityName = eksTable
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
//		var certificateAuthorityData string
//		var status string
//		var version string
//		var endpoint string
//		var roleArn string
//		var platformVersion string
//
//		var zero api.AgentResource = api.AgentResource{}
//
//		if !model.GetRequiredValue[string](sqlRow, "arn", &arn) ||
//			!model.GetRequiredValue[string](sqlRow, "name", &name) ||
//			!model.GetOptionalValue[model.Tags](sqlRow, "tags", &tags) ||
//			!model.GetRequiredValue[string](sqlRow, "account_id", &accountId) ||
//			!model.GetRequiredValue[string](sqlRow, "region", &region) ||
//			!model.GetRequiredValue[string](sqlRow, "certificate_authority.Data", &certificateAuthorityData) ||
//			!model.GetRequiredValue[string](sqlRow, "status", &status) ||
//			!model.GetRequiredValue[string](sqlRow, "version", &version) ||
//			!model.GetRequiredValue[string](sqlRow, "endpoint", &endpoint) ||
//			!model.GetRequiredValue[string](sqlRow, "role_arn", &roleArn) ||
//			!model.GetRequiredValue[string](sqlRow, "platform_version", &platformVersion) {
//			return zero, false
//		}
//
//		metadata := model.BuildMetadata(map[string]string{
//			"aws/account-id":       accountId,
//			"aws/arn":              arn,
//			"aws/eks-role-arn":     roleArn,
//			"aws/region":           region,
//			"aws/platform-version": platformVersion,
//		}).AppendTags(tags)
//
//		return api.AgentResource{
//			Identifier: arn,
//			Name:       name,
//			Version:    "kubernetes/v1",
//			Kind:       "ClusterAPI",
//			Config: map[string]interface{}{
//				"auth": map[string]string{
//					"method":      "aws/eks",
//					"region":      region,
//					"accountId":   accountId,
//					"clusterName": name,
//				},
//				"name": name,
//				"server": map[string]string{
//					"endpoint":                   endpoint,
//					"certificationAuthorityData": certificateAuthorityData,
//				},
//				"status": status,
//			},
//			Metadata: metadata,
//		}, true
//	},
//}
