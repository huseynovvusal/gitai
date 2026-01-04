# Output file
Output assets/usage.gif

# Settings
Set FontSize 24
Set Width 1600
Set Height 900
Set Padding 40
Set FontFamily "Monaco"
Set LineHeight 1.2
Set WindowBar Colorful

# Setup the environment (hidden)
Hide
Type "mkdir -p demo_env/pkg/auth" Enter
Type "cd demo_env" Enter
Type "git init" Enter
Type "git config user.name 'Demo User'" Enter
Type "git config user.email 'demo@gitai.local'" Enter
Type "echo 'package auth\n\nfunc Login(user string) bool {\n    return true\n}' > pkg/auth/login.go" Enter
Type "git add ." Enter
Type "git commit -m 'feat: initial auth implementation'" Enter

# Make a change (add a nil check)
Type "echo 'package auth\n\nfunc Login(user *string) bool {\n    if user == nil {\n        return false\n    }\n    return true\n}' > pkg/auth/login.go" Enter
Type "clear" Enter
Show

# Start the Demo
Type "git status" Sleep 500ms Enter
Sleep 1.5s

Type "../bin/gitai suggest" Sleep 500ms Enter
Sleep 2s

# Select the file (Space to toggle, Enter to confirm)
Type " " Sleep 500ms
Enter Sleep 500ms

# Hint input (Enter to skip)
Enter
Sleep 5s

# Result is visible. Commit it.
Type "c"
Sleep 2s

# Cleanup (hidden)
Hide
Type "cd .." Enter
Type "rm -rf demo_env" Enter