package git

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/filemode"
	"github.com/go-git/go-git/v6/plumbing/format/index"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// GetHunks returns all diff hunks for the specified files.
func (s *Service) GetHunks(files []string) ([]DiffHunk, error) {
	ctx, err := s.getRepoContext()
	if err != nil {
		return nil, err
	}

	var headTree *object.Tree
	if ctx.head != nil {
		headTree, _ = ctx.head.Tree()
	}

	hunks := make([]DiffHunk, 0, len(files))
	globalID := 1

	for _, p := range files {
		rel, err := s.toRel(p, ctx.root)
		if err != nil {
			continue
		}

		fileHunks := s.extractFileHunks(rel, p, headTree, &globalID)
		hunks = append(hunks, fileHunks...)
	}

	return hunks, nil
}

func (s *Service) extractFileHunks(rel, full string, oldTree *object.Tree, idCounter *int) []DiffHunk {
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
		if isBinary(newBytes) {
			isBin = true
		}
	}

	if (isNew && isDel) || (oldText == newText && !isNew && !isDel) || isBin {
		return nil
	}

	if len(oldText) > MaxDiffSize || len(newText) > MaxDiffSize {
		return nil
	}

	dmp := diffmatchpatch.New()
	dmp.PatchMargin = 1 // Use 1-line context to save tokens for AI
	a, b, c := dmp.DiffLinesToChars(oldText, newText)
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, c)
	dmp.DiffCleanupSemantic(diffs)
	patches := dmp.PatchMake(diffs)

	hunks := make([]DiffHunk, 0, len(patches))

	for _, p := range patches {
		singlePatchList := []diffmatchpatch.Patch{p}
		hunkContent, _ := url.PathUnescape(dmp.PatchToText(singlePatchList))

		endLine := p.Start2 + p.Length2
		if p.Length2 > 0 {
			endLine--
		}

		hunk := DiffHunk{
			ID:        *idCounter,
			File:      rel,
			StartLine: p.Start2,
			EndLine:   endLine,
			Content:   hunkContent,
			Patch:     singlePatchList,
		}
		hunks = append(hunks, hunk)
		*idCounter++
	}

	return hunks
}

func isBinary(data []byte) bool {
	l := len(data)
	if l > BinaryScanLimit {
		l = BinaryScanLimit
	}
	for i := 0; i < l; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

// ApplyHunks applies specific hunks to the index (staging them).
func (s *Service) ApplyHunks(hunks []DiffHunk) error {
	ctx, err := s.getRepoContext()
	if err != nil {
		return err
	}

	fileHunks := make(map[string][]diffmatchpatch.Patch)
	for _, h := range hunks {
		fileHunks[h.File] = append(fileHunks[h.File], h.Patch...)
	}

	dmp := diffmatchpatch.New()

	// Get Index once
	idx, err := ctx.repo.Storer.Index()
	if err != nil {
		return fmt.Errorf("failed to get index: %w", err)
	}

	for file, patches := range fileHunks {
		var content string
		if ctx.head != nil {
			tree, _ := ctx.head.Tree()
			if f, err := tree.File(file); err == nil {
				content, _ = f.Contents()
			}
		}

		newContent, results := dmp.PatchApply(patches, content)
		for i, success := range results {
			if !success {
				return fmt.Errorf("failed to apply patch %d for file %s", i, file)
			}
		}

		obj := ctx.repo.Storer.NewEncodedObject()
		obj.SetType(plumbing.BlobObject)
		obj.SetSize(int64(len(newContent)))

		writer, err := obj.Writer()
		if err != nil {
			return fmt.Errorf("failed to create blob writer: %w", err)
		}
		if _, err := io.Copy(writer, bytes.NewBufferString(newContent)); err != nil {
			writer.Close()
			return fmt.Errorf("failed to write blob content: %w", err)
		}
		if err := writer.Close(); err != nil {
			return fmt.Errorf("failed to close blob writer: %w", err)
		}

		hash, err := ctx.repo.Storer.SetEncodedObject(obj)
		if err != nil {
			return fmt.Errorf("failed to store blob: %w", err)
		}

		entry, _ := idx.Entry(file)
		if entry == nil {
			entry = &index.Entry{Name: file}
			idx.Entries = append(idx.Entries, entry)
		}

		entry.Hash = hash
		entry.ModifiedAt = time.Now()
		entry.Size = uint32(len(newContent))
		entry.Mode = filemode.Regular

		if info, err := os.Stat(filepath.Join(ctx.root, file)); err == nil {
			entry.ModifiedAt = info.ModTime()
		}
	}

	if err := ctx.repo.Storer.SetIndex(idx); err != nil {
		return fmt.Errorf("failed to update index: %w", err)
	}

	return nil
}
