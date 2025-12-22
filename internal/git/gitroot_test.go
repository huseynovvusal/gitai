package git

import (
	"os/exec"
	"strings"
	"testing"
)

func TestGetGitRoot(t *testing.T) {
	testCases := []struct {
		name            string
		stdout          string
		stderr          string
		exitCode        int
		expected        string
		expectedErr     string
		expectedCmdArgs []string
	}{
		{
			name:            "successful",
			stdout:          "/path/to/repo\n",
			expected:        "/path/to/repo",
			expectedCmdArgs: []string{"git", "rev-parse", "--show-toplevel"},
		},
		{
			name:            "git error",
			stderr:          "fatal: not a git repository",
			exitCode:        128,
			expectedErr:     "exit status 128",
			expectedCmdArgs: []string{"git", "rev-parse", "--show-toplevel"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			command = newMockExecCommand(t, tc.expectedCmdArgs, tc.stdout, tc.stderr, tc.exitCode)
			defer func() { command = exec.Command }()

			result, err := GetGitRoot()

			if err != nil && tc.expectedErr == "" {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && tc.expectedErr != "" {
				t.Fatalf("expected error but got none")
			}
			if err != nil && !strings.Contains(err.Error(), tc.expectedErr) {
				t.Fatalf("expected error '%s', got '%s'", tc.expectedErr, err.Error())
			}
			if result != tc.expected {
				t.Errorf("expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}
