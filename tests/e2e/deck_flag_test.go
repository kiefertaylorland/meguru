package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// --deck with a recognized slug scopes a --plain session to that deck only
// (007-deck-filter FR-001, FR-003).
func TestReview_DeckFlag_ScopesPlainSession(t *testing.T) {
	bin := buildBinary(t)
	dataDir := t.TempDir()

	cmd := exec.Command(bin, "review", "--plain", "--deck", "kana-hiragana")
	cmd.Env = withCoverEnv(append(cmd.Environ(), "XDG_DATA_HOME="+dataDir))

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stdin = strings.NewReader("a\ngood\n")

	require.NoError(t, cmd.Run())
	require.Contains(t, stdout.String(), "Expression:")
}

// An unrecognized --deck value fails clearly, before any database work, and
// exits non-zero (FR-004, SC-003).
func TestReview_DeckFlag_UnrecognizedValueFailsClearly(t *testing.T) {
	bin := buildBinary(t)
	dataDir := t.TempDir()

	cmd := exec.Command(bin, "review", "--plain", "--deck", "bogus")
	cmd.Env = withCoverEnv(append(cmd.Environ(), "XDG_DATA_HOME="+dataDir))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	require.Error(t, err, "an unrecognized --deck value must exit non-zero")
	combined := stdout.String() + stderr.String()
	require.Contains(t, combined, `unknown deck "bogus"`)
	require.Contains(t, combined, "kana-hiragana")

	_, statErr := os.Stat(filepath.Join(dataDir, "meguru", "meguru.db"))
	require.True(t, os.IsNotExist(statErr), "an invalid --deck value must fail before any database file is created")
}
