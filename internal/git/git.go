package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/crypto/ssh"
)

// openRepo opens the git repository from the current directory or a parent.
func openRepo() (*git.Repository, error) {
	root, err := GetGitRoot()
	if err != nil {
		return nil, err
	}
	return git.PlainOpen(root)
}

// GetStatusForFiles returns the `git status --porcelain` style output, but only for
// the specified files.
func GetStatusForFiles(files []string) (string, error) {
	if len(files) == 0 {
		return "", nil
	}

	r, err := openRepo()
	if err != nil {
		return "", err
	}

	w, err := r.Worktree()
	if err != nil {
		return "", err
	}

	status, err := w.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get git status: %w", err)
	}

	var sb strings.Builder
	// Filter status for specific files
	// status is a map[string]*FileStatus
	for _, file := range files {
		// go-git paths are relative to repo root
		relPath, err := toRepoRelativePath(file)
		if err != nil {
			continue
		}

		if s, ok := status[relPath]; ok {
			// Format similar to git status --porcelain: "XY path"
			// X = Staging, Y = Worktree
			x := string(s.Staging)
			y := string(s.Worktree)
			if x == "?" {
				x = "?"
			}
			if y == "?" {
				y = "?"
			}
			// go-git uses ' ' for unmodified, match porcelain output
			if x == "\x00" { x = " " }
			if y == "\x00" { y = " " }

			sb.WriteString(fmt.Sprintf("%s%s %s\n", x, y, relPath))
		}
	}

	return sb.String(), nil
}

// GetChangedFiles returns a list of changed (modified, new, etc.) files.
func GetChangedFiles() ([]string, error) {
	r, err := openRepo()
	if err != nil {
		return nil, err
	}

	w, err := r.Worktree()
	if err != nil {
		return nil, err
	}

	status, err := w.Status()
	if err != nil {
		return nil, err
	}

	var changedFiles []string
	for path, s := range status {
		// Check if file has changes (Staging or Worktree is not Unmodified)
		if s.Staging != git.Unmodified || s.Worktree != git.Unmodified {
			changedFiles = append(changedFiles, path)
		}
	}
	sort.Strings(changedFiles)
	return changedFiles, nil
}

// GetChangesForFiles returns the git diff for the specified files against HEAD.
func GetChangesForFiles(files []string) (string, error) {
	if len(files) == 0 {
		return "", nil
	}

	r, err := openRepo()
	if err != nil {
		return "", err
	}

	// We want diff between HEAD and the current worktree (including staged and unstaged)
	// go-git doesn't have a direct "diff worktree vs HEAD" convenience that returns a patch string easily for specific files.
	// Strategy:
	// 1. Get HEAD commit tree.
	// 2. Diff HEAD tree vs Worktree is complex because Worktree isn't a Tree object directly.
	// Alternative: Iterate files, compute diff using internal utils or verify how go-git exposes "git diff" logic.
	// go-git's Worktree.Diff() returns changes between Index and Worktree? No, it's not fully exposed.
	
	// Better approach for "diff":
	// 1. Staged changes: Diff HEAD tree vs Index.
	// 2. Unstaged changes: Diff Index vs Worktree (filesystem).
	
	// However, simply running `git diff HEAD` (CLI) combines both.
	// Since reproducing exact `git diff` output with go-git manually is error-prone and verbose, 
	// we will use the `Patch` method from `object.Commit` if we just want staged, but for worktree it's harder.
	
	// ACTUALLY, checking the requirements: The AI needs the content of the changes.
	// If the file is untracked, we should just read it.
	// If it is tracked and modified, we need the patch.

	// Let's try to get a Patch object.
	// Current go-git limitation: High-level "diff" that mimics CLI output is not fully feature-complete for all cases (renames etc).
	// But we can try to approximate it or read file content directly if diff is too hard.
	// Wait, the prompt implies "pure golang git layer".
	
	// Simplification:
	// For each file:
	// - If it's new (untracked or added), read the whole file.
	// - If it's modified, try to generate a diff.
	
	// To strictly follow "use go-git", let's use what we can.
	// Worktree.Status gives us the state.
	
	// Using `exec` for diff was reliable. Doing it in pure Go is hard.
	// Let's do a best effort with `r.Head()` and comparing.
	
	// Note: Generating a unified diff string manually is complex.
	// Let's check if we can simply return the file content for now if diffing is too hard, 
	// BUT the AI performs better with diffs.
	
	// Let's try to use `diff.Do` from a library or just read the file content and marking it as "CURRENT CONTENT".
	// Many AI models handle full files well.
	// HOWEVER, the previous implementation fell back to reading full file if diff failed.
	
	// Let's implement a "dumb" diff or full content for now, or use `go-git`'s patch generation if possible.
	// `commit.Tree()` -> `diff.Tree(otherTree)` -> `patch.String()`.
	// But we don't have a tree for the worktree.
	
	// Let's return the FULL CONTENT of the files for now. This is a safe "pure Go" fallback.
	// The AI might actually prefer this for context, though it uses more tokens.
	// Wait, `go-git` v5 has `w.Diff()`? No.
	
	// Let's stick to: Read full file content. It's safe, pure Go (os.ReadFile), and works.
	// If the user really wants DIFFs, we'd need a diff library (like `github.com/sergi/go-diff/diffmatchpatch`).
	// `go-git` depends on `go-diff`.
	
	// Let's use `sergi/go-diff` to generate a diff between HEAD version and local version!
	
	head, err := r.Head()
	hasHead := err == nil

	var sb strings.Builder

	for _, file := range files {
		relPath, err := toRepoRelativePath(file)
		if err != nil {
			continue // Should not happen if file is valid
		}

		// Read local content
		localContentBytes, err := openFile(file)
		if err != nil {
			continue // Deleted or inaccessible
		}
		localContent := string(localContentBytes)

		if !hasHead {
			// No HEAD, everything is new
			sb.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", relPath, relPath))
			sb.WriteString("new file mode 100644\n")
			sb.WriteString("--- /dev/null\n")
			sb.WriteString(fmt.Sprintf("+++ b/%s\n", relPath))
			sb.WriteString(localContent) // TODO: Turn into + lines? Or just dump content.
			// Just dumping content is often enough for AI if labeled.
			continue
		}

		// Get content from HEAD
		headCommit, err := r.CommitObject(head.Hash())
		if err != nil {
			sb.WriteString(localContent)
			continue
		}
		
		tree, err := headCommit.Tree()
		if err != nil {
			sb.WriteString(localContent)
			continue
		}
		
		fileObj, err := tree.File(relPath)
		if err != nil {
			// File not in HEAD (New file)
			dmp := diffmatchpatch.New()
			diffs := dmp.DiffMain("", localContent, false)
			patches := dmp.PatchMake("", diffs)
			diffText := dmp.PatchToText(patches)

			sb.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", relPath, relPath))
			sb.WriteString("new file mode 100644\n")
			sb.WriteString("--- /dev/null\n")
			sb.WriteString(fmt.Sprintf("+++ b/%s\n", relPath))
			sb.WriteString(diffText)
			sb.WriteString("\n")
		} else {
			// File exists in HEAD, compute diff
			reader, err := fileObj.Blob.Reader()
			if err != nil {
				sb.WriteString(localContent)
				continue
			}
			headContentBytes := make([]byte, fileObj.Size)
			_, _ = reader.Read(headContentBytes)
			reader.Close()
			headContent := string(headContentBytes)

			dmp := diffmatchpatch.New()
			diffs := dmp.DiffMain(headContent, localContent, false)
			patches := dmp.PatchMake(headContent, diffs)
			diffText := dmp.PatchToText(patches)

			// Add header similar to git diff
			sb.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", relPath, relPath))
			sb.WriteString(fmt.Sprintf("--- a/%s\n", relPath))
			sb.WriteString(fmt.Sprintf("+++ b/%s\n", relPath))
			sb.WriteString(diffText)
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}

// openFile is a helper to read a file from disk
func openFile(path string) ([]byte, error) {
	// In a real implementation we might need to handle absolute/relative paths carefully
	return os.ReadFile(path)
}

// Commit stages and commits the specified files.
func Commit(files []string, message string) error {
	if len(files) == 0 {
		return errors.New("no files provided to commit")
	}

	r, err := openRepo()
	if err != nil {
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}

	// Add specific files to the index (staging)
	for _, file := range files {
		relPath, err := toRepoRelativePath(file)
		if err != nil {
			return err
		}
		_, err = w.Add(relPath)
		if err != nil {
			return fmt.Errorf("failed to add file %s: %w", file, err)
		}
	}

	// Commit
	_, err = w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  getGitConfig("user.name"),
			Email: getGitConfig("user.email"),
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

// getGitConfig is a helper to fetch user config.
// go-git can read config, but it's often easier to rely on system config if not found in repo.
func getGitConfig(key string) string {
	// Simplified config loading
	r, err := openRepo()
	if err != nil {
		return "Gitai User <gitai@example.com>"
	}
	cfg, err := r.Config()
	if err != nil {
		return "Gitai User <gitai@example.com>"
	}
	
	// Split "user.name" -> section "user", key "name"
	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return ""
	}
	
	sec := cfg.Raw.Section(parts[0])
	if sec == nil {
		return ""
	}
	return sec.Option(parts[1])
}


// GetCurrentBranch returns the name of the current branch.
func GetCurrentBranch() (string, error) {
	r, err := openRepo()
	if err != nil {
		return "", err
	}

	head, err := r.Head()
	if err != nil {
		return "", err
	}

	if head.Name().IsBranch() {
		return head.Name().Short(), nil
	}
	return head.Hash().String(), nil
}

// GetRemoteURL returns the URL for the specified remote.
func GetRemoteURL(remoteName string) (string, error) {
	r, err := openRepo()
	if err != nil {
		return "", err
	}

	rem, err := r.Remote(remoteName)
	if err != nil {
		return "", err
	}

	cfg := rem.Config()
	if len(cfg.URLs) > 0 {
		return cfg.URLs[0], nil
	}
	return "", fmt.Errorf("no URL for remote %s", remoteName)
}

// GetPullRequestURL constructs a pull request URL based on the current branch and remote "origin".
func GetPullRequestURL() (string, error) {
	branch, err := GetCurrentBranch()
	if err != nil {
		return "", err
	}

	remoteURL, err := GetRemoteURL("origin")
	if err != nil {
		return "", err
	}

	// Normalize URL logic (same as before)
	remoteURL = strings.TrimSpace(remoteURL)
	remoteURL = strings.TrimSuffix(remoteURL, ".git")

	if strings.HasPrefix(remoteURL, "git@") {
		remoteURL = strings.TrimPrefix(remoteURL, "git@")
		if i := strings.Index(remoteURL, ":"); i != -1 {
			remoteURL = remoteURL[:i] + "/" + remoteURL[i+1:]
		}
		remoteURL = "https://" + remoteURL
	} else if !strings.HasPrefix(remoteURL, "http://") && !strings.HasPrefix(remoteURL, "https://") {
		if i := strings.Index(remoteURL, ":"); i != -1 {
			remoteURL = remoteURL[:i] + "/" + remoteURL[i+1:]
		}
		remoteURL = "https://" + remoteURL
	}

	if strings.Contains(remoteURL, "github.com") {
		return fmt.Sprintf("%s/pull/new/%s", remoteURL, branch), nil
	}
	if strings.Contains(remoteURL, "gitlab.com") {
		return fmt.Sprintf("%s/-/merge_requests/new?merge_request[source_branch]=%s", remoteURL, branch), nil
	}
	if strings.Contains(remoteURL, "bitbucket.org") {
		return fmt.Sprintf("%s/pull-requests/new?source=%s", remoteURL, branch), nil
	}

	return "", fmt.Errorf("unknown remote host: %s", remoteURL)
}

// Push pushes the current branch to the remote repository.
func Push() (string, error) {
	r, err := openRepo()
	if err != nil {
		return "", err
	}

	remote, err := r.Remote("origin")
	if err != nil {
		return "", err
	}

	auth := getAuth(remote.Config().URLs[0])

	err = r.Push(&git.PushOptions{
		Auth: auth,
	})
	if err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return "Already up-to-date", nil
		}
		return "", err
	}
	return "Push successful", nil
}

// getAuth attempts to find authentication for the given URL.
func getAuth(url string) transport.AuthMethod {
	if !isSSH(url) {
		return nil
	}

	user := extractUser(url)

	// 1. Try SSH Agent
	if auth, err := gitssh.NewSSHAgentAuth(user); err == nil {
		auth.HostKeyCallback = ssh.InsecureIgnoreHostKey()
		return auth
	}

	// 2. Try default key files
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	keys := []string{"id_ed25519", "id_rsa", "id_ecdsa", "id_dsa"}
	for _, key := range keys {
		keyPath := filepath.Join(home, ".ssh", key)
		if _, err := os.Stat(keyPath); err == nil {
			if auth, err := gitssh.NewPublicKeysFromFile(user, keyPath, ""); err == nil {
				auth.HostKeyCallback = ssh.InsecureIgnoreHostKey()
				return auth
			}
		}
	}

	return nil
}

func extractUser(url string) string {
	user := "git"
	if strings.Contains(url, "@") {
		// Handle git@github.com... or ssh://user@github.com...
		parts := strings.Split(url, "@")
		userPart := parts[0]
		if i := strings.Index(userPart, "://"); i != -1 {
			userPart = userPart[i+3:]
		}
		if userPart != "" {
			user = userPart
		}
	}
	return user
}

func isSSH(url string) bool {
	return strings.HasPrefix(url, "git@") || strings.HasPrefix(url, "ssh://")
}

// ResolvePath resolves a file path to repo-relative paths.
// It supports directories by returning all files within them that are tracked or changed.
func ResolvePath(path string) ([]string, error) {
	root, err := GetGitRoot()
	if err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	// Check if the path is within the repo
	relToRoot, err := filepath.Rel(root, absPath)
	if err != nil || strings.HasPrefix(relToRoot, "..") {
		return nil, fmt.Errorf("path %s is outside the repository", path)
	}

	fi, err := os.Stat(absPath)
	if err != nil {
		// If it doesn't exist on disk, it might be a deleted file still in git
		// or a glob. For now, we return the path as-is if it's within the repo.
		return []string{relToRoot}, nil
	}

	if !fi.IsDir() {
		return []string{relToRoot}, nil
	}

	// It's a directory. We need to find all files in the repo under this directory.
	r, err := openRepo()
	if err != nil {
		return nil, err
	}

	// Use the status to find all changed/tracked files
	w, err := r.Worktree()
	if err != nil {
		return nil, err
	}

	status, err := w.Status()
	if err != nil {
		return nil, err
	}

	// Also consider files in HEAD
	head, err := r.Head()
	var headFiles []string
	if err == nil {
		if commit, err := r.CommitObject(head.Hash()); err == nil {
			if tree, err := commit.Tree(); err == nil {
				_ = tree.Files().ForEach(func(f *object.File) error {
					headFiles = append(headFiles, f.Name)
					return nil
				})
			}
		}
	}

	var results []string
	seen := make(map[string]bool)

	prefix := relToRoot
	if prefix == "." {
		prefix = ""
	} else if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}

	// Add files from status
	for p := range status {
		if strings.HasPrefix(p, prefix) || relToRoot == "." {
			if !seen[p] {
				results = append(results, p)
				seen[p] = true
			}
		}
	}

	// Add files from HEAD
	for _, p := range headFiles {
		if strings.HasPrefix(p, prefix) || relToRoot == "." {
			if !seen[p] {
				results = append(results, p)
				seen[p] = true
			}
		}
	}

	sort.Strings(results)
	return results, nil
}

// Helper to convert any path to repo-relative path
func toRepoRelativePath(path string) (string, error) {
	root, err := GetGitRoot()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Rel(root, abs)
}
