# Glow Architecture

## High-Level Design

Glow operates in two primary modes with a shared rendering pipeline:

```
┌──────────────────────────────────────────────────────┐
│                    CLI Entry (main.go)                │
│                                                      │
│  stdin ─┐                                            │
│  file ──┤  sourceFromArg()  ┌──────────────────────┐ │
│  URL ───┤ ──────────────────│   executeCLI()       │ │
│  GitHub ┤                   │   Glamour render     │ │
│  GitLab ┘                   │   → stdout / $PAGER  │ │
│                             └──────────────────────┘ │
│  dir ───┐                   ┌──────────────────────┐ │
│  (none) ┘ ──────────────────│   runTUI()           │ │
│                             │   Bubble Tea program │ │
│                             └──────────────────────┘ │
└──────────────────────────────────────────────────────┘
```

**CLI Mode**: Direct rendering of a single source to stdout. The source is resolved (stdin, file, HTTP URL, or GitHub/GitLab API), rendered through Glamour, and output directly or piped through a pager.

**TUI Mode**: Interactive file browser and document viewer. Launched when no file argument is given or when a directory is passed. Uses the Bubble Tea framework for a full-screen terminal UI.

## Execution Flow

### Startup

```
main()
├── setupLog()                      # File-based logging
├── init()                          # Cobra flag setup, Viper config loading
│   └── tryLoadConfigFromDefaultPlaces()
│       ├── Scan config dirs (GLOW_CONFIG_HOME, XDG, OS default)
│       ├── viper.ReadInConfig()
│       └── ensureConfigFile()      # Create default if missing
└── rootCmd.Execute()
    └── PersistentPreRunE: validateOptions()
        ├── Merge Viper + flag values
        ├── validateStyle()
        ├── Detect terminal width
        └── Set notty style if not a terminal
```

### CLI Mode Path

```
execute(cmd, args)
├── stdinIsPipe() → true
│   └── executeCLI(cmd, source{stdin}, stdout)
└── stdinIsPipe() → false, args present
    └── executeArg(cmd, arg, stdout)
        ├── sourceFromArg(arg)
        │   ├── "-"          → stdin reader
        │   ├── readmeURL()  → GitHub/GitLab API fetch
        │   ├── "http(s)://" → HTTP GET
        │   ├── directory    → walk for README.md
        │   └── file         → os.Open
        └── executeCLI(cmd, source, stdout)
            ├── io.ReadAll(source)
            ├── RemoveFrontmatter()
            ├── glamour.NewTermRenderer()
            ├── r.Render(content)
            └── Output:
                ├── --pager → exec $PAGER
                ├── --tui   → runTUI(path, content)
                └── default → fmt.Fprint(stdout)
```

### TUI Mode Path

```
runTUI(path, content)
├── env.ParseAs[Config]()
├── ui.NewProgram(cfg, content)
│   └── tea.NewProgram(model, WithAltScreen)
└── program.Run()
    └── Bubble Tea event loop
```

## Bubble Tea Architecture

Glow's TUI follows the [Elm Architecture](https://guide.elm-lang.org/architecture/) via Bubble Tea:

```
                    ┌─────────────┐
                    │   Program   │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
          Init()      Update(msg)    View()
              │            │            │
              │     ┌──────┴──────┐     │
              │     │   model     │     │
              │     │  ┌────────┐ │     │
              │     │  │ state  │ │     │
              │     │  └────────┘ │     │
              │     │  ┌────────┐ │     │
              │     │  │ stash  │─┼─────┼──▶ stash.view()
              │     │  └────────┘ │     │
              │     │  ┌────────┐ │     │
              │     │  │ pager  │─┼─────┼──▶ pager.View()
              │     │  └────────┘ │     │
              │     └─────────────┘     │
              │                         │
              ▼                         ▼
         tea.Cmd                    string
      (side effects)             (terminal output)
```

### Model Hierarchy

```
model (ui/ui.go)
├── common *commonModel          # Shared: config, cwd, width, height
├── state                        # stateShowStash | stateShowDocument
├── stash stashModel             # File listing sub-model
│   ├── spinner                  # Loading animation
│   ├── filterInput              # Text input for fuzzy search
│   ├── sections []section       # Tabbed views (documents, filter results)
│   ├── markdowns []*markdown    # All discovered files
│   └── filteredMarkdowns        # Fuzzy-filtered subset
└── pager pagerModel             # Document viewer sub-model
    ├── viewport                 # Scrollable content area
    ├── currentDocument          # Loaded markdown content
    ├── searchInput              # Text input for / search prompt
    ├── searchQuery/Matches/Index # Active search state
    ├── lineInput                # Text input for : jump prompt
    └── watcher *fsnotify.Watcher # File change detection
```

### Message Flow

Messages flow through a single `Update` function and are delegated to sub-models:

```
tea.Msg arrives
    │
    ▼
model.Update(msg)
    │
    ├── Global handling (ctrl+c, ctrl+z, window resize)
    │
    ├── Key interception with inInputMode() gate:
    │   esc, q, h/delete are intercepted at top level
    │   BUT if pager.inInputMode() is true (search/jump active),
    │   keys are passed through to pager.update() instead
    │   Note: left arrow is NOT intercepted — it flows to pager
    │   for page-back navigation (←/→ = page back/forward)
    │
    ├── File search lifecycle:
    │   initLocalFileSearchMsg → store channel
    │   foundLocalFileMsg      → add to stash, find next
    │   localFileSearchFinished → mark loaded
    │
    ├── Document lifecycle:
    │   fetchedMarkdownMsg  → set current doc, render
    │   contentRenderedMsg  → switch to pager view
    │
    └── Delegate to active sub-model:
        ├── stateShowStash    → stash.update(msg)
        └── stateShowDocument → pager.update(msg)
                                ├── pagerStateBrowse     → handleBrowseKeys()
                                ├── pagerStateSearch     → handleSearchInput()
                                ├── pagerStateJumpToLine → handleJumpInput()
                                └── pagerStateStatusMessage → any key returns to browse
```

## Component Deep Dives

### File Discovery Pipeline

File discovery is async and non-blocking, streaming results via a Go channel:

```
findLocalFiles()                    # tea.Cmd
    │
    ▼
gitcha.FindFilesExcept()            # Returns chan SearchResult
    │                               # Respects .gitignore
    ▼
initLocalFileSearchMsg{ch}          # Channel stored in model
    │
    ▼
findNextLocalFile()  ◄──────┐       # tea.Cmd: reads one result
    │                       │
    ├── result available    │
    │   ▼                   │
    │   foundLocalFileMsg   │
    │   ├── localFileToMarkdown()
    │   ├── stash.addMarkdowns()
    │   ├── filterMarkdowns()  (if filter active)
    │   └── findNextLocalFile() ───┘  (loop)
    │
    └── channel closed
        ▼
        localFileSearchFinished
        └── stash.loaded = true
```

### Rendering Pipeline

Markdown rendering uses Glamour (built on goldmark + chroma):

```
renderWithGlamour(pagerModel, body)     # tea.Cmd
    │
    ▼
glamourRender(pager, markdown)
    │
    ├── Determine if file is code (non-.md extension)
    │   └── If code: WrapCodeBlock() wraps in ```lang fence
    │
    ├── Select style:
    │   ├── auto → detect dark/light background
    │   ├── named → dark, light, pink, dracula, tokyo-night, notty
    │   └── path  → custom JSON style file
    │
    ├── glamour.NewTermRenderer(style, width, options...)
    │   └── goldmark parser → chroma highlighting → ANSI output
    │
    ├── r.Render(content)
    │
    └── Post-processing:
        ├── Add line numbers (if enabled or code file)
        └── Truncate lines to viewport width
    │
    ▼
contentRenderedMsg(string)
    │
    ▼
pager.setContent()                      # Updates viewport
```

### Filtering System

Fuzzy filtering uses the `sahilm/fuzzy` package:

```
User presses "/"
    │
    ▼
filterState = filtering
filterInput.Focus()
    │
    ▼ (on each keystroke)
filterMarkdowns(stashModel)             # tea.Cmd
    │
    ├── Build targets: markdown.filterValue for each doc
    │   (filterValue = normalized path + note, diacritics removed)
    │
    ├── fuzzy.Find(query, targets)
    │   └── Returns ranked matches
    │
    └── sort.Stable(ranks)
    │
    ▼
filteredMarkdownMsg([]*markdown)
    │
    ▼
stash.filteredMarkdowns = msg
stash.updatePagination()
```

### File Watching

The pager watches the current document's directory for changes:

```
contentRenderedMsg received
    │
    ▼
pager.watchFile()                       # tea.Cmd (blocking)
    │
    ├── watcher.Add(dir)                # Watch parent directory
    │
    └── Event loop:
        ├── Write/Create on current file → reloadMsg
        └── Error → log and continue
    │
    ▼
reloadMsg
    │
    ▼
loadLocalMarkdown(&currentDocument)
    │
    ▼
fetchedMarkdownMsg → renderWithGlamour → contentRenderedMsg
```

## Data Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         User Input                              │
│  File path │ Directory │ URL │ GitHub shorthand │ stdin │ keys  │
└──────┬──────────┬────────┬──────────┬──────────────┬──────┬─────┘
       │          │        │          │              │      │
       ▼          ▼        ▼          ▼              ▼      │
   ┌────────────────────────────────────────────┐          │
   │           sourceFromArg()                  │          │
   │  Resolves input to io.ReadCloser + URL     │          │
   └────────────────┬───────────────────────────┘          │
                    │                                      │
         ┌──────────┴──────────┐                           │
         ▼                     ▼                           │
   ┌───────────┐      ┌──────────────┐                     │
   │ CLI Mode  │      │   TUI Mode   │◄────────────────────┘
   │           │      │              │
   │ Glamour   │      │ ┌──────────┐ │
   │ Render    │      │ │  Stash   │ │  File listing
   │           │      │ │  Model   │ │  + fuzzy filter
   │ stdout /  │      │ └────┬─────┘ │
   │ $PAGER    │      │      │       │
   │           │      │ ┌────▼─────┐ │
   └───────────┘      │ │  Pager   │ │  Document viewer
                      │ │  Model   │ │  + file watching
                      │ └──────────┘ │
                      └──────────────┘
```

## Source Resolution

Glow accepts multiple input types, resolved in priority order by `sourceFromArg()`:

```
Input                    Resolution
─────                    ──────────
"-"                      stdin
"owner/repo"             GitHub API → fetch README download_url
"gitlab.com/group/proj"  GitLab API → fetch readme_url
"https://example.com/f"  HTTP GET
"./docs"                 Walk directory for README.md variants
"./file.md"              Open file directly
(no args)                Launch TUI on current directory
```

README filename variants searched (case-insensitive): `README.md`, `README`, `Readme.md`, `Readme`, `readme.md`, `readme`.

## Styling Architecture

### Terminal Adaptation

Glow adapts its rendering to terminal capabilities:

```
Terminal Detection
    │
    ├── Background color → auto-select dark/light Glamour style
    ├── Color profile    → ANSI 16 / 256 / TrueColor
    ├── Terminal width   → word wrap (max 120 columns)
    └── Is TTY?          → use "notty" style if piped
```

### Style Layers

```
┌─────────────────────────────────────┐
│        Glamour (Markdown)           │  goldmark → chroma → ANSI
│  Styles: dark, light, pink,        │
│  dracula, tokyo-night, notty,      │
│  custom JSON                        │
├─────────────────────────────────────┤
│        Lipgloss (UI Chrome)         │  Status bars, logos, borders,
│  Adaptive colors for light/dark     │  pagination, help overlays
├─────────────────────────────────────┤
│        Reflow (Text Layout)         │  Word wrap, truncation,
│  ANSI-aware width calculation       │  printable rune width
└─────────────────────────────────────┘
```

### Color Palette (ui/styles.go)

The UI uses a fixed adaptive palette defined as `lipgloss.AdaptiveColor` values that automatically select appropriate colors for light and dark terminals:

- **Accent colors**: fuchsia, yellowGreen, green, red
- **Text hierarchy**: cream (primary), gray/brightGray (secondary), darkGray/dimGray (tertiary)
- **Status bar**: mintGreen/darkGreen background with adaptive foreground

## Configuration Layering

Configuration values are resolved with increasing priority:

```
Defaults (viper.SetDefault)
    ▼
Config file (glow.yml)
    ▼
Environment variables (GLOW_*)
    ▼
CLI flags (--style, --width, etc.)
```

Config file search paths (first found wins):
1. `$GLOW_CONFIG_HOME/glow.yml`
2. `$XDG_CONFIG_HOME/glow/glow.yml`
3. OS default via `go-app-paths`:
   - macOS: `~/Library/Application Support/glow/glow.yml`
   - Linux: `~/.config/glow/glow.yml`
   - Windows: `%APPDATA%\glow\glow.yml`

## External Integrations

### GitHub API

```
findGitHubREADME(owner, repo string)
    │
    ├── GET https://api.github.com/repos/{owner}/{repo}/readme
    ├── Parse JSON → extract download_url
    └── GET download_url → return source{reader, url}
```

### GitLab API

```
findGitLabREADME(projectID string)
    │
    ├── GET https://gitlab.com/api/v4/projects/{id}
    ├── Parse JSON → extract readme_url
    └── GET readme_url → return source{reader, url}
```

### System Integrations

| Integration | Implementation | Trigger |
|-------------|---------------|---------|
| External editor | `charmbracelet/x/editor` | `e` key in stash/pager |
| System clipboard | `atotto/clipboard` + OSC 52 | `c` key in pager |
| Pager | `exec.Command($PAGER)` or `less -r` | `--pager` flag |
| File watching | `fsnotify/fsnotify` | Automatic in pager view |

## Platform-Specific Behavior

### Windows (`console_windows.go`)

Enables ANSI escape code processing via Windows API:
```go
kernel32.SetConsoleMode(handle, mode | ENABLE_VIRTUAL_TERMINAL_PROCESSING)
```

### macOS (`ignore_darwin.go`)

Additional gitignore patterns for file discovery:
- `~/Library` (application support files)
- `$GOPATH`
- `node_modules`
- Dotfiles (`.*`)

### General Unix (`ignore_general.go`)

Standard ignore patterns:
- `$GOPATH`
- `node_modules`
- Dotfiles (`.*`)

## Concurrency Model

Glow uses Bubble Tea's command-based concurrency:

- **File search**: Runs in a goroutine via `gitcha`, results stream through a channel. Each result triggers a `tea.Cmd` to read the next one, keeping the UI responsive.
- **Glamour rendering**: Runs as a `tea.Cmd` (goroutine). The pager displays a loading state until `contentRenderedMsg` arrives.
- **File watching**: The `fsnotify` watcher runs as a blocking `tea.Cmd`. Events produce `reloadMsg` to trigger re-rendering.
- **Status message timers**: `time.Timer` wrapped in `tea.Cmd` for auto-dismissal.

All side effects are expressed as `tea.Cmd` functions, ensuring the main update loop remains pure and predictable.

## Build & Release

### Container Image

```dockerfile
FROM gcr.io/distroless/static    # Minimal, no shell, ~5-10MB
COPY glow /
ENTRYPOINT ["/glow"]
```

### GoReleaser Targets

Multi-platform binary builds via `.goreleaser.yml`:
- **OS**: macOS, Linux, Windows, FreeBSD, OpenBSD
- **Arch**: amd64, arm64, arm (v6/v7), i386
- **Distribution**: Homebrew tap, GitHub releases, Docker
