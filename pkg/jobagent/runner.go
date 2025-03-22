package jobagent

import (
	"context"
	"fmt"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
)

// Runner defines the interface for job execution.
// Start initiates a job and returns an external ID or error.
// The implementation should handle status updates when the job completes.
type Runner interface {
	Start(job api.Job, jobDetails map[string]interface{}, statusUpdateFunc func(jobID string, status api.JobStatus, message string)) (string, api.JobStatus, error)
}

func NewJobAgent(
	client *api.ClientWithResponses,
	config api.UpsertJobAgentJSONRequestBody,
	runner Runner,
) (*JobAgent, error) {
	agent, err := client.UpsertJobAgentWithResponse(context.Background(), config)
	if err != nil {
		return nil, err
	}

	if agent.JSON200 == nil {
		return nil, fmt.Errorf("failed to create job agent")
	}

	ja := &JobAgent{
		client:      client,
		id:          agent.JSON200.Id,
		workspaceId: config.WorkspaceId,
		runner:      runner,
	}

	return ja, nil
}

type JobAgent struct {
	client *api.ClientWithResponses

	workspaceId string
	id          string

	runner Runner
}

// updateJobStatus is a helper function to update job status via the API
func (a *JobAgent) updateJobStatus(jobID string, status api.JobStatus, message string, externalID *string) {
	body := api.UpdateJobJSONRequestBody{
		Status: &status,
	}
	if message != "" {
		body.Message = &message
	}
	if externalID != nil {
		body.ExternalId = externalID
	}

	_, err := a.client.UpdateJobWithResponse(
		context.Background(),
		jobID,
		body,
	)
	if err != nil {
		log.Error("Failed to update job", "error", err, "jobId", jobID)
	}
}

// RunQueuedJobs retrieves and executes any queued jobs for this agent.
// For each job, it starts execution using the runner's Start method, which
// will update the job status when the job completes.
func (a *JobAgent) RunQueuedJobs() error {
	jobs, err := a.client.GetNextJobsWithResponse(context.Background(), a.id)
	if err != nil {
		return err
	}
	if jobs.JSON200 == nil {
		return fmt.Errorf("failed to get job")
	}

	log.Debug("Got jobs", "count", len(*jobs.JSON200.Jobs))
	var wg sync.WaitGroup
	for _, job := range *jobs.JSON200.Jobs {
		jobDetails, err := fetchJobDetails(context.Background(), job.Id.String())
		if err != nil {
			log.Error("Failed to fetch job details", "error", err, "jobId", job.Id.String())
			continue
		}
		wg.Add(1)
		go func(job api.Job) {
			defer wg.Done()
			
			// Create a status update callback for this job
			statusUpdateFunc := func(jobID string, status api.JobStatus, message string) {
				a.updateJobStatus(jobID, status, message, nil)
			}
			
			externalId, status, err := a.runner.Start(job, jobDetails, statusUpdateFunc)
			
			if err != nil {
				status := api.JobStatusInProgress
				message := fmt.Sprintf("Failed to start job: %s", err.Error())
				log.Error("Failed to start job", "error", err, "jobId", job.Id.String())
				a.updateJobStatus(job.Id.String(), status, message, nil)
				return
			}

			if status == api.JobStatusFailure {
				message := fmt.Sprintf("Failed to start job: %s", err.Error())
				log.Error("Failed to start job", "error", err, "jobId", job.Id.String())
				a.updateJobStatus(job.Id.String(), status, message, nil)
				return
			}
			
			if externalId != "" {
				status := api.JobStatusInProgress
				a.updateJobStatus(job.Id.String(), status, "", &externalId)
			}
		}(job)
	}
	wg.Wait()

	return nil
}
