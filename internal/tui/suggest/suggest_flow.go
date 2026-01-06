package suggest

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	tea "github.com/charmbracelet/bubbletea"
	"huseynovvusal/gitai/internal/ai"
)

var LinksRegex = regexp.MustCompile(`remote:\s*(https?://\S+)`)

type GitRepoStatus interface {
	GetChangedFiles() ([]string, error)
}

type GitResolver interface {
	ResolvePath(path string) ([]string, error)
}

type GitPRGenerator interface {
	GetPullRequestURL(remoteName string) (string, error)
}

type GitDiffStatus interface {
	GetChangesForFiles(files []string) (string, error)
	GetStatusForFiles(files []string) (string, error)
}

type GitCommitter interface {
	Commit(files []string, message string) error
	Push(ctx context.Context, remoteName string) (string, error)
}

// We keep a private combined interface for convenience in Flow but
// use the smaller ones where appropriate if we were to pass them around.
type suggestGitService interface {
	GitRepoStatus
	GitResolver
	GitPRGenerator
	GitDiffStatus
	GitCommitter
	GetLastCommitMessage() (string, error)
	GetFilesInLastCommit() ([]string, error)
	GetAmendChangesForFiles(files []string) (string, error)
	CommitAmend(files []string, message string) error
	PushForce(ctx context.Context, remoteName string) (string, error)
}

type Flow struct {
	generator      ai.CommitMessageGenerator
	gitService     suggestGitService
	config         FlowConfig
	hintProcessors []HintProcessor
	hint           string
	skipHint       bool
}

type FlowConfig struct {
	EditorMode       string
	SecurityKeywords []string
	Amend            bool
	ForcePush        bool
}

func NewFlow(generator ai.CommitMessageGenerator, gs suggestGitService, cfg FlowConfig, hintProcessors ...HintProcessor) *Flow {
	return &Flow{
		generator:      generator,
		gitService:     gs,
		config:         cfg,
		hintProcessors: hintProcessors,
	}
}

func (s *Flow) WithHint(hint string) *Flow {
	s.hint = hint

	return s
}

func (s *Flow) WithSkipHint(skip bool) *Flow {
	s.skipHint = skip

	return s
}

func (s *Flow) Run(ctx context.Context, filesFromArgs []string) {
	// 1. Get all changed files from Git
	changedFiles, err := s.gitService.GetChangedFiles()
	if err != nil {
		panic(err)
	}

	var preSelectedFiles []string
	if s.config.Amend {
		prevFiles, err := s.gitService.GetFilesInLastCommit()
		if err == nil {
			preSelectedFiles = prevFiles
			// Merge previous files into available files so they appear in the list
			changedFiles = uniqueStrings(append(changedFiles, prevFiles...))
		}
	}

	if len(changedFiles) == 0 {
		println("No changed files to commit.")

		return
	}

	// 2. Determine which files to use (via Args or UI)
	selectedFiles := s.selectFiles(changedFiles, filesFromArgs, preSelectedFiles)
	if len(selectedFiles) == 0 {
		println("No valid files selected.")

		return
	}

	// 3. Get User Hint
	hint := s.hint
	if !s.skipHint && hint == "" {
		hint, err = s.runHintInput()
		if err != nil {
			return // User quit
		}
	}

	// 4. Run AI Generation Flow
	aiModel := NewAIMessageModel(ctx, selectedFiles, s.generator, s.gitService, MessageConfig{
		EditorMode:       s.config.EditorMode,
		SecurityKeywords: s.config.SecurityKeywords,
		Amend:            s.config.Amend,
		ForcePush:        s.config.ForcePush,
	}, hint)
	aiModelProgram := tea.NewProgram(&aiModel, tea.WithContext(ctx))

	finalModel, err := aiModelProgram.Run()
	if err != nil {
		aiModelProgram.Println(err)
	}

	// 5. Post-Run Actions (PR Links)
	if m, ok := finalModel.(*AIMessageModel); ok && m.state == StatePushed {
		s.printPullRequestInfo(m.pushOutput)
	}
}

// selectFiles determines if we filter arguments or show the UI selector.
func (s *Flow) selectFiles(availableFiles []string, args []string, preSelected []string) []string {
	if len(args) > 0 {
		// Logic extracted here for testability
		// If args are provided, we ignore preSelected from Amend?
		// Or should we merge them?
		// Standard git commit --amend [files] usually ONLY updates [files].
		// But here we are generating a message for the *result*.
		// If user provides args, they probably mean "only these files".
		return s.FilterCompatibleFiles(availableFiles, args)
	}

	// Fallback to TUI if no args provided
	fileSelectorModel := NewFileSelectorModel(availableFiles, preSelected...)

	fileSelectorProgram := tea.NewProgram(&fileSelectorModel)
	if _, err := fileSelectorProgram.Run(); err != nil {
		panic(err)
	}

	if fileSelectorModel.quitting {
		return nil
	}

	return fileSelectorModel.GetSelectedFiles()
}

// FilterCompatibleFiles takes a list of changed files and a list of patterns (args),
// resolves the patterns, and returns only the files that actually exist in the changed list.
func (s *Flow) FilterCompatibleFiles(availableFiles []string, patterns []string) []string {
	validMap := make(map[string]bool, len(availableFiles))
	for _, f := range availableFiles {
		validMap[f] = true
	}

	var selected []string

	for _, arg := range patterns {
		resolvedPaths, err := s.gitService.ResolvePath(arg)
		if err != nil {
			fmt.Printf("Warning: error resolving file '%s': %v\n", arg, err)

			continue
		}

		if len(resolvedPaths) == 0 {
			fmt.Printf("Warning: '%s' matched no tracked files\n", arg)

			continue
		}

		foundAnyChange := false

		for _, resolved := range resolvedPaths {
			if validMap[resolved] {
				selected = append(selected, resolved)
				foundAnyChange = true
			}
		}

		if !foundAnyChange {
			fmt.Printf("Warning: no changed files found matching '%s'\n", arg)
		}
	}

	return uniqueStrings(selected)
}

// runHintInput isolates the TUI logic for hints.
func (s *Flow) runHintInput() (string, error) {
	hintInputModel := NewHintInputModel(s.hintProcessors...)

	hintInputProgram := tea.NewProgram(&hintInputModel)
	if _, err := hintInputProgram.Run(); err != nil {
		return "", fmt.Errorf("error running hint input: %w", err)
	}

	if hintInputModel.quitting {
		return "", errors.New("canceled")
	}

	return hintInputModel.GetHint(), nil
}

// printPullRequestInfo handles the output parsing for PR links.
func (s *Flow) printPullRequestInfo(pushOutput string) {
	// Preferred: Git config
	prURL, err := s.gitService.GetPullRequestURL("origin")
	if err == nil && prURL != "" {
		fmt.Printf("\nCreate a Pull Request: %s\n", prURL)

		return
	}

	// Fallback: Regex scan of output
	matches := LinksRegex.FindAllStringSubmatch(pushOutput, -1)
	if len(matches) > 0 {
		fmt.Println()
	}

	for _, match := range matches {
		if len(match) > 1 {
			fmt.Printf("Create a Pull Request: %s\n", match[1])
		}
	}
}

func uniqueStrings(slice []string) []string {
	keys := make(map[string]bool)

	list := make([]string, 0, len(slice))

	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true

			list = append(list, entry)
		}
	}

	return list
}
