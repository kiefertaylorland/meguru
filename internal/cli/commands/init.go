package commands

import (
	"fmt"

	"meguru/internal/storage"
)

// InitCommand creates the default ~/.meguru directory with a starter deck
// and config file if it does not already exist.
func InitCommand() error {
	if err := storage.EnsureDir(); err != nil {
		return fmt.Errorf("init: %w", err)
	}
	if err := storage.WriteDefaultConfig(); err != nil {
		return fmt.Errorf("init: %w", err)
	}
	if err := storage.WriteDefaultDeck(); err != nil {
		return fmt.Errorf("init: %w", err)
	}
	fmt.Println("Initialized ~/.meguru with default config and starter deck.")
	return nil
}