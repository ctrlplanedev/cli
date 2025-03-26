package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

func NewAPIKeyClientWithResponses(server string, apiKey string) (*ClientWithResponses, error) {
	server = strings.TrimSuffix(server, "/")
	server = strings.TrimSuffix(server, "/api")
	return NewClientWithResponses(server+"/api",
		WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("X-API-Key", apiKey)
			return nil
		}),
	)
}

func (v *Variable_Value) SetString(value string) {
	v.union = json.RawMessage("\"" + value + "\"")
}

// TemplateJobDetails applies the job details template to the script
func (c *ClientWithResponses) TemplateJobDetails(job Job, jobDetails map[string]interface{}) (string, error) {
	// Extract script from JobAgentConfig
	script, ok := job.JobAgentConfig["script"].(string)
	if !ok {
		return "", fmt.Errorf("script not found in job agent config")
	}

	// Parse the script template
	templatedScript, err := template.New("script").Parse(script)
	if err != nil {
		return "", fmt.Errorf("failed to parse script template: %w", err)
	}

	// Execute the template with job details
	buf := new(bytes.Buffer)
	if err := templatedScript.Execute(buf, jobDetails); err != nil {
		return "", fmt.Errorf("failed to execute script template: %w", err)
	}

	return buf.String(), nil
}

// UpdateJobStatus updates the job status via API
func (c *ClientWithResponses) UpdateJobStatus(jobID string, status JobStatus, message string, externalID *string) error {
	body := UpdateJobJSONRequestBody{
		Status: &status,
	}
	if message != "" {
		body.Message = &message
	}
	if externalID != nil {
		body.ExternalId = externalID
	}

	resp, err := c.UpdateJobWithResponse(context.Background(), jobID, body)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}
	if resp.JSON200 == nil {
		return fmt.Errorf("failed to update job status: received empty response")
	}
	return nil
}

// GetJobDetails retrieves job details for templating
func (c *ClientWithResponses) GetJobDetails(ctx context.Context, jobID string) (map[string]interface{}, error) {
	resp, err := c.GetJobWithResponse(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job details: %w", err)
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("received empty response from job details API")
	}

	var details map[string]interface{}
	detailsBytes, err := json.Marshal(resp.JSON200)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job response: %w", err)
	}
	if err := json.Unmarshal(detailsBytes, &details); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job details: %w", err)
	}
	return details, nil
}
