package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func setupTestRepo(t *testing.T) (string, *git.Repository) {
	t.Helper()

	// Create a temp directory
	tempDir, err := os.MkdirTemp("", "gitai-test")
	if err != nil {
		t.Fatal(err)
	}

	// Initialize a new git repository
	r, err := git.PlainInit(tempDir, false)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatal(err)
	}

	// Configure user
	cfg, _ := r.Config()
	cfg.User.Name = "Test User"
	cfg.User.Email = "test@example.com"
	r.SetConfig(cfg)

	return tempDir, r
}

func TestGetGitRoot(t *testing.T) {
	dir, _ := setupTestRepo(t)
	defer os.RemoveAll(dir)

	// Change to the repo directory
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)

	root, err := GetGitRoot()
	if err != nil {
		t.Fatalf("GetGitRoot failed: %v", err)
	}

	// On macOS, temp dirs might be symlinked (/var/folders vs /private/var/folders)
	// simple comparison might fail.
	// But let's check if they point to same place.
	if filepath.Clean(root) != filepath.Clean(dir) {
		// Use EvalSymlinks to be sure
		r1, _ := filepath.EvalSymlinks(root)
		d1, _ := filepath.EvalSymlinks(dir)
		if r1 != d1 {
			t.Errorf("expected root %s, got %s", dir, root)
		}
	}
}

func TestGetChangedFiles(t *testing.T) {
	dir, wRepo := setupTestRepo(t)
	defer os.RemoveAll(dir)

	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)

	w, _ := wRepo.Worktree()

	// 1. Create a file
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content1"), 0644)

	// 2. Untracked file should be detected
	files, err := GetChangedFiles()
	if err != nil {
		t.Fatalf("GetChangedFiles failed: %v", err)
	}
	if len(files) != 1 || files[0] != "file1.txt" {
		t.Errorf("expected [file1.txt], got %v", files)
	}

	// 3. Stage the file
	w.Add("file1.txt")
	
	// 4. Staged file should be detected
	files, err = GetChangedFiles()
	if err != nil {
		t.Fatalf("GetChangedFiles failed: %v", err)
	}
	if len(files) != 1 || files[0] != "file1.txt" {
		t.Errorf("expected [file1.txt], got %v", files)
	}

	// 5. Commit
	w.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "Me", Email: "me@me.com", When: time.Now()},
	})

	// 6. No changes
	files, err = GetChangedFiles()
	if err != nil {
		t.Fatalf("GetChangedFiles failed: %v", err)
	}
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

	// Create and commit a file
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	
	err := Commit([]string{"main.go"}, "Add main.go")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify log
	head, _ := wRepo.Head()
	c, _ := wRepo.CommitObject(head.Hash())
	if c.Message != "Add main.go" {
		t.Errorf("expected commit message 'Add main.go', got '%s'", c.Message)
	}
}

func TestGetChangesForFiles(t *testing.T) {
	dir, wRepo := setupTestRepo(t)
	defer os.RemoveAll(dir)

	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(dir)
	w, _ := wRepo.Worktree()

	// Case 1: New file (untracked)
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello world"), 0644)
	diff, err := GetChangesForFiles([]string{"new.txt"})
	if err != nil {
		t.Fatalf("GetChangesForFiles failed: %v", err)
	}
	// Our simplified implementation just dumps content for new files
	if diff == "" {
		t.Error("expected diff/content, got empty")
	}

	// Case 2: Modified file
	w.Add("new.txt")
	w.Commit("init", &git.CommitOptions{
		Author: &object.Signature{Name: "Me", Email: "me@me.com", When: time.Now()},
	})

	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello world updated"), 0644)
	
	diff, err = GetChangesForFiles([]string{"new.txt"})
	if err != nil {
		t.Fatalf("GetChangesForFiles failed: %v", err)
	}
	
	// Should contain the new content
	if len(diff) < 10 {
		t.Errorf("expected diff content, got: %s", diff)
	}
}
