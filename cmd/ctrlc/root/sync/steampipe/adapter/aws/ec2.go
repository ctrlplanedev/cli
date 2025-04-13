package aws

import (
	_ "embed"
	"encoding/json"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/steampipe/adapter/model"
	"github.com/xeipuuv/gojsonschema"
)

const ec2Table = "aws_ec2_instance"

//go:embed schema/ec2_row.schema.json
var ec2RowSchema string

var rowSchemaLoader = gojsonschema.NewStringLoader(ec2RowSchema)

type Ec2Row struct {
	Arn            string            `json:"arn"`
	InstanceID     string            `json:"instance_id"`
	AccountID      string            `json:"account_id"`
	Region         string            `json:"region"`
	InstanceState  string            `json:"instance_state"`
	PrivateDNSName string            `json:"private_dns_name" default:""`
	Tags           map[string]string `json:"tags"`
}

type Ec2Resource struct {
	WorkspaceId string            `json:"workspaceId"`
	Config      Ec2ResConfig      `json:"config"`
	Identifier  string            `json:"identifier"`
	Kind        string            `json:"kind"`
	Metadata    map[string]string `json:"metadata"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
}

type Ec2ResConfig struct {
	Auth   Ec2ResConfigAuth   `json:"auth"`
	Name   string             `json:"name"`
	Server Ec2ResConfigServer `json:"server"`
	Status string             `json:"status"`
}

type Ec2ResConfigAuth struct {
	Method     string `json:"method"`
	Region     string `json:"region"`
	AccountId  string `json:"accountId"`
	InstanceId string `json:"instanceId"`
}

type Ec2ResConfigServer struct {
	Endpoint string `json:"endpoint"`
}

type Ec2ResRequiredMetadata struct {
	AccountID string `json:"aws/account-id"`
	Arn       string `json:"aws/arn"`
	Region    string `json:"aws/region"`
}

var EC2 model.SteampipeAdapter = &model.SteampipeAdapterStruct{
	Table: ec2Table,
	Convert: func(workspaceId string, rowJsonStr string) (string, bool) {
		// Validate row json schema and build into type
		row, ok := model.ValidateAndUnmarshal[Ec2Row](rowSchemaLoader, rowJsonStr)
		if !ok {
			return "", false
		}

		// Add required metadata and merge with tags
		metadata := make(model.Metadata)
		metadata["aws/account-id"] = row.AccountID
		metadata["aws/arn"] = row.Arn
		metadata["aws/region"] = row.Region
		metadata = metadata.AppendTags(row.Tags)

		// Build resource specific to the resource type
		result := Ec2Resource{
			WorkspaceId: workspaceId,
			Identifier:  row.Arn,
			Name:        row.InstanceID,
			Version:     "vm/v1",
			Kind:        "VM",
			Config: Ec2ResConfig{
				Auth: Ec2ResConfigAuth{
					Method:     "aws/ec2",
					Region:     row.Region,
					AccountId:  row.AccountID,
					InstanceId: row.InstanceID,
				},
				Name: row.InstanceID,
				Server: Ec2ResConfigServer{
					Endpoint: row.PrivateDNSName,
				},
				Status: row.InstanceState,
			},
			Metadata: metadata,
		}

		var resultJson []byte
		var err error
		if resultJson, err = json.Marshal(result); err != nil {
			log.Errorf("failed to marshal EC2 resource: %v", err)
			return "", false
		}

		return string(resultJson), true
	},
}
