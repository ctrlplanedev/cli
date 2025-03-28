package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
)

// JobWithDetails extends the Job type with additional fields and methods
type JobWithDetails struct {
	Job
	Details    map[string]interface{}
	Client     *ClientWithResponses
	ExternalID string
}

// NewJobWithDetails creates a new JobWithDetails with the client and job data
func NewJobWithDetails(client *ClientWithResponses, job Job) (*JobWithDetails, error) {
	j := &JobWithDetails{
		Job:    job,
		Client: client,
	}

	// Fetch job details
	var err error
	j.Details, err = j.GetJobDetails(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get job details: %w", err)
	}

	return j, nil
}

// GetJobDetails retrieves job details for templating
func (j *JobWithDetails) GetJobDetails(ctx context.Context) (map[string]interface{}, error) {
	resp, err := j.Client.GetJobWithResponse(ctx, j.Id.String())
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

// TemplateJobDetails applies the job details template to the script
func (j *JobWithDetails) TemplateJobDetails() (string, error) {
	// Extract script from JobAgentConfig
	script, ok := j.JobAgentConfig["script"].(string)
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
	if err := templatedScript.Execute(buf, j.Details); err != nil {
		return "", fmt.Errorf("failed to execute script template: %w", err)
	}

	return buf.String(), nil
}

// UpdateStatus updates the job status via API
func (j *JobWithDetails) UpdateStatus(status JobStatus, message string) error {
	body := UpdateJobJSONRequestBody{
		Status: &status,
	}
	if message != "" {
		body.Message = &message
	}
	if j.ExternalID != "" {
		body.ExternalId = &j.ExternalID
	}

	resp, err := j.Client.UpdateJobWithResponse(context.Background(), j.Id.String(), body)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}
	if resp.JSON200 == nil {
		return fmt.Errorf("failed to update job status: received empty response")
	}
	return nil
}

// SetExternalID sets the external ID for the job
func (j *JobWithDetails) SetExternalID(externalID string) {
	j.ExternalID = externalID
}
