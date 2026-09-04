package storage

import (
	"os"
	"path/filepath"
)

// EnsureDir creates the default ~/.meguru directory if it does not exist.
func EnsureDir() error {
	dir := filepath.Join(os.UserHomeDir(), ".meguru")
	return os.MkdirAll(dir, 0o755)
}

// WriteDefaultConfig writes a default config file to ~/.meguru/config.json.
func WriteDefaultConfig() error {
	dir := filepath.Join(os.UserHomeDir(), ".meguru")
	path := filepath.Join(dir, "config.json")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(`{"review_interval_days": 1}`)
	return err
}

// WriteDefaultDeck writes a sample deck JSON to ~/.meguru/deck.json.
func WriteDefaultDeck() error {
	dir := filepath.Join(os.UserHomeDir(), ".meguru")
	path := filepath.Join(dir, "deck.json")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(`[{"front":"hello","back":"こんにちは","hint":"greeting"}]`)
	return err
}