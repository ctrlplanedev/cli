package cliutil

import (
	"github.com/charmbracelet/huh"
)

// ConfirmAction prompts the user with a yes/no confirmation using charmbracelet/huh.
func ConfirmAction(message string) (bool, error) {
	var confirmed bool
	err := huh.NewConfirm().
		Title(message).
		Value(&confirmed).
		Run()
	if err != nil {
		return false, err
	}
	return confirmed, nil
}
