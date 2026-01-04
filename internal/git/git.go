package git

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// ErrOutsideRepo is returned when a provided path is not within the git repository.
var ErrOutsideRepo = errors.New("path is outside the repository")

// GitService provides methods for interacting with a Git repository.
type GitService struct{}

// NewGitService creates a new GitService instance.
func NewGitService() *GitService {
	return &GitService{}
}

// GetStatusForFiles returns the porcelain status of the specified files.
func (s *GitService) GetStatusForFiles(ctx context.Context, files []string) (string, error) {
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
			if st, ok := status[rel]; ok {
				sb.WriteString(fmt.Sprintf("%c%c %s\n", formatStatusCode(st.Staging), formatStatusCode(st.Worktree), rel))
			}
		}
	}
	return sb.String(), nil
}

// GetChangedFiles returns a sorted list of all modified, added, or deleted files in the repository.
func (s *GitService) GetChangedFiles(ctx context.Context) ([]string, error) {
	_, w, _, err := getRepo()
	if err != nil {
		return nil, err
	}

	status, err := w.Status()
	if err != nil {
		return nil, err
	}

	var changed []string
	for path, st := range status {
		if st.Staging != git.Unmodified || st.Worktree != git.Unmodified {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return changed, nil
}

// GetChangesForFiles generates a unified diff for the specified files against the HEAD commit.
func (s *GitService) GetChangesForFiles(ctx context.Context, files []string) (string, error) {
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
		sb.WriteString(generateDiffString(rel, oldText, newText, isNew, isDeleted))
	}
	return sb.String(), nil
}

// Commit stages the specified files and creates a new commit with the given message.
func (s *GitService) Commit(ctx context.Context, files []string, message string) error {
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
	if cfg, err := r.Config(); err == nil {
		if cfg.User.Name != "" {
			user = cfg.User.Name
		}
		if cfg.User.Email != "" {
			email = cfg.User.Email
		}
	}

	// Fallback to global config if local is empty
	if user == "Gitai User" || email == "gitai@example.com" {
		if global, err := config.LoadConfig(config.GlobalScope); err == nil {
			if user == "Gitai User" && global.User.Name != "" {
				user = global.User.Name
			}
			if email == "gitai@example.com" && global.User.Email != "" {
				email = global.User.Email
			}
		}
	}

	_, err = w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{Name: user, Email: email, When: time.Now()},
	})
	return err
}

// Push pushes the current branch to the specified remote.
func (s *GitService) Push(ctx context.Context, remoteName string) (string, error) {
	r, _, _, err := getRepo()
	if err != nil {
		return "", err
	}

	remote, err := r.Remote(remoteName)
	if err != nil {
		return "", fmt.Errorf("remote '%s' not found", remoteName)
	}

	urls := remote.Config().URLs
	if len(urls) == 0 {
		return "", errors.New("origin has no URLs")
	}

	auth, err := resolveAuth(urls[0])
	if err != nil {
		return "", err
	}

	err = r.PushContext(ctx, &git.PushOptions{Auth: auth})
	if err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return "Already up-to-date", nil
		}
		return "", err
	}
	return "Push successful", nil
}

// ResolvePath returns a list of all repository files within the given path.
func (s *GitService) ResolvePath(ctx context.Context, path string) ([]string, error) {
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
		if strings.HasPrefix(rel, "..") {
			return nil, ErrOutsideRepo
		}
		return []string{rel}, nil
	}

	status, _ := w.Status()
	headTree, _ := getHeadTree(r)

	prefix := rel
	if prefix == "." {
		prefix = ""
	}
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	seen := make(map[string]bool)
	var results []string

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

// GetPullRequestURL generates a web URL to create a new pull request for the current branch.
func (s *GitService) GetPullRequestURL(ctx context.Context, remoteName string) (string, error) {
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

	rem, err := r.Remote(remoteName)
	if err != nil || len(rem.Config().URLs) == 0 {
		return "", fmt.Errorf("no remote %s", remoteName)
	}

	url := rem.Config().URLs[0]
	url = normalizeGitURL(url)

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

// --- Internal Helper Functions ---

func resolveAuth(url string) (transport.AuthMethod, error) {
	if strings.HasPrefix(url, "http") {
		return nil, nil
	}

	if !strings.HasPrefix(url, "ssh://") && !strings.Contains(url, "@") {
		return nil, nil
	}

	user := "git"
	if parts := strings.Split(url, "@"); len(parts) > 1 {
		user = parts[0]
		if strings.Contains(user, "://") {
			user = strings.Split(user, "://")[1]
		}
	}

	keyCallback := func() (signers []ssh.Signer, err error) {
		if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
			if conn, err := net.Dial("unix", sock); err == nil {
				if s, err := agent.NewClient(conn).Signers(); err == nil {
					signers = append(signers, s...)
				}
			}
		}

		home, _ := os.UserHomeDir()
		files, _ := os.ReadDir(filepath.Join(home, ".ssh"))
		for _, f := range files {
			if f.IsDir() || strings.HasSuffix(f.Name(), ".pub") ||
				strings.HasPrefix(f.Name(), "known_") || strings.HasPrefix(f.Name(), "config") {
				continue
			}

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

func getRepo() (*git.Repository, *git.Worktree, string, error) {
	root, err := GetGitRoot()
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

// formatStatusCode maps a git status code to its porcelain rune representation.
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

// generateDiffString creates a unified diff compatible with standard git output.
//
// NOTE: We intentionally use the verbose "diff --git" header format (instead of
// simpler custom formats like "M file.go") because LLMs perform significantly
// better with it. The standard git headers act as strong "mental anchors" that
// prevent context bleeding between files and align with the model's training data.
func generateDiffString(path, old, new string, isNew, isDel bool) string {
	dmp := diffmatchpatch.New()
	patches := dmp.PatchMake(old, new)
	patchText := dmp.PatchToText(patches)

	// Normalize the text to avoid URL encoding
	decoded, _ := url.PathUnescape(patchText)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", path, path))
	if isNew {
		sb.WriteString(fmt.Sprintf("new file mode 100644\n--- /dev/null\n+++ b/%s\n", path))
	} else if isDel {
		sb.WriteString(fmt.Sprintf("deleted file mode 100644\n--- a/%s\n+++ /dev/null\n", path))
	} else {
		sb.WriteString(fmt.Sprintf("--- a/%s\n+++ b/%s\n", path, path))
	}
	sb.WriteString(decoded)

	return sb.String()
}

// normalizeGitURL converts git SSH or git:// URLs to an HTTPS web URL format.
func normalizeGitURL(url string) string {
	url = strings.TrimSpace(url)
	url = strings.TrimSuffix(url, ".git")
	if strings.HasPrefix(url, "git@") {
		url = "https://" + strings.Replace(strings.TrimPrefix(url, "git@"), ":", "/", 1)
	} else if strings.HasPrefix(url, "ssh://") {
		url = "https://" + strings.TrimPrefix(strings.Split(url, "@")[1], ":")
	}

	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}
	return url
}
