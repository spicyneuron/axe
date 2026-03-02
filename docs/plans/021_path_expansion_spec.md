# 021 — Path Expansion Spec

Status: **Draft**
Depends on: 020 (manual testing fixes — skill resolution fallback chain)

---

## Goal

Support `~` (tilde) and `$VAR` (environment variable) expansion in all
user-supplied paths throughout axe. Users should be able to write:

```toml
workdir = "~/projects/myapp"
skill = "$HOME/.config/axe/skills/review/SKILL.md"
files = ["~/notes/*.md"]
```

and in SKILL.md:

```bash
$HOME/.config/axe/skills/yti/scripts/get_transcript.sh <url>
```

without paths failing due to Go treating `~` and `$HOME` as literal strings.

---

## Background

Go does not expand `~` or environment variables in path strings. Tilde
expansion is a shell feature (handled by `sh`, `bash`, etc.), and `$VAR`
expansion requires an explicit call to `os.ExpandEnv()`. Today, every
user-supplied path in axe — TOML fields, CLI flags, and memory config — is
used as a raw literal string with zero transformation.

The single exception is the `run_command` tool: commands are passed to
`sh -c`, so the shell natively expands `~` and `$HOME` within the command
string. No change is needed there.

---

## Non-Goals

- No changes to the `run_command` tool. `sh -c` already handles expansion.
- No changes to file tool path arguments (`read_file`, `write_file`,
  `edit_file`, `list_directory`). These accept relative paths within the
  workdir and reject absolute paths. The workdir itself is expanded upstream
  before being passed to these tools.
- No expansion of `~user` (other users' home directories). Only `~/` and
  bare `~` are expanded.
- No custom variable syntax. Only standard POSIX `$VAR` and `${VAR}` forms
  are supported (whatever `os.ExpandEnv` handles).
- No recursive expansion (expanding a variable whose value contains another
  variable reference).
- No new CLI flags.
- No new external dependencies.

---

## Scope

1. Create an `ExpandPath` utility function.
2. Apply it at every entry point where user-supplied paths are consumed.

---

## 1. ExpandPath Function

### Location

`internal/resolve/resolve.go` — add to the existing resolve package. No new
files needed.

### Signature

```
func ExpandPath(path string) (string, error)
```

### Behavior

The function performs two transformations in order:

1. **Tilde expansion**: If `path` starts with `~/`, replace the leading `~`
   with the current user's home directory (`os.UserHomeDir()`). If `path` is
   exactly `~`, replace the entire string with the home directory. All other
   forms (e.g. `~user/foo`, `foo/~/bar`) are left unchanged.

2. **Environment variable expansion**: Call `os.ExpandEnv(path)` on the
   result from step 1. This replaces `$VAR` and `${VAR}` with their values
   from the process environment.

### Return Values

- On success: the expanded path string and `nil` error.
- If `os.UserHomeDir()` fails during tilde expansion: return `""` and the
  error wrapped with context (e.g. `"failed to expand ~: <underlying error>"`).
- If the path does not start with `~`: `os.UserHomeDir()` is never called,
  so that error path is not possible.

### Edge Cases

| Input | Output | Notes |
|-------|--------|-------|
| `""` | `""` | Empty string returned as-is, no error |
| `"~/foo/bar"` | `"/home/user/foo/bar"` | Tilde replaced with home dir |
| `"~"` | `"/home/user"` | Bare tilde replaced with home dir |
| `"$HOME/foo"` | `"/home/user/foo"` | Env var expanded |
| `"${HOME}/foo"` | `"/home/user/foo"` | Braced env var expanded |
| `"~/foo/$PROJECT"` | `"/home/user/foo/myapp"` | Both tilde and env var expanded |
| `"/absolute/path"` | `"/absolute/path"` | No transformation |
| `"relative/path"` | `"relative/path"` | No transformation |
| `"~user/foo"` | `"~user/foo"` | Not expanded (other user's home) |
| `"foo/~/bar"` | `"foo/~/bar"` | Tilde not at start — not expanded |
| `"$UNSET_VAR/foo"` | `"/foo"` | Unset env vars expand to empty string (os.ExpandEnv behavior) |
| `"no-vars-here"` | `"no-vars-here"` | No transformation |

### Order of Operations

Tilde expansion happens **before** environment variable expansion. This means
`~/$PROJECT` correctly expands `~` first, then `$PROJECT`. If the order were
reversed, `$HOME` could be used but `~` would be processed after env var
expansion, which is the standard POSIX shell order.

---

## 2. Call Sites

Apply `ExpandPath` at these locations. Each call site must handle the
returned error appropriately.

### 2a. `resolve.Workdir()` — `internal/resolve/resolve.go:14`

**Current signature:** `func Workdir(flagValue, tomlValue string) string`

**Change:** The function must call `ExpandPath` on the selected path before
returning it. Because `ExpandPath` can return an error (from
`os.UserHomeDir`), the function signature must change:

**New signature:** `func Workdir(flagValue, tomlValue string) (string, error)`

**Logic:**

1. Determine the raw path using existing priority chain: `flagValue` >
   `tomlValue` > `os.Getwd()` > `"."`.
2. Call `ExpandPath(rawPath)` on the result.
3. Return the expanded path or the error.

The `os.Getwd()` and `"."` fallback values do not contain `~` or `$VAR`, so
`ExpandPath` is a no-op for those cases. But calling it unconditionally keeps
the logic simple.

**Caller updates:** Three call sites must handle the new error return:

1. `cmd/run.go:104`:
```go
workdir, err := resolve.Workdir(flagWorkdir, cfg.Workdir)
if err != nil {
    return &ExitError{Code: 2, Err: err}
}
```

2. `internal/tool/tool.go:142` (`ExecuteCallAgent`):
```go
workdir, err := resolve.Workdir("", cfg.Workdir)
if err != nil {
    return errorResult(call.ID, agentName, fmt.Sprintf("failed to resolve workdir for agent %q: %s", agentName, err), opts)
}
```

3. `internal/tool/tool.go:302` (`runConversationLoop`):
```go
toolWorkdir, err := resolve.Workdir("", cfg.Workdir)
if err != nil {
    return nil, fmt.Errorf("failed to resolve workdir: %w", err)
}
```

### 2b. `resolve.Skill()` — `internal/resolve/resolve.go:326`

**Current:** `skillPath` is used directly. If not absolute, it is joined with
`configDir`.

**Change:** Call `ExpandPath(skillPath)` at the top of the function, before
the `filepath.IsAbs` check. Replace `skillPath` with the expanded value for
all subsequent resolution steps.

If `ExpandPath` returns an error, `Skill` returns `("", error)`.

This means `skill = "~/skills/review/SKILL.md"` correctly expands to an
absolute path, which then passes the `filepath.IsAbs` check and is used
directly (step 1 of the fallback chain).

### 2c. `resolve.Files()` — `internal/resolve/resolve.go:36`

**Current:** Each pattern from the TOML `files` array is used as a raw glob
pattern, joined with the workdir.

**Change:** Call `ExpandPath` on each pattern before passing it to
`simpleGlob` or `doubleStarGlob`. This happens inside the `for _, pattern
:= range patterns` loop (line 56), before the `strings.Contains(pattern,
"**")` check.

If `ExpandPath` returns an error on any pattern, `Files` returns the error
immediately.

**Note:** After expansion, a pattern like `~/docs/*.md` becomes
`/home/user/docs/*.md`. This is an absolute path pattern. When joined with
`absWorkdir` via `filepath.Join`, it produces a valid absolute glob. However,
the path traversal check (`strings.HasPrefix(relPath, "..")`) will reject
matches that fall outside the workdir. This is the correct and expected
behavior — expanded absolute patterns that point outside the workdir are
blocked by the existing security checks.

### 2d. `memory.FilePath()` — `internal/memory/memory.go:21`

**Current:** `customPath` is returned as-is when non-empty.

**Change:** When `customPath` is non-empty, call `resolve.ExpandPath` on it
before returning. This requires importing `github.com/jrswab/axe/internal/resolve`.

If `ExpandPath` returns an error, `FilePath` returns `("", error)`.

The default path (from XDG) does not need expansion — `xdg.GetDataDir()`
already resolves to an absolute path.

---

## 3. Affected Files

| File | Change |
|------|--------|
| `internal/resolve/resolve.go` | Add `ExpandPath` function; update `Workdir` signature to return error; call `ExpandPath` in `Workdir`, `Skill`, and `Files` |
| `internal/resolve/resolve_test.go` | Add `ExpandPath` tests; update `Workdir` tests for new error return |
| `internal/memory/memory.go` | Call `resolve.ExpandPath` on `customPath` in `FilePath` |
| `internal/memory/memory_test.go` | Add test for tilde in `customPath` |
| `cmd/run.go` | Update `Workdir` call site to handle error return |
| `internal/tool/tool.go` | Update 2 `Workdir` call sites to handle error return (lines 142, 302) |

---

## 4. Tests

All tests call real code — no mocks. Table-driven where appropriate.

### 4a. `ExpandPath` Tests — `internal/resolve/resolve_test.go`

#### Test: `TestExpandPath`

Table-driven test with the following cases:

| Name | Input | Setup | Expected Output | Error? |
|------|-------|-------|----------------|--------|
| `empty` | `""` | — | `""` | no |
| `tilde_slash` | `"~/foo/bar"` | — | `<home>/foo/bar` | no |
| `bare_tilde` | `"~"` | — | `<home>` | no |
| `env_var` | `"$HOME/foo"` | — | `<home>/foo` | no |
| `braced_env_var` | `"${HOME}/foo"` | — | `<home>/foo` | no |
| `tilde_and_env` | `"~/$TEST_AXE_VAR"` | `TEST_AXE_VAR=proj` | `<home>/proj` | no |
| `absolute` | `"/abs/path"` | — | `"/abs/path"` | no |
| `relative` | `"rel/path"` | — | `"rel/path"` | no |
| `tilde_user` | `"~user/foo"` | — | `"~user/foo"` | no |
| `mid_tilde` | `"foo/~/bar"` | — | `"foo/~/bar"` | no |
| `unset_var` | `"$AXE_UNSET_VAR_TEST/foo"` | — | `"/foo"` | no |
| `no_vars` | `"just-a-name"` | — | `"just-a-name"` | no |

Where `<home>` is obtained by calling `os.UserHomeDir()` at the start of
the test. If `os.UserHomeDir()` fails (unlikely in test environments), skip
the test.

For the `tilde_and_env` case, set `TEST_AXE_VAR` via `t.Setenv("TEST_AXE_VAR", "proj")`.

For the `unset_var` case, ensure `AXE_UNSET_VAR_TEST` is not set (it should
not be by default; if paranoid, call `t.Setenv("AXE_UNSET_VAR_TEST", "")`
to explicitly clear it — but note `os.ExpandEnv` treats unset and empty the
same).

### 4b. Updated `Workdir` Tests — `internal/resolve/resolve_test.go`

The existing three `Workdir` tests must be updated to handle the new
`(string, error)` return:

#### `TestWorkdir_FlagOverride`

Change: `result := Workdir(...)` → `result, err := Workdir(...)`. Assert
`err == nil`. Expected `result` unchanged.

#### `TestWorkdir_TOMLFallback`

Same update.

#### `TestWorkdir_CWDFallback`

Same update.

#### New: `TestWorkdir_TildeExpansion`

Call `Workdir("", "~/projects")`. Assert:
- `err == nil`
- `result == <home>/projects` (where `<home>` is from `os.UserHomeDir()`)

#### New: `TestWorkdir_EnvVarExpansion`

Set `t.Setenv("TEST_AXE_WORKDIR", "/tmp/testwd")`. Call
`Workdir("", "$TEST_AXE_WORKDIR")`. Assert:
- `err == nil`
- `result == "/tmp/testwd"`

### 4c. Skill Expansion Test — `internal/resolve/resolve_test.go`

#### New: `TestSkill_TildeInPath`

Setup:
- Get `home` from `os.UserHomeDir()`.
- Create `<home>/.axe-test-skill-<random>/SKILL.md` with content
  `"tilde skill"` in a temp location. Actually, to avoid writing to the
  user's real home dir, use a different approach:

Instead, set a temp dir as a known env var and use `$VAR` expansion:

- `t.Setenv("AXE_TEST_SKILL_DIR", t.TempDir())`
- Create `$AXE_TEST_SKILL_DIR/SKILL.md` with content `"expanded skill"`.
- Call `Skill("$AXE_TEST_SKILL_DIR/SKILL.md", "/irrelevant")`.
- Assert: no error, result equals `"expanded skill"`.

This tests env var expansion in skill paths without touching the user's home
directory.

### 4d. Memory FilePath Test — `internal/memory/memory_test.go`

#### New: `TestFilePath_TildeExpansion`

Call `FilePath("agent", "~/my-memory.md")`. Assert:
- `err == nil`
- Result equals `<home>/my-memory.md` (where `<home>` is from
  `os.UserHomeDir()`).

#### New: `TestFilePath_EnvVarExpansion`

Set `t.Setenv("AXE_TEST_MEM_DIR", "/tmp/mem")`. Call
`FilePath("agent", "$AXE_TEST_MEM_DIR/notes.md")`. Assert:
- `err == nil`
- Result equals `"/tmp/mem/notes.md"`.

### 4e. Existing Tests That Must Still Pass

All existing tests in `internal/resolve/resolve_test.go` must continue to
pass. The `Workdir` tests require signature updates (error return). All
`Skill` tests are unaffected — `ExpandPath` on paths without `~` or `$VAR`
is a no-op. All `Files` tests are unaffected — test patterns like `"*.txt"`
and `"**/*.go"` contain no expansion triggers.

All existing tests in `internal/memory/memory_test.go` must continue to
pass. The `FilePath` function's behavior for empty `customPath` and default
XDG paths is unchanged.

---

## 5. Acceptance Criteria

1. `ExpandPath("~/foo")` returns `<home>/foo` and no error.
2. `ExpandPath("$HOME/foo")` returns `<home>/foo` and no error.
3. `ExpandPath("")` returns `""` and no error.
4. `ExpandPath("/abs/path")` returns `"/abs/path"` unchanged.
5. `ExpandPath("~user/foo")` returns `"~user/foo"` unchanged (no expansion).
6. `Workdir("", "~/projects")` returns `<home>/projects`.
7. `Skill("$VARNAME/SKILL.md", configDir)` resolves the env var and loads the file.
8. `memory.FilePath("agent", "~/mem.md")` returns `<home>/mem.md`.
9. `run_command` is not affected — `sh -c` continues to handle expansion.
10. `make test` passes with zero failures.

---

## 6. Constraints

- No new external dependencies. All changes use Go stdlib.
- `ExpandPath` is a pure function (aside from reading env vars and home dir).
  It does not touch the filesystem.
- The function must not panic. All error conditions return an error.
- Tests call real code — no mocks.
- All test helpers call `t.Helper()`.
- `make test` must pass with zero failures after all changes.

---

## 7. Test Summary

| # | Test Name | File | Section |
|---|-----------|------|---------|
| 1 | `TestExpandPath` (table-driven, 12 cases) | `internal/resolve/resolve_test.go` | 4a |
| 2 | `TestWorkdir_FlagOverride` (updated) | `internal/resolve/resolve_test.go` | 4b |
| 3 | `TestWorkdir_TOMLFallback` (updated) | `internal/resolve/resolve_test.go` | 4b |
| 4 | `TestWorkdir_CWDFallback` (updated) | `internal/resolve/resolve_test.go` | 4b |
| 5 | `TestWorkdir_TildeExpansion` (new) | `internal/resolve/resolve_test.go` | 4b |
| 6 | `TestWorkdir_EnvVarExpansion` (new) | `internal/resolve/resolve_test.go` | 4b |
| 7 | `TestSkill_TildeInPath` (new) | `internal/resolve/resolve_test.go` | 4c |
| 8 | `TestFilePath_TildeExpansion` (new) | `internal/memory/memory_test.go` | 4d |
| 9 | `TestFilePath_EnvVarExpansion` (new) | `internal/memory/memory_test.go` | 4d |

---

## 8. File Change Summary

| File | Type | Description |
|------|------|-------------|
| `internal/resolve/resolve.go` | Modify | Add `ExpandPath`; change `Workdir` to return `(string, error)`; call `ExpandPath` in `Workdir`, `Skill`, `Files` |
| `internal/resolve/resolve_test.go` | Modify | Add `TestExpandPath`; update 3 existing `Workdir` tests; add 4 new tests |
| `internal/memory/memory.go` | Modify | Call `resolve.ExpandPath` on `customPath` in `FilePath` |
| `internal/memory/memory_test.go` | Modify | Add 2 new tests for path expansion |
| `cmd/run.go` | Modify | Update `Workdir` call to handle error return |
| `internal/tool/tool.go` | Modify | Update 2 `Workdir` calls to handle error return (lines 142, 302) |
