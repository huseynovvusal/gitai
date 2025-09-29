# 🤖 **Gitai** — AI-powered Git Assistant

Gitai is an open-source CLI tool that helps developers generate **high-quality git commit messages** using AI. It inspects repository changes (diff + status) and provides concise, actionable suggestions via an interactive TUI.

Below is a quick animated demo of gitai running in a terminal:

![Gitai usage demo](./assets/usage.gif)

The project supports multiple AI backends (OpenAI, Google Gemini via genai, and local models via Ollama) and is intended to be used as a developer helper (interactive CLI, pre-commit hooks, CI helpers).

## ✨ Key features

- **AI-generated commit message suggestions** based on repo diffs
- _Interactive TUI_ to select files and review suggestions 🖱️
- Pluggable AI backends: OpenAI, Google GenAI, Ollama (local)
- Small single-binary distribution (Go) ⚙️

## ⚡️ Quick start

### 🛠️ Prerequisites

- Go 1.20+ (Go modules are used; CONTRIBUTING recommends Go 1.24+ for development)
- One of the supported AI providers (optional):
  - OpenAI API key (OPENAI_API_KEY)
  - Google API key for genai (GOOGLE_API_KEY)
  - Ollama binary available and OLLAMA_API_PATH set (for local models)

### 📦 Build and install

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

### ▶️ Run (example)

Generate commit message suggestions using the _interactive TUI_:

```sh
gitai suggest
```

`gitai suggest` will:

- list changed files (using `git status --porcelain`)
- allow selecting files via an interactive file selector
- fetch diffs for selected files and call the configured AI backend to produce suggestions

See `internal/tui/suggest` for the implementation of the flow.

## 🔧 Configuration

**API keys and settings are provided via environment variables:**

- `OPENAI_API_KEY` — API key for OpenAI (for GPT-3.5/4 series)
- `GOOGLE_API_KEY` — API key used by Google GenAI client
- `OLLAMA_API_PATH` — path to the Ollama binary for local model calls (e.g. `/usr/local/bin/ollama`)

_Set these in your shell or CI environment._ Example:

```sh
export OPENAI_API_KEY="sk-..."
export GOOGLE_API_KEY="..."
export OLLAMA_API_PATH="/usr/local/bin/ollama"
```

## ⚙️ Behaviour and defaults

- The code includes adapters for multiple backends. The current default selection is implemented in **`internal/ai/ai.go`**. Edit that file to change preference/selection order if you need a different default.

## 🧩 How it works (internals)

Core components live under `internal/`:

- `internal/ai` — adapters for AI backends and the main prompt (`GenerateCommitMessage`)
- `internal/git` — helpers that run git commands and parse diffs/status (helpers used by the TUI)
- `internal/tui/suggest` — TUI flow (file selector → AI message view)

The entrypoint is `main.go` which dispatches to the Cobra-based CLI under `cmd/`.

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

## 🤝 Contributing

Contributions are welcome. Please follow the guidelines in [CONTRIBUTING.md](CONTRIBUTING.md).

Suggested contribution workflow:

1. Fork the repo and create a topic branch
2. Implement your feature or fix
3. Add/adjust tests where appropriate
4. Open a pull request describing the change and rationale

If you'd like help designing an enhancement (hooks, CI integrations, new backends), open an issue first to discuss.

## 🔒 Security & Privacy

- The tool may send diffs and repository content to third-party AI providers when generating messages — treat this like any other service that may upload code. Do not send secrets or sensitive data to remote AI providers.
- If you need an offline-only workflow, prefer running local models via Ollama and keep `OLLAMA_API_PATH` configured.

## 📜 License

This project is released under the MIT License. See [LICENSE](LICENSE) for details.

## 👤 Authors

Vusal Huseynov — original author
