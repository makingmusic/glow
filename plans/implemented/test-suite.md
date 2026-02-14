# Plan: Comprehensive Test Suite for Glow

## Context

The glow project has near-zero test coverage (2 test functions total). We need a safety net before implementing new pager features (search, jump-to-line, page indicators). This plan creates unit tests for all testable pure functions and logic, plus E2E CLI integration tests.

## Architecture

- **Unit tests**: In-package `_test.go` files (`package ui`, `package utils`) for white-box access to unexported functions
- **E2E tests**: `tests/e2e_test.go` (`package tests`) — builds the binary and tests CLI behavior
- **Framework**: Standard `testing` package only (project convention)
- **Pattern**: Table-driven tests with `t.Run()` subtests
- **Formatting**: Must pass `gofumpt` and `goimports`

## Files to Create

### 1. `utils/utils_test.go` — Utility function tests

**Functions under test** (all exported, pure):

| Function | Test cases |
|----------|-----------|
| `RemoveFrontmatter` | YAML frontmatter stripped; no frontmatter unchanged; empty input; single delimiter; only at position 0 |
| `IsMarkdownFile` | `.md/.mdown/.mkdn/.mkd/.markdown` → true; `.go/.txt/.rs` → false; no extension → true; case insensitive `.MD`; multi-dot `file.tar.md` |
| `WrapCodeBlock` | Normal wrap; empty string; empty language; multiline |
| `ExpandPath` | Tilde `~/foo`; env var `$HOME/foo`; absolute unchanged; empty string |
| `GlamourStyle` | Known styles ("dark", "light", "notty"); isCode=true modifies margin; "auto" style |

### 2. `ui/markdown_test.go` — Markdown type tests

**Functions under test** (unexported):

| Function | Test cases |
|----------|-----------|
| `normalize(string)` | Diacritics: "café"→"cafe", "naïve"→"naive", "München"→"Munchen"; ASCII unchanged; empty string |
| `relativeTime(time.Time)` | <1 min → "just now"; minutes/hours/days ago → relative; >1 week → formatted date "02 Jan 2006 15:04 MST" |
| `(*markdown).buildFilterValue()` | Sets filterValue to normalized Note; diacritics; empty Note |

### 3. `ui/sort_test.go` — Sort tests

| Function | Test cases |
|----------|-----------|
| `sortMarkdowns` | Alphabetical by Note; empty slice; single item; already sorted; reverse; duplicate notes (stable) |

### 4. `ui/pager_test.go` — Pager rendering tests

**Helper** — `testPagerModel(width, height int, cfg Config) pagerModel`:
```go
func testPagerModel(width, height int, cfg Config) pagerModel {
    config = cfg
    common := &commonModel{cfg: cfg, width: width, height: height}
    vp := viewport.New(width, height)
    return pagerModel{
        common: common, viewport: vp,
        state: pagerStateBrowse,
    }
}
```

| Function | Test cases |
|----------|-----------|
| `glamourRender` | GlamourEnabled=false → raw markdown; markdown file renders non-empty; code file wraps in code block; ShowLineNumbers=true adds prefixes; code files always get line numbers |
| `statusBarView` | Browse state shows Note; status message state shows message; narrow width no panic; normal width has all components |
| `helpView` | Contains key bindings (g/home, G/end, esc, q); pads to width; non-empty |
| `setSize` | Viewport dimensions correct; accounts for statusBarHeight; showHelp reduces height |
| `localDir` | Returns dir of currentDocument.localPath |

**Global config handling**: Save/restore `config` with `t.Cleanup`.

### 5. `ui/stash_test.go` — Stash logic tests

**Helper** — `testStashModel(numMarkdowns, perPage int) stashModel`:
Must call `initSections()` first, create sections slice with paginator, populate markdowns, and call `buildFilterValue()` on each markdown for filter tests.

| Function | Test cases |
|----------|-----------|
| `moveCursorUp` | At top of first page → stays 0; middle → decrements; top of page 2 → prev page, last cursor |
| `moveCursorDown` | Middle → increments; bottom of non-last page → next page cursor=0; bottom of last page → stays |
| `updatePagination` | Correct page count; empty markdowns → 1 page; PerPage from available height |
| `filterMarkdowns` | No filter → returns all; fuzzy match; no matches → empty (execute tea.Cmd, type-assert `filteredMarkdownMsg`) |
| `selectedMarkdown` | Valid cursor → correct markdown; empty list → nil; out of bounds → nil |
| `getVisibleMarkdowns` | Not filtering → markdowns; filtering → filteredMarkdowns |
| `markdownIndex` | Page 0 cursor 2 perPage 5 → 2; page 1 cursor 3 perPage 5 → 8 |

### 6. `tests/e2e_test.go` — CLI integration tests

**Setup**: `TestMain` builds the binary once to a temp path, all tests invoke it.

| Test | Command | Assertion |
|------|---------|-----------|
| `TestRenderMarkdownFile` | `glow testdata/test.md` | Exit 0, non-empty output |
| `TestRenderWithStyle` | `glow -s dark testdata/test.md` | Exit 0 |
| `TestRenderWithWidth` | `glow -w 40 testdata/test.md` | Exit 0 |
| `TestRenderWithLineNumbers` | `glow -l testdata/test.md` | Output contains line number patterns |
| `TestStdinPipe` | `echo "# Hello" \| glow` | Output contains "Hello" |
| `TestInvalidFile` | `glow nonexistent.md` | Non-zero exit |
| `TestHelpFlag` | `glow --help` | Contains "glow" and usage text |

### 7. `tests/testdata/test.md` — Sample markdown for E2E

Simple markdown with heading, list, code block, bold/italic.

## Key Implementation Details

1. **Global `config` variable** (`ui/ui.go:25`): Many pager/stash functions read from `var config Config`. Tests must set it and restore via `t.Cleanup`.

2. **`initSections()`** (`ui/stash.go:113`): Must be called before constructing `stashModel` — it populates the `sections` map used by `newStashModel`.

3. **`filterMarkdowns` returns `tea.Cmd`**: Execute with `cmd := filterMarkdowns(m); msg := cmd()` then type-assert `msg.(filteredMarkdownMsg)`.

4. **`updatePagination` calls `helpView`** (`stash.go:256`): The stash's `helpView` needs `common.width`/`common.height` set, and the stash model needs its filter input initialized (it reads `filterInput.Prompt` width).

5. **`GlamourStyle("auto", ...)` queries terminal**: Skip or use explicit style in tests to avoid terminal-dependent failures in CI.

6. **E2E binary path**: Build with `go build -o <tempdir>/glow-test ..` from the `tests/` dir. Use `os.TempDir()` for the output path.

## Verification

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run specific packages
go test -v ./utils/...
go test -v ./ui/...
go test -v ./tests/...

# Check formatting
gofumpt -l .
goimports -l .
```

## Estimated Scope

| File | ~Lines | Functions tested |
|------|--------|-----------------|
| `utils/utils_test.go` | 200 | 5 |
| `ui/markdown_test.go` | 120 | 3 |
| `ui/sort_test.go` | 80 | 1 |
| `ui/pager_test.go` | 280 | 5 |
| `ui/stash_test.go` | 300 | 7 |
| `tests/e2e_test.go` | 180 | 7 |
| `tests/testdata/test.md` | 20 | — |
| **Total** | **~1,180** | **28** |
