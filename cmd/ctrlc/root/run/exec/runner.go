package exec

import (
	"context"
	"fmt"
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

// Start creates a temporary script file, starts the process, and updates job status when the process completes.
func (r *ExecRunner) Start(ctx context.Context, job api.Job, jobDetails map[string]interface{}, 
	statusUpdateFunc func(jobID string, status api.JobStatus, message string)) (string, api.JobStatus, error) {
	
	// Template the script using the API client
	script, err := r.client.TemplateJobDetails(job, jobDetails)
	if err != nil {
		return "", api.JobStatusFailure, fmt.Errorf("failed to template job details: %w", err)
	}

	// Determine file extension based on OS
	ext := ".sh"
	if runtime.GOOS == "windows" {
		ext = ".ps1"
	}

	tmpFile, err := os.CreateTemp("", "script*"+ext)
	if err != nil {
		return "", api.JobStatusFailure, fmt.Errorf("failed to create temp script file: %w", err)
	}

	// Write the script to the temporary file
	if _, err := tmpFile.WriteString(script); err != nil {
		os.Remove(tmpFile.Name())
		return "", api.JobStatusFailure, fmt.Errorf("failed to write script file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", api.JobStatusFailure, fmt.Errorf("failed to close script file: %w", err)
	}

	// Make the script executable on Unix-like systems
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpFile.Name(), 0700); err != nil {
			os.Remove(tmpFile.Name())
			return "", api.JobStatusFailure, fmt.Errorf("failed to make script executable: %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", tmpFile.Name())
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-File", tmpFile.Name())
	}

	// Set up command environment
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the process
	if err := cmd.Start(); err != nil {
		os.Remove(tmpFile.Name())
		return "", api.JobStatusFailure, fmt.Errorf("failed to start process: %w", err)
	}

	// Use the pointer address as the handle
	handle := fmt.Sprintf("%p", cmd)

	// Create a running job record
	runningJob := &RunningJob{
		cmd:    cmd,
		jobID:  job.Id.String(),
		client: r.client,
	}

	// Register the running job
	r.mu.Lock()
	r.runningJobs[handle] = runningJob
	r.mu.Unlock()

	// Spawn a goroutine to wait for the process to finish and update the job status
	go func(handle, scriptPath string) {
		defer os.Remove(scriptPath)
		defer func() {
			r.mu.Lock()
			delete(r.runningJobs, handle)
			r.mu.Unlock()
		}()

		err := cmd.Wait()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		}

		if runningJob.cancelled {
			statusUpdateFunc(runningJob.jobID, api.JobStatusCancelled, "Job was cancelled")
		} else if err != nil {
			statusUpdateFunc(runningJob.jobID, api.JobStatusFailure, fmt.Sprintf("Process exited with code %d: %v", exitCode, err))
		} else {
			statusUpdateFunc(runningJob.jobID, api.JobStatusSuccessful, "")
		}
	}(handle, tmpFile.Name())

	return handle, api.JobStatusInProgress, nil
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
				}
			}
		}
	}

	if cancelled {
		r.runningJobs = make(map[string]*RunningJob)
	}
}