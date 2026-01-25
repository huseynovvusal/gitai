package git

import (
	"fmt"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// DiffHunk represents a single atomic change block in a file.
type DiffHunk struct {
	ID        int                    // Unique ID for AI reference
	File      string                 // Relative path
	StartLine int                    // Start line in the new file
	EndLine   int                    // End line in the new file
	Header    string                 // Git diff header (diff --git ..., --- a/..., +++ b/...)
	Content   string                 // The actual hunk content (@@ ... @@\n...)
	FullText  string                 // Helper: Header + Content (valid patch for this single hunk)
	Patch     []diffmatchpatch.Patch // Raw patch object for application
}

func (h DiffHunk) String() string {
	return fmt.Sprintf("Hunk %d (%s:%d-%d):\n%s", h.ID, h.File, h.StartLine, h.EndLine, h.Content)
}
