package exec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"runtime"

	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/pkg/jobagent"
)

var _ jobagent.Runner = &ExecRunner{}

type ExecRunner struct{}

type ExecConfig struct {
	WorkingDir string `json:"workingDir,omitempty"`
	Script     string `json:"script"`
}

// Start creates a temporary script file, starts the process, and updates job status when the process completes.
func (r *ExecRunner) Start(job api.Job, jobDetails map[string]interface{}, statusUpdateFunc func(jobID string, status api.JobStatus, message string)) (string, api.JobStatus, error) {
	// Determine file extension based on OS.
	ext := ".sh"
	if runtime.GOOS == "windows" {
		ext = ".ps1"
	}

	tmpFile, err := os.CreateTemp("", "script*"+ext)
	if err != nil {
		return "", api.JobStatusFailure, fmt.Errorf("failed to create temp script file: %w", err)
	}

	config := ExecConfig{}
	jsonBytes, err := json.Marshal(job.JobAgentConfig)
	if err != nil {
		return "", api.JobStatusFailure, fmt.Errorf("failed to marshal job agent config: %w", err)
	}
	if err := json.Unmarshal(jsonBytes, &config); err != nil {
		return "", api.JobStatusFailure, fmt.Errorf("failed to unmarshal job agent config: %w", err)
	}

	templatedScript, err := template.New("script").Parse(config.Script)
	if err != nil {
		return "", api.JobStatusFailure, fmt.Errorf("failed to parse script template: %w", err)
	}

	buf := new(bytes.Buffer)
	if err := templatedScript.Execute(buf, jobDetails); err != nil {
		return "", api.JobStatusFailure, fmt.Errorf("failed to execute script template: %w", err)
	}
	script := buf.String()

	if _, err := tmpFile.WriteString(script); err != nil {
		return "", api.JobStatusFailure, fmt.Errorf("failed to write script file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", api.JobStatusFailure, fmt.Errorf("failed to close script file: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpFile.Name(), 0700); err != nil {
			return "", api.JobStatusFailure, fmt.Errorf("failed to make script executable: %w", err)
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
		return "", api.JobStatusFailure, fmt.Errorf("failed to start process: %w", err)
	}

	// Use the pointer address as the handle
	handle := fmt.Sprintf("%p", cmd)

	// Spawn a goroutine to wait for the process to finish and update the job status
	go func(handle, scriptPath string) {
		defer os.Remove(scriptPath)

		err := cmd.Wait()

		if err != nil {
			log.Error("Process execution failed", "handle", handle, "error", err)
			statusUpdateFunc(job.Id.String(), api.JobStatusFailure, err.Error())
		} else {
			log.Info("Process execution succeeded", "handle", handle)
			statusUpdateFunc(job.Id.String(), api.JobStatusSuccessful, "")
		}
	}(handle, tmpFile.Name())

	return handle, api.JobStatusInProgress, nil
}
