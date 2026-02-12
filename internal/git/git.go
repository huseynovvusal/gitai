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

	"github.com/go-git/go-git/v6"
	gitconfig "github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/transport"
	gitssh "github.com/go-git/go-git/v6/plumbing/transport/ssh"
	"github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	MaxDiffSize     = 500 * 1024
	BinaryScanLimit = 8000
)

var ErrOutsideRepo = errors.New("path is outside the repository")

var ignoredFiles = map[string]bool{
	"go.sum":            true,
	"package-lock.json": true,
	"yarn.lock":         true,
	"pnpm-lock.yaml":    true,
	"composer.lock":     true,
	"Cargo.lock":        true,
	"Gemfile.lock":      true,
	"mix.lock":          true,
	"poetry.lock":       true,
	"uv.lock":           true,
}

var hunkHeaderRegex = regexp.MustCompile(`(?m)^@@\s.*\s@@\n`)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

// --- Public API ---

func (s *Service) GetStatusForFiles(files []string) (string, error) {
	ctx, err := s.getRepoContext()
	if err != nil {
		return "", err
	}
	status, err := ctx.worktree.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree status: %w", err)
	}
	var b strings.Builder
	for _, f := range files {
		if rel, err := s.toRel(f, ctx.root); err == nil {
			if st, ok := status[rel]; ok {
				b.WriteString(fmt.Sprintf("%c%c %s\n", formatStatusCode(st.Staging), formatStatusCode(st.Worktree), rel))
			}
		}
	}
	return b.String(), nil
}

func (s *Service) GetChangedFiles() ([]string, error) {
	ctx, err := s.getRepoContext()
	if err != nil {
		return nil, err
	}
	status, _ := ctx.worktree.Status()
	var changed []string
	for path, st := range status {
		if st.Staging != git.Unmodified || st.Worktree != git.Unmodified {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return changed, nil
}

func (s *Service) GetChangesForFiles(files []string) (string, error) {
	ctx, err := s.getRepoContext()
	if err != nil {
		return "", err
	}
	var headTree *object.Tree
	if ctx.head != nil {
		headTree, _ = ctx.head.Tree()
	}
	return s.generateBatchDiff(files, headTree, ctx.root), nil
}

func (s *Service) GetAmendChangesForFiles(files []string) (string, error) {
	ctx, err := s.getRepoContext()
	if err != nil {
		return "", err
	}
	var parentTree *object.Tree
	if ctx.head != nil && ctx.head.NumParents() > 0 {
		if p, err := ctx.head.Parent(0); err == nil {
			parentTree, _ = p.Tree()
		}
	}
	filesMap := make(map[string]bool)
	for _, f := range files {
		filesMap[f] = true
	}
	if ctx.head != nil {
		if ht, err := ctx.head.Tree(); err == nil {
			_ = ht.Files().ForEach(func(f *object.File) error {
				filesMap[f.Name] = true
				return nil
			})
		}
	}

	combined := make([]string, 0, len(filesMap))
	for f := range filesMap {
		combined = append(combined, f)
	}
	sort.Strings(combined)
	return s.generateBatchDiff(combined, parentTree, ctx.root), nil
}

func (s *Service) GetFilesInLastCommit() ([]string, error) {
	ctx, err := s.getRepoContext()
	if err != nil {
		return nil, err
	}
	if ctx.head == nil {
		return nil, nil
	}
	curr, _ := ctx.head.Tree()
	var prev *object.Tree
	if ctx.head.NumParents() > 0 {
		if p, err := ctx.head.Parent(0); err == nil {
			prev, _ = p.Tree()
		}
	}
	changes, err := object.DiffTree(prev, curr)
	if err != nil {
		return nil, fmt.Errorf("failed to diff trees: %w", err)
	}
	var files []string
	for _, c := range changes {
		name := c.To.Name
		if name == "" {
			name = c.From.Name
		}
		if name != "" {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	return files, nil
}

func (s *Service) GetLastCommitMessage() (string, error) {
	ctx, err := s.getRepoContext()
	if err != nil {
		return "", err
	}
	if ctx.head == nil {
		return "", errors.New("no commits found")
	}
	return ctx.head.Message, nil
}

func (s *Service) ResolvePath(path string) ([]string, error) {
	ctx, err := s.getRepoContext()
	if err != nil {
		return nil, err
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	rel, err := filepath.Rel(ctx.root, abs)
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

	status, _ := ctx.worktree.Status()
	headTree, _ := s.getHeadTree(ctx.repo)

	prefix := rel
	if prefix == "." {
		prefix = ""
	} else if !strings.HasSuffix(prefix, "/") {
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

func (s *Service) GetPullRequestURL(remoteName string) (string, error) {
	ctx, err := s.getRepoContext()
	if err != nil {
		return "", err
	}

	head, err := ctx.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get head: %w", err)
	}
	branch := head.Name().Short()

	remote, err := ctx.repo.Remote(remoteName)
	if err != nil || len(remote.Config().URLs) == 0 {
		return "", fmt.Errorf("remote %s not found: %w", remoteName, err)
	}

	urlStr := normalizeGitURL(remote.Config().URLs[0])
	switch {
	case strings.Contains(urlStr, "github.com"):
		return fmt.Sprintf("%s/pull/new/%s", urlStr, branch), nil
	case strings.Contains(urlStr, "gitlab.com"):
		return fmt.Sprintf("%s/-/merge_requests/new?merge_request[source_branch]=%s", urlStr, branch), nil
	case strings.Contains(urlStr, "bitbucket.org"):
		return fmt.Sprintf("%s/pull-requests/new?source=%s", urlStr, branch), nil
	}
	return "", fmt.Errorf("unsupported host for PR URL: %s", urlStr)
}

// --- Internal Engine ---

func (s *Service) toRel(p, root string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", ErrOutsideRepo
	}
	return rel, nil
}

func (s *Service) generateBatchDiff(files []string, oldTree *object.Tree, root string) string {
	var b strings.Builder
	for _, p := range files {
		rel, _ := s.toRel(p, root)
		if ignoredFiles[filepath.Base(rel)] {
			continue
		}
		diff := s.diffFile(rel, p, oldTree)
		b.WriteString(diff)
	}
	return b.String()
}

func (s *Service) diffFile(rel, full string, oldTree *object.Tree) string {
	var oldText string
	isNew, isBin := true, false
	if oldTree != nil {
		if f, err := oldTree.File(rel); err == nil {
			isBin, _ = f.IsBinary()
			oldText, _ = f.Contents()
			isNew = false
		}
	}
	newBytes, err := os.ReadFile(filepath.Clean(full))
	isDel := err != nil
	newText := string(newBytes)
	if !isBin && !isDel {
		l := len(newBytes)
		if l > BinaryScanLimit {
			l = BinaryScanLimit
		}
		for i := 0; i < l; i++ {
			if newBytes[i] == 0 {
				isBin = true
				break
			}
		}
	}
	if (isNew && isDel) || (oldText == newText && !isNew && !isDel) {
		return ""
	}
	if isBin {
		return fmt.Sprintf("diff --git a/%s b/%s\nBinary files differ\n", rel, rel)
	}
	if len(oldText) > MaxDiffSize || len(newText) > MaxDiffSize {
		return fmt.Sprintf("diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\nBinary files or large files differ\n", rel, rel, rel, rel)
	}
	return generateDiffString(rel, oldText, newText, isNew, isDel)
}

// --- Git Commands ---

func (s *Service) Commit(files []string, msg string) error {
	return s.performCommit(files, msg, false)
}

func (s *Service) CommitAmend(files []string, msg string) error {
	return s.performCommit(files, msg, true)
}

func (s *Service) performCommit(files []string, msg string, amend bool) error {
	ctx, err := s.getRepoContext()
	if err != nil {
		return err
	}
	for _, f := range files {
		rel, _ := s.toRel(f, ctx.root)
		if _, err := ctx.worktree.Add(rel); err != nil {
			return fmt.Errorf("failed to add file to worktree: %w", err)
		}
	}
	sig := s.getAuthorSignature(ctx.repo)
	opts := &git.CommitOptions{Author: sig}
	if amend && ctx.head != nil {
		opts.Parents = ctx.head.ParentHashes
	}
	_, err = ctx.worktree.Commit(msg, opts)
	if err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}
	return nil
}

func (s *Service) Push(ctx context.Context, remoteName string) (string, error) {
	return s.performPush(ctx, remoteName, false)
}

func (s *Service) PushForce(ctx context.Context, remoteName string) (string, error) {
	return s.performPush(ctx, remoteName, true)
}

func (s *Service) performPush(ctx context.Context, remoteName string, force bool) (string, error) {
	rctx, err := s.getRepoContext()
	if err != nil {
		return "", err
	}
	h, err := rctx.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get head: %w", err)
	}
	r, err := rctx.repo.Remote(remoteName)
	if err != nil {
		return "", fmt.Errorf("failed to get remote: %w", err)
	}
	auth := resolveAuth(r.Config().URLs[0])
	branch := h.Name()
	err = rctx.repo.PushContext(ctx, &git.PushOptions{
		RemoteName: remoteName, Auth: auth, Force: force,
		RefSpecs: []gitconfig.RefSpec{gitconfig.RefSpec(fmt.Sprintf("%s:%s", branch, branch))},
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return "", fmt.Errorf("failed to push to remote: %w", err)
	}
	return "Push successful", nil
}

func (s *Service) getAuthorSignature(r *git.Repository) *object.Signature {
	cfg, _ := r.Config()
	name, email := cfg.User.Name, cfg.User.Email
	if name == "" || email == "" {
		global, _ := gitconfig.LoadConfig(gitconfig.GlobalScope)
		if name == "" {
			name = global.User.Name
		}
		if email == "" {
			email = global.User.Email
		}
	}
	return &object.Signature{Name: name, Email: email, When: time.Now()}
}

func resolveAuth(urlStr string) transport.AuthMethod {
	if strings.HasPrefix(urlStr, "http") {
		return nil
	}
	if !strings.Contains(urlStr, "@") && !strings.HasPrefix(urlStr, "ssh://") {
		return nil
	}
	user := "git"
	if parts := strings.Split(urlStr, "@"); len(parts) > 1 {
		user = strings.TrimPrefix(parts[0], "ssh://")
	}
	auth := &gitssh.PublicKeysCallback{
		User: user,
		Callback: func() ([]ssh.Signer, error) {
			var signers []ssh.Signer
			if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
				if conn, err := (&net.Dialer{}).Dial("unix", sock); err == nil {
					agentSigners, _ := agent.NewClient(conn).Signers()
					signers = append(signers, agentSigners...)
				}
			}
			home, _ := os.UserHomeDir()
			for _, n := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
				if k, err := os.ReadFile(filepath.Join(home, ".ssh", n)); err == nil {
					if signer, err := ssh.ParsePrivateKey(k); err == nil {
						signers = append(signers, signer)
					}
				}
			}
			return signers, nil
		},
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
		return nil, nil, "", fmt.Errorf("failed to open repository: %w", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to get worktree: %w", err)
	}
	return repo, wt, root, nil
}

func (s *Service) getHeadTree(repo *git.Repository) (*object.Tree, error) {
	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get head: %w", err)
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get commit object: %w", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree from commit: %w", err)
	}
	return tree, nil
}

func formatStatusCode(c git.StatusCode) rune {
	mapping := map[git.StatusCode]rune{
		git.Unmodified: ' ', git.Modified: 'M', git.Added: 'A',
		git.Deleted: 'D', git.Renamed: 'R', git.Copied: 'C', git.Untracked: '?',
	}
	if r, ok := mapping[c]; ok {
		return r
	}
	return '?'
}

func normalizeGitURL(rawURL string) string {
	u := strings.TrimSuffix(strings.TrimSpace(rawURL), ".git")
	if strings.HasPrefix(u, "git@") {
		u = "https://" + strings.Replace(strings.TrimPrefix(u, "git@"), ":", "/", 1)
	} else if strings.HasPrefix(u, "ssh://") {
		parts := strings.Split(u, "@")
		if len(parts) > 1 {
			u = "https://" + parts[1]
		}
	}
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}
	return u
}

func generateDiffString(path, oldText, newText string, isNew, isDel bool) (result string) {
	defer func() {
		if r := recover(); r != nil {
			result = fmt.Sprintf("diff --git a/%s b/%s\n(Diff failed: %v)\n", path, path, r)
		}
	}()
	dmp := diffmatchpatch.New()

	a, b, c := dmp.DiffLinesToChars(oldText, newText)
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, c)
	dmp.DiffCleanupSemantic(diffs)

	patches := dmp.PatchMake(diffs)
	decoded, _ := url.PathUnescape(dmp.PatchToText(patches))
	decoded = hunkHeaderRegex.ReplaceAllString(decoded, "")

	var bld strings.Builder
	bld.WriteString(fmt.Sprintf("--- %s\n", path))

	bld.WriteString(decoded)
	return bld.String()
}

func (s *Service) HasRemotes() (bool, error) {
	ctx, err := s.getRepoContext()
	if err != nil {
		return false, err
	}
	remotes, err := ctx.repo.Remotes()
	if err != nil {
		return false, fmt.Errorf("failed to list remotes: %w", err)
	}
	return len(remotes) > 0, nil
}

func (s *Service) CommitStaged(msg string) error {
	ctx, err := s.getRepoContext()
	if err != nil {
		return err
	}
	sig := s.getAuthorSignature(ctx.repo)
	opts := &git.CommitOptions{Author: sig}
	_, err = ctx.worktree.Commit(msg, opts)
	if err != nil {
		return fmt.Errorf("failed to commit staged changes: %w", err)
	}
	return nil
}
