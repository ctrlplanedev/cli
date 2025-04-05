package steampipe

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/log"
)

type SteampipePluginList struct {
	Installed []PluginInfo `json:"installed"`
	Failed    *interface{} `json:"failed"`
	Warnings  *interface{} `json:"warnings"`
}

type PluginInfo struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Connections []string `json:"connections"`
}

type ResourceGroup struct {
	ConnectionType string
	Name           string
	ResourceType   string
}

func (c *SteampipeClient) ListResourceGroups() ([]ResourceGroup, error) {

	cmd := exec.Command("steampipe", "plugin", "list", "--output", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to execute steampipe command: %w", err)
	}

	// Simulate processing the output (e.g., parsing JSON)
	// fmt.Printf("Command output: %s\n", output)

	pluginList := &SteampipePluginList{}
	err = json.Unmarshal(output, pluginList)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal plugin list: %w", err)
	}

	if pluginList.Failed != nil {
		log.Errorf("Resource Group list failures: %v", pluginList.Failed)
	}

	if pluginList.Warnings != nil {
		log.Errorf("Resource Group list warnings: %v", pluginList.Warnings)
	}

	resourceGroups := make([]ResourceGroup, 0)

	for _, plugin := range pluginList.Installed {
		connectionType := getConnectionType(plugin.Name)
		fmt.Printf("%s.*:[resource]\n", connectionType)
		resourceGroups = append(resourceGroups, ResourceGroup{
			ConnectionType: connectionType,
			Name:           "*",
			ResourceType:   "*",
		})
		for _, connection := range plugin.Connections {
			// if pluginName == connection name, then it's covered by `pluginName.*`
			if connection != connectionType {
				fmt.Printf("%s.%s:[resource]\n", connectionType, connection)
				resourceGroups = append(resourceGroups, ResourceGroup{
					ConnectionType: connectionType,
					Name:           connection,
					ResourceType:   "*",
				})
			}
		}
	}

	return resourceGroups, nil
}

func getConnectionType(pluginName string) string {
	sansVersion := strings.Split(pluginName, "@")[0]
	delimitedName := strings.Split(sansVersion, "/")
	return delimitedName[len(delimitedName)-1]
}
