package git

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// ErrOutsideRepo is returned when a provided path is not within the git repository.
var ErrOutsideRepo = errors.New("path is outside the repository")

// Service provides methods for interacting with a Git repository.
type Service struct{}

// NewService creates a new Service instance.
func NewService() *Service {
	return &Service{}
}

// GetStatusForFiles returns the porcelain status of the specified files.
func (s *Service) GetStatusForFiles(files []string) (string, error) {
	_, worktree, _, err := getRepo()
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w", err)
	}

	var builder strings.Builder
	for _, file := range files {
		if rel, err := toRel(file); err == nil {
			if st, ok := status[rel]; ok {
				builder.WriteString(fmt.Sprintf("%c%c %s\n", formatStatusCode(st.Staging), formatStatusCode(st.Worktree), rel))
			}
		}
	}
	return builder.String(), nil
}

// GetChangedFiles returns a sorted list of all modified, added, or deleted files in the repository.
func (s *Service) GetChangedFiles() ([]string, error) {
	_, worktree, _, err := getRepo()
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
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
func (s *Service) GetChangesForFiles(files []string) (string, error) {
	repo, _, _, err := getRepo()
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	headTree, _ := getHeadTree(repo)
	var builder strings.Builder

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

		newBytes, err := os.ReadFile(filepath.Clean(file))
		newText := string(newBytes)
		isDeleted := err != nil

		if isNew && isDeleted {
			continue
		}
		builder.WriteString(generateDiffString(rel, oldText, newText, isNew, isDeleted))
	}
	return builder.String(), nil
}

// Commit stages the specified files and creates a new commit with the given message.
func (s *Service) Commit(files []string, message string) error {
	repo, worktree, _, err := getRepo()
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	for _, file := range files {
		rel, err := toRel(file)
		if err != nil {
			return err
		}
		if _, err := worktree.Add(rel); err != nil {
			return fmt.Errorf("failed to add file %s: %w", rel, err)
		}
	}

	sig, err := s.getAuthorSignature(repo)
	if err != nil {
		return err
	}

	_, err = worktree.Commit(message, &git.CommitOptions{
		Author: sig,
	})
	if err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}
	return nil
}

func (s *Service) getAuthorSignature(r *git.Repository) (*object.Signature, error) {
	cfg, _ := r.Config()
	name, email := getNameEmail(cfg)

	if name == "" || email == "" {
		global, _ := gitconfig.LoadConfig(gitconfig.GlobalScope)
		gName, gEmail := getNameEmail(global)
		if name == "" {
			name = gName
		}
		if email == "" {
			email = gEmail
		}
	}

	if name == "" || email == "" {
		return nil, errors.New("git user name or email not found in local or global config")
	}

	return &object.Signature{
		Name:  name,
		Email: email,
		When:  time.Now(),
	}, nil
}

func getNameEmail(c *gitconfig.Config) (string, string) {
	if c == nil {
		return "", ""
	}
	return c.User.Name, c.User.Email
}

// Push pushes the current branch to the specified remote.
func (s *Service) Push(ctx context.Context, remoteName string) (string, error) {
	repo, _, _, err := getRepo()
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	remote, err := repo.Remote(remoteName)
	if err != nil {
		return "", fmt.Errorf("remote '%s' not found: %w", remoteName, err)
	}

	urls := remote.Config().URLs
	if len(urls) == 0 {
		return "", fmt.Errorf("remote '%s' has no URLs", remoteName)
	}

	auth, err := resolveAuth(urls[0])
	if err != nil {
		return "", err
	}

	err = repo.PushContext(ctx, &git.PushOptions{Auth: auth})
	if err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return "Already up-to-date", nil
		}
		return "", fmt.Errorf("failed to push: %w", err)
	}
	return "Push successful", nil
}

// ResolvePath returns a list of all repository files within the given path.
func (s *Service) ResolvePath(path string) ([]string, error) {
	repo, worktree, root, err := getRepo()
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return nil, fmt.Errorf("failed to get relative path: %w", err)
	}

	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		if strings.HasPrefix(rel, "..") {
			return nil, ErrOutsideRepo
		}
		return []string{rel}, nil
	}

	status, _ := worktree.Status()
	headTree, _ := getHeadTree(repo)

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

// ExtractVersionFromDiff scans a unified diff for lines that look like version updates.
// It returns a string representing the change, e.g., "0.4.0 -> 0.5.0".
func ExtractVersionFromDiff(diffText string) string {
	lines := strings.Split(diffText, "\n")

	// Improved regex: look for something that starts with a digit,
	// contains at least one dot, and then more digits/dots/alphanumerics.
	versionRegex := regexp.MustCompile(`([0-9]+\.[0-9][0-9a-z.-]*)`)

	var oldVersion, newVersion string
	var currentFile string

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				currentFile = filepath.Base(parts[3])
			}
			continue
		}

		// Only look for versions in specific files likely to contain project versioning
		// and EXCLUDE test files explicitly.
		isPotentialVersionFile := (strings.EqualFold(currentFile, "VERSION") ||
			strings.EqualFold(currentFile, "package.json") ||
			strings.EqualFold(currentFile, "go.mod") ||
			strings.EqualFold(currentFile, "Cargo.toml") ||
			strings.EqualFold(currentFile, "pyproject.toml") ||
			strings.EqualFold(currentFile, "composer.json") ||
			strings.EqualFold(currentFile, "Gemfile") ||
			strings.EqualFold(currentFile, "mix.exs") ||
			strings.EqualFold(currentFile, "version.rb") ||
			strings.EqualFold(currentFile, "version.py") ||
			strings.EqualFold(currentFile, "setup.py") ||
			strings.EqualFold(currentFile, "CMakeLists.txt")) &&
			!strings.Contains(strings.ToLower(currentFile), "test") &&
			!strings.Contains(strings.ToLower(currentFile), "_spec")

		lowerLine := strings.ToLower(line)
		containsVersionKeyword := strings.Contains(lowerLine, "version") &&
			!strings.Contains(lowerLine, "versioning") &&
			!strings.Contains(strings.ToLower(currentFile), "test")

		if isPotentialVersionFile || containsVersionKeyword {
			// Extract from removed line
			if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
				content := line[1:]
				if matches := versionRegex.FindStringSubmatch(content); len(matches) > 1 {
					oldVersion = matches[1]
				}
			}
			// Extract from added line
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				content := line[1:]
				if matches := versionRegex.FindStringSubmatch(content); len(matches) > 1 {
					newVersion = matches[1]
				}
			}
		}

		// If we found both in the same file hunk, and they are different, we're likely done
		if oldVersion != "" && newVersion != "" && oldVersion != newVersion {
			return fmt.Sprintf("%s -> %s", oldVersion, newVersion)
		}
	}

	return newVersion
}

// GetPullRequestURL generates a web URL to create a new pull request for the current branch.
func (s *Service) GetPullRequestURL(remoteName string) (string, error) {
	repo, _, _, err := getRepo()
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}
	branch := head.Name().Short()
	if !head.Name().IsBranch() {
		branch = head.Hash().String()
	}

	remote, err := repo.Remote(remoteName)
	if err != nil || len(remote.Config().URLs) == 0 {
		return "", fmt.Errorf("no remote %s found", remoteName)
	}

	remoteUrl := remote.Config().URLs[0]
	remoteUrl = normalizeGitURL(remoteUrl)

	switch {
	case strings.Contains(remoteUrl, "github.com"):
		return fmt.Sprintf("%s/pull/new/%s", remoteUrl, branch), nil
	case strings.Contains(remoteUrl, "gitlab.com"):
		return fmt.Sprintf("%s/-/merge_requests/new?merge_request[source_branch]=%s", remoteUrl, branch), nil
	case strings.Contains(remoteUrl, "bitbucket.org"):
		return fmt.Sprintf("%s/pull-requests/new?source=%s", remoteUrl, branch), nil
	default:
		return "", fmt.Errorf("unknown host: %s", remoteUrl)
	}
}

// --- Internal Helper Functions ---

func resolveAuth(urlStr string) (transport.AuthMethod, error) {
	if strings.HasPrefix(urlStr, "http") {
		return nil, errors.New("http URLs are not supported")
	}

	if !strings.HasPrefix(urlStr, "ssh://") && !strings.Contains(urlStr, "@") {
		return nil, errors.New("invalid URL format")
	}

	user := "git"
	if parts := strings.Split(urlStr, "@"); len(parts) > 1 {
		user = parts[0]
		if strings.Contains(user, "://") {
			user = strings.Split(user, "://")[1]
		}
	}

	keyCallback := func() ([]ssh.Signer, error) {
		var signers []ssh.Signer
		if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
			dialer := &net.Dialer{}
			if conn, err := dialer.Dial("unix", sock); err == nil {
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

			if key, err := os.ReadFile(filepath.Join(home, ".ssh", filepath.Clean(f.Name()))); err == nil {
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
		return nil, nil, "", fmt.Errorf("failed to get git root: %w", err)
	}
	repo, err := git.PlainOpen(root)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to open git repo at %s: %w", root, err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to get worktree: %w", err)
	}
	return repo, worktree, root, nil
}

func toRel(path string) (string, error) {
	root, err := GetGitRoot()
	if err != nil {
		return "", fmt.Errorf("failed to get git root: %w", err)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", fmt.Errorf("failed to get relative path: %w", err)
	}

	if strings.HasPrefix(rel, "..") {
		return "", ErrOutsideRepo
	}
	return rel, nil
}

func getHeadTree(repo *git.Repository) (*object.Tree, error) {
	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}
	commitObj, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get commit object: %w", err)
	}
	tree, err := commitObj.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree: %w", err)
	}
	return tree, nil
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

func generateDiffString(path, oldText, newText string, isNew, isDel bool) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldText, newText, false)
	dmp.DiffCleanupSemantic(diffs)

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", path, path))
	switch {
	case isNew:
		builder.WriteString(fmt.Sprintf("new file mode 100644\n--- /dev/null\n+++ b/%s\n", path))
	case isDel:
		builder.WriteString(fmt.Sprintf("deleted file mode 100644\n--- a/%s\n+++ /dev/null\n", path))
	default:
		builder.WriteString(fmt.Sprintf("--- a/%s\n+++ b/%s\n", path, path))
	}

	builder.WriteString("@@ -1 +1 @@\n")

	for _, diffObj := range diffs {
		lines := strings.Split(diffObj.Text, "\n")
		prefix := " "
		switch diffObj.Type {
		case diffmatchpatch.DiffInsert:
			prefix = "+"
		case diffmatchpatch.DiffDelete:
			prefix = "-"
		}

		for i, line := range lines {
			if i == len(lines)-1 && line == "" {
				continue
			}
			builder.WriteString(prefix)
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

func normalizeGitURL(rawURL string) string {
	cleanURL := strings.TrimSpace(rawURL)
	cleanURL = strings.TrimSuffix(cleanURL, ".git")
	if strings.HasPrefix(cleanURL, "git@") {
		cleanURL = "https://" + strings.Replace(strings.TrimPrefix(cleanURL, "git@"), ":", "/", 1)
	} else if strings.HasPrefix(cleanURL, "ssh://") {
		cleanURL = "https://" + strings.TrimPrefix(strings.Split(cleanURL, "@")[1], ":")
	}

	if !strings.HasPrefix(cleanURL, "http") {
		cleanURL = "https://" + cleanURL
	}
	return cleanURL
}
