package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/pkg/jobagent"
)

var _ jobagent.Runner = &ExecRunner{}

type ExecConfig struct {
	WorkingDir string `json:"workingDir,omitempty"`
	Script     string `json:"script"`
}

type RunningJob struct {
	cmd       *exec.Cmd
	jobID     string
	client    *api.ClientWithResponses
	exitCode  int
	cancelled bool
}

type ExecRunner struct {
	runningJobs map[string]*RunningJob
	client      *api.ClientWithResponses
	mu          sync.Mutex
	wg          sync.WaitGroup
}

// NewExecRunner creates a new ExecRunner
func NewExecRunner(client *api.ClientWithResponses) *ExecRunner {
	runner := &ExecRunner{
		runningJobs: make(map[string]*RunningJob),
		client:      client,
	}

	// Set up signal handling for graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Info("Received shutdown signal, terminating all jobs")
		runner.ExitAll(true)
		os.Exit(0)
	}()

	return runner
}

func (r *ExecRunner) Status(job api.Job) (api.JobStatus, string) {
	r.mu.Lock()
	runningJob, exists := r.runningJobs[job.Id.String()]
	r.mu.Unlock()

	if !exists || runningJob == nil {
		return api.JobStatusSuccessful, ""
	}

	// Check for completed job with exit code
	if runningJob.cmd.ProcessState != nil {
		r.mu.Lock()
		delete(r.runningJobs, job.Id.String())
		r.mu.Unlock()

		if runningJob.cancelled {
			return api.JobStatusCancelled, "Job was cancelled"
		}

		if runningJob.exitCode != 0 {
			return api.JobStatusFailed, fmt.Sprintf("Job failed with exit code: %d", runningJob.exitCode)
		}
		
		return api.JobStatusSuccessful, ""
	}

	// If process exists but has not completed
	if runningJob.cmd.Process != nil {
		// Check if process is still running
		if err := runningJob.cmd.Process.Signal(syscall.Signal(0)); err != nil {
			// Process is no longer running but wasn't captured by ProcessState (unusual case)
			r.mu.Lock()
			delete(r.runningJobs, job.Id.String())
			r.mu.Unlock()
			
			return api.JobStatusSuccessful, ""
		}
		
		// Process is still running
		return api.JobStatusInProgress, ""
	}

	// Process hasn't started yet
	return api.JobStatusInProgress, ""
}

// ExitAll stops all currently running commands
func (r *ExecRunner) ExitAll(cancelled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, runningJob := range r.runningJobs {
		if runningJob != nil && runningJob.cmd != nil && runningJob.cmd.Process != nil {
			// Check if process is still running before attempting to kill
			if err := runningJob.cmd.Process.Signal(syscall.Signal(0)); err == nil {
				log.Info("Killing job", "id", id)
				runningJob.cancelled = cancelled
				
				// Process is running, kill it and its children
				if runtime.GOOS == "windows" {
					exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(runningJob.cmd.Process.Pid)).Run()
				} else {
					// Send SIGTERM first for graceful shutdown
					runningJob.cmd.Process.Signal(syscall.SIGTERM)
					
					// Update job status to cancelled
					if cancelled {
						status := api.JobStatusCancelled
						message := "Job was cancelled"
						runningJob.client.UpdateJobWithResponse(
							context.Background(),
							runningJob.jobID,
							api.UpdateJobJSONRequestBody{
								Status:  &status,
								Message: &message,
							},
						)
					}
				}
			}
		}
	}
	
	if cancelled {
		r.runningJobs = make(map[string]*RunningJob)
	}
}

func (r *ExecRunner) Start(job api.Job) (string, error) {
	// Initialize map if nil
	if r.runningJobs == nil {
		r.runningJobs = make(map[string]*RunningJob)
	}

	// Create temp script file
	ext := ".sh"
	if runtime.GOOS == "windows" {
		ext = ".ps1"
	}

	tmpFile, err := os.CreateTemp("", "script*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp script file: %w", err)
	}
	
	// Delete temp file when command completes
	scriptPath := tmpFile.Name()
	defer func() {
		// Don't remove the file immediately as the command might still be using it
		// Schedule it for removal after the command completes
		go func() {
			r.wg.Wait()
			os.Remove(scriptPath)
		}()
	}()

	config := ExecConfig{}
	jsonBytes, err := json.Marshal(job.JobAgentConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal job agent config: %w", err)
	}
	if err := json.Unmarshal(jsonBytes, &config); err != nil {
		return "", fmt.Errorf("failed to unmarshal job agent config: %w", err)
	}

	templatedScript, err := template.New("script").Parse(config.Script)
	if err != nil {
		return "", fmt.Errorf("failed to parse script template: %w", err)
	}

	buf := new(bytes.Buffer)
	if err := templatedScript.Execute(buf, job); err != nil {
		return "", fmt.Errorf("failed to execute script template: %w", err)
	}
	script := buf.String()

	// Write script contents
	if _, err := tmpFile.WriteString(script); err != nil {
		return "", fmt.Errorf("failed to write script file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close script file: %w", err)
	}

	// Make executable on Unix systems
	if runtime.GOOS != "windows" {
		if err := os.Chmod(scriptPath, 0700); err != nil {
			return "", fmt.Errorf("failed to make script executable: %w", err)
		}
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-File", scriptPath)
	} else {
		cmd = exec.Command("bash", scriptPath)
	}

	cmd.Dir = config.WorkingDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Create running job struct
	runningJob := &RunningJob{
		cmd:    cmd,
		jobID:  job.Id.String(),
		client: r.client,
	}

	r.mu.Lock()
	r.runningJobs[job.Id.String()] = runningJob
	r.mu.Unlock()

	// Launch command in goroutine
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		err := cmd.Start()
		if err != nil {
			log.Error("Failed to start command", "error", err, "jobID", job.Id.String())
			status := api.JobStatusFailed
			message := fmt.Sprintf("Failed to start command: %s", err.Error())
			r.client.UpdateJobWithResponse(
				context.Background(),
				job.Id.String(),
				api.UpdateJobJSONRequestBody{
					Status:  &status,
					Message: &message,
				},
			)
			
			r.mu.Lock()
			delete(r.runningJobs, job.Id.String())
			r.mu.Unlock()
			return
		}

		// Update job to InProgress with PID
		pid := strconv.Itoa(cmd.Process.Pid)
		status := api.JobStatusInProgress
		r.client.UpdateJobWithResponse(
			context.Background(),
			job.Id.String(),
			api.UpdateJobJSONRequestBody{
				Status:     &status,
				ExternalId: &pid,
			},
		)

		// Wait for command to complete
		err = cmd.Wait()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
				log.Warn("Command failed", "exitCode", exitCode, "jobID", job.Id.String())
			} else {
				log.Error("Command error", "error", err, "jobID", job.Id.String())
				exitCode = 1
			}
		}

		runningJob.exitCode = exitCode

		// Update job with final status
		var finalStatus api.JobStatus
		var message string
		
		r.mu.Lock()
		if runningJob.cancelled {
			finalStatus = api.JobStatusCancelled
			message = "Job was cancelled"
		} else if exitCode != 0 {
			finalStatus = api.JobStatusFailed
			message = fmt.Sprintf("Job failed with exit code: %d", exitCode)
		} else {
			finalStatus = api.JobStatusSuccessful
			message = "Job completed successfully"
		}
		r.mu.Unlock()

		r.client.UpdateJobWithResponse(
			context.Background(),
			job.Id.String(),
			api.UpdateJobJSONRequestBody{
				Status:  &finalStatus,
				Message: &message,
			},
		)
	}()

	return strconv.Itoa(cmd.Process.Pid), nil
}
