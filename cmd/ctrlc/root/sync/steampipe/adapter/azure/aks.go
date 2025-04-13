package azure

//import (
//	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/steampipe/adapter/model"
//	"github.com/ctrlplanedev/cli/internal/api"
//)
//
//const aksTable = "azure_kubernetes_cluster"
//
//var AKS model.SteampipeAdapter = &model.SteampipeAdapterStruct{
//	Table: aksTable,
//	Translate: func(data map[string]interface{}) (api.AgentResource, bool) {
//		var entityName = aksTable
//		var sqlRow model.SqlRow = model.SqlRow{
//			EntityName: entityName,
//			Data:       data,
//		}
//
//		var name string
//		var id string
//		var tags model.Tags
//		var subscriptionId string
//		var location string
//		var status string
//		var version string
//		var autoscalerProfile map[string]interface{}
//		var autoscaler string
//
//		var zero api.AgentResource = api.AgentResource{}
//
//		if !model.GetRequiredValue[string](sqlRow, "id", &id) ||
//			!model.GetRequiredValue[string](sqlRow, "name", &name) ||
//			!model.GetOptionalValue[model.Tags](sqlRow, "tags", &tags) ||
//			!model.GetRequiredValue[string](sqlRow, "subscription_id", &subscriptionId) ||
//			!model.GetRequiredValue[string](sqlRow, "location", &location) ||
//			!model.GetOptionalValue[string](sqlRow, "power_state.code", &status) ||
//			!model.GetRequiredValue[string](sqlRow, "kubernetes_version", &version) ||
//			!model.GetRequiredValue[map[string]interface{}](sqlRow, "auto_scaler_profile", &autoscalerProfile) {
//			return zero, false
//		}
//
//		if autoscalerProfile != nil {
//			autoscaler = "true"
//		} else {
//			autoscaler = "false"
//		}
//
//		metadata := model.BuildMetadata(map[string]string{
//			"ctrlplane/external-id":         id,
//			"azure/id":                      id,
//			"azure/location":                location,
//			"azure/subscription-id":         subscriptionId,
//			"kubernetes/version":            version,
//			"kubernetes/autoscaler-enabled": autoscaler,
//		}).AppendTags(tags)
//
//		return api.AgentResource{
//			Identifier: id,
//			Name:       name,
//			Version:    "kubernetes/v1",
//			Kind:       "ClusterAPI",
//			Config: map[string]interface{}{
//				"auth": map[string]string{
//					"method":         "azure/aks",
//					"location":       location,
//					"subscriptionId": subscriptionId,
//					"clusterName":    name,
//				},
//				"name":   name,
//				"server": map[string]string{},
//				"status": status,
//			},
//			Metadata: metadata,
//		}, true
//	},
//}
