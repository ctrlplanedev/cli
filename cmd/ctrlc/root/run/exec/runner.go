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
	"strconv"
	"syscall"

	"github.com/ctrlplanedev/cli/internal/api"
	"github.com/ctrlplanedev/cli/pkg/jobagent"
	"github.com/spf13/viper"
)

var _ jobagent.Runner = &ExecRunner{}

type ExecRunner struct{}

type ExecConfig struct {
	WorkingDir string `json:"workingDir,omitempty"`
	Script     string `json:"script"`
}

func (r *ExecRunner) Status(job api.Job) (api.JobStatus, string) {
	if job.ExternalId == nil {
		return api.JobStatusExternalRunNotFound, fmt.Sprintf("external ID is nil: %v", job.ExternalId)
	}

	externalId, err := strconv.Atoi(*job.ExternalId)
	if err != nil {
		return api.JobStatusExternalRunNotFound, fmt.Sprintf("invalid process id: %v", err)
	}

	process, err := os.FindProcess(externalId)
	if err != nil {
		return api.JobStatusExternalRunNotFound, fmt.Sprintf("failed to find process: %v", err)
	}

	// On Unix systems, FindProcess always succeeds, so we need to send signal 0
	// to check if process exists
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return api.JobStatusSuccessful, fmt.Sprintf("process not running: %v", err)
	}

	return api.JobStatusInProgress, fmt.Sprintf("process running with pid %d", externalId)
}

func (r *ExecRunner) Start(job api.Job) (string, error) {
	// Create temp script file
	ext := ".sh"
	if runtime.GOOS == "windows" {
		ext = ".ps1"
	}

	tmpFile, err := os.CreateTemp("", "script*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp script file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	config := ExecConfig{}
	jsonBytes, err := json.Marshal(job.JobAgentConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal job agent config: %w", err)
	}
	if err := json.Unmarshal(jsonBytes, &config); err != nil {
		return "", fmt.Errorf("failed to unmarshal job agent config: %w", err)
	}

	client, err := api.NewAPIKeyClientWithResponses(
		viper.GetString("url"),
		viper.GetString("api-key"),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create API client for job details: %w", err)
	}

	resp, err := client.GetJobWithResponse(context.Background(), job.Id.String())
	if err != nil {
		return "", fmt.Errorf("failed to get job details: %w", err)
	}

	if resp.JSON200 == nil {
		return "", fmt.Errorf("received empty response from job details API")
	}

	var jobDetails map[string]interface{}
	detailsBytes, _ := json.Marshal(resp.JSON200)
	json.Unmarshal(detailsBytes, &jobDetails)

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

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to execute script: %w", err)
	}

	return strconv.Itoa(cmd.Process.Pid), nil
}
