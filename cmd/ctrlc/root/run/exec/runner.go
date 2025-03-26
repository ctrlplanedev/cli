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
	"time"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/pkg/jobagent"
)

var _ jobagent.Runner = &ExecRunner{}

type RunningJob struct {
	cmd       *exec.Cmd
	jobID     string
	job       *api.JobWithDetails
	cancelled bool
}

type ExecRunner struct {
	runningJobs map[string]*RunningJob
	client      *api.ClientWithResponses
	mu          sync.Mutex
}

// Helper function to update job status and handle error logging
func updateJobStatus(job *api.JobWithDetails, status api.JobStatus, message string, jobID string) {
	if err := job.UpdateStatus(status, message); err != nil {
		log.Error("Failed to update job status", "error", err, "jobId", jobID)
	}
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
		sig := <-c
		log.Info("Received shutdown signal, terminating all jobs", "signal", sig)

		// Update all job statuses before exiting
		runner.mu.Lock()
		for _, runningJob := range runner.runningJobs {
			if runningJob.job != nil {
				log.Info("Marking job as cancelled due to shutdown", "id", runningJob.jobID)
				runningJob.cancelled = true
				// Update the job status to failed with a specific message
				updateJobStatus(runningJob.job, api.JobStatusFailure, fmt.Sprintf("Job terminated due to signal: %v", sig), runningJob.jobID)
			}
		}
		runner.mu.Unlock()

		// Now terminate all processes
		runner.ExitAll(true)
		log.Info("Shutdown complete, exiting")
		os.Exit(0)
	}()

	return runner
}

// Start creates a temporary script file, starts the process, and updates job status when the process completes.
func (r *ExecRunner) Start(ctx context.Context, job *api.JobWithDetails) (api.JobStatus, error) {
	// Template the script using the job
	script, err := job.TemplateJobDetails()
	if err != nil {
		return api.JobStatusFailure, fmt.Errorf("failed to template job details: %w", err)
	}

	// Determine file extension based on OS
	ext := ".sh"
	if runtime.GOOS == "windows" {
		ext = ".ps1"
	}

	tmpFile, err := os.CreateTemp("", "script*"+ext)
	if err != nil {
		return api.JobStatusFailure, fmt.Errorf("failed to create temp script file: %w", err)
	}

	// Write the script to the temporary file
	if _, err := tmpFile.WriteString(script); err != nil {
		os.Remove(tmpFile.Name())
		return api.JobStatusFailure, fmt.Errorf("failed to write script file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return api.JobStatusFailure, fmt.Errorf("failed to close script file: %w", err)
	}

	// Make the script executable on Unix-like systems
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpFile.Name(), 0700); err != nil {
			os.Remove(tmpFile.Name())
			return api.JobStatusFailure, fmt.Errorf("failed to make script executable: %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", tmpFile.Name())
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell", "-File", tmpFile.Name())
	} else {
		// On Unix-like systems, create a new process group so we can terminate all child processes
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	// Set up command environment
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the process
	if err := cmd.Start(); err != nil {
		os.Remove(tmpFile.Name())
		return api.JobStatusFailure, fmt.Errorf("failed to start process: %w", err)
	}

	// Use the pointer address as the handle
	handle := fmt.Sprintf("%p", cmd)
	job.SetExternalID(handle)

	// Create a running job record
	runningJob := &RunningJob{
		cmd:       cmd,
		jobID:     job.Id.String(),
		job:       job,
		cancelled: false,
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
			log.Debug("Job cleanup complete", "id", runningJob.jobID)
		}()

		log.Debug("Waiting for command to complete", "id", runningJob.jobID, "handle", handle)
		err := cmd.Wait()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		}
		log.Debug("Command completed", "id", runningJob.jobID, "exitCode", exitCode, "error", err != nil)

		if runningJob.cancelled {
			updateJobStatus(job, api.JobStatusCancelled, "Job was cancelled", runningJob.jobID)
		} else if err != nil {
			updateJobStatus(job, api.JobStatusFailure, 
				fmt.Sprintf("Process exited with code %d: %v", exitCode, err),  
				runningJob.jobID)
		} else {
			updateJobStatus(job, api.JobStatusSuccessful, "", runningJob.jobID)
		}
	}(handle, tmpFile.Name())

	return api.JobStatusInProgress, nil
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

				// Update job status if cancellation requested and we have a job reference
				if cancelled && runningJob.job != nil {
					// Use JobStatusCancelled if it's an explicit cancellation, or JobStatusFailure if it's due to external termination
					status := api.JobStatusCancelled
					message := "Job was cancelled by user"

					// Update the status
					updateJobStatus(runningJob.job, status, message, runningJob.jobID)
				}

				// Kill the process
				killProcess(runningJob.cmd, runningJob.jobID)
			}
		}
	}

	if cancelled {
		r.runningJobs = make(map[string]*RunningJob)
	}
}

// killProcess terminates a process and its children in a cross-platform way without exposing PIDs
func killProcess(cmd *exec.Cmd, jobID string) {
	if runtime.GOOS == "windows" {
		pid := cmd.Process.Pid
		if err := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run(); err != nil {
			log.Error("Failed to kill process tree", "error", err, "jobId", jobID)
		}
	} else {
		pgid, err := syscall.Getpgid(cmd.Process.Pid)
		if err == nil && pgid > 0 {
			if err = syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
				log.Error("Failed to terminate process group gracefully", "error", err, "jobId", jobID)
			}
		} else {
			if err = cmd.Process.Signal(syscall.SIGTERM); err != nil {
				log.Error("Failed to terminate process gracefully", "error", err, "jobId", jobID)
			}
		}

		go func() {
			time.Sleep(2 * time.Second)
			if err := cmd.Process.Kill(); err != nil {
				log.Error("Failed to kill process", "error", err, "jobId", jobID)
			}
		}()
	}
}
