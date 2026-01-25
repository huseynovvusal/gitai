# 🤖 **Gitai** — AI-powered Git Assistant

[![codecov](https://codecov.io/gh/artback/gitai/branch/main/graph/badge.svg)](https://codecov.io/gh/artback/gitai)

Gitai is an AI-powered CLI tool that helps you write better git commit messages, faster. It analyzes your changes (diffs) to generate concise, standardized commits that follow best practices.

It supports two main workflows:
- **Interactive Mode**: A terminal UI (TUI) to visually select files, add context hints, and review/edit suggestions.
- **Targeted Mode**: A quick CLI command to generate messages for specific files or directories instantly.

Below is a quick animated demo of gitai running in a terminal:

![Gitai usage demo](./assets/usage.gif)

The project supports multiple AI backends (OpenAI, Google Gemini via genai, and local models via Ollama) and is intended to be used as a developer helper (interactive CLI, pre-commit hooks, CI helpers).

## ✨ Key features

- **AI-generated commit message suggestions** based on repo diffs
- _Interactive TUI_ to select files and review suggestions 🖱️
- **Edit & Regenerate**: Tweak suggestions in-place or regenerate them with a keystroke 🔄
- **Security scanning**: Automatically detects sensitive data (keys, passwords) in diffs before sending to AI 🔒
- **Smart ticket extraction**: Automatically parses Jira and GitHub issue URLs from hints to format commit headers 🎫
- **Smart release detection**: Automatically identifies version bumps in files like `VERSION`, `package.json`, `go.mod`, `Cargo.toml`, etc., and formats them as `chore(release)` commits 🚀
- Conventional Commits: Generates messages following the Conventional Commits specification by default 📝
- Pluggable AI backends: OpenAI, Anthropic (Claude), Google Gemini, Groq, DeepSeek, XAI (Grok), Ollama (local)
- **Zero external dependencies**: Built in pure Go (no `git` binary required) ⚙️

## ⚡️ Quick start

### 🛠️ Prerequisites

- Go 1.20+ (Go modules are used; CONTRIBUTING recommends Go 1.24+ for development)
- One of the supported AI providers (optional):
  - OpenAI API key (`OPENAI_API_KEY`)
  - Anthropic API key (`ANTHROPIC_API_KEY`)
  - Google Gemini API key (`GEMINI_API_KEY` or `GOOGLE_API_KEY`)
  - Groq API key (`GROQ_API_KEY`) (uses OpenAI compatible client)
  - DeepSeek API key (`DEEPSEEK_API_KEY`) (uses OpenAI compatible client)
  - XAI (Grok) API key (`XAI_API_KEY`) (uses OpenAI compatible client)
  - Ollama binary available and `OLLAMA_API_PATH` set (for local models)
  - Gemini CLI installed (for `geminicli` provider)

### 📦 Build and install

#### Homebrew (macOS & Linux)

You can install `gitai` using Homebrew:

```sh
brew tap artback/gitai
brew install gitai
```

#### Manual Build

1. Clone the repository and build:

```sh
git clone https://github.com/artback/gitai.git
cd gitai
make build
```

1. Install (**recommended**)

```sh
make install
```

The `make install` target builds the `gitai` binary and moves it to `/usr/local/bin/` (may prompt for sudo). Alternatively copy `./bin/gitai` to a directory in your PATH.

### ▶️ Usage

#### Interactive Mode (Default)
Run the command without arguments to start the interactive TUI flow:

```sh
gitai suggest
```

1.  **Select files**: Choose changed files from the list.
2.  **Add hints**: Optionally provide context (e.g., ticket URL or "fixes login bug").
3.  **Review**: The AI generates a message. You can **Edit** (`e`), **Regenerate** (`r`), or **Commit** (`c`).

#### Atomic Commits (Experimental)
Automatically split changes in selected files into multiple atomic commits based on context.

```sh
gitai suggest --atomic
```

1.  **Review Plan**: Gitai proposes a sequence of atomic commits.
2.  **Reorder**: Use `J` (Shift+Down) / `K` (Shift+Up) to reorder commits if needed.
3.  **Edit**: Press `e` to edit any commit message.
4.  **Confirm**: Press `c` to apply and commit them sequentially.

#### Targeted Mode
Skip the file selector by passing specific files or directories directly:

```sh
gitai suggest internal/main.go README.md
# or
gitai suggest internal/
```

#### Speeding up with Hints
You can skip the interactive hint prompt by providing it via CLI or disabling it:

```sh
gitai suggest -H "Fix login race condition"  # Use hint and skip prompt
gitai suggest --no-hint                      # Skip prompt without hint
```

#### Selecting an AI Provider
You can override the configured AI backend for a single run:

```sh
gitai suggest --provider=gpt      # OpenAI
gitai suggest --provider=anthropic # Anthropic (Claude)
gitai suggest --provider=groq     # Groq
gitai suggest --provider=deepseek # DeepSeek
gitai suggest --provider=xai      # XAI (Grok)
gitai suggest --provider=ollama   # Local Ollama
gitai suggest --provider=gemini   # Google Gemini
```

## 🔧 Configuration

Configuration is managed with Viper and supports CLI flags, environment variables, and config files (e.g., `gitai.yaml`).

**Priorities:** Flags > Env Vars > Config Files > Defaults.

### Documentation
Detailed configuration guides are available in the [**Project Wiki**](https://github.com/artback/gitai/wiki):
- [**Configuration Reference**](https://github.com/artback/gitai/wiki/Configuration) (All options & files)
- [**AI Providers**](https://github.com/artback/gitai/wiki/AI-Providers) (Setup for GPT, Claude, Groq, DeepSeek, XAI, Gemini, Ollama)
- [**Customization**](https://github.com/artback/gitai/wiki/Customization) (Editors, styles)
- [**Security**](https://github.com/artback/gitai/wiki/Security) (Keyword scanner & privacy)
- [**Internals**](https://github.com/artback/gitai/wiki/Internals) (Architecture & How it works)

### Quick Example (`gitai.yaml`)
```yaml
ai:
  provider: gpt     # gpt | anthropic | groq | deepseek | xai | gemini | ollama
  model: ""         # Optional: specific model (e.g. "claude-3-opus", "llama3-70b")
  temperature: 0.7

suggest:
  editor: builtin   # builtin | system | "code -w"
```

### Common Environment Variables
- `GITAI_AI_PROVIDER`: Override provider (e.g. `ollama`)
- `GITAI_AI_API_KEY`: API key for cloud providers


## 🧑‍💻 Development

To run locally while developing:

1. Ensure Go is installed and `GOPATH`/`GOMOD` are configured (this repo uses Go modules).
2. Run the CLI directly from source:

```sh
go run ./main.go suggest
```

### 🧪 Testing & Linting

Run the unit tests using `make` or the standard Go toolchain. Tests are designed to run without external dependencies (using mocks for AI providers).

```sh
# Run all tests
make test

# Or using the Go toolchain directly
go test ./...
```

If you have `golangci-lint` installed, you can also run the linter:

```sh
make lint
```

### ➕ Adding a new AI backend

1. Add a new adapter under `internal/ai` that implements a function returning (string, error).
2. Wire it into `GenerateCommitMessage` or create a configuration switch.

### 🚀 Releasing

This project uses [GoReleaser](https://goreleaser.com/) and GitHub Actions for releases. To create a new release:

1. **Update the VERSION file**: Change the version number in the `VERSION` file (e.g., `1.2.3`).
2. **Push to main**: When the change is merged or pushed to the `main` branch, a GitHub Action automatically:
   - Creates a new git tag (e.g., `v1.2.3`).
   - Builds binaries for multiple platforms via GoReleaser.
   - Creates a GitHub Release with a changelog and assets.
   - Updates the Homebrew tap at `artback/homebrew-gitai`.
   - Publishes Linux packages (DEB/RPM).

> **Note**: Ensure `TAP_GITHUB_TOKEN` is configured in the repository secrets for the Homebrew tap update to work.

## Star History

<a href="https://www.star-history.com/#huseynovvusal/gitai&Date">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=huseynovvusal/gitai&type=Date&theme=dark" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=huseynovvusal/gitai&type=Date" />
   <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=huseynovvusal/gitai&type=Date" />
 </picture>
</a>

## 🤝 Contributing

Contributions are welcome! Please see our [Contributing Guide](https://github.com/artback/gitai/wiki/Contributing) for details.

## 🔒 Security & Privacy

- **Local Keyword Scanner**: Gitai includes a built-in security layer that scans your diffs for sensitive information (like `api_key`, `password`, `private_key`) locally. If a match is found, it will warn you and block the request to the AI provider.
- **Third-party AI**: The tool sends diffs and repository metadata to third-party AI providers when generating messages. Treat this like any other service that may upload code. Do not send secrets or sensitive data to remote AI providers.
- **Offline Workflow**: For maximum privacy, run local models via **Ollama**. Gitai supports local Ollama endpoints, ensuring your code never leaves your machine.

## 📜 License

This project is released under the MIT License. See [LICENSE](LICENSE) for details.

## 👤 Authors

Vusal Huseynov — original author

Jonathan Artback - contributor 
