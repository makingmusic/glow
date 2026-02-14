# Test Gaps

## 1. Missing flag tests in `glow_test.go`

The existing flag parsing tests only cover `-p`, `-s`, `-w`. Add tests for `-l` (line numbers), `-t` (tui), `-a` (all files), `-n` (preserve newlines).

## 2. `--version` flag e2e test

No e2e test for `glow --version`. Should verify it exits 0 and outputs a version string.

## 3. Combined flags e2e tests

Existing e2e tests exercise flags one at a time. Add tests for combinations like:
- `glow -s dark -w 40 file.md`
- `glow -l -w 60 file.md`

## 4. Non-markdown file rendering e2e test

Glow renders code files wrapped in a fenced code block via `utils.WrapCodeBlock`. No e2e test exercises `glow somefile.go` or similar. Should verify output is non-empty and contains the file content.

## 5. `sourceFromArg` unit tests

`sourceFromArg()` in `main.go` has several branches (stdin `-`, HTTP URL, directory walk for README, plain file) but zero unit tests. The directory-walk-for-README logic in particular is completely untested at the unit level.

Suggested cases:
- Plain file path returns reader + absolute URL
- `-` returns stdin reader
- Directory containing a README.md returns that file
- Directory with no README returns error
- Nonexistent file returns error

## 6. `validateOptions` unit tests

The following logic in `validateOptions()` is untested:
- `--pager` + `--tui` mutual exclusion returns error
- Non-terminal stdout falls back to `notty` style
- Terminal width detection and the 120-column cap
