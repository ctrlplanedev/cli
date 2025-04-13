package adapter

import (
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/steampipe/adapter/aws"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/steampipe/adapter/model"
	"strings"
)

var adapters = []model.SteampipeAdapter{
	aws.EC2,
	//aws.EKS,
	//aws.RDS,
	//aws.VPC,
	//gcp.GKE,
	//gcp.SQL,
	//gcp.Compute,
	//azure.AKS,
}

func buildRegistry() map[string]model.SteampipeAdapter {
	registry := make(map[string]model.SteampipeAdapter)
	for _, adapter := range adapters {
		registry[adapter.EntityName()] = adapter
	}
	return registry
}

var registry = buildRegistry()

func SelectAdapter(table string) model.SteampipeAdapter {
	var result model.SteampipeAdapter
	var ok bool
	tableName := stripSchema(table)

	if result, ok = registry[tableName]; !ok {
		log.Warnf("could not find adapter for table %s", tableName)
		return nil
	}
	return result
}

func stripSchema(table string) string {
	if strings.Contains(table, ".") {
		return strings.Split(table, ".")[1]
	}
	return table
}
