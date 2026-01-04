package suggest

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// --- Mocks ---

type mockGenerator struct {
	response string
	err      error
}

func (m *mockGenerator) Generate(ctx context.Context, diff, status, hint string) (string, error) {
	return m.response, m.err
}

type mockGitService struct {
	statusResponse  string
	changedFiles    []string
	changesResponse string
	resolveResponse []string
	commitErr       error
	pushResponse    string
	prURL           string
}

func (m *mockGitService) GetStatusForFiles(ctx context.Context, files []string) (string, error) {
	return m.statusResponse, nil
}
func (m *mockGitService) GetChangedFiles(ctx context.Context) ([]string, error) { return m.changedFiles, nil }
func (m *mockGitService) GetChangesForFiles(ctx context.Context, files []string) (string, error) {
	return m.changesResponse, nil
}
func (m *mockGitService) Commit(ctx context.Context, files []string, message string) error { return m.commitErr }
func (m *mockGitService) Push(ctx context.Context, remoteName string) (string, error) {
	return m.pushResponse, nil
}
func (m *mockGitService) ResolvePath(ctx context.Context, path string) ([]string, error) {
	return m.resolveResponse, nil
}
func (m *mockGitService) GetPullRequestURL(ctx context.Context, remoteName string) (string, error) {
	return m.prURL, nil
}

// --- Tests ---

func TestFileSelectorModel(t *testing.T) {
	files := []string{"file1.go", "file2.go"}
	m := NewFileSelectorModel(files)

	// 1. Initial State
	if len(m.GetSelectedFiles()) != 0 {
		t.Error("expected 0 selected files initially")
	}

	// 2. Select All
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = *m2.(*FileSelectorModel)
	if len(m.GetSelectedFiles()) != 2 {
		t.Errorf("expected 2 selected files, got %d", len(m.GetSelectedFiles()))
	}

	// 3. Confirm (Enter)
	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = *m3.(*FileSelectorModel)
	if !m.done {
		t.Error("expected model to be done after Enter")
	}

	// 4. View when done
	view := m.View()
	if !strings.Contains(view, "Selected files:") {
		t.Errorf("view missing header: %s", view)
	}

	// 5. Init
	if cmd := m.Init(); cmd != nil {
		t.Error("expected Init to return nil")
	}

	// 6. Window Resize
	m4, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = *m4.(*FileSelectorModel)
}

func TestHintInputModel(t *testing.T) {
	m := NewHintInputModel()

	// 1. Initial View
	if !strings.Contains(m.View(), "Provide a hint") {
		t.Error("view missing header")
	}

	// 2. Init
	if m.Init() == nil {
		t.Error("expected Init to return textinput.Blink")
	}

	// 3. Type a hint
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f', 'i', 'x'}})
	m = *m2.(*HintInputModel)

	// 4. Press Enter
	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = *m3.(*HintInputModel)

	if !m.done {
		t.Error("expected model to be done after Enter")
	}
	if m.GetHint() != "fix" {
		t.Errorf("expected hint 'fix', got %q", m.GetHint())
	}

	// 5. Done View
	if !strings.Contains(m.View(), "Hint provided:") {
		t.Error("done view missing header")
	}

	// 6. Escape/Quit
	m = NewHintInputModel()
	m4, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = *m4.(*HintInputModel)
	if !m.quitting || m.View() != "" {
		t.Error("expected empty view on quit")
	}
}

func TestAIMessageModel_States(t *testing.T) {
	ctx := context.Background()
	gen := &mockGenerator{response: "feat: add tests"}
	gs := &mockGitService{}
	m := NewAIMessageModel(ctx, []string{"test.go"}, gen, gs, MessageConfig{
		EditorMode:       "builtin",
		SecurityKeywords: []string{"secret"},
	}, "hint")

	// 1. Initial state & View
	if m.state != StateGenerating {
		t.Errorf("expected StateGenerating, got %v", m.state)
	}
	if !strings.Contains(m.View(), "Generating") {
		t.Error("expected generating view")
	}

	// 2. Receive AI response
	m2, _ := m.Update(aiDoneMsg{message: "feat: add tests"})
	m = *m2.(*AIMessageModel)
	if m.state != StateGenerated || m.commitMessage != "feat: add tests" {
		t.Errorf("state mismatch after aiDoneMsg: %v", m.state)
	}
	if !strings.Contains(m.View(), "AI commit message suggestion") {
		t.Error("expected generated view")
	}

	// 3. Start Commit (Press 'c')
	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = *m3.(*AIMessageModel)
	if m.state != StateCommitting {
		t.Error("expected StateCommitting after 'c'")
	}
	if !strings.Contains(m.View(), "Committing") {
		t.Error("expected committing view")
	}

	// 4. Commit Success
	m4, _ := m.Update(commitResultMsg{err: nil})
	m = *m4.(*AIMessageModel)
	if m.state != StateCommitted {
		t.Error("expected StateCommitted after success")
	}
	if !strings.Contains(m.View(), "Committed successfully") {
		t.Error("expected committed view")
	}

	// 5. Start Push (Press 'p')
	m5, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = *m5.(*AIMessageModel)
	if m.state != StatePushing {
		t.Error("expected StatePushing after 'p'")
	}
	if !strings.Contains(m.View(), "Pushing") {
		t.Error("expected pushing view")
	}

	// 6. Push Success
	m6, _ := m.Update(pushResultMsg{output: "pushed", err: nil})
	m = *m6.(*AIMessageModel)
	if m.state != StatePushed {
		t.Error("expected StatePushed after success")
	}
	if !strings.Contains(m.View(), "Pushed successfully") {
		t.Error("expected pushed view")
	}
}

func TestAIMessageModel_ErrorsAndSecurity(t *testing.T) {
	ctx := context.Background()
	gs := &mockGitService{}
	m := NewAIMessageModel(ctx, nil, nil, gs, MessageConfig{
		EditorMode:       "builtin",
		SecurityKeywords: []string{"secret"},
	}, "")

	// 1. AI Error
	m2, _ := m.Update(aiErrorMsg{err: context.DeadlineExceeded})
	m = *m2.(*AIMessageModel)
	if m.state != StateError || !strings.Contains(m.View(), "Commit failed") {
		t.Error("expected error state and view")
	}

	// 2. Security Warning
	m = NewAIMessageModel(ctx, nil, nil, gs, MessageConfig{
		EditorMode:       "builtin",
		SecurityKeywords: []string{"secret"},
	}, "")
	m3, _ := m.Update(commitSecurityWarningMsg{err: context.Canceled, diff: "diff", status: "status"})
	m = *m3.(*AIMessageModel)
	if m.state != StateSecurityWarning || !strings.Contains(m.View(), "potential sensitive data") {
		t.Error("expected security warning state and view")
	}

	// 3. Confirm Security Warning (Yes)
	m4, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = *m4.(*AIMessageModel)
	if m.state != StateGenerating {
		t.Error("expected generating state after confirming security")
	}

	// 4. Deny Security Warning (No)
	m = NewAIMessageModel(ctx, nil, nil, gs, MessageConfig{
		EditorMode:       "builtin",
		SecurityKeywords: []string{"secret"},
	}, "")
	m.state = StateSecurityWarning
	m5, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = *m5.(*AIMessageModel)
	if m.state != StateError {
		t.Error("expected error state after denying security")
	}
}

func TestAIMessageModel_Editing(t *testing.T) {
	ctx := context.Background()
	gs := &mockGitService{}
	m := NewAIMessageModel(ctx, nil, nil, gs, MessageConfig{
		EditorMode:       "builtin",
		SecurityKeywords: []string{"secret"},
	}, "")
	m.state = StateGenerated
	m.commitMessage = "old"

	// 1. Press 'e' to edit
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = *m2.(*AIMessageModel)
	if m.state != StateEditing {
		t.Error("expected StateEditing")
	}

	// 2. Type new message in textarea
	m.textArea.SetValue("new message")

	// 3. Save (Ctrl+S)
	m3, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = *m3.(*AIMessageModel)
	if m.state != StateGenerated || m.commitMessage != "new message" {
		t.Errorf("failed to save edit: state=%v, msg=%s", m.state, m.commitMessage)
	}
}

func TestFlow_Options(t *testing.T) {
	f := NewFlow(nil, nil, FlowConfig{
		EditorMode: "builtin",
	})
	f.WithHint("myhint").WithSkipHint(true)
	if f.hint != "myhint" || !f.skipHint {
		t.Error("failed to set flow options")
	}
}

func TestFlow_PrintPullRequestInfo(t *testing.T) {
	ctx := context.Background()
	gs := &mockGitService{prURL: "https://github.com/pr/1"}
	f := NewFlow(nil, gs, FlowConfig{
		EditorMode: "builtin",
	})

	// Capturing stdout is complex, but we can at least call it to ensure no panics
	f.printPullRequestInfo(ctx, "remote: https://github.com/pr/2")

	gs.prURL = ""
	f.printPullRequestInfo(ctx, "remote: https://github.com/pr/2")
}

func TestFilterCompatibleFiles(t *testing.T) {
	ctx := context.Background()
	gs := &mockGitService{
		resolveResponse: []string{"a.go"},
	}
	f := NewFlow(nil, gs, FlowConfig{
		EditorMode: "builtin",
	})

	available := []string{"a.go", "b.go"}
	
	// Exact match (tracked file)
	res := f.FilterCompatibleFiles(ctx, available, []string{"a.go"})
	if len(res) != 1 || res[0] != "a.go" {
		t.Errorf("expected [a.go], got %v", res)
	}

	// Non-existent
	gs.resolveResponse = nil
	res = f.FilterCompatibleFiles(ctx, available, []string{"nonexistent.go"})
	if len(res) != 0 {
		t.Errorf("expected empty, got %v", res)
	}
}
func TestUniqueStrings(t *testing.T) {
	input := []string{"a", "b", "a", "c", "b"}
	expected := []string{"a", "b", "c"}
	res := uniqueStrings(input)
	if len(res) != 3 {
		t.Fatalf("expected 3, got %d", len(res))
	}
	for i, v := range expected {
		if res[i] != v {
			t.Errorf("at %d: expected %s, got %s", i, v, res[i])
		}
	}
}
