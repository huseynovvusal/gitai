package git

import (
	"reflect"
	"testing"
)

func TestCleanPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`simple.txt`, `simple.txt`},
		{`"quoted.txt"`, `quoted.txt`},
		{`"space file.txt"`, `space file.txt`},
		{`"weird -> name.txt"`, `weird -> name.txt`},
		{`unbalanced"`, `unbalanced"`},
		{`"unbalanced`, `"unbalanced`},
	}

	for _, tc := range tests {
		got := cleanPath(tc.input)
		if got != tc.expected {
			t.Errorf("cleanPath(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestExpandFiles(t *testing.T) {
	tests := []struct {
		input    []string
		expected []string
	}{
		{
			[]string{"file1.txt", "file2.txt"},
			[]string{"file1.txt", "file2.txt"},
		},
		{
			[]string{"old.txt -> new.txt"},
			[]string{"old.txt", "new.txt"},
		},
		{
			[]string{`"old name.txt" -> "new name.txt"`},
			[]string{"old name.txt", "new name.txt"},
		},
		{
			[]string{"mixed.txt", "old -> new"},
			[]string{"mixed.txt", "old", "new"},
		},
	}

	for _, tc := range tests {
		got := expandFiles(tc.input)
		if !reflect.DeepEqual(got, tc.expected) {
			t.Errorf("expandFiles(%v) = %v; want %v", tc.input, got, tc.expected)
		}
	}
}
