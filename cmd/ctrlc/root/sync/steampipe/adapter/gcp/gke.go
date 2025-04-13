package gcp

//import (
//	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/steampipe/adapter/model"
//	"github.com/ctrlplanedev/cli/internal/api"
//	"strconv"
//)
//
//const gkeTable = "gcp_kubernetes_cluster"
//
//var GKE model.SteampipeAdapter = &model.SteampipeAdapterStruct{
//	Table: gkeTable,
//	Translate: func(data map[string]interface{}) (api.AgentResource, bool) {
//		var entityName = gkeTable
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
//		var certificateAuthorityData string
//		var status string
//		var version string
//		var endpoint string
//		var selfLink string
//		var autopilot bool
//		var autoscaling string
//
//		var zero api.AgentResource = api.AgentResource{}
//
//		if !model.GetRequiredValue[string](sqlRow, "id", &id) ||
//			!model.GetRequiredValue[string](sqlRow, "name", &name) ||
//			!model.GetOptionalValue[model.Tags](sqlRow, "tags", &tags) ||
//			!model.GetRequiredValue[string](sqlRow, "project", &project) ||
//			!model.GetRequiredValue[string](sqlRow, "location", &location) ||
//			!model.GetRequiredValue[string](sqlRow, "master_auth.clusterCaCertificate", &certificateAuthorityData) ||
//			!model.GetRequiredValue[string](sqlRow, "status", &status) ||
//			!model.GetRequiredValue[string](sqlRow, "current_master_version", &version) ||
//			!model.GetRequiredValue[string](sqlRow, "endpoint", &endpoint) ||
//			!model.GetRequiredValue[string](sqlRow, "self_link", &selfLink) ||
//			!model.GetRequiredValue[bool](sqlRow, "autopilot_enabled", &autopilot) ||
//			!model.GetOptionalValue[string](sqlRow, "autoscaling.autoscalingProfile", &autoscaling) {
//			return zero, false
//		}
//
//		metadata := model.BuildMetadata(map[string]string{
//			"ctrlplane/external-id":  id,
//			"google/account-id":      project,
//			"google/id":              id,
//			"google/location":        location,
//			"google/project":         project,
//			"google/self-link":       selfLink,
//			"google/autopilot":       strconv.FormatBool(autopilot),
//			"kubernetes/autoscaling": autoscaling,
//		}).AppendTags(tags)
//
//		return api.AgentResource{
//			Identifier: id,
//			Name:       name,
//			Version:    "kubernetes/v1",
//			Kind:       "ClusterAPI",
//			Config: map[string]interface{}{
//				"auth": map[string]string{
//					"method":      "google/gke",
//					"location":    location,
//					"project":     project,
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
