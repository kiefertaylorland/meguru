package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldUsePlain(t *testing.T) {
	cases := []struct {
		name        string
		plainFlag   bool
		stdoutIsTTY bool
		want        bool
	}{
		{"plain flag forces it even on a TTY", true, true, true},
		{"plain flag forces it on a non-TTY too", true, false, true},
		{"non-TTY forces it without the flag", false, false, true},
		{"TTY without the flag uses the interactive TUI", false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, shouldUsePlain(tc.plainFlag, tc.stdoutIsTTY))
		})
	}
}

func TestProgramOptions_NoColorSet(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	opts := programOptions()

	require.Len(t, opts, 1)
}

func TestProgramOptions_NoColorUnset(t *testing.T) {
	require.NoError(t, os.Unsetenv("NO_COLOR"))

	opts := programOptions()

	require.Empty(t, opts)
}

func TestNewReviewCommand_RegistersPlainFlag(t *testing.T) {
	cmd := newReviewCommand()

	flag := cmd.Flags().Lookup("plain")
	require.NotNil(t, flag)
	require.Equal(t, "false", flag.DefValue)
}
