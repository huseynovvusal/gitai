package git

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// TestHelperProcess isn't a real test. It's a helper process that acts as a fake command.
// It's executed by the *exec.Cmd created in the mock command functions.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	fmt.Fprint(os.Stdout, os.Getenv("STDOUT"))
	fmt.Fprint(os.Stderr, os.Getenv("STDERR"))

	i, err := strconv.Atoi(os.Getenv("EXIT_CODE"))
	if err != nil {
		os.Exit(1)
	}
	os.Exit(i)
}

// newMockExecCommand returns a function that mocks exec.Command for a single command call.
func newMockExecCommand(t *testing.T, expectedArgs []string, stdout, stderr string, exitCode int) func(string, ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		fullArgs := append([]string{name}, args...)
		if !reflect.DeepEqual(fullArgs, expectedArgs) {
			t.Errorf("expected command %v, got %v", expectedArgs, fullArgs)
		}

		cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", name)
		cmd.Args = append(cmd.Args, args...)
		cmd.Env = []string{
			"GO_WANT_HELPER_PROCESS=1",
			fmt.Sprintf("STDOUT=%s", stdout),
			fmt.Sprintf("STDERR=%s", stderr),
			fmt.Sprintf("EXIT_CODE=%d", exitCode),
		}
		return cmd
	}
}

type mockCall struct {
	expectedArgs []string
	stdout       string
	stderr       string
	exitCode     int
}

// newMultiMockExecCommand returns a function that mocks exec.Command for a sequence of calls.
func newMultiMockExecCommand(t *testing.T, calls []mockCall) func(string, ...string) *exec.Cmd {
	callNum := 0
	return func(name string, args ...string) *exec.Cmd {
		if callNum >= len(calls) {
			t.Fatalf("unexpected command call: %s %v", name, args)
		}
		c := calls[callNum]
		callNum++

		fullArgs := append([]string{name}, args...)
		if !reflect.DeepEqual(fullArgs, c.expectedArgs) {
			t.Errorf("call %d: expected command %v, got %v", callNum-1, c.expectedArgs, fullArgs)
		}

		cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", name)
		cmd.Args = append(cmd.Args, args...)
		cmd.Env = []string{
			"GO_WANT_HELPER_PROCESS=1",
			fmt.Sprintf("STDOUT=%s", c.stdout),
			fmt.Sprintf("STDERR=%s", c.stderr),
			fmt.Sprintf("EXIT_CODE=%d", c.exitCode),
		}
		return cmd
	}
}

func TestGetStatusForFiles(t *testing.T) {
	testCases := []struct {
		name            string
		files           []string
		stdout          string
		stderr          string
		exitCode        int
		expected        string
		expectedErr     string
		expectedCmdArgs []string
	}{
		{
			name:     "no files",
			files:    []string{},
			expected: "",
		},
		{
			name:  "files with status",
			files: []string{"file1.go", "file2.go"},
			stdout: " M file1.go\n" +
				" A file3.go\n" +
				" D file4.go\n" +
				"?? file2.go",
			expected:        " M file1.go\n?? file2.go",
			expectedCmdArgs: []string{"git", "status", "--porcelain", "--", "file1.go", "file2.go"},
		},
		{
			name:            "renamed file",
			files:           []string{"new-name.go"},
			stdout:          "R  new-name.go -> old-name.go",
			expected:        "R  new-name.go -> old-name.go",
			expectedCmdArgs: []string{"git", "status", "--porcelain", "--", "new-name.go"},
		},
		{
			name:            "renamed file string",
			files:           []string{"old.txt -> new.txt"},
			stdout:          "R  old.txt -> new.txt",
			expected:        "R  old.txt -> new.txt",
			expectedCmdArgs: []string{"git", "status", "--porcelain", "--", "old.txt", "new.txt"},
		},
		{
			name:            "git status error",
			files:           []string{"file1.go"},
			stdout:          "git error", // CombinedOutput puts stderr into the output buffer
			exitCode:        1,
			expectedErr:     "failed to run git status",
			expectedCmdArgs: []string{"git", "status", "--porcelain", "--", "file1.go"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectedCmdArgs != nil {
				command = newMockExecCommand(t, tc.expectedCmdArgs, tc.stdout, tc.stderr, tc.exitCode)
				defer func() { command = exec.Command }()
			}

			result, err := GetStatusForFiles(tc.files)

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

func TestGetChangedFiles(t *testing.T) {
	testCases := []struct {
		name            string
		stdout          string
		stderr          string
		exitCode        int
		expected        []string
		expectedErr     string
		expectedCmdArgs []string
	}{
		{
			name:            "files with status",
			stdout:          " M file1.go\n A file2.go",
			expected:        []string{"file1.go", "file2.go"},
			expectedCmdArgs: []string{"git", "status", "--porcelain"},
		},
		{
			name:            "no changed files",
			stdout:          "",
			expected:        nil,
			expectedCmdArgs: []string{"git", "status", "--porcelain"},
		},
		{
			name:            "git status error",
			stderr:          "git error",
			exitCode:        1,
			expectedErr:     "exit status 1",
			expectedCmdArgs: []string{"git", "status", "--porcelain"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			command = newMockExecCommand(t, tc.expectedCmdArgs, tc.stdout, tc.stderr, tc.exitCode)
			defer func() { command = exec.Command }()

			result, err := GetChangedFiles()

			if err != nil && tc.expectedErr == "" {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && tc.expectedErr != "" {
				t.Fatalf("expected error but got none")
			}
			if err != nil && !strings.Contains(err.Error(), tc.expectedErr) {
				t.Fatalf("expected error '%s', got '%s'", tc.expectedErr, err.Error())
			}
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("expected '%v', got '%v'", tc.expected, result)
			}
		})
	}
}

func TestGetChangesForFiles(t *testing.T) {
	testCases := []struct {
		name            string
		files           []string
		stdout          string
		stderr          string
		exitCode        int
		expected        string
		expectedErr     string
		expectedCmdArgs []string
	}{
		{
			name:     "no files",
			files:    []string{},
			expected: "",
		},
		{
			name:  "files with changes",
			files: []string{"file1.go", "file2.go"},
			stdout: "diff --git a/file1.go b/file1.go\n" +
				"--- a/file1.go\n" +
				"+++ b/file1.go\n" +
				"@@ -1,1 +1,1 @@\n" +
				"-hello\n" +
				"+world\n",
			expected: "diff --git a/file1.go b/file1.go\n" +
				"--- a/file1.go\n" +
				"+++ b/file1.go\n" +
				"@@ -1,1 +1,1 @@\n" +
				"-hello\n" +
				"+world\n",
			expectedCmdArgs: []string{"git", "diff", "HEAD", "--", "file1.go", "file2.go"},
		},
		{
			name:            "git diff error",
			files:           []string{"file1.go"},
			stderr:          "git error",
			exitCode:        1,
			expectedErr:     "git diff failed: exit status 1\ngit error",
			expectedCmdArgs: []string{"git", "diff", "HEAD", "--", "file1.go"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectedCmdArgs != nil {
				command = newMockExecCommand(t, tc.expectedCmdArgs, tc.stdout, tc.stderr, tc.exitCode)
				defer func() { command = exec.Command }()
			}

			result, err := GetChangesForFiles(tc.files)

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

func TestCommit(t *testing.T) {
	testCases := []struct {
		name        string
		files       []string
		message     string
		calls       []mockCall
		expectedErr string
	}{
		{
			name:        "no files",
			files:       []string{},
			message:     "test commit",
			expectedErr: "no files provided to commit",
		},
		{
			name:    "successful commit",
			files:   []string{"file1.go"},
			message: "test commit",
			calls: []mockCall{
				{expectedArgs: []string{"git", "ls-files", "--", "file1.go"}, stdout: "file1.go"},
				{expectedArgs: []string{"git", "diff", "--name-only", "--cached", "--", "file1.go"}},
				{expectedArgs: []string{"git", "add", "--", "file1.go"}},
				{expectedArgs: []string{"git", "commit", "-m", "test commit", "--", "file1.go"}, stdout: "[main 12345] test commit"},
			},
		},
		{
			name:    "git add fails",
			files:   []string{"file1.go"},
			message: "test commit",
			calls: []mockCall{
				{expectedArgs: []string{"git", "ls-files", "--", "file1.go"}, stdout: "file1.go"},
				{expectedArgs: []string{"git", "diff", "--name-only", "--cached", "--", "file1.go"}},
				{expectedArgs: []string{"git", "add", "--", "file1.go"}, stdout: "error adding", exitCode: 1},
			},
			expectedErr: "failed to stage files: exit status 1\nerror adding",
		},
		{
			name:    "git commit fails",
			files:   []string{"file1.go"},
			message: "test commit",
			calls: []mockCall{
				{expectedArgs: []string{"git", "ls-files", "--", "file1.go"}, stdout: "file1.go"},
				{expectedArgs: []string{"git", "diff", "--name-only", "--cached", "--", "file1.go"}},
				{expectedArgs: []string{"git", "add", "--", "file1.go"}},
				{expectedArgs: []string{"git", "commit", "-m", "test commit", "--", "file1.go"}, stdout: "error committing", exitCode: 1},
			},
			expectedErr: "git commit failed: exit status 1\nerror committing",
		},
		{
			name:    "nothing to commit",
			files:   []string{"file1.go"},
			message: "test commit",
			calls: []mockCall{
				{expectedArgs: []string{"git", "ls-files", "--", "file1.go"}, stdout: "file1.go"},
				{expectedArgs: []string{"git", "diff", "--name-only", "--cached", "--", "file1.go"}},
				{expectedArgs: []string{"git", "add", "--", "file1.go"}},
				{expectedArgs: []string{"git", "commit", "-m", "test commit", "--", "file1.go"}, stdout: "nothing to commit, working tree clean", exitCode: 1},
			},
			expectedErr: "", // Should not return an error
		},
		{
			name:    "commit rename",
			files:   []string{"old.txt -> new.txt"},
			message: "rename commit",
			calls: []mockCall{
				{expectedArgs: []string{"git", "ls-files", "--", "old.txt", "new.txt"}, stdout: "new.txt"},
				{expectedArgs: []string{"git", "diff", "--name-only", "--cached", "--", "old.txt", "new.txt"}, stdout: "old.txt"},
				{expectedArgs: []string{"git", "add", "--", "new.txt"}},
				{expectedArgs: []string{"git", "commit", "-m", "rename commit", "--", "new.txt", "old.txt"}, stdout: "[main 12345] rename commit"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.calls != nil {
				command = newMultiMockExecCommand(t, tc.calls)
				defer func() { command = exec.Command }()
			}

			err := Commit(tc.files, tc.message)

			if err != nil && tc.expectedErr == "" {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && tc.expectedErr != "" {
				t.Fatalf("expected error but got none")
			}
			if err != nil && !strings.Contains(err.Error(), tc.expectedErr) {
				t.Fatalf("expected error '%s', got '%s'", tc.expectedErr, err.Error())
			}
		})
	}
}

func TestPush(t *testing.T) {
	testCases := []struct {
		name            string
		stdout          string
		stderr          string
		exitCode        int
		expectedErr     string
		expectedCmdArgs []string
	}{
		{
			name:            "successful push",
			stdout:          "Everything up-to-date",
			expectedCmdArgs: []string{"git", "push"},
		},
		{
			name:            "push fails",
			stdout:          "fatal: No configured push destination",
			exitCode:        128,
			expectedErr:     "git push failed: fatal: No configured push destination",
			expectedCmdArgs: []string{"git", "push"},
		},
	}

	for _, tc := range testCases {
		        t.Run(tc.name, func(t *testing.T) {
		            command = newMockExecCommand(t, tc.expectedCmdArgs, tc.stdout, tc.stderr, tc.exitCode)
		            defer func() { command = exec.Command }()
		
		            _, err := Push()
		
		            if err != nil && tc.expectedErr == "" {
		                t.Fatalf("unexpected error: %v", err)
		            }
		            if err == nil && tc.expectedErr != "" {
		                t.Fatalf("expected error but got none")
		            }
		            if err != nil && !strings.Contains(err.Error(), tc.expectedErr) {
		                t.Fatalf("expected error '%s', got '%s'", tc.expectedErr, err.Error())
		            }
		        })	}
}

func TestGetPullRequestURL(t *testing.T) {
	// We need to mock command calls for "rev-parse" and "remote get-url"
	// Sequence of calls:
	// 1. git rev-parse --abbrev-ref HEAD -> returns branch
	// 2. git remote get-url origin -> returns remote URL

	tests := []struct {
		name           string
		mockBranch     string
		mockRemote     string
		expectedURL    string
		expectedErr    string
		cmdErr         bool // if true, simulates command failure
	}{
		{
			name:        "github https",
			mockBranch:  "feature/abc",
			mockRemote:  "https://github.com/user/repo.git",
			expectedURL: "https://github.com/user/repo/pull/new/feature/abc",
		},
		{
			name:        "github ssh",
			mockBranch:  "main",
			mockRemote:  "git@github.com:user/repo.git",
			expectedURL: "https://github.com/user/repo/pull/new/main",
		},
		{
			name:        "gitlab ssh",
			mockBranch:  "fix/123",
			mockRemote:  "git@gitlab.com:org/group/project.git",
			expectedURL: "https://gitlab.com/org/group/project/-/merge_requests/new?merge_request[source_branch]=fix/123",
		},
		{
			name:        "bitbucket ssh",
			mockBranch:  "dev",
			mockRemote:  "git@bitbucket.org:user/repo.git",
			expectedURL: "https://bitbucket.org/user/repo/pull-requests/new?source=dev",
		},
		{
			name:        "unknown host",
			mockBranch:  "main",
			mockRemote:  "git@example.com:user/repo.git",
			expectedErr: "unknown remote host: https://example.com/user/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			command = func(name string, args ...string) *exec.Cmd {
				callCount++
				
				var output string
				if len(args) > 0 && args[0] == "rev-parse" {
					output = tt.mockBranch
				} else if len(args) > 0 && args[0] == "remote" {
					output = tt.mockRemote
				} else {
					output = ""
				}

				cs := []string{"-test.run=TestHelperProcess", "--", name}
				cs = append(cs, args...)
				cmd := exec.Command(os.Args[0], cs...)
				env := []string{"GO_WANT_HELPER_PROCESS=1", "STDOUT=" + output}
				if tt.cmdErr {
					env = append(env, "EXIT_CODE=1")
				} else {
					env = append(env, "EXIT_CODE=0")
				}
				cmd.Env = env
				return cmd
			}

			url, err := GetPullRequestURL()
			if tt.expectedErr != "" {
				if err == nil || err.Error() != tt.expectedErr {
					t.Errorf("expected error %v, got %v", tt.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if url != tt.expectedURL {
					t.Errorf("expected url %s, got %s", tt.expectedURL, url)
				}
			}
		})
	}
}
