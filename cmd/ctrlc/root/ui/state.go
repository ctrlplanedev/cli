package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
)

const stateFileName = ".ctrlc-ui-state"

func statePath() string {
	home, err := homedir.Dir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, stateFileName)
}

// loadLastView reads the last-viewed resource type from the state file
func loadLastView() resourceType {
	p := statePath()
	if p == "" {
		return resourceTypeResources
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return resourceTypeResources
	}
	name := strings.TrimSpace(string(data))
	if rt, ok := resourceTypeFromString(name); ok {
		return rt
	}
	return resourceTypeResources
}

// saveLastView persists the current resource type to disk
func saveLastView(rt resourceType) {
	p := statePath()
	if p == "" {
		return
	}
	_ = os.WriteFile(p, []byte(rt.String()), 0644)
}

func resourceTypeFromString(s string) (resourceType, bool) {
	switch strings.ToLower(s) {
	case "deployments":
		return resourceTypeDeployments, true
	case "resources":
		return resourceTypeResources, true
	case "jobs":
		return resourceTypeJobs, true
	case "environments":
		return resourceTypeEnvironments, true
	case "deployment versions":
		return resourceTypeVersions, true
	default:
		return 0, false
	}
}
