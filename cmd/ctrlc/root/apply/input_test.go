package apply

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseInput_HTTP(t *testing.T) {
	payload := `type: Resource
name: test-resource
identifier: test-id
kind: Cluster
version: v1
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, payload)
	}))
	defer server.Close()

	documents, err := ParseInput(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(documents))
	}

	if _, ok := documents[0].(*ResourceDocument); !ok {
		t.Fatalf("expected ResourceDocument, got %T", documents[0])
	}
}

func TestParseInput_HTTPStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	_, err := ParseInput(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error for non-2xx response")
	}

	if !strings.Contains(err.Error(), "status 404") {
		t.Fatalf("unexpected error: %v", err)
	}
}
