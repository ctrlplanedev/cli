package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/pkg/jobagent"
)

var _ jobagent.Runner = &ExecRunner{}

type ProcessInfo struct {
	cmd       *exec.Cmd
	finished  error
	startTime time.Time
}
type ExecRunner struct {
	mu        sync.Mutex
	processes map[string]*ProcessInfo

	// For graceful shutdown
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
}

func NewExecRunner() *ExecRunner {
	ctx, cancel := context.WithCancel(context.Background())

	runner := &ExecRunner{
		processes:  make(map[string]*ProcessInfo),
		ctx:        ctx,
		cancelFunc: cancel,
	}

	runner.wg.Add(1)
	go runner.startHousekeeping()

	return runner
}

func (r *ExecRunner) Shutdown() {
	if r.cancelFunc != nil {
		r.cancelFunc()
		r.wg.Wait()
	}
}

func (r *ExecRunner) startHousekeeping() {
	defer r.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.cleanupOldProcesses(time.Now(), 1*time.Minute)
		case <-r.ctx.Done():
			log.Debug("Housekeeping goroutine shutting down")
			return
		}
	}
}

func (r *ExecRunner) cleanupOldProcesses(now time.Time, retentionPeriod time.Duration) {

	r.mu.Lock()
	defer r.mu.Unlock()

	for handle, proc := range r.processes {
		if proc.finished != nil {
			age := now.Sub(proc.startTime)
			if age > retentionPeriod {
				log.Debug("Cleaning up old process", "handle", handle, "age", age.String())
				delete(r.processes, handle)
			}
		}
	}
}

type ExecConfig struct {
	WorkingDir string `json:"workingDir,omitempty"`
	Script     string `json:"script"`
}

// Status looks up the process using the unique ID (stored in job.ExternalId)
// rather than the PID. It returns the status based on whether the process has
// finished and if so, whether it exited with an error.
func (r *ExecRunner) Status(job api.Job) (api.JobStatus, string) {
	if job.ExternalId == nil {
		return api.JobStatusExternalRunNotFound, "external ID is nil"
	}
	handle := *job.ExternalId

	r.mu.Lock()
	proc, exists := r.processes[handle]
	r.mu.Unlock()

	if !exists {
		return api.JobStatusExternalRunNotFound, "process info not found"
	}

	if proc.cmd.ProcessState != nil {
		if proc.finished != nil && proc.finished.Error() != "" {
			return api.JobStatusFailure, fmt.Sprintf("process exited with error: %s", proc.finished.Error())
		}
		return api.JobStatusSuccessful, "process completed successfully"
	}

	// Process is still running
	return api.JobStatusInProgress, fmt.Sprintf("process running with handle %s", handle)
}

// Start creates a temporary script file, starts the process, and stores a unique
// identifier along with the process handle in the runner's in-memory map.
func (r *ExecRunner) Start(job api.Job, jobDetails map[string]interface{}) (string, error) {
	// Determine file extension based on OS.
	ext := ".sh"
	if runtime.GOOS == "windows" {
		ext = ".ps1"
	}

	tmpFile, err := os.CreateTemp("", "script*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp script file: %w", err)
	}

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
	if err := templatedScript.Execute(buf, jobDetails); err != nil {
		return "", fmt.Errorf("failed to execute script template: %w", err)
	}
	script := buf.String()

	if _, err := tmpFile.WriteString(script); err != nil {
		return "", fmt.Errorf("failed to write script file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close script file: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpFile.Name(), 0700); err != nil {
			return "", fmt.Errorf("failed to make script executable: %w", err)
		}
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-File", tmpFile.Name())
	} else {
		cmd = exec.Command("bash", "-c", tmpFile.Name())
	}

	cmd.Dir = config.WorkingDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to start process: %w", err)
	}

	// Create the ProcessInfo object
	procInfo := &ProcessInfo{
		cmd:       cmd,
		startTime: time.Now(),
	}

	// Use the pointer address as the handle
	handle := fmt.Sprintf("%p", procInfo)

	// Store the process handle in the runner's map.
	r.mu.Lock()
	r.processes[handle] = procInfo
	r.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	go func(ctx context.Context, handle, scriptPath string) {
		defer cancel()
		defer os.Remove(scriptPath)

		err := cmd.Wait()

		r.mu.Lock()
		if proc, exists := r.processes[handle]; exists {
			proc.finished = err
		}
		r.mu.Unlock()

		if err != nil {
			log.Error("Process execution failed", "handle", handle, "error", err)
		} else {
			log.Info("Process execution succeeded", "handle", handle)
		}
	}(ctx, handle, tmpFile.Name())

	return handle, nil
}
