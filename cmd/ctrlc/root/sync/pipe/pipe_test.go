package pipe

import "testing"

func TestParseResources_ArrayWithVariables(t *testing.T) {
	input := []byte(`[
		{"name":"web-1","identifier":"web-1-prod","version":"custom/v1","kind":"Server","variables":{"env":"prod"}},
		{"name":"web-2","identifier":"web-2-prod","version":"custom/v1","kind":"Server"}
	]`)

	resources, err := parseResources(input)
	if err != nil {
		t.Fatalf("parseResources returned error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	if !resources[0].hasVariables {
		t.Fatalf("expected resource[0] to have variables")
	}
	if resources[0].Variables["env"] != "prod" {
		t.Fatalf("expected resource[0].variables.env to be prod, got %#v", resources[0].Variables["env"])
	}

	if resources[1].hasVariables {
		t.Fatalf("expected resource[1] to not have variables")
	}
}

func TestParseResources_SingleObject(t *testing.T) {
	input := []byte(`{"name":"web-1","identifier":"web-1-prod","version":"custom/v1","kind":"Server","variables":null}`)

	resources, err := parseResources(input)
	if err != nil {
		t.Fatalf("parseResources returned error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	if !resources[0].hasVariables {
		t.Fatalf("expected resource to have variables present")
	}
	if resources[0].Variables == nil {
		t.Fatalf("expected null variables to normalize to empty map")
	}
}

func TestParseResources_InvalidVariablesType(t *testing.T) {
	input := []byte(`{"name":"web-1","identifier":"web-1-prod","version":"custom/v1","kind":"Server","variables":"bad"}`)

	_, err := parseResources(input)
	if err == nil {
		t.Fatalf("expected parseResources to fail for invalid variables type")
	}
}

func TestValidateResources_UnchangedRequiredFields(t *testing.T) {
	resources := []resourceInput{
		{
			Name:       "",
			Identifier: "id-1",
			Version:    "",
			Kind:       "Server",
		},
	}

	err := validateResources(resources)
	if err == nil {
		t.Fatalf("expected validation to fail")
	}
}
