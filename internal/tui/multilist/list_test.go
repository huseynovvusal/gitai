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

func TestModel_NavigationAndToggle(t *testing.T) {
	data := []string{"item1", "item2", "item3"}
	m := New(data, "Nav Test")

	// 1. Move down to "item2"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	// 2. Toggle "item2"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})

	selected := m.GetSelected()
	if len(selected) != 1 || selected[0] != "item2" {
		t.Errorf("expected [item2], got %v", selected)
	}

	// 3. Move down to "item3"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	// 4. Toggle "item3"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})

	selected = m.GetSelected()
	if len(selected) != 2 {
		t.Errorf("expected 2 items selected, got %d", len(selected))
	}
}

func TestModel_Options(t *testing.T) {
	data := []string{"A"}
	m := New(data, "Options Test", WithHeight(50), WithWidth(80), WithStatusBar(true))

	if m.List.Height() != 50 {
		t.Errorf("expected height 50, got %d", m.List.Height())
	}

	if m.List.Width() != 80 {
		t.Errorf("expected width 80, got %d", m.List.Width())
	}

	// Note: checking ShowStatusBar directly might require inspecting the model structure
	// or assuming the option func logic is correct if height/width worked.
}

func TestRegexFilter(t *testing.T) {
	targets := []string{"main.go", "internal/ai/ai.go", "internal/git/git.go", "README.md"}

	tests := []struct {
		name     string
		term     string
		expected []int // Indices in targets
	}{
		{
			name:     "Exact match",
			term:     "main.go",
			expected: []int{0},
		},
		{
			name: "Simplified Wildcard",
			term: "*.go", // Should match .*.go -> anything ending in .go (if suffix match works)
			// Actually * at start means .* at start. ".*.go" matches "file.go".
			expected: []int{0, 1, 2},
		},
		{
			name:     "Suffix match",
			term:     "\\.go$",
			expected: []int{0, 1, 2},
		},
		{
			name:     "Directory match",
			term:     "internal/",
			expected: []int{1, 2},
		},
		{
			name:     "Case sensitive (default re)",
			term:     "readme",
			expected: nil,
		},
		{
			name:     "Invalid regex",
			term:     "[",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranks := regexFilter(tt.term, targets)
			if len(ranks) != len(tt.expected) {
				t.Fatalf("expected %d matches, got %d", len(tt.expected), len(ranks))
			}

			for i, r := range ranks {
				if r.Index != tt.expected[i] {
					t.Errorf("expected match at index %d, got %d", tt.expected[i], r.Index)
				}
			}
		})
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
