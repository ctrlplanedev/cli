//go:build !linux && !darwin

package ptysession

import (
	"fmt"
	"runtime"
)

func getUserShell(username string) (string, error) {
	return "", fmt.Errorf("operating system %s not supported", runtime.GOOS)
}
