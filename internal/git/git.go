package git

import (
	"errors"
	"fmt"
	"golang.org/x/crypto/ssh/agent"
	"net"
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

var ErrOutsideRepo = errors.New("path is outside the repository")

// --- Core Helpers ---

func getRepoAndWorktree() (*git.Repository, *git.Worktree, string, error) {
	root, err := GetGitRoot()
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

func GetStatusForFiles(files []string) (string, error) {
	if len(files) == 0 {
		return "", nil
	}
	_, w, _, err := getRepoAndWorktree()
	if err != nil {
		return "", err
	}

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
			sb.WriteString(fmt.Sprintf("%c%c %s\n", formatStatusCode(s.Staging), formatStatusCode(s.Worktree), relPath))
		}
	}
	return sb.String(), nil
}

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

func GetChangesForFiles(files []string) (string, error) {
	if len(files) == 0 {
		return "", nil
	}
	r, _, _, err := getRepoAndWorktree()
	if err != nil {
		return "", err
	}

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

		newContentBytes, err := os.ReadFile(file)
		newContent := string(newContentBytes)
		isDeleted := err != nil

		if isNewFile && isDeleted {
			continue
		}
		sb.WriteString(generateDiffString(relPath, oldContent, newContent, isNewFile, isDeleted))
	}
	return sb.String(), nil
}

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

	user, email := "Gitai User", "gitai@example.com"
	if cfg, _ := r.Config(); cfg != nil {
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

func Push() (string, error) {
	r, _, _, err := getRepoAndWorktree()
	if err != nil {
		return "", err
	}

	remote, err := r.Remote("origin")
	if err != nil {
		return "", fmt.Errorf("remote 'origin' not found")
	}

	urls := remote.Config().URLs
	if len(urls) == 0 {
		return "", errors.New("origin has no URLs")
	}

	// Smart Auth Resolution
	auth, err := resolveAuth(urls[0])
	if err != nil {
		// If we couldn't resolve SSH auth, abort early
		return "", fmt.Errorf("ssh auth failed: %w", err)
	}

	err = r.Push(&git.PushOptions{Auth: auth})
	if err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return "Already up-to-date", nil
		}
		return "", err
	}
	return "Push successful", nil
}

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
	if err != nil || !info.IsDir() {
		return []string{relToRoot}, nil
	}

	status, err := w.Status()
	if err != nil {
		return nil, err
	}

	prefix := relToRoot
	if prefix == "." {
		prefix = ""
	} else if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}

	var results []string
	seen := make(map[string]bool)

	// Add from Status
	for p := range status {
		if strings.HasPrefix(p, prefix) {
			results = append(results, p)
			seen[p] = true
		}
	}

	// Add from HEAD (catch unmodified files)
	if head, err := r.Head(); err == nil {
		if commit, err := r.CommitObject(head.Hash()); err == nil {
			if tree, err := commit.Tree(); err == nil {
				_ = tree.Files().ForEach(func(f *object.File) error {
					if strings.HasPrefix(f.Name, prefix) && !seen[f.Name] {
						results = append(results, f.Name)
					}
					return nil
				})
			}
		}
	}
	sort.Strings(results)
	return results, nil
}

func GetPullRequestURL() (string, error) {
	branch, err := GetCurrentBranch()
	if err != nil {
		return "", err
	}

	remoteURL, err := GetRemoteURL("origin")
	if err != nil {
		return "", err
	}

	repoURL := normalizeGitURL(remoteURL)

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

// --- Utilities ---

// resolveAuth intelligently finds the best authentication method by scanning ALL keys.
func resolveAuth(url string) (transport.AuthMethod, error) {
	if strings.HasPrefix(url, "http") {
		return nil, nil
	}

	user := "git"
	if parts := strings.Split(url, "@"); len(parts) > 1 {
		// Handle git@github.com -> user: git
		user = strings.Split(parts[0], "://")[len(strings.Split(parts[0], "://"))-1]
	}

	// Define the logic to find keys
	callback := func() ([]ssh.Signer, error) {
		var signers []ssh.Signer
		seenKeys := make(map[string]bool)

		// 1. Try System SSH Agent (Priority #1)
		if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
			if conn, err := net.Dial("unix", sock); err == nil {
				ag := agent.NewClient(conn)
				if agentSigners, err := ag.Signers(); err == nil {
					for _, s := range agentSigners {
						signers = append(signers, s)
						seenKeys[ssh.FingerprintSHA256(s.PublicKey())] = true
					}
				}
			}
		}

		// 2. Scan ~/.ssh for ALL private keys (Priority #2)
		home, _ := os.UserHomeDir()
		sshDir := filepath.Join(home, ".ssh")
		files, _ := os.ReadDir(sshDir)

		for _, f := range files {
			// Skip public keys, directories, and known_hosts
			if f.IsDir() || strings.HasSuffix(f.Name(), ".pub") ||
				strings.HasPrefix(f.Name(), "known_hosts") ||
				strings.HasPrefix(f.Name(), "config") ||
				strings.HasPrefix(f.Name(), "authorized_keys") {
				continue
			}

			// Try to read file
			path := filepath.Join(sshDir, f.Name())
			keyData, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			// Check if it's a valid private key
			signer, err := ssh.ParsePrivateKey(keyData)
			if err != nil {
				continue
			}

			// Deduplicate
			fp := ssh.FingerprintSHA256(signer.PublicKey())
			if !seenKeys[fp] {
				signers = append(signers, signer)
				seenKeys[fp] = true
			}
		}

		if len(signers) == 0 {
			return nil, errors.New("no valid ssh keys (agent or disk) found")
		}

		return signers, nil
	}

	// Construct the AuthMethod
	auth := &gitssh.PublicKeysCallback{
		User:     user,
		Callback: callback,
	}

	// FIX: Assign this field separately to avoid "not a member" compilation errors
	// due to how Go handles embedded struct literals.
	auth.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	return auth, nil
}

func normalizeGitURL(url string) string {
	url = strings.TrimSpace(url)
	url = strings.TrimSuffix(url, ".git")
	if strings.HasPrefix(url, "git@") {
		url = strings.TrimPrefix(url, "git@")
		url = strings.Replace(url, ":", "/", 1)
	} else if strings.HasPrefix(url, "ssh://") {
		url = strings.TrimPrefix(url, "ssh://")
		if i := strings.Index(url, "@"); i != -1 {
			url = url[i+1:]
		}
	}
	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}
	return url
}

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

func generateDiffString(path, oldText, newText string, isNew, isDeleted bool) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldText, newText, false)
	patches := dmp.PatchMake(oldText, diffs)
	diffBody := dmp.PatchToText(patches)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", path, path))
	if isNew {
		sb.WriteString("new file mode 100644\n--- /dev/null\n")
		sb.WriteString(fmt.Sprintf("+++ b/%s\n", path))
	} else if isDeleted {
		sb.WriteString("deleted file mode 100644\n")
		sb.WriteString(fmt.Sprintf("--- a/%s\n+++ /dev/null\n", path))
	} else {
		sb.WriteString(fmt.Sprintf("--- a/%s\n+++ b/%s\n", path, path))
	}
	sb.WriteString(diffBody)
	return sb.String()
}
