package kubernetes

import (
	"fmt"
	"slices"
	"strings"
)

type ResourceType string

const (
	ResourceNamespace  ResourceType = "namespaces"
	ResourceNode       ResourceType = "nodes"
	ResourceDeployment ResourceType = "deployments"
)

func (r ResourceType) String() string {
	return string(r)
}

func ParseResourceType(s string) (ResourceType, error) {
	switch ResourceType(s) {
	case ResourceNamespace, ResourceNode, ResourceDeployment:
		return ResourceType(s), nil
	default:
		return "", fmt.Errorf("invalid resource type %q", s)
	}
}

type ResourceTypes []ResourceType

func (r ResourceTypes) ShouldFetch(target ResourceType) bool {
	return slices.Contains(r, target) || len(r) == 0
}

func (r *ResourceTypes) String() string {
	if r == nil {
		return ""
	}
	out := make([]string, len(*r))
	for i, v := range *r {
		out[i] = v.String()
	}
	return strings.Join(out, ",")
}

func (s *ResourceTypes) Type() string {
	return "resourceType"
}

func (r *ResourceTypes) Set(value string) error {
	// supports repeated flags like:
	// --resource namespace --resource node
	rt, err := ParseResourceType(value)
	if err != nil {
		return err
	}
	*r = append(*r, rt)
	return nil
}
