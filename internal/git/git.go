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
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/sergi/go-diff/diffmatchpatch"
)

var ErrOutsideRepo = errors.New("path is outside the repository")

// getRepoAndWorktree creates a standard entry point for most git operations.
func getRepoAndWorktree() (*git.Repository, *git.Worktree, string, error) {
	root, err := GetGitRoot() // Assumes this function exists in your package
	if err != nil {
		return nil, nil, "", err
	}

	r, err := git.PlainOpen(root)
	if err != nil {
		return nil, nil, "", err
	}

	w, err := r.Worktree()
	if err != nil {
		return nil, nil, "", err
	}

	return r, w, root, nil
}

// toRelativePath converts any path to a repo-relative path.
func toRelativePath(path string) (string, error) {
	root, err := GetGitRoot()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") {
		return "", ErrOutsideRepo
	}
	return rel, nil
}

// --- Public API ---

// GetStatusForFiles returns `git status --porcelain` style output for specific files.
func GetStatusForFiles(files []string) (string, error) {
	if len(files) == 0 {
		return "", nil
	}

	_, w, _, err := getRepoAndWorktree()
	if err != nil {
		return "", err
	}

	// Status() is expensive on large repos, but necessary here.
	status, err := w.Status()
	if err != nil {
		return "", fmt.Errorf("git status failed: %w", err)
	}

	var sb strings.Builder
	for _, file := range files {
		relPath, err := toRelativePath(file)
		if err != nil {
			continue
		}

		if s, ok := status[relPath]; ok {
			// Map internal status codes to porcelain format (XY Path)
			x := formatStatusCode(s.Staging)
			y := formatStatusCode(s.Worktree)
			sb.WriteString(fmt.Sprintf("%c%c %s\n", x, y, relPath))
		}
	}

	return sb.String(), nil
}

// GetChangedFiles returns a sorted list of all modified, added, or deleted files.
func GetChangedFiles() ([]string, error) {
	_, w, _, err := getRepoAndWorktree()
	if err != nil {
		return nil, err
	}

	status, err := w.Status()
	if err != nil {
		return nil, err
	}

	var changed []string
	for path, s := range status {
		if s.Staging != git.Unmodified || s.Worktree != git.Unmodified {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return changed, nil
}

// GetChangesForFiles generates a unified diff for the specified files.
// It handles New, Modified, and Deleted files.
func GetChangesForFiles(files []string) (string, error) {
	if len(files) == 0 {
		return "", nil
	}

	r, _, _, err := getRepoAndWorktree()
	if err != nil {
		return "", err
	}

	// Resolve HEAD tree to compare against
	var headTree *object.Tree
	if head, err := r.Head(); err == nil {
		if commit, err := r.CommitObject(head.Hash()); err == nil {
			headTree, _ = commit.Tree()
		}
	}

	var sb strings.Builder

	for _, file := range files {
		relPath, err := toRelativePath(file)
		if err != nil {
			continue
		}

		// 1. Get Old Content (from HEAD)
		oldContent := ""
		isNewFile := true
		if headTree != nil {
			if f, err := headTree.File(relPath); err == nil {
				if content, err := f.Contents(); err == nil {
					oldContent = content
					isNewFile = false
				}
			}
		}

		// 2. Get New Content (from Disk)
		newContentBytes, err := os.ReadFile(file)
		newContent := string(newContentBytes)
		isDeleted := err != nil // If read fails, assume deleted

		// 3. Generate Diff
		if isNewFile && isDeleted {
			continue // File doesn't exist in either
		}

		sb.WriteString(generateDiffString(relPath, oldContent, newContent, isNewFile, isDeleted))
	}

	return sb.String(), nil
}

// Commit stages and commits the files.
func Commit(files []string, message string) error {
	if len(files) == 0 {
		return errors.New("no files to commit")
	}

	r, w, _, err := getRepoAndWorktree()
	if err != nil {
		return err
	}

	for _, file := range files {
		relPath, err := toRelativePath(file)
		if err != nil {
			return err
		}
		if _, err := w.Add(relPath); err != nil {
			return fmt.Errorf("failed to add %s: %w", file, err)
		}
	}

	cfg, _ := r.Config()
	user, email := "Gitai User", "gitai@example.com"
	if cfg != nil {
		if u := cfg.Raw.Section("user"); u != nil {
			if n := u.Option("name"); n != "" {
				user = n
			}
			if e := u.Option("email"); e != "" {
				email = e
			}
		}
	}

	_, err = w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{Name: user, Email: email, When: time.Now()},
	})
	return err
}

// Push pushes the current branch to origin.
func Push() (string, error) {
	r, _, _, err := getRepoAndWorktree()
	if err != nil {
		return "", err
	}

	remote, err := r.Remote("origin")
	if err != nil {
		return "", fmt.Errorf("remote 'origin' not found")
	}

	// Auto-detect auth based on the first URL found
	urls := remote.Config().URLs
	if len(urls) == 0 {
		return "", errors.New("origin has no URLs")
	}

	err = r.Push(&git.PushOptions{
		Auth: resolveAuth(urls[0]),
	})

	if err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return "Already up-to-date", nil
		}
		return "", err
	}
	return "Push successful", nil
}

// ResolvePath finds all tracked files within a directory or validates a single file.
func ResolvePath(path string) ([]string, error) {
	r, w, root, err := getRepoAndWorktree()
	if err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	relToRoot, err := filepath.Rel(root, absPath)
	if err != nil || strings.HasPrefix(relToRoot, "..") {
		return nil, ErrOutsideRepo
	}

	info, err := os.Stat(absPath)
	// If it's a specific file (or deleted file), return just that path
	if err != nil || !info.IsDir() {
		return []string{relToRoot}, nil
	}

	// If it's a directory, we need to gather all tracked files inside it
	// NOTE: w.Status() is used here to find files.
	status, err := w.Status()
	if err != nil {
		return nil, err
	}

	// Normalize prefix for searching
	prefix := relToRoot
	if prefix == "." {
		prefix = ""
	} else if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}

	var results []string
	for p := range status {
		if strings.HasPrefix(p, prefix) {
			results = append(results, p)
		}
	}

	// Ensure we don't miss unmodified files that exist in HEAD but not Status
	if head, err := r.Head(); err == nil {
		if commit, err := r.CommitObject(head.Hash()); err == nil {
			if tree, err := commit.Tree(); err == nil {
				_ = tree.Files().ForEach(func(f *object.File) error {
					if strings.HasPrefix(f.Name, prefix) {
						// Only add if not already added (simple dedupe)
						found := false
						for _, existing := range results {
							if existing == f.Name {
								found = true
								break
							}
						}
						if !found {
							results = append(results, f.Name)
						}
					}
					return nil
				})
			}
		}
	}

	sort.Strings(results)
	return results, nil
}

// --- Utilities ---

func formatStatusCode(c git.StatusCode) rune {
	switch c {
	case git.Unmodified:
		return ' '
	case git.Modified:
		return 'M'
	case git.Added:
		return 'A'
	case git.Deleted:
		return 'D'
	case git.Renamed:
		return 'R'
	case git.Copied:
		return 'C'
	case git.Untracked:
		return '?'
	default:
		return '?'
	}
}

// generateDiffString creates a unified diff using sergi/go-diff
func generateDiffString(path, oldText, newText string, isNew, isDeleted bool) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldText, newText, false)
	patches := dmp.PatchMake(oldText, diffs)
	diffBody := dmp.PatchToText(patches)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", path, path))

	if isNew {
		sb.WriteString("new file mode 100644\n")
		sb.WriteString("--- /dev/null\n")
		sb.WriteString(fmt.Sprintf("+++ b/%s\n", path))
	} else if isDeleted {
		sb.WriteString("deleted file mode 100644\n")
		sb.WriteString(fmt.Sprintf("--- a/%s\n", path))
		sb.WriteString("+++ /dev/null\n")
	} else {
		sb.WriteString(fmt.Sprintf("--- a/%s\n", path))
		sb.WriteString(fmt.Sprintf("+++ b/%s\n", path))
	}

	sb.WriteString(diffBody)
	return sb.String()
}

// resolveAuth tries to find an SSH agent or keys, otherwise returns nil (for HTTPS/Pass).
func resolveAuth(url string) transport.AuthMethod {
	if strings.HasPrefix(url, "http") {
		return nil // Rely on git credential helpers or nil
	}

	// 1. SSH Agent
	if auth, err := ssh.NewSSHAgentAuth("git"); err == nil {
		return auth
	}

	// 2. Common Key Files
	home, _ := os.UserHomeDir()
	for _, key := range []string{"id_ed25519", "id_rsa"} {
		path := filepath.Join(home, ".ssh", key)
		if _, err := os.Stat(path); err == nil {
			if auth, err := ssh.NewPublicKeysFromFile("git", path, ""); err == nil {
				return auth
			}
		}
	}
	return nil
}

// GetCurrentBranch returns the name of the current branch.
func GetCurrentBranch() (string, error) {
	r, _, _, err := getRepoAndWorktree()
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

	// Convert SSH or git:// URLs to proper HTTPS web URLs
	repoURL := normalizeGitURL(remoteURL)

	// Construct the PR URL based on the host
	switch {
	case strings.Contains(repoURL, "github.com"):
		return fmt.Sprintf("%s/pull/new/%s", repoURL, branch), nil
	case strings.Contains(repoURL, "gitlab.com"):
		return fmt.Sprintf("%s/-/merge_requests/new?merge_request[source_branch]=%s", repoURL, branch), nil
	case strings.Contains(repoURL, "bitbucket.org"):
		return fmt.Sprintf("%s/pull-requests/new?source=%s", repoURL, branch), nil
	default:
		return "", fmt.Errorf("unknown remote host in URL: %s", repoURL)
	}
}

// GetRemoteURL returns the fetch URL for the specified remote.
func GetRemoteURL(remoteName string) (string, error) {
	r, _, _, err := getRepoAndWorktree()
	if err != nil {
		return "", err
	}

	rem, err := r.Remote(remoteName)
	if err != nil {
		return "", fmt.Errorf("remote %s not found: %w", remoteName, err)
	}

	cfg := rem.Config()
	if len(cfg.URLs) == 0 {
		return "", fmt.Errorf("no URL found for remote %s", remoteName)
	}
	return cfg.URLs[0], nil
}

// normalizeGitURL converts git@ or ssh:// URLs to https:// format for web linking.
func normalizeGitURL(url string) string {
	url = strings.TrimSpace(url)
	url = strings.TrimSuffix(url, ".git")

	// Handle "git@github.com:user/repo" -> "github.com/user/repo"
	if strings.HasPrefix(url, "git@") {
		url = strings.TrimPrefix(url, "git@")
		url = strings.Replace(url, ":", "/", 1)
	} else if strings.HasPrefix(url, "ssh://") {
		url = strings.TrimPrefix(url, "ssh://")
	}

	// Ensure it starts with https:// if it doesn't already
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	return url
}
