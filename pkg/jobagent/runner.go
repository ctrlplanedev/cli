package jobagent

import (
	"context"
	"fmt"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
)

// Runner defines the interface for job execution.
// Start initiates a job and returns a status or error.
type Runner interface {
	Start(ctx context.Context, job *api.JobWithDetails) (api.JobStatus, error)
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
		client: client,
		id:     agent.JSON200.Id,
		runner: runner,
	}

	return ja, nil
}

type JobAgent struct {
	client *api.ClientWithResponses
	id     string
	runner Runner
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
	for _, apiJob := range *jobs.JSON200.Jobs {
		if apiJob.Status == api.JobStatusInProgress {
			continue
		}

		job, err := api.NewJobWithDetails(a.client, apiJob)
		if err != nil {
			log.Error("Failed to create job with details", "error", err, "jobId", apiJob.Id.String())
			continue
		}

		// Update job status to InProgress before starting execution
		if err := job.UpdateStatus(api.JobStatusInProgress, "Job execution started"); err != nil {
			log.Error("Failed to update job status to InProgress", "error", err, "jobId", job.Id.String())
			// Continue anyway
		}

		wg.Add(1)
		go func(job *api.JobWithDetails) {
			defer wg.Done()

			// Start the job - status updates happen inside Start
			status, err := a.runner.Start(context.Background(), job)

			if err != nil {
				log.Error("Failed to start job", "error", err, "jobId", job.Id.String())
				if updErr := job.UpdateStatus(api.JobStatusFailure, fmt.Sprintf("Failed to start job: %s", err.Error())); updErr != nil {
					log.Error("Failed to update job status", "error", updErr, "jobId", job.Id.String())
				}
				return
			}

			// If we got a status that's not InProgress, update it
			if status != api.JobStatusInProgress {
				if updErr := job.UpdateStatus(status, ""); updErr != nil {
					log.Error("Failed to update job status", "error", updErr, "jobId", job.Id.String())
				}
			}
		}(job)
	}
	wg.Wait()

	return nil
}
