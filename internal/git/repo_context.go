package git

import (
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
)

// repoContext centralizes the common git objects needed for operations.
type repoContext struct {
	repo     *git.Repository
	worktree *git.Worktree
	root     string
	head     *object.Commit // Nil if initial commit
}

func (s *Service) getRepoContext() (*repoContext, error) {
	repo, wt, root, err := getRepo()
	if err != nil {
		return nil, err
	}
	ctx := &repoContext{repo: repo, worktree: wt, root: root}
	if h, err := repo.Head(); err == nil {
		ctx.head, _ = repo.CommitObject(h.Hash())
	}
	return ctx, nil
}
