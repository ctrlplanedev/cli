package apply

import (
	"context"

	"github.com/ctrlplanedev/cli/internal/api"
)

type DocContext struct {
	Context     context.Context
	WorkspaceID string
	Client      *api.ClientWithResponses
	Resolver    *Resolver
}

func NewDocContext(workspaceID string, client *api.ClientWithResponses) *DocContext {
	resolver := NewResolver(client, workspaceID)
	return &DocContext{
		Context:     context.Background(),
		WorkspaceID: workspaceID,
		Client:      client,
		Resolver:    resolver,
	}
}

// DocumentType represents the type of document in a YAML file
type DocumentType string

type Document interface {
	Order() int
	Apply(ctx *DocContext) (ApplyResult, error)
	Delete(ctx *DocContext) (DeleteResult, error)
}

type BaseDocument struct {
	Type DocumentType `yaml:"type"`
	Name string       `yaml:"name"`
}
