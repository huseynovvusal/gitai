# 🤖 **Gitai** — AI-powered Git Assistant

Gitai is an open-source CLI tool that helps developers generate **high-quality git commit messages** using AI. It inspects repository changes (diff + status) and provides concise, actionable suggestions via an interactive TUI.

Below is a quick animated demo of gitai running in a terminal:

![Gitai usage demo](./assets/usage.gif)

The project supports multiple AI backends (OpenAI, Google Gemini via genai, and local models via Ollama) and is intended to be used as a developer helper (interactive CLI, pre-commit hooks, CI helpers).

## ✨ Key features

- **AI-generated commit message suggestions** based on repo diffs
- _Interactive TUI_ to select files and review suggestions 🖱️
- **Edit & Regenerate**: Tweak suggestions in-place or regenerate them with a keystroke 🔄
- **Security scanning**: Automatically detects sensitive data (keys, passwords) in diffs before sending to AI 🔒
- **Smart ticket extraction**: Automatically parses Jira and GitHub issue URLs from hints to format commit headers 🎫
- **Conventional Commits**: Generates messages following the Conventional Commits specification by default 📝
- Pluggable AI backends: OpenAI, Google GenAI, Ollama (local)
- Small single-binary distribution (Go) ⚙️

## ⚡️ Quick start

### 🛠️ Prerequisites

- Go 1.20+ (Go modules are used; CONTRIBUTING recommends Go 1.24+ for development)
t - One of the supported AI providers (optional):
  - OpenAI API key (`OPENAI_API_KEY`)
  - Google Gemini API key (`GEMINI_API_KEY` or `GOOGLE_API_KEY`)
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
git clone https://github.com/yourusername/gitai.git
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

#### Targeted Mode
Skip the file selector by passing specific files or directories directly:

```sh
gitai suggest internal/main.go README.md
# or
gitai suggest internal/
```

#### Selecting an AI Provider
You can override the configured AI backend for a single run:

```sh
gitai suggest --provider=gpt      # OpenAI
gitai suggest --provider=ollama   # Local Ollama
gitai suggest --provider=gemini   # Google Gemini
```

## 🔧 Configuration

Configuration is managed with Viper and supports CLI flags, environment variables, and config files (e.g., `gitai.yaml`).

**Priorities:** Flags > Env Vars > Config Files > Defaults.

### Documentation
Detailed configuration guides are available in the `docs/wiki/` directory:
- [**Configuration Reference**](docs/wiki/Configuration.md) (All options & files)
- [**AI Providers**](docs/wiki/AI-Providers.md) (Setup for GPT, Gemini, Ollama)
- [**Customization**](docs/wiki/Customization.md) (Editors, styles)
- [**Security**](docs/wiki/Security.md) (Keyword scanner & privacy)
- [**Internals**](docs/wiki/Internals.md) (Architecture & How it works)

### Quick Example (`gitai.yaml`)
```yaml
ai:
  provider: gpt     # gpt | gemini | ollama | geminicli
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

### 🧪 Running unit tests

If tests are added, run them with:

```sh
go test ./...
```

### ➕ Adding a new AI backend

1. Add a new adapter under `internal/ai` that implements a function returning (string, error).
2. Wire it into `GenerateCommitMessage` or create a configuration switch.

### 🚀 Releasing

This project uses [GoReleaser](https://goreleaser.com/) and GitHub Actions for releases. To create a new release:

1. **Tag the commit**: Create a new semantic version tag.
   ```sh
   git tag -a v1.2.3 -m "Release v1.2.3"
   ```
2. **Push the tag**:
   ```sh
   git push origin v1.2.3
   ```
3. **Automated Release**: The GitHub Action will automatically:
   - Build binaries for multiple platforms.
   - Create a GitHub Release with a changelog and assets.
   - Update the Homebrew tap at `artback/homebrew-gitai`.
   - Publish Linux packages (DEB/RPM).

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

Contributions are welcome. Please follow the guidelines in [CONTRIBUTING.md](CONTRIBUTING.md).

Suggested contribution workflow:

1. Fork the repo and create a topic branch
2. Implement your feature or fix
3. Add/adjust tests where appropriate
4. Open a pull request describing the change and rationale

If you'd like help designing an enhancement (hooks, CI integrations, new backends), open an issue first to discuss.

## 🔒 Security & Privacy

- **Local Keyword Scanner**: Gitai includes a built-in security layer that scans your diffs for sensitive information (like `api_key`, `password`, `private_key`) locally. If a match is found, it will warn you and block the request to the AI provider.
- **Third-party AI**: The tool sends diffs and repository metadata to third-party AI providers when generating messages. Treat this like any other service that may upload code. Do not send secrets or sensitive data to remote AI providers.
- **Offline Workflow**: For maximum privacy, run local models via **Ollama**. Gitai supports local Ollama endpoints, ensuring your code never leaves your machine.

## 📜 License

This project is released under the MIT License. See [LICENSE](LICENSE) for details.

## 👤 Authors

Vusal Huseynov — original author

Jonathan Artback - contributor 
