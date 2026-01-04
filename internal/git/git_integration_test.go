package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func setupTestRepo(t *testing.T) (string, *git.Repository) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "gitai-test")
	if err != nil {
		t.Fatal(err)
	}

	r, err := git.PlainInit(tempDir, false)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatal(err)
	}

	cfg, _ := r.Config()
	cfg.User.Name = "Test User"
	cfg.User.Email = "test@example.com"
	r.SetConfig(cfg)

	// Create initial commit to ensure 'master' branch exists
	w, _ := r.Worktree()
	os.WriteFile(filepath.Join(tempDir, "README.md"), []byte("# Test Repo"), 0644)
	w.Add("README.md")
	w.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "Test User", Email: "test@example.com", When: time.Now()},
	})

	return tempDir, r
}

func TestGetGitRoot(t *testing.T) {
	dir, _ := setupTestRepo(t)
	defer os.RemoveAll(dir)

	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)

	root, err := GetGitRoot()
	if err != nil {
		t.Fatalf("GetGitRoot failed: %v", err)
	}

	r1, _ := filepath.EvalSymlinks(root)
	d1, _ := filepath.EvalSymlinks(dir)
	if r1 != d1 {
		t.Errorf("expected root %s, got %s", dir, root)
	}
}

func TestGetGitRoot_NotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gitai-no-git")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(tempDir)

	_, err = GetGitRoot()
	if err == nil {
		t.Error("expected error when not in a git repository, got nil")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("expected 'not a git repository' error, got: %v", err)
	}
}

func TestFindGitRoot(t *testing.T) {
	// 1. Valid git repo
	dir, _ := setupTestRepo(t)
	defer os.RemoveAll(dir)
	root, err := findGitRoot(dir)
	if err != nil || root == "" {
		t.Errorf("findGitRoot failed on valid dir: %v", err)
	}

	// 2. Not a git repo
	tempDir, _ := os.MkdirTemp("", "gitroot-fail")
	defer os.RemoveAll(tempDir)
	_, err = findGitRoot(tempDir)
	if err == nil {
		t.Error("expected error for non-git dir")
	}

	// 3. Invalid path
	_, err = findGitRoot("\x00")
	if err == nil {
		t.Log("Note: null byte didn't trigger filepath.Abs error in this environment")
	}
}

func TestGetChangedFiles(t *testing.T) {
	dir, wRepo := setupTestRepo(t)
	defer os.RemoveAll(dir)
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)

	gs := NewService()
	w, _ := wRepo.Worktree()
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content1"), 0644)

	files, _ := gs.GetChangedFiles()
	if len(files) != 1 || files[0] != "file1.txt" {
		t.Errorf("expected [file1.txt], got %v", files)
	}

	w.Add("file1.txt")
	w.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "Me", Email: "me@me.com", When: time.Now()},
	})

	files, _ = gs.GetChangedFiles()
	if len(files) != 0 {
		t.Errorf("expected [], got %v", files)
	}
}

func TestCommit(t *testing.T) {
	dir, wRepo := setupTestRepo(t)
	defer os.RemoveAll(dir)
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)

	gs := NewService()

	// 1. Add/Commit
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	if err := gs.Commit([]string{"main.go"}, "Add main"); err != nil {
		t.Fatal(err)
	}

	// 2. Delete/Commit
	os.Remove(filepath.Join(dir, "main.go"))
	if err := gs.Commit([]string{"main.go"}, "Delete main"); err != nil {
		t.Fatal(err)
	}

	// 3. Rename/Commit
	os.WriteFile(filepath.Join(dir, "old.txt"), []byte("content"), 0644)
	gs.Commit([]string{"old.txt"}, "Add old")
	os.Rename(filepath.Join(dir, "old.txt"), filepath.Join(dir, "new.txt"))
	if err := gs.Commit([]string{"old.txt", "new.txt"}, "Rename"); err != nil {
		t.Fatal(err)
	}

	head, _ := wRepo.Head()
	iter, _ := wRepo.Log(&git.LogOptions{From: head.Hash()})
	var messages []string
	iter.ForEach(func(c *object.Commit) error {
		messages = append(messages, c.Message)
		return nil
	})

	for _, msg := range []string{"Rename", "Add old", "Delete main", "Add main"} {
		found := false
		for _, m := range messages {
			if strings.Contains(m, msg) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing commit message: %s", msg)
		}
	}
}

func TestGetChangesForFiles(t *testing.T) {
	dir, wRepo := setupTestRepo(t)
	defer os.RemoveAll(dir)
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	
	gs := NewService()
	w, _ := wRepo.Worktree()

	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0644)
	diff, _ := gs.GetChangesForFiles([]string{"new.txt"})
	if !strings.Contains(diff, "new file") {
		t.Error("expected new file diff")
	}

	w.Add("new.txt")
	w.Commit("init", &git.CommitOptions{
		Author: &object.Signature{Name: "Me", Email: "me@me.com", When: time.Now()},
	})

	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello world"), 0644)
	diff, _ = gs.GetChangesForFiles([]string{"new.txt"})
	if !strings.Contains(diff, "hello") || !strings.Contains(diff, "world") {
		t.Errorf("diff missing expected content: %s", diff)
	}
}

func TestGetStatusForFiles(t *testing.T) {
	dir, _ := setupTestRepo(t)
	defer os.RemoveAll(dir)
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)

	gs := NewService()

	os.WriteFile(filepath.Join(dir, "status.txt"), []byte("data"), 0644)
	status, err := gs.GetStatusForFiles([]string{"status.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "?? status.txt") {
		t.Errorf("expected untracked status, got: %q", status)
	}
}

func TestResolvePath(t *testing.T) {
	dir, wRepo := setupTestRepo(t)
	defer os.RemoveAll(dir)
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	
	gs := NewService()
	w, _ := wRepo.Worktree()

	os.Mkdir(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "subdir/a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "subdir/b.txt"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(dir, "root.txt"), []byte("root"), 0644)

	w.Add("subdir/a.txt")
	w.Commit("init", &git.CommitOptions{
		Author: &object.Signature{Name: "Me", Email: "me@me.com", When: time.Now()},
	})

	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{"Single File", "subdir/a.txt", []string{"subdir/a.txt"}},
		{"Directory", "subdir", []string{"subdir/a.txt", "subdir/b.txt"}},
		{"Root Dot", ".", []string{"README.md", "root.txt", "subdir/a.txt", "subdir/b.txt"}},
		{"Non-existent", "missing.txt", []string{"missing.txt"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := gs.ResolvePath(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != len(tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, results)
			}
			for i, r := range results {
				if r != tt.expected[i] {
					t.Errorf("at index %d: expected %s, got %s", i, tt.expected[i], r)
				}
			}
		})
	}
}

func TestGetPullRequestURL(t *testing.T) {
	dir, r := setupTestRepo(t)
	defer os.RemoveAll(dir)
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)

	gs := NewService()

	tests := []struct {
		name     string
		remote   string
		expected string
	}{
		{"GitHub", "https://github.com/user/repo.git", "https://github.com/user/repo/pull/new/master"},
		{"GitLab", "git@gitlab.com:user/repo.git", "https://gitlab.com/user/repo/-/merge_requests/new?merge_request[source_branch]=master"},
		{"Bitbucket", "https://bitbucket.org/user/repo", "https://bitbucket.org/user/repo/pull-requests/new?source=master"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r.CreateRemote(&config.RemoteConfig{
				Name: "origin",
				URLs: []string{tt.remote},
			})
			defer r.DeleteRemote("origin")

			url, err := gs.GetPullRequestURL("origin")
			if err != nil {
				t.Fatal(err)
			}
			if url != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, url)
			}
		})
	}
}

func TestPush(t *testing.T) {
	remoteDir, err := os.MkdirTemp("", "gitai-remote")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(remoteDir)
	_, err = git.PlainInit(remoteDir, true)
	if err != nil {
		t.Fatal(err)
	}

	localDir, r := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(localDir)

	ctx := context.Background()
	gs := NewService()

	_, err = r.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{remoteDir},
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := gs.Push(ctx, "origin")
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	if !strings.Contains(out, "Push successful") {
		t.Errorf("unexpected push output: %s", out)
	}

	remoteRepo, _ := git.PlainOpen(remoteDir)
	_, err = remoteRepo.Head()
	if err != nil {
		t.Errorf("remote repo has no HEAD after push: %v", err)
	}
}
