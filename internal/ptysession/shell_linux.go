//go:build linux

package ptysession

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func getUserShell(username string) (string, error) {
	file, err := os.Open("/etc/passwd")
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			fields := strings.Split(line, ":")
			if len(fields) < 7 {
				continue
			}
			if fields[0] == username {
				shell := fields[6]
				return shell, nil
			}
		}
		if err := scanner.Err(); err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("user %s not found in /etc/passwd: %w", username, err)
}
