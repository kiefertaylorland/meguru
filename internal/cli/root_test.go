package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRootCommand_RegistersReviewSubcommand(t *testing.T) {
	root := NewRootCommand()

	reviewCmd, _, err := root.Find([]string{"review"})
	require.NoError(t, err)
	require.Equal(t, "review", reviewCmd.Name())
}

func TestNewRootCommand_SilencesUsageAndErrors(t *testing.T) {
	root := NewRootCommand()

	require.True(t, root.SilenceUsage)
	require.True(t, root.SilenceErrors)
}

// With no subcommand, root prints help and exits 0 (M1 scope).
func TestNewRootCommand_NoArgsPrintsHelpAndReturnsNil(t *testing.T) {
	root := NewRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{})

	err := root.Execute()

	require.NoError(t, err)
	require.Contains(t, out.String(), "meguru")
}
