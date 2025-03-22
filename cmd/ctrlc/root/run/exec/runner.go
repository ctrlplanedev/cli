package exec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/pkg/jobagent"
)

var _ jobagent.Runner = &ExecRunner{}

type ExecRunner struct {
	mu       sync.Mutex
	finished map[int]error
}

func NewExecRunner() *ExecRunner {
	return &ExecRunner{
		finished: make(map[int]error),
	}
}

type ExecConfig struct {
	WorkingDir string `json:"workingDir,omitempty"`
	Script     string `json:"script"`
}

func (r *ExecRunner) Status(job api.Job) (api.JobStatus, string) {
	if job.ExternalId == nil {
		return api.JobStatusExternalRunNotFound, fmt.Sprintf("external ID is nil: %v", job.ExternalId)
	}

	pid, err := strconv.Atoi(*job.ExternalId)
	if err != nil {
		return api.JobStatusExternalRunNotFound, fmt.Sprintf("invalid process id: %v", err)
	}

	// Check if we've recorded a finished result for this process
	r.mu.Lock()
	finishedErr, exists := r.finished[pid]
	r.mu.Unlock()
	if exists {
		if finishedErr != nil {
			return api.JobStatusFailure, fmt.Sprintf("process exited with error: %v", finishedErr)
		}
		return api.JobStatusSuccessful, "process completed successfully"
	}

	// If not finished yet, try to check if the process is still running.
	process, err := os.FindProcess(pid)
	if err != nil {
		return api.JobStatusExternalRunNotFound, fmt.Sprintf("failed to find process: %v", err)
	}
	// On Unix, Signal 0 will error if the process is not running.
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Process is not running but we haven't recorded its result.
		return api.JobStatusFailure, fmt.Sprintf("process not running: %v", err)
	}
	return api.JobStatusInProgress, fmt.Sprintf("process running with pid %d", pid)
}

func (r *ExecRunner) Start(job api.Job, jobDetails map[string]interface{}) (string, error) {
	// Create temp script file
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

	// Write script contents
	if _, err := tmpFile.WriteString(script); err != nil {
		return "", fmt.Errorf("failed to write script file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close script file: %w", err)
	}

	// Make executable on Unix systems
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

	pid := cmd.Process.Pid

	// Launch a goroutine to wait for process completion and store the result.
	go func(pid int, scriptPath string) {
		err := cmd.Wait()
		// Ensure the map is not nil; if there's any chance ExecRunner is used as a zero-value, initialize it.
		r.mu.Lock()
		if r.finished == nil {
			r.finished = make(map[int]error)
		}
		r.finished[pid] = err
		r.mu.Unlock()
		
		if err != nil {
			log.Error("Process execution failed", "pid", pid, "error", err)
		} else {
			log.Info("Process execution succeeded", "pid", pid)
		}

		os.Remove(scriptPath)
	}(pid, tmpFile.Name())

	return strconv.Itoa(cmd.Process.Pid), nil
}
