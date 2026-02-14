# Plan: Pager Keyboard Navigation & Pagination


## Objective
I want to build keyboard based pagination and navigation when i am viewing a markdown file.

## Context

The pager (document viewer) in `ui/pager.go` already has solid line/page scrolling via the `viewport` component from `charmbracelet/bubbles`. This plan adds visual page indicators, in-document search, line jump, and percentage jump — plus the test infrastructure this repo is missing.

## What Already Exists

| Feature | Keys | Status |
|---------|------|--------|
| Line scroll | `j/k`, `↑/↓` | Works (viewport) |
| Full page | `f/b`, `pgup/pgdn` | Works (viewport) |
| Half page | `d/u` | Works (explicit) |
| Jump to top/bottom | `g/G`, `home/end` | Works (explicit) |
| Copy, edit, reload | `c`, `e`, `r` | Works |
| Scroll percentage | — | Shown in status bar |

## Critical: Top-Level Key Interception in `ui/ui.go`

The top-level `model.Update()` in `ui/ui.go` intercepts several keys **before** they reach `pagerModel.update()`. This has major implications for Phases 3-5:

| Key | `ui.go` behavior | Line | Impact |
|-----|-----------------|------|--------|
| `esc` | Calls `m.unloadDocument()` when `stateShowDocument` | 218-222 | Pager never sees `esc`. Search/jump cancel won't work. |
| `q` | Calls `tea.Quit` (always, except during stash filtering) | 235-247 | Pager never sees `q`. Typing `q` in a search query quits the app. |
| `left`, `h`, `delete` | Calls `m.unloadDocument()` when `stateShowDocument` | 249-253 | Pager never sees these. `h` can't be typed in search, `left`/`delete` can't edit input. |

**Consequence:** The pager's existing `"q", keyEsc` handler at `pager.go:190` is dead code — neither key ever reaches it.

**Required fix (Phase 3):** Add a method like `pagerModel.consumesKey()` or `pagerModel.inInputMode()` that returns `true` when the pager is in a search/jump/active-search state. In `ui.go`, check this before intercepting `esc`, `q`, `left`, `h`, and `delete` — if the pager wants the key, skip the top-level handler and let the message flow to `pager.update()` via the normal child-update path at lines 318-322.

## What's Missing

1. Visual page indicator ("page X of Y") in the pager status bar
2. In-document search (`/pattern`, `n`/`N` for next/prev)
3. Jump to line (`:N`)
4. Jump to percentage

## Current Test Coverage

Nearly zero. Two test functions total:
- `glow_test.go` — 3 CLI flag parse checks
- `url_test.go` — skipped (network-dependent)

No tests for UI logic, rendering, utils, filtering, or navigation.

---

## UX Decisions (Approved)

These decisions were confirmed by the user and must be followed during implementation.

### Status Bar & Indicators

- **Page indicator placement**: Next to scroll % on the right side of the status bar. Layout becomes: `[Logo] [Note] [Padding] [pg 3/12] [50%] [? Help]`
- **Search match counter placement**: A `"3/17"` badge next to scroll % on the right side (note area stays visible). Layout when search active: `[Logo] [Note] [Padding] [3/17] [50%] [? Help]`

### Search Prompt (`/`)

- **Prompt style**: Replaces the entire status bar (like less/vim). The full bar becomes a `/query_` input line.
- **Case sensitivity**: Always case-insensitive. Searching "foo" matches "Foo", "FOO", etc.
- **Highlighting**: Line-level only (jump to matching line, no character-level highlighting). Matches how `less` works. Character-level highlighting could be added later.

### Search Behavior

- **Esc during search input**: Cancel and clear everything — dismisses input, clears query, removes match counter, returns to normal browse mode.
- **Esc while browsing with active results**: Clears the search results and match counter, returns to normal mode.
- **Wrap behavior (n/N at boundaries)**: Wraps around with a brief "search wrapped" status message (like vim's "search hit BOTTOM, continuing at TOP").

### Jump Prompt (`:`)

- **Prompt style**: Same as search — replaces the entire status bar with a `:` prefix input line. Consistent with `/`.

---

## Phase 1: Test Infrastructure (Safety Net)

Build tests for existing pure functions and core logic before changing anything.

### 1.1 — `utils/utils_test.go`

Test the shared utility functions:

- **`RemoveFrontmatter`**: YAML frontmatter stripped correctly; content without frontmatter unchanged; edge cases (empty, single delimiter, no closing delimiter)
- **`IsMarkdownFile`**: `.md`, `.mdown`, `.mkdn`, `.mkd`, `.markdown` → true; `.go`, `.txt`, `.rs` → false; empty extension → true (default assumption); case insensitivity
- **`WrapCodeBlock`**: wraps content in fenced code block with language tag
- **`ExpandPath`**: tilde expansion, env var expansion
- **`GlamourStyle`**: returns correct option for each named style; code vs markdown mode removes margin

**Files:** `utils/utils_test.go` (new)
**Effort:** Small
**Risk:** None

### 1.2 — `ui/markdown_test.go`

Test the markdown type's pure methods:

- **`normalize`**: diacritics removed ("ö" → "o"), ASCII unchanged
- **`buildFilterValue`**: sets `filterValue` to normalized `Note`
- **`relativeTime`**: "just now" for recent, relative format for <1 week, absolute format for older

**Files:** `ui/markdown_test.go` (new)
**Effort:** Small
**Risk:** None

### 1.3 — `ui/sort_test.go`

- **`sortMarkdowns`**: sorts by `Note` field alphabetically, stable sort

**Files:** `ui/sort_test.go` (new)
**Effort:** Trivial
**Risk:** None

### 1.4 — `ui/pager_test.go` (rendering)

Test `glamourRender` — the core rendering function:

- Renders markdown content to non-empty string
- Line numbers added when `ShowLineNumbers` is true
- Code files get line numbers by default
- Non-markdown files are wrapped in code blocks
- Glamour disabled → returns raw content

**Files:** `ui/pager_test.go` (new)
**Effort:** Medium — needs to construct a `pagerModel` with a `commonModel` and viewport
**Risk:** Low — `glamourRender` is a pure function (model passed by value)

### 1.5 — `ui/stash_test.go` (filtering & cursor)

- **`filterMarkdowns`**: returns all when filter empty; fuzzy matches when filter set; results ordered by relevance
- **Cursor movement**: `moveCursorUp`/`moveCursorDown` stay in bounds, page transitions work
- **`updatePagination`**: correct page count for various document counts and viewport sizes

**Files:** `ui/stash_test.go` (new)
**Effort:** Medium
**Risk:** Low

---

## Phase 2: Page Indicator in Status Bar

### What

Show "pg X/Y" next to the scroll percentage on the right side of the status bar.

### How

In `ui/pager.go`, compute page info from viewport state:

```go
height := max(1, m.viewport.Height) // guard against zero-height terminal
currentPage := m.viewport.YOffset/height + 1
totalPages := (m.viewport.TotalLineCount() + height - 1) / height
currentPage = min(currentPage, totalPages) // clamp
```

Render it in `statusBarView()` — insert a styled `" pg 3/12 "` string to the left of the existing scroll percentage on the right side.

**Important:** The note truncation math in `statusBarView()` (currently at line 332-337) calculates available width by subtracting logo, scrollPercent, and helpNote widths. The new page indicator string must also be subtracted from this calculation, otherwise the note will overlap with the new element. Add `ansi.PrintableRuneWidth(pageIndicator)` to both the note truncation and the padding width computations.

### Status Bar Layout (After Phase 2)

```
[Logo] [Note (truncated)] [Padding] [pg 3/12] [50%] [? Help]
```

### Files

- `ui/pager.go` — `statusBarView()` only (~15 lines changed)

### Tests

- `ui/pager_test.go` — Add test: given known content length and viewport height, assert correct page/total calculation
- Edge cases: zero-height viewport returns page 1/1, single-line content, content exactly filling one page

### Effort: Small
### Risk: Low — purely additive display change, no state or key handling modified

---

## Phase 3: In-Document Search

### What

`/` enters search mode, type a query, matches are found, `n`/`N` jump between matches, `esc` cancels/clears search.

### How

#### 3.0 — Route intercepted keys from `ui.go` to pager (CRITICAL)

This step **must** be done first. Without it, `esc`, `q`, `h`, `left`, and `delete` never reach the pager (see "Critical: Top-Level Key Interception" section above).

**Add a method to `pagerModel`:**

```go
// inInputMode returns true when the pager is in a state that consumes
// arbitrary key input (search prompt, jump prompt) or has active search
// results that esc should clear before unloading the document.
func (m pagerModel) inInputMode() bool {
    return m.state == pagerStateSearch ||
        m.state == pagerStateJumpToLine ||
        m.searchQuery != ""
}
```

**Modify `model.Update()` in `ui/ui.go`** — wrap each intercepted key with a pager input-mode check:

```go
case "esc":
    if m.state == stateShowDocument && m.pager.inInputMode() {
        break // let it fall through to pager.update() via child dispatch
    }
    if m.state == stateShowDocument || m.stash.viewState == stashStateLoadingDocument {
        batch := m.unloadDocument()
        return m, tea.Batch(batch...)
    }

case "q":
    if m.state == stateShowDocument && m.pager.inInputMode() {
        break // let pager handle (typing 'q' in search input, or clearing search)
    }
    // ...existing quit logic...

case "left", "h", "delete":
    if m.state == stateShowDocument && m.pager.inInputMode() {
        break // let pager textinput handle cursor/delete
    }
    // ...existing unload logic...
```

The `break` exits the inner `switch msg.String()` and falls through to the child-model dispatch at lines 318-322, where `m.pager.update(msg)` is called normally.

#### 3.1 — New state and fields on `pagerModel`

```go
// Add to pagerState enum
pagerStateSearch  // user is typing search query

// Add to pagerModel struct
searchInput   textinput.Model   // text input for search query
searchQuery   string            // active search term
searchMatches []int             // line numbers with matches (0-indexed)
searchIndex   int               // current match index (-1 = none)
```

Initialize `searchInput` in `newPagerModel()` — set prompt to `"/"`, configure character limit, disable cursor blink (consistent with the stash's text input pattern in `stash.go`).

#### 3.2 — Key handling in `pagerModel.update()`

Restructure the existing key switch to check state first. The current `"q", keyEsc` case at line 190 is dead code (keys intercepted by ui.go) and should be replaced with proper per-state handling:

**In browse state (`pagerStateBrowse`):**
- `/` → set `state = pagerStateSearch`, focus `searchInput`, clear previous input
- `n` (when `searchQuery != ""`) → jump to next match (`searchIndex++`, wrap with "search wrapped" message)
- `N` (when `searchQuery != ""`) → jump to previous match (`searchIndex--`, wrap with "search wrapped" message)
- `esc` (when `searchQuery != ""`) → **clear search results**: clear query, clear matches, remove match counter
- All other existing keys (`g`, `G`, `d`, `u`, `e`, `c`, `r`, `?`) — unchanged

**In search state (`pagerStateSearch`):**
- `enter` → apply search: scan content lines for matches, jump to first match, set `state = pagerStateBrowse`. If query is empty, cancel and return to browse with a brief "no pattern" status message.
- `esc` → **cancel and clear everything**: clear query, clear matches, return to browse
- All other keys → delegate to `searchInput.Update(msg)` (this handles typing, backspace, cursor movement, etc.)

**In status message state (`pagerStateStatusMessage`):**
- Any key → return to browse (existing behavior)

#### 3.3 — Match finding

Search against ANSI-stripped content lines. `muesli/reflow/ansi` is already imported in `pager.go` (line 19) — use its `Writer` or the `ansi.Strip()` helper if available, or write a small strip function using the existing import. Store matching line numbers in `searchMatches`. **Always case-insensitive.**

**Caveat — line-number gutter:** When `ShowLineNumbers` is enabled, `glamourRender` prepends line numbers to each rendered line. Searching the rendered content would match against these numbers (e.g., searching "12" matches line 12's gutter). To avoid this, prefer searching against `m.currentDocument.Body` (the raw markdown before glamour rendering) and map match indices to rendered line positions. Since glamour can reflow text (wrapping long lines into multiple rendered lines), a 1:1 line mapping may not hold. The simplest correct approach: search against `m.currentDocument.Body` lines, then for navigation, compute the rendered line offset by scanning the rendered content for the Nth original-content line. Alternatively, if line numbers are off and the document isn't reflowed, searching rendered content directly is fine.

```go
func findMatches(content string, query string) []int {
    lines := strings.Split(content, "\n")
    lowerQuery := strings.ToLower(query)
    var matches []int
    for i, line := range lines {
        if strings.Contains(strings.ToLower(line), lowerQuery) {
            matches = append(matches, i)
        }
    }
    return matches
}
```

#### 3.4 — Navigation to matches

Jump viewport to matching line: `m.viewport.SetYOffset(matchLineNumber)`.

When wrapping around (past last match with `n`, or before first match with `N`), show a brief **"search wrapped"** status message using the existing `statusMessageTimeout` pattern.

#### 3.5 — Search prompt (replacing status bar)

When `/` is pressed, the **entire status bar is replaced** by a search input line:

```
/query_
```

This matches the less/vim convention. The logo, note, scroll %, and help hint are all hidden while typing.

#### 3.6 — Match counter in status bar (browse mode)

When search results are active and the user is back in browse mode, show a **match counter badge next to scroll %** on the right side:

```
[Logo] [Note] [Padding] [3/17] [pg 3/12] [50%] [? Help]
```

As with the page indicator (Phase 2), subtract the match counter width from the note truncation and padding calculations in `statusBarView()`.

If no matches found, show `"no matches"` briefly as a status message (using existing `showStatusMessage` pattern), then return to browse with no match counter visible.

#### 3.7 — Help view

Add search keys to `helpView()`:
- `/` search
- `n` next match
- `N` prev match
- `esc` clear search

### Files

- **`ui/ui.go`** — key interception routing (step 3.0, ~15 lines changed)
- **`ui/pager.go`** — `inInputMode()`, state enum, model fields, update (key handling), statusBarView, helpView (~120-150 lines)
- `ui/styles.go` — match counter style (optional, could reuse existing styles)
- `ui/keys.go` — add key constants if needed

### Tests

- `ui/pager_test.go`:
  - `findMatches`: query against known content, assert correct line numbers
  - `findMatches`: case insensitivity (always case-insensitive)
  - `findMatches`: no matches returns empty slice
  - `findMatches`: multiple matches on same line counted once
  - Search index wrapping: after last match, `n` wraps to first; before first, `N` wraps to last
  - `inInputMode`: returns true in search/jump states and when searchQuery is set
  - Empty query on enter: returns to browse with status message

### Effort: Medium-Large

The core search logic is straightforward. The complexity is in:
- **`ui.go` key routing** (step 3.0) — the most critical change; must be done first or nothing else works
- Correctly integrating a new `textinput.Model` into the pager's update loop
- Status bar layout changes to accommodate the full-bar search input and match counter badge
- Line-number gutter and glamour reflow affecting match-to-rendered-line mapping

### Risk: Medium

- **Key routing in ui.go**: The biggest risk. Must ensure that `inInputMode()` correctly gates all intercepted keys. If it misses a state, keys will be swallowed; if it's too broad, normal pager esc/q behavior breaks. The `break` approach (falling through to child dispatch) is safe because the existing child-update path at lines 318-322 already handles all pager messages.
- **Line-number gutter matching**: Searching rendered content with line numbers enabled produces false positives. Searching raw `Body` avoids this but requires a line-mapping strategy for navigation. Start with raw-body search; defer rendered-content search to a follow-up if needed.
- **Re-rendering interaction**: Search doesn't re-render through glamour — it operates on already-available content. This avoids the expensive glamour pipeline but means highlights are line-level only.
- **State complexity**: Adding search/jump input modes to the pager. The `inInputMode()` method centralizes the state check. `n`/`N` don't conflict with existing keys — `n` is unused in the pager.
- **Esc layering**: `esc` now has three meanings: (1) in search/jump input → cancel, (2) in browse with active search → clear results, (3) in browse with no search → unload document (handled by ui.go). The `inInputMode()` gate makes this clean.

### Design Decision: Line-level vs Character-level Highlighting

**Chosen: Line-level.** Jump to the matching line. The line is visible in the viewport. No re-rendering of content. Matches how `less` works. Character-level highlighting could be added later as a follow-up.

---

## Phase 4: Jump to Line

### What

`:` enters line-jump mode, type a number, `enter` jumps to that line.

### How

#### 4.1 — New state

```go
pagerStateJumpToLine  // user is typing line number
```

#### 4.2 — New field

Reuse `searchInput` or add a second `textinput.Model` for the line input (simpler to add a dedicated one to avoid state conflicts):

```go
lineInput textinput.Model
```

Initialize in `newPagerModel()` with prompt `":"`.

#### 4.3 — Key handling

- `:` (in browse state) → set `state = pagerStateJumpToLine`, focus `lineInput`, clear previous input
- `enter` → parse number, clamp to valid range, `m.viewport.SetYOffset(n-1)`, return to browse. If input is empty or non-numeric (and not `%`), show brief error status message and return to browse.
- `esc` → cancel, return to browse
- All other keys → delegate to `lineInput.Update(msg)`

**Note:** The ui.go key routing from step 3.0 already covers `pagerStateJumpToLine` via `inInputMode()`. No additional ui.go changes needed.

#### 4.4 — Jump prompt (replacing status bar)

When `:` is pressed, the **entire status bar is replaced** by a jump input line (same pattern as search):

```
:123_
```

Consistent with the `/` search prompt style.

### Files

- `ui/pager.go` — state enum, model field, update, statusBarView (~30-40 lines)

### Tests

- `ui/pager_test.go`:
  - Valid line number sets correct offset
  - Line number > total lines clamps to last line
  - Line number < 1 clamps to first line
  - Non-numeric input is rejected or ignored

### Effort: Small
### Risk: Low — isolated feature, relies on ui.go routing already done in Phase 3

---

## Phase 5: Jump to Percentage

### What

Extend the `:` input to accept `N%` syntax (e.g., `:50%` jumps to 50% of the document).

### How

In the `enter` handler for jump-to-line, check if input ends with `%`:

```go
input := m.lineInput.Value()
if strings.HasSuffix(input, "%") {
    pct, err := strconv.Atoi(strings.TrimSuffix(input, "%"))
    if err == nil {
        pct = max(0, min(100, pct)) // clamp to [0, 100]
        totalLines := m.viewport.TotalLineCount()
        if pct == 0 {
            m.viewport.GotoTop()
        } else if pct == 100 {
            m.viewport.GotoBottom()
        } else {
            target := int(math.Round(float64(totalLines) * float64(pct) / 100))
            m.viewport.SetYOffset(target)
        }
    }
} else {
    // existing line-number logic
}
```

Explicit `GotoTop`/`GotoBottom` for 0%/100% avoids off-by-one issues with the viewport's max offset clamping.

### Files

- `ui/pager.go` — extend jump-to-line handler (~10 lines)

### Tests

- `ui/pager_test.go`:
  - `50%` on 100-line doc jumps to line 50
  - `0%` jumps to top
  - `100%` jumps to bottom
  - `150%` clamps to bottom

### Effort: Trivial
### Risk: Negligible — extends Phase 4 with a small conditional

---

## Implementation Order

```
Phase 1 (tests)  ──→  Phase 2 (page indicator)  ──→  Phase 3 (search)
                                                         │
                                                         ├──→  Phase 4 (line jump)
                                                         │
                                                         └──→  Phase 5 (% jump)
```

Phase 1 first as a safety net. Phase 2 is a quick win. Phases 4-5 are independent of Phase 3 and can be done in any order, but share the `:` input pattern.

## Risk Summary

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| ui.go key routing breaks normal esc/q behavior | Medium | High | `inInputMode()` centralizes the check; test both input-mode and browse-mode key paths |
| Line-number gutter produces false search matches | Medium | Medium | Search against raw `Body` instead of rendered content; map back to viewport offsets |
| ANSI escape sequences break search matching | Medium | Medium | Search raw body (no ANSI); only strip ANSI if searching rendered content |
| New key bindings conflict with viewport defaults | Low | Low | `n`, `N`, `/`, `:` are not used by viewport or pager currently |
| State machine complexity grows unwieldy | Low | Medium | Keep search and line-jump as isolated states; `inInputMode()` centralizes routing |
| Esc key layering causes confusion | Low | Medium | Three clear layers: input cancel → clear results → unload document (ui.go) |
| Performance on large documents | Low | Low | Search is O(lines); no re-rendering through glamour |
| High-performance rendering mode interaction | Medium | Medium | Call `viewport.Sync()` after search jumps, same pattern as existing `d`/`u`/`g`/`G` handlers |
| Division by zero on tiny terminals | Low | Medium | Guard with `max(1, viewport.Height)` in page calculations |

## Resolved Gaps

These were identified during review and are now integrated into the phases above:

1. **Top-level key interception in ui.go** — Addressed in Phase 3, step 3.0 (`inInputMode()` + routing changes). Also covers `q`, `h`, `left`, `delete` which the original plan missed entirely.
2. **Division-by-zero in page calculations** — Addressed in Phase 2 with `max(1, viewport.Height)` guard.
3. **Search matches line-number gutter** — Addressed in Phase 3, step 3.3 (search raw `Body` instead of rendered content).
4. **Percentage-jump rounding and bounds** — Addressed in Phase 5 with explicit `GotoTop`/`GotoBottom` for 0%/100% and clamping.
5. **Empty-input behavior for prompts** — Addressed in Phase 3 step 3.2 (empty query → status message) and Phase 4 step 4.3 (invalid input → error message).
6. **Pager's existing `"q", keyEsc` dead code** — Noted in "Critical: Top-Level Key Interception" section. The dead code at `pager.go:190` should be replaced with proper per-state handling in Phase 3.

## Remaining Minor Gaps

1. **Status-message timer interaction with search/jump states**: If a status message timeout fires while the user is in search/jump input mode, the `statusMessageTimeoutMsg` handler at `pager.go:273` sets `state = pagerStateBrowse`, which would cancel the input. Mitigation: in the `statusMessageTimeoutMsg` handler, only transition to browse if state is `pagerStateStatusMessage`; ignore the timeout if in search/jump state.

2. **Help view layout**: The existing `helpView()` uses hardcoded manual two-column alignment with exact character spacing. Adding new entries (search, jump keys) requires matching this spacing exactly. Consider whether a third column is needed or if the existing two-column layout can accommodate the new entries.

## Files Changed Summary

| File | Phase | Change |
|------|-------|--------|
| `utils/utils_test.go` (new) | 1.1 | Tests for utility functions |
| `ui/markdown_test.go` (new) | 1.2 | Tests for markdown type |
| `ui/sort_test.go` (new) | 1.3 | Tests for sort |
| `ui/pager_test.go` (new) | 1.4, 2, 3, 4, 5 | Tests for rendering, page indicator, search, jumps |
| `ui/stash_test.go` (new) | 1.5 | Tests for filtering and cursor |
| **`ui/ui.go`** | **3** | **Key interception routing — `inInputMode()` gate for esc, q, h, left, delete** |
| `ui/pager.go` | 2, 3, 4, 5 | `inInputMode()`, page indicator, search, line jump, % jump |
| `ui/styles.go` | 3 | Match counter style (optional) |
| `ui/keys.go` | 3, 4 | New key constants (optional) |
