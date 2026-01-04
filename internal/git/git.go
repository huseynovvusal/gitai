package git

import (
	"errors"
	"fmt"
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
	"golang.org/x/crypto/ssh/agent"
)

var ErrOutsideRepo = errors.New("path is outside the repository")

// resolveAuth automatically finds keys from the Agent OR any private key file in ~/.ssh
func resolveAuth(url string) (transport.AuthMethod, error) {
	if strings.HasPrefix(url, "http") {
		return nil, nil
	}

	// Extract user (default "git")
	user := "git"
	if parts := strings.Split(url, "@"); len(parts) > 1 {
		user = strings.Split(parts[0], "://")[0]
	}

	// Define the callback logic separately
	keyCallback := func() (signers []ssh.Signer, err error) {
		// A. Try SSH Agent
		if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
			if conn, err := net.Dial("unix", sock); err == nil {
				if s, err := agent.NewClient(conn).Signers(); err == nil {
					signers = append(signers, s...)
				}
			}
		}

		// B. Try all files in ~/.ssh (Simpler than guessing names)
		home, _ := os.UserHomeDir()
		files, _ := os.ReadDir(filepath.Join(home, ".ssh"))
		for _, f := range files {
			// Skip public keys and known config files
			if f.IsDir() || strings.HasSuffix(f.Name(), ".pub") ||
				strings.HasPrefix(f.Name(), "known_") || strings.HasPrefix(f.Name(), "config") {
				continue
			}

			// Try to parse as private key (ignores non-keys automatically)
			if key, err := os.ReadFile(filepath.Join(home, ".ssh", f.Name())); err == nil {
				if signer, err := ssh.ParsePrivateKey(key); err == nil {
					signers = append(signers, signer)
				}
			}
		}

		if len(signers) == 0 {
			return nil, errors.New("no ssh keys found in agent or ~/.ssh")
		}
		return signers, nil
	}

	auth := &gitssh.PublicKeysCallback{
		User:     user,
		Callback: keyCallback,
	}

	auth.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	return auth, nil
}

// --- 2. Core Logic ---

func getRepo() (*git.Repository, *git.Worktree, string, error) {
	root, err := GetGitRoot() // Assumes external existence
	if err != nil {
		return nil, nil, "", err
	}
	r, err := git.PlainOpen(root)
	if err != nil {
		return nil, nil, "", err
	}
	w, err := r.Worktree()
	return r, w, root, err
}

func toRel(path string) (string, error) {
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

// Push pushes the current branch to origin using the smart auth.
func Push() (string, error) {
	r, _, _, err := getRepo()
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

	auth, err := resolveAuth(urls[0])
	if err != nil {
		return "", err
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

func Commit(files []string, message string) error {
	r, w, _, err := getRepo()
	if err != nil {
		return err
	}

	for _, f := range files {
		rel, err := toRel(f)
		if err != nil {
			return err
		}
		if _, err := w.Add(rel); err != nil {
			return err
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

// --- 3. Status & Diffing ---

func GetStatusForFiles(files []string) (string, error) {
	_, w, _, err := getRepo()
	if err != nil {
		return "", err
	}

	status, err := w.Status()
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, f := range files {
		if rel, err := toRel(f); err == nil {
			if s, ok := status[rel]; ok {
				sb.WriteString(fmt.Sprintf("%c%c %s\n", statusCode(s.Staging), statusCode(s.Worktree), rel))
			}
		}
	}
	return sb.String(), nil
}

func GetChangedFiles() ([]string, error) {
	_, w, _, err := getRepo()
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
	r, _, _, err := getRepo()
	if err != nil {
		return "", err
	}

	headTree, _ := getHeadTree(r)
	var sb strings.Builder

	for _, file := range files {
		rel, err := toRel(file)
		if err != nil {
			continue
		}

		oldText, isNew := "", true
		if headTree != nil {
			if f, err := headTree.File(rel); err == nil {
				if c, err := f.Contents(); err == nil {
					oldText, isNew = c, false
				}
			}
		}

		newBytes, err := os.ReadFile(file)
		newText := string(newBytes)
		isDeleted := err != nil

		if isNew && isDeleted {
			continue
		}
		sb.WriteString(diffString(rel, oldText, newText, isNew, isDeleted))
	}
	return sb.String(), nil
}

func ResolvePath(path string) ([]string, error) {
	r, w, root, err := getRepo()
	if err != nil {
		return nil, err
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(root, abs)

	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		// Return single file (even if deleted/missing)
		if strings.HasPrefix(rel, "..") {
			return nil, ErrOutsideRepo
		}
		return []string{rel}, nil
	}

	// It's a directory: find all tracked files inside
	status, _ := w.Status()
	headTree, _ := getHeadTree(r)

	// Normalize path prefix
	prefix := rel
	if prefix == "." {
		prefix = ""
	}
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	seen := make(map[string]bool)
	var results []string

	// Helper to add if matches prefix
	add := func(p string) {
		if (prefix == "" || strings.HasPrefix(p, prefix)) && !seen[p] {
			results = append(results, p)
			seen[p] = true
		}
	}

	for p := range status {
		add(p)
	}

	if headTree != nil {
		_ = headTree.Files().ForEach(func(f *object.File) error {
			add(f.Name)
			return nil
		})
	}

	sort.Strings(results)
	return results, nil
}

// --- 4. Utilities ---

func getHeadTree(r *git.Repository) (*object.Tree, error) {
	head, err := r.Head()
	if err != nil {
		return nil, err
	}
	c, err := r.CommitObject(head.Hash())
	if err != nil {
		return nil, err
	}
	return c.Tree()
}

func statusCode(c git.StatusCode) rune {
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
	case git.Untracked:
		return '?'
	default:
		return '?'
	}
}

func diffString(path, old, new string, isNew, isDel bool) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(old, new, false)
	patches := dmp.PatchMake(old, diffs)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", path, path))
	if isNew {
		sb.WriteString(fmt.Sprintf("new file mode 100644\n--- /dev/null\n+++ b/%s\n", path))
	} else if isDel {
		sb.WriteString(fmt.Sprintf("deleted file mode 100644\n--- a/%s\n+++ /dev/null\n", path))
	} else {
		sb.WriteString(fmt.Sprintf("--- a/%s\n+++ b/%s\n", path, path))
	}
	sb.WriteString(dmp.PatchToText(patches))
	return sb.String()
}

func GetPullRequestURL() (string, error) {
	r, _, _, err := getRepo()
	if err != nil {
		return "", err
	}

	head, err := r.Head()
	if err != nil {
		return "", err
	}
	branch := head.Name().Short()
	if !head.Name().IsBranch() {
		branch = head.Hash().String()
	}

	rem, err := r.Remote("origin")
	if err != nil || len(rem.Config().URLs) == 0 {
		return "", errors.New("no remote origin")
	}

	url := rem.Config().URLs[0]
	// Normalize URL
	url = strings.TrimSuffix(strings.TrimSpace(url), ".git")
	if strings.HasPrefix(url, "git@") {
		url = "https://" + strings.Replace(strings.TrimPrefix(url, "git@"), ":", "/", 1)
	} else if strings.HasPrefix(url, "ssh://") {
		url = "https://" + strings.TrimPrefix(strings.Split(url, "@")[1], ":")
	}

	switch {
	case strings.Contains(url, "github.com"):
		return fmt.Sprintf("%s/pull/new/%s", url, branch), nil
	case strings.Contains(url, "gitlab.com"):
		return fmt.Sprintf("%s/-/merge_requests/new?merge_request[source_branch]=%s", url, branch), nil
	case strings.Contains(url, "bitbucket.org"):
		return fmt.Sprintf("%s/pull-requests/new?source=%s", url, branch), nil
	default:
		return "", fmt.Errorf("unknown host: %s", url)
	}
}
