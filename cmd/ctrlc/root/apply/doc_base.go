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

// Document is the legacy interface for document types.
// New implementations should use ResourceSpec instead.
type Document interface {
	Order() int
	Apply(ctx *DocContext) (ApplyResult, error)
	Delete(ctx *DocContext) (DeleteResult, error)
}

// BaseDocument provides common fields for all document types.
type BaseDocument struct {
	Type DocumentType `yaml:"type"`
	Name string       `yaml:"name"`
}

// GetName returns the document name (implements naming interface).
func (b *BaseDocument) GetName() string {
	return b.Name
}

// GetType returns the document type.
func (b *BaseDocument) GetType() DocumentType {
	return b.Type
}
