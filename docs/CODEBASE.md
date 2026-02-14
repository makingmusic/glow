# Glow Codebase Guide

## Overview

Glow is a terminal-based markdown reader built in Go. It renders markdown with styled output directly in the CLI, supporting both an interactive TUI (Terminal User Interface) for browsing files and a direct CLI mode for rendering single documents. When CLI output exceeds the terminal height, Glow automatically opens the TUI pager for comfortable reading. It is part of the [Charm](https://charm.sh) ecosystem.

**Repository:** `github.com/charmbracelet/glow/v2`
**Language:** Go 1.24+
**License:** MIT

## Project Structure

```
glow/
├── main.go                  # CLI entry point, command routing, source parsing
├── config_cmd.go            # `glow config` subcommand
├── style.go                 # Lipgloss styling helpers (keyword, paragraph)
├── url.go                   # URL/shorthand parsing (GitHub/GitLab)
├── github.go                # GitHub README API integration
├── gitlab.go                # GitLab README API integration
├── log.go                   # File-based logging setup
├── man_cmd.go               # Man page generation subcommand
├── console_windows.go       # Windows ANSI terminal support
├── glow_test.go             # CLI flag tests
├── url_test.go              # URL parsing tests
│
├── ui/                      # TUI implementation (Bubble Tea)
│   ├── ui.go                # Main TUI model, Init/Update/View, top-level orchestration
│   ├── config.go            # TUI Config struct with env var tags
│   ├── stash.go             # File listing/browsing view (stashModel)
│   ├── pager.go             # Document viewing pager (pagerModel)
│   ├── markdown.go          # Markdown document data type
│   ├── stashhelp.go         # Help text rendering for stash view
│   ├── stashitem.go         # Individual file item rendering
│   ├── styles.go            # Color palette and style definitions
│   ├── keys.go              # Key constants (keyEnter, keyEsc)
│   ├── sort.go              # Markdown sorting (by modification time)
│   ├── editor.go            # External editor invocation ($EDITOR)
│   ├── ignore_darwin.go     # macOS-specific gitignore patterns
│   └── ignore_general.go    # Cross-platform gitignore patterns
│
├── utils/
│   └── utils.go             # Shared utilities (frontmatter, paths, glamour)
│
├── .github/
│   └── workflows/           # CI/CD pipelines
│       ├── build.yml        # Build + vulnerability scanning
│       ├── lint.yml          # golangci-lint
│       ├── coverage.yml     # Test coverage reporting
│       └── goreleaser.yml   # Release automation
│
├── go.mod / go.sum          # Go module dependencies
├── Taskfile.yaml            # Task runner (lint, test, log)
├── Dockerfile               # Distroless container build
├── .golangci.yml            # Linter configuration (26 linters)
├── .goreleaser.yml          # Multi-platform release config
└── .editorconfig            # Editor formatting rules
```

## Key Source Files

### Root Package (`main`)

| File | Lines | Purpose |
|------|-------|---------|
| `main.go` | 468 | Entry point. Cobra CLI setup, source parsing, config loading, execution routing between TUI and CLI modes. |
| `url.go` | 86 | Parses GitHub/GitLab shorthand URLs (e.g., `charmbracelet/glow`) into API calls. |
| `github.go` | 57 | Fetches README via GitHub REST API (`/repos/{owner}/{repo}/readme`). |
| `gitlab.go` | 61 | Fetches README via GitLab REST API (`/api/v4/projects/{id}`). |
| `config_cmd.go` | 88 | `glow config` command that opens the config file in `$EDITOR`. |
| `log.go` | 40 | Sets up file-based logging to the OS-specific data directory. |
| `style.go` | 14 | Lipgloss style helpers for CLI output formatting. |
| `man_cmd.go` | 29 | Generates Unix man pages from Cobra command tree. |

### UI Package

| File | Lines | Purpose |
|------|-------|---------|
| `ui.go` | 452 | Top-level Bubble Tea model. Coordinates stash and pager sub-models, handles global keys and file search lifecycle. |
| `stash.go` | 891 | File listing view. Handles browsing, filtering, pagination, and document opening. Largest UI component. |
| `pager.go` | 535 | Document viewer. Viewport scrolling, glamour rendering, file watching, clipboard, editor integration. |
| `markdown.go` | 88 | `markdown` struct: path, body, note, modtime, filter value. Includes normalization for fuzzy matching. |
| `stashhelp.go` | 297 | Help overlay for the stash view. Renders two-column key binding reference. |
| `stashitem.go` | 120 | Renders a single markdown file entry in the stash list (title, path, date). |
| `styles.go` | 46 | Adaptive color palette (16 colors) with light/dark terminal support. |
| `editor.go` | 19 | Opens a file in the user's `$EDITOR` with line number positioning. |

### Utils Package

| File | Lines | Purpose |
|------|-------|---------|
| `utils.go` | 113 | `RemoveFrontmatter`, `ExpandPath`, `WrapCodeBlock`, `IsMarkdownFile`, `GlamourStyle`. |

## Core Types

### `source` (main.go:67)

Represents a readable markdown source. Created from stdin, files, URLs, or API responses.

```go
type source struct {
    reader io.ReadCloser
    URL    string
}
```

### `model` (ui/ui.go:100)

Top-level Bubble Tea model that owns the application state.

```go
type model struct {
    common          *commonModel              // Shared config, dimensions
    state           state                     // stateShowStash | stateShowDocument
    fatalErr        error                     // Fatal error to display
    stash           stashModel                // File listing sub-model
    pager           pagerModel                // Document viewer sub-model
    localFileFinder chan gitcha.SearchResult   // Async file search channel
}
```

### `stashModel` (ui/stash.go:139)

File listing and browsing interface.

```go
type stashModel struct {
    common             *commonModel
    spinner            spinner.Model
    filterInput        textinput.Model
    viewState          stashViewState    // ready | loadingDocument | showingError
    filterState        filterState       // unfiltered | filtering | filterApplied
    sections           []section         // documentsSection, filterSection
    markdowns          []*markdown       // Master document list
    filteredMarkdowns  []*markdown       // Filtered subset for display
    loaded             bool              // Whether file search is complete
}
```

### `pagerModel` (ui/pager.go)

Document rendering, viewing, search, and navigation.

```go
type pagerModel struct {
    common          *commonModel
    viewport        viewport.Model      // Scrollable content viewport
    state           pagerState          // browse | statusMessage | search | jumpToLine
    showHelp        bool
    currentDocument markdown            // Document being displayed
    searchInput     textinput.Model     // Text input for search query
    searchQuery     string              // Active search term (persists after confirm)
    searchMatches   []int               // Matching line numbers (0-indexed)
    searchIndex     int                 // Current match index (-1 = none)
    lineInput       textinput.Model     // Text input for line/% jump
    watcher         *fsnotify.Watcher   // File change detection
}
```

### `markdown` (ui/markdown.go)

Internal representation of a markdown document.

```go
type markdown struct {
    localPath   string      // Absolute file path
    filterValue string      // Normalized string for fuzzy matching
    Body        string      // Raw file content
    Note        string      // Display name (relative path)
    Modtime     time.Time   // Last modification time
}
```

### `Config` (ui/config.go)

TUI configuration populated from environment variables and CLI flags.

```go
type Config struct {
    ShowAllFiles         bool   `env:"GLOW_SHOW_ALL_FILES"`
    ShowLineNumbers      bool   `env:"GLOW_SHOW_LINE_NUMBERS"`
    GlamourMaxWidth      uint
    GlamourStyle         string `env:"GLOW_STYLE"`
    EnableMouse          bool
    PreserveNewLines     bool
    Path                 string
    HighPerformancePager bool   `env:"GLOW_HIGH_PERFORMANCE_PAGER" envDefault:"true"`
    GlamourEnabled       bool   `env:"GLOW_ENABLE_GLAMOUR" envDefault:"true"`
}
```

## State Machine

The application has two levels of state:

**Top Level** (`model.state`):
- `stateShowStash` — File listing view is active
- `stateShowDocument` — Pager view is active

**Stash View** (`stashModel.viewState`):
- `stashStateReady` — Browsing files normally
- `stashStateLoadingDocument` — Loading a document for viewing
- `stashStateShowingError` — Displaying an error

**Filter** (`stashModel.filterState`):
- `unfiltered` — No filter active
- `filtering` — User is typing a filter query
- `filterApplied` — Filter results are shown, input is blurred

**Pager** (`pagerModel.state`):
- `pagerStateBrowse` — Normal document viewing
- `pagerStateStatusMessage` — Showing a temporary status message
- `pagerStateSearch` — User is typing a search query (`/`)
- `pagerStateJumpToLine` — User is typing a line number or percentage (`:`)

The pager also has a `searchQuery` field that persists after confirming a search. When set, `esc` in browse mode clears the search results before unloading the document. The `inInputMode()` method returns true when the pager is in search/jump state or has active search results.

## Message Types

Bubble Tea uses typed messages for communication between components:

| Message | Source | Purpose |
|---------|--------|---------|
| `errMsg` | Various | Error propagation |
| `initLocalFileSearchMsg` | `findLocalFiles` | File search channel ready |
| `foundLocalFileMsg` | `findNextLocalFile` | Individual file discovered |
| `localFileSearchFinished` | `findNextLocalFile` | Search complete |
| `filteredMarkdownMsg` | `filterMarkdowns` | Fuzzy filter results |
| `fetchedMarkdownMsg` | `loadLocalMarkdown` | Document content loaded |
| `contentRenderedMsg` | `renderWithGlamour` | Glamour rendering complete |
| `reloadMsg` | `watchFile` | File changed on disk |
| `editorFinishedMsg` | `openEditor` | External editor closed |
| `statusMessageTimeoutMsg` | Timer | Clear ephemeral status |

## Key Bindings

### Stash View (File Listing)

| Key | Action |
|-----|--------|
| `j/k/↑/↓` | Navigate up/down |
| `enter` | Open selected file |
| `e` | Edit file in `$EDITOR` |
| `/` | Start filtering |
| `esc` | Clear filter / go back |
| `?` | Toggle help |
| `tab/shift+tab` | Switch sections |
| `g/home` | Go to top |
| `G/end` | Go to bottom |
| `r` | Reload file list |
| `q` | Quit |

### Pager View (Document)

| Key | Action |
|-----|--------|
| `j/k/↑/↓` | Scroll up/down |
| `←/→` | Page back/forward |
| `f/pgdn` | Page down |
| `b/pgup` | Page up |
| `d/u` | Half page down/up |
| `g/home` | Go to top |
| `G/end` | Go to bottom |
| `/` | Search in document |
| `n` | Next search match |
| `N` | Previous search match |
| `:` | Jump to line (`:42`) or percentage (`:50%`) |
| `c` | Copy to clipboard |
| `e` | Edit in `$EDITOR` |
| `r` | Reload document |
| `?` | Toggle help |
| `esc` | Clear search results / back to file listing |
| `h` | Back to file listing |
| `q` | Quit |

## Dependencies

### Core Framework

| Package | Purpose |
|---------|---------|
| `charmbracelet/bubbletea` | TUI framework (Elm architecture) |
| `charmbracelet/bubbles` | Reusable UI components (viewport, spinner, textinput, paginator) |
| `charmbracelet/glamour` | Markdown → styled terminal rendering |
| `charmbracelet/lipgloss` | Terminal styling (colors, borders, padding) |
| `charmbracelet/log` | Structured logging |
| `spf13/cobra` | CLI command framework |
| `spf13/viper` | Configuration management (YAML, env, flags) |

### File Discovery & Watching

| Package | Purpose |
|---------|---------|
| `muesli/gitcha` | Git-aware file searching (respects `.gitignore`) |
| `fsnotify/fsnotify` | Cross-platform file system event watching |
| `sabhiram/go-gitignore` | `.gitignore` pattern matching (indirect) |

### Text & Terminal

| Package | Purpose |
|---------|---------|
| `sahilm/fuzzy` | Fuzzy string matching for filtering |
| `muesli/reflow` | ANSI-aware text truncation and wrapping |
| `muesli/termenv` | Terminal capability detection, OSC 52 clipboard |
| `mattn/go-runewidth` | Unicode character width calculation |
| `atotto/clipboard` | System clipboard access |

### Rendering

| Package | Purpose |
|---------|---------|
| `yuin/goldmark` | Markdown parser (indirect, used by glamour) |
| `alecthomas/chroma` | Syntax highlighting (indirect, used by glamour) |
| `microcosm-cc/bluemonday` | HTML sanitization (indirect) |

## Configuration

### Config File

Located at platform-specific paths (searched in order):
1. `$GLOW_CONFIG_HOME/glow.yml`
2. `$XDG_CONFIG_HOME/glow/glow.yml`
3. OS default: `~/Library/Application Support/glow/glow.yml` (macOS), `~/.config/glow/glow.yml` (Linux)

```yaml
style: "auto"           # auto | dark | light | dracula | tokyo-night | notty | path/to/custom.json
width: 80               # Word wrap width (0 = terminal width, max 120)
pager: false             # Pipe output through $PAGER
mouse: false             # Enable mouse wheel in TUI
all: false               # Show hidden/gitignored files
showLineNumbers: false   # Line numbers in pager
preserveNewLines: false  # Preserve newlines in output
```

### Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `GLOW_STYLE` | `"auto"` | Glamour rendering style |
| `GLOW_HIGH_PERFORMANCE_PAGER` | `true` | Optimized viewport rendering |
| `GLOW_ENABLE_GLAMOUR` | `true` | Enable/disable glamour rendering |
| `GLOW_CONFIG_HOME` | — | Override config directory |

### CLI Flags

```
glow [SOURCE|DIR]

Flags:
  -a, --all                 Show system files and directories (TUI)
  -l, --line-numbers        Show line numbers (TUI)
  -n, --preserve-new-lines  Preserve newlines in output
  -p, --pager               Display with pager ($PAGER or less -r)
  -t, --tui                 Display with TUI
  -s, --style string        Style name or JSON path (default "auto")
  -w, --width uint          Word-wrap width (default: terminal width)
      --config string       Config file path

Subcommands:
  config                    Edit the config file
  man                       Generate man pages
  completion                Generate shell completions
```

## Build & Test

### Task Runner (Taskfile.yaml)

```bash
task lint       # Run golangci-lint
task test       # Run go test ./...
task log:tail   # Tail glow log file
```

### CI Pipelines

- **build.yml**: `go build`, GoReleaser snapshot, govulncheck, semgrep
- **lint.yml**: golangci-lint with 26 linters + gofumpt/goimports
- **coverage.yml**: `go test -race -covermode atomic`, Goveralls reporting
- **goreleaser.yml**: Multi-platform releases (macOS, Linux, Windows, FreeBSD, OpenBSD)

### Running Locally

```bash
go build -o glow .
./glow README.md           # Render a file
./glow .                   # Browse current directory
echo "# Hello" | ./glow -  # Render from stdin
./glow charmbracelet/glow  # Fetch GitHub README
```

## Platform Support

| Platform | Notes |
|----------|-------|
| macOS | Full support. Excludes `~/Library` from file search. |
| Linux | Full support. |
| Windows | ANSI support via `ENABLE_VIRTUAL_TERMINAL_PROCESSING`. |
| FreeBSD / OpenBSD | Binary builds available via GoReleaser. |
| Android (Termux) | ARM binary support. |
