package multilist

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModel(t *testing.T) {
	data := []string{"file1.go", "file2.go", "README.md"}
	m := New(data, "Select Files")

	// 1. Check Initial State
	if len(m.List.Items()) != 3 {
		t.Errorf("expected 3 items, got %d", len(m.List.Items()))
	}

	if len(m.GetSelected()) != 0 {
		t.Errorf("expected 0 selected items, got %d", len(m.GetSelected()))
	}

	// 2. Toggle First Item (file1.go)
	// Bubble Tea testing: send a space key message
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})

	selected := m.GetSelected()
	if len(selected) != 1 || selected[0] != "file1.go" {
		t.Errorf("expected [file1.go], got %v", selected)
	}

	// 3. Toggle it again (unselect)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if len(m.GetSelected()) != 0 {
		t.Errorf("expected 0 selected after untoggle, got %d", len(m.GetSelected()))
	}

	// 4. Select All (press 'a')
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if len(m.GetSelected()) != 3 {
		t.Errorf("expected all 3 selected, got %d", len(m.GetSelected()))
	}

	// 5. Unselect All (press 'a' again)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if len(m.GetSelected()) != 0 {
		t.Errorf("expected 0 selected after toggle all, got %d", len(m.GetSelected()))
	}
}

func TestModel_Filtering(t *testing.T) {
	data := []string{"abc", "def", "ghi"}
	m := New(data, "Test")

	// Enter filtering mode
	m.List.SetFilterState(1) // list.Filtering

	// Sending 'a' should NOT toggle all when filtering
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if len(m.GetSelected()) != 0 {
		t.Error("Select all should be disabled during filtering")
	}
}
