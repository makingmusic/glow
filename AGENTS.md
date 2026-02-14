# Agents

## Repository Setup

This is a fork of `charmbracelet/glow`. **NEVER push to the upstream repository.** All pushes must go to `origin` (`makingmusic/glow`) only.

After cloning on a new machine, run:

```bash
./scripts/setup-remotes.sh
```

This configures the `upstream` remote (for fetching updates from `charmbracelet/glow`) with push disabled. If you see `upstream` without `DISABLED` as its push URL, run the script before doing anything else.

**Rules for agents:**
- Never run `git push upstream` or `git push` targeting `charmbracelet/glow`
- Only push to `origin` (`makingmusic/glow`)
- To sync upstream changes: `git fetch upstream && git merge upstream/master`

## Build, Lint, Test

```bash
go build ./...                # Build
golangci-lint run             # Lint (26 linters + gofumpt/goimports)
go test ./...                 # Test
go test -race ./...           # Test with race detector
```

Or via task runner: `task lint`, `task test`.

Lint must pass before any PR. The linter config is in `.golangci.yml` and enforces `gofumpt` formatting and `goimports` ordering. Run `gofumpt -w .` and `goimports -w .` if the formatter complains.

## Project Structure

This is a Go CLI application with two execution modes:

- **CLI mode**: Render a single markdown source (file, URL, stdin) to stdout. Entry path: `main.go` → `executeCLI()`.
- **TUI mode**: Interactive file browser + document pager. Entry path: `main.go` → `runTUI()` → `ui/ui.go`.

```
main.go              CLI entry, Cobra commands, source resolution
url.go               GitHub/GitLab shorthand URL parsing
github.go            GitHub README API fetch
gitlab.go            GitLab README API fetch
config_cmd.go        `glow config` subcommand
log.go               Logging setup
style.go             CLI output styling helpers
utils/utils.go       Shared utilities (frontmatter, paths, glamour style selection)
ui/                  Entire TUI lives here
  ui.go              Top-level Bubble Tea model, Init/Update/View
  stash.go           File listing view (browsing, filtering, pagination)
  pager.go           Document viewer (viewport, rendering, file watching)
  markdown.go        Markdown document type
  stashitem.go       Single file item rendering in list
  stashhelp.go       Help overlay rendering
  styles.go          Color palette (lipgloss adaptive colors)
  sort.go            Markdown sorting by note (path)
  editor.go          External editor invocation
  keys.go            Key constants
  config.go          Config struct with env tags
  ignore_darwin.go   macOS gitignore patterns
  ignore_general.go  Non-macOS gitignore patterns
```

## Architecture: Bubble Tea Pattern

The TUI follows the Elm architecture (Init → Update → View) via the Bubble Tea framework.

**Model hierarchy**: `model` owns `stashModel` (file listing) and `pagerModel` (document viewer). Both share a `*commonModel` for config, cwd, and terminal dimensions.

**State**: `model.state` is either `stateShowStash` or `stateShowDocument`. The top-level `Update` delegates messages to the active sub-model.

**Side effects**: All I/O is done via `tea.Cmd` functions that return `tea.Msg` values. Never perform I/O directly in `Update` or `View`. Commands include: `findLocalFiles`, `findNextLocalFile`, `loadLocalMarkdown`, `renderWithGlamour`, `filterMarkdowns`, `openEditor`, `watchFile`.

**Message types** (defined across ui.go, stash.go, pager.go):
- `initLocalFileSearchMsg` / `foundLocalFileMsg` / `localFileSearchFinished` — async file discovery
- `fetchedMarkdownMsg` — document content loaded from disk
- `contentRenderedMsg` — glamour rendering complete
- `filteredMarkdownMsg` — fuzzy filter results
- `reloadMsg` — file changed on disk (fsnotify)
- `editorFinishedMsg` — external editor closed
- `statusMessageTimeoutMsg` — ephemeral status message expired
- `errMsg` — error propagation

## Coding Conventions

**Formatting**: Code must pass `gofumpt` and `goimports`. No exceptions.

**Nolint directives**: Used sparingly with specific linter names (e.g., `//nolint:gosec`, `//nolint:nestif`). The `nolintlint` linter enforces that every `//nolint` specifies which linter it suppresses.

**Error handling**: Errors in the TUI are wrapped in `errMsg` and propagated through the message system. Fatal errors are stored in `model.fatalErr` and displayed with `errorView()`. CLI errors bubble up through `cobra.Command.RunE`.

**Styling**: All terminal colors use `lipgloss.AdaptiveColor` (light/dark variants) defined in `ui/styles.go`. Rendering functions are stored as `lipgloss.NewStyle().Foreground(color).Render` variables (e.g., `grayFg`, `fuchsiaFg`). The status bar and help views in `pager.go` use locally-scoped style variables.

**Bubble Tea commands**: Commands are standalone functions that return `tea.Cmd`. They capture needed values from the model by value (not pointer) to avoid races. See `renderWithGlamour`, `filterMarkdowns`, `loadLocalMarkdown` for examples.

**Platform-specific code**: Use build tags (`//go:build darwin` / `//go:build !darwin`). Currently only `ignore_darwin.go` and `ignore_general.go` differ. Windows has `console_windows.go` for ANSI support.

**Config resolution order**: CLI flags → environment variables → config file → defaults. Viper handles merging. The `ui.Config` struct uses `env` struct tags for environment variable parsing via `caarlos0/env`.

## Key Patterns to Follow

1. **Adding a new key binding in the stash view**: Handle it in `stashModel.handleDocumentBrowsing()` (stash.go) for browse mode or `stashModel.handleFiltering()` for filter mode. Update the help text in `stashhelp.go`.

2. **Adding a new key binding in the pager**: Handle it in `pagerModel.handleBrowseKeys()` (pager.go) for browse-mode keys. For keys that need to work during search/jump input, handle them in `handleSearchInput()` or `handleJumpInput()`. Update `pagerModel.helpView()` in the same file.

3. **Adding a new message type**: Define the type in the appropriate file (ui.go for top-level, stash.go for stash-specific, pager.go for pager-specific). Handle it in `model.Update()` if it's global, or in the sub-model's `update()` method.

4. **Adding a new CLI flag**: Add the flag variable and `rootCmd.Flags()` call in `main.go:init()`. Bind it to Viper with `viper.BindPFlag()`. Read it in `validateOptions()`.

5. **Modifying glamour rendering**: The render pipeline is in `pager.go:glamourRender()`. Style selection logic is in `utils/utils.go:GlamourStyle()`.

6. **Adding a new source type**: Extend `sourceFromArg()` in `main.go`. Follow the existing pattern: attempt resolution, return `*source` on success, fall through on failure.

## Common Gotchas

- The stash model uses **pointer-based sections** (`m.paginator()` returns `*paginator.Model`). Be careful about mutation vs. copy semantics in View methods.
- `filterMarkdowns` captures the stash model **by value** to avoid data races. If you add fields to `stashModel` that the filter needs, they'll be captured at call time.
- File search results arrive **one at a time** via channel. Each `foundLocalFileMsg` triggers another `findNextLocalFile` command — it's a recursive command chain, not a loop.
- `renderWithGlamour` is called on **every window resize** (see `pager.update` handling of `tea.WindowSizeMsg`). Keep the render path efficient.
- The `watcher` in pagerModel watches the **directory**, not the file directly, because some editors do atomic saves (write temp + rename).
- Non-markdown files are rendered as code blocks: `utils.WrapCodeBlock` wraps content in a fenced code block, and `glamourRender` sets `width = 0` to disable word wrap for code.
- **Top-level key interception in `ui.go`**: The top-level `model.Update()` intercepts `esc`, `q`, `h`/`delete` before they reach the pager. When the pager is in input mode (search, jump, or has active search results), `pagerModel.inInputMode()` returns `true` and `ui.go` skips interception, letting the key flow through to `pager.update()`. `left` arrow is NOT intercepted — it flows through to the pager's `handleBrowseKeys()` for page-back navigation. Any new pager input state must be covered by `inInputMode()`.
- **Pager search uses raw `Body`**: `findMatches()` searches against `m.currentDocument.Body` (pre-glamour), not the rendered viewport content. This avoids false matches on line-number gutters and ANSI escape codes. Navigation uses `viewport.SetYOffset()` which maps to raw content lines.
