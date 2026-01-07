package git

import "testing"

func TestExtractVersionFromDiff(t *testing.T) {
	tests := []struct {
		name     string
		diff     string
		expected string
	}{
		{
			name: "Simple VERSION file update",
			diff: `diff --git a/VERSION b/VERSION
--- a/VERSION
+++ b/VERSION
@@ -1 +1 @@
-0.4.0
+0.5.0`,
			expected: "0.4.0 -> 0.5.0",
		},
		{
			name: "Package.json update",
			diff: `diff --git a/package.json b/package.json
--- a/package.json
+++ b/package.json
@@ -3,1 +3,1 @@
-  "version": "1.2.3",
+  "version": "1.2.4",`,
			expected: "1.2.3 -> 1.2.4",
		},
		{
			name: "Initial version",
			diff: `diff --git a/VERSION b/VERSION
new file mode 100644
--- /dev/null
+++ b/VERSION
@@ -0,0 +1 @@
+1.0.0`,
			expected: "1.0.0",
		},
		{
			name: "Cargo.toml update (Rust)",
			diff: `diff --git a/Cargo.toml b/Cargo.toml
--- a/Cargo.toml
+++ b/Cargo.toml
@@ -1,3 +1,3 @@
 [package]
-version = "0.1.0"
+version = "0.2.0"`,
			expected: "0.1.0 -> 0.2.0",
		},
		{
			name:     "composer.json update (PHP)",
			diff:     "diff --git b/composer.json --- a/composer.json\n+++ b/composer.json\n@@ -2,1 +2,1 @@\n-    \"version\": \"1.0.0\",\n+    \"version\": \"1.1.0\",",
			expected: "1.0.0 -> 1.1.0",
		},
		{
			name: "pyproject.toml update (Python)",
			diff: `diff --git a/pyproject.toml b/pyproject.toml
--- a/pyproject.toml
+++ b/pyproject.toml
@@ -1,1 +1,1 @@
-version = "2.0.0"
+version = "2.1.0"`,
			expected: "2.0.0 -> 2.1.0",
		},
		{
			name: "VERSION file with leading zeros collision",
			diff: `diff --git a/VERSION b/VERSION
--- a/VERSION
+++ b/VERSION
@@ -1 +1 @@
-0.5.0
+0.6.0`,
			expected: "0.5.0 -> 0.6.0",
		},
		{
			name: "Bump version message collision in VERSION file",
			diff: `diff --git a/VERSION b/VERSION
--- a/VERSION
+++ b/VERSION
@@ -1 +1 @@
-chore(release): Bump version to 5.0
+chore(release): Bump version to 6.0`,
			expected: "5.0 -> 6.0",
		},
		{
			name: "Complex semver in VERSION file",
			diff: `diff --git a/VERSION b/VERSION
--- a/VERSION
+++ b/VERSION
@@ -1 +1 @@
-1.2.3-alpha.1
+1.2.3-beta.2`,
			expected: "1.2.3-alpha.1 -> 1.2.3-beta.2",
		},
		{
			name: "Ignore version change in test file",
			diff: `diff --git a/git_test.go b/git_test.go
--- a/git_test.go
+++ b/git_test.go
@@ -1 +1 @@
-func TestVersion() { v := "0.4.0" }
+func TestVersion() { v := "0.5.0" }`,
			expected: "",
		},
		{
			name:     "No version change",
			diff:     "diff --git a/main.go b/main.go\n+func main() {}",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractVersionFromDiff(tt.diff)
			if result != tt.expected {
				t.Errorf("%s: ExtractVersionFromDiff() = %q, want %q", tt.name, result, tt.expected)
			}
		})
	}
}
