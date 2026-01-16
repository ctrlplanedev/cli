//go:build darwin

package ptysession

import (
	"fmt"
	"os/exec"
	"strings"
)

func getUserShell(username string) (string, error) {
	cmd := exec.Command("dscl", ".", "-read", fmt.Sprintf("/Users/%s", username), "UserShell")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get user shell on macOS: %v", err)
	}

	// Parse the output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "UserShell:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1], nil
			}
		}
	}
	return "", fmt.Errorf("shell not found in dscl output")
}
