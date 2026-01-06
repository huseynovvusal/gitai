package git

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
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
		isBinary := false

		if headTree != nil {
			if f, err := headTree.File(rel); err == nil {
				if bin, _ := f.IsBinary(); bin {
					isBinary = true
				}
				if c, err := f.Contents(); err == nil {
					oldText, isNew = c, false
				}
			}
		}

		newBytes, err := os.ReadFile(filepath.Clean(file))
		newText := string(newBytes)
		isDeleted := err != nil

		if !isBinary && !isDeleted {
			limit := 8000
			if len(newBytes) < limit {
				limit = len(newBytes)
			}
			for i := 0; i < limit; i++ {
				if newBytes[i] == 0 {
					isBinary = true
					break
				}
			}
		}

		if isNew && isDeleted {
			continue
		}

		if isBinary {
			builder.WriteString(fmt.Sprintf("diff --git a/%s b/%s\nBinary files differ\n", rel, rel))
			continue
		}

		if len(oldText) > 500*1024 || len(newText) > 500*1024 {
			builder.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\nBinary files or large files differ\n", rel, rel, rel, rel))
			continue
		}

		builder.WriteString(generateDiffString(rel, oldText, newText, isNew, isDeleted))
	}
	return builder.String(), nil
}

// GetLastCommitMessage returns the message of the HEAD commit.
func (s *Service) GetLastCommitMessage() (string, error) {
	repo, _, _, err := getRepo()
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	commitObj, err := repo.CommitObject(head.Hash())
	if err != nil {
		return "", fmt.Errorf("failed to get commit object: %w", err)
	}

	return commitObj.Message, nil
}

// GetFilesInLastCommit returns a list of files changed in the HEAD commit.
func (s *Service) GetFilesInLastCommit() ([]string, error) {
	repo, _, _, err := getRepo()
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	commitObj, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get commit object: %w", err)
	}

	currentTree, err := commitObj.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree: %w", err)
	}

	var parentTree *object.Tree
	if commitObj.NumParents() > 0 {
		parent, err := commitObj.Parent(0)
		if err == nil {
			parentTree, _ = parent.Tree()
		}
	}

	// If no parent (initial commit), Diff returns all files as inserts.
	// If parent exists, it returns changes.
	changes, err := object.DiffTree(parentTree, currentTree)
	if err != nil {
		return nil, fmt.Errorf("failed to diff tree: %w", err)
	}

	var files []string
	for _, change := range changes {
		// usage of names depends on action (insert/delete/modify)
		// For From (old) and To (new).
		// We generally want the name of the file in the resulting commit (To),
		// unless it was deleted (then From).
		// But if we are amending, we probably want to know what files were involved.
		// If a file was deleted in HEAD, and we Amend, do we want to "select" it?
		// "Selecting" it in the UI usually means "Add to index".
		// If we select a file that is currently deleted in WorkingTree (and was deleted in HEAD),
		// `worktree.Add` might behave as expected (keep it deleted).
		
		var name string
		if change.To.Name != "" {
			name = change.To.Name
		} else if change.From.Name != "" {
			name = change.From.Name
		}
		
		if name != "" {
			files = append(files, name)
		}
	}

	sort.Strings(files)
	return files, nil
}

// GetAmendChangesForFiles generates a diff comparing HEAD~1 to the current working tree
// for the specified files (plus files already in HEAD), simulating an --amend view.
func (s *Service) GetAmendChangesForFiles(files []string) (string, error) {
	repo, _, _, err := getRepo()
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	headCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD commit: %w", err)
	}

	// 1. Get the parent tree (HEAD~1)
	var parentTree *object.Tree
	if headCommit.NumParents() > 0 {
		parent, err := headCommit.Parent(0)
		if err == nil {
			parentTree, _ = parent.Tree()
		}
	}
	// If no parent (initial commit), parentTree is nil, which acts as empty tree (new files).

	// 2. Determine the set of files to diff.
	// This should include files currently in HEAD (so we see what's being kept)
	// PLUS the new files being amended in.
	filesMap := make(map[string]bool)
	for _, f := range files {
		filesMap[f] = true
	}

	// Add files from HEAD to the map
	headTree, err := headCommit.Tree()
	if err == nil {
		_ = headTree.Files().ForEach(func(f *object.File) error {
			// We need the full path relative to repo root. f.Name is relative to tree (root).
			// Our 'files' arg is usually relative to cwd or repo root? 
			// internal/tui calls usually pass paths that ResolvePath returns, which are relative to Repo Root.
			filesMap[f.Name] = true
			return nil
		})
	}

	combinedFiles := make([]string, 0, len(filesMap))
	for f := range filesMap {
		combinedFiles = append(combinedFiles, f)
	}
	sort.Strings(combinedFiles)

	// 3. Generate Diff against Parent Tree
	var builder strings.Builder
	for _, file := range combinedFiles {
		// oldText comes from Parent Tree
		oldText, isNew := "", true
		isBinary := false

		if parentTree != nil {
			if f, err := parentTree.File(file); err == nil {
				if bin, _ := f.IsBinary(); bin {
					isBinary = true
				}
				if c, err := f.Contents(); err == nil {
					oldText, isNew = c, false
				}
			}
		}

		// newText comes from Working Tree (simulating what will be committed)
		newBytes, err := os.ReadFile(filepath.Clean(file))
		newText := string(newBytes)
		isDeleted := err != nil

		if !isBinary && !isDeleted {
			// Check for binary content in new file (simple heuristic)
			limit := 8000
			if len(newBytes) < limit {
				limit = len(newBytes)
			}
			for i := 0; i < limit; i++ {
				if newBytes[i] == 0 {
					isBinary = true
					break
				}
			}
		}

		if isNew && isDeleted {
			continue
		}
		
		// Optimization: if content matches, skip (no diff)
		if oldText == newText && !isNew && !isDeleted {
			continue
		}

		if isBinary {
			builder.WriteString(fmt.Sprintf("diff --git a/%s b/%s\nBinary files differ\n", file, file))
			continue
		}

		// Safety check: skip diff generation for large files to avoid panics in diffmatchpatch
		if len(oldText) > 500*1024 || len(newText) > 500*1024 {
			builder.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\nBinary files or large files differ\n", file, file, file, file))
			continue
		}

		builder.WriteString(generateDiffString(file, oldText, newText, isNew, isDeleted))
	}

	return builder.String(), nil
}

// CommitAmend amends the last commit with the staged changes of the specified files.
func (s *Service) CommitAmend(files []string, message string) error {
	repo, worktree, _, err := getRepo()
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	// 1. Stage the files
	for _, file := range files {
		rel, err := toRel(file)
		if err != nil {
			return err
		}
		if _, err := worktree.Add(rel); err != nil {
			return fmt.Errorf("failed to add file %s: %w", rel, err)
		}
	}

	// 2. Get HEAD and its parents to preserve/update lineage
	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}
	headCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return fmt.Errorf("failed to get HEAD commit: %w", err)
	}

	parents := headCommit.ParentHashes
	// If we are amending, we keep the SAME parents as the commit we are replacing.
	// So we don't include HEAD itself, but HEAD's parents.

	sig, err := s.getAuthorSignature(repo)
	if err != nil {
		return err
	}

	// 3. Commit with explicit parents (Amending)
	_, err = worktree.Commit(message, &git.CommitOptions{
		Author:  sig,
		Parents: parents,
	})
	if err != nil {
		return fmt.Errorf("failed to amend commit: %w", err)
	}
	return nil
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

	auth := resolveAuth(urls[0])

	err = repo.PushContext(ctx, &git.PushOptions{Auth: auth})
	if err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return "Already up-to-date", nil
		}
		return "", fmt.Errorf("failed to push: %w", err)
	}
	return "Push successful", nil
}

// PushForce pushes the current branch to the specified remote with --force.
func (s *Service) PushForce(ctx context.Context, remoteName string) (string, error) {
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

	auth := resolveAuth(urls[0])

	err = repo.PushContext(ctx, &git.PushOptions{
		Auth:  auth,
		Force: true,
	})
	if err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return "Already up-to-date", nil
		}
		return "", fmt.Errorf("failed to force push: %w", err)
	}
	return "Force Push successful", nil
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
			// Extract from a removed line
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

func resolveAuth(urlStr string) transport.AuthMethod {
	if strings.HasPrefix(urlStr, "http") {
		return nil
	}

	if !strings.HasPrefix(urlStr, "ssh://") && !strings.Contains(urlStr, "@") {
		return nil
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

	return auth
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

// generateDiffString creates a unified diff compatible with standard git output.
//
// NOTE: We intentionally use the verbose "diff --git" header format (instead of
// simpler custom formats like "M file.go") because LLMs perform significantly
// better with it. The standard git headers act as strong "mental anchors" that
// prevent context bleeding between files and align with the model's training data.
func generateDiffString(path, oldText, newText string, isNew, isDel bool) (result string) {
	defer func() {
		if r := recover(); r != nil {
			// Fallback if diffmatchpatch panics
			result = fmt.Sprintf("diff --git a/%s b/%s\n(Diff generation failed: %v)\n", path, path, r)
		}
	}()

	dmp := diffmatchpatch.New()
	patches := dmp.PatchMake(oldText, newText)
	patchText := dmp.PatchToText(patches)

	// Normalize the text to avoid URL encoding
	decoded, _ := url.PathUnescape(patchText)

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

	builder.WriteString(decoded)

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
