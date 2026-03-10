package review

import (
	"testing"

	"huseynovvusal/gitai/internal/ai"
)

func TestFormatPlain_NoFindings(t *testing.T) {
	result := &ai.ReviewResult{Findings: []ai.Finding{}}
	out := FormatPlain(result)
	expected := "No issues found. Your changes look good!\n"
	if out != expected {
		t.Errorf("FormatPlain() = %q, want %q", out, expected)
	}
}

func TestFormatPlain_WithFindings(t *testing.T) {
	result := &ai.ReviewResult{
		Findings: []ai.Finding{
			{Severity: "critical", File: "auth.go", Line: "42", Description: "SQL injection", Suggestion: "Use params"},
			{Severity: "warning", File: "auth.go", Line: "50", Description: "Unused error", Suggestion: "Handle it"},
			{Severity: "info", File: "main.go", Line: "10", Description: "Style", Suggestion: "Rename"},
		},
	}

	out := FormatPlain(result)

	// Check it contains key elements
	if !contains(out, "auth.go") {
		t.Error("FormatPlain() missing file name 'auth.go'")
	}
	if !contains(out, "main.go") {
		t.Error("FormatPlain() missing file name 'main.go'")
	}
	if !contains(out, "SQL injection") {
		t.Error("FormatPlain() missing finding description")
	}
	if !contains(out, "1 critical") {
		t.Error("FormatPlain() missing critical count")
	}
	if !contains(out, "1 warning") {
		t.Error("FormatPlain() missing warning count")
	}
	if !contains(out, "1 info") {
		t.Error("FormatPlain() missing info count")
	}
	if !contains(out, "2 file(s)") {
		t.Error("FormatPlain() missing file count")
	}
}

func TestFormatJSON(t *testing.T) {
	result := &ai.ReviewResult{
		Findings: []ai.Finding{
			{Severity: "warning", File: "a.go", Line: "1", Description: "test", Suggestion: "fix"},
		},
	}

	out, err := FormatJSON(result)
	if err != nil {
		t.Fatalf("FormatJSON() error = %v", err)
	}

	if !contains(out, `"severity": "warning"`) {
		t.Errorf("FormatJSON() missing severity, got: %s", out)
	}
	if !contains(out, `"file": "a.go"`) {
		t.Errorf("FormatJSON() missing file, got: %s", out)
	}
}

func TestGroupByFile(t *testing.T) {
	findings := []ai.Finding{
		{File: "a.go", Severity: "warning"},
		{File: "b.go", Severity: "info"},
		{File: "a.go", Severity: "critical"},
	}

	groups := groupByFile(findings)

	if len(groups) != 2 {
		t.Fatalf("groupByFile() groups = %d, want 2", len(groups))
	}

	if groups[0].file != "a.go" || len(groups[0].findings) != 2 {
		t.Errorf("groupByFile() first group = %v, want a.go with 2 findings", groups[0])
	}

	if groups[1].file != "b.go" || len(groups[1].findings) != 1 {
		t.Errorf("groupByFile() second group = %v, want b.go with 1 finding", groups[1])
	}
}

func TestSeverityIcon(t *testing.T) {
	tests := []struct {
		severity string
		want     string
	}{
		{"critical", "X"},
		{"warning", "!"},
		{"info", "*"},
		{"unknown", "?"},
	}

	for _, tt := range tests {
		got := severityIcon(tt.severity)
		if got != tt.want {
			t.Errorf("severityIcon(%q) = %q, want %q", tt.severity, got, tt.want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
