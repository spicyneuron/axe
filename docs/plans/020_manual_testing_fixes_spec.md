# 020 — Manual Testing Fixes Spec

Status: **Draft**
Depends on: M1-M8 (all complete)

---

## Goal

Fix two issues found during manual testing:

1. **Bad UX:** Commands that require an argument (`run`, `agents show`, `agents init`, `agents edit`) print `"accepts 1 arg(s), received 0"` with no hint about what argument is needed or how to use the command.
2. **Bug:** The `skill` field in agent TOML does not resolve intuitive short names. `skill = "yti"` fails with `"is a directory"` instead of resolving to `skills/yti/SKILL.md`.

A third item from the manual testing doc ("Multi-Turn Needed?") was investigated and found to be **not a code bug** — the multi-turn conversation loop is correctly implemented (`cmd/run.go:301-366`). The observed behavior (LLM describing tool calls as text) is caused by the agent TOML having no `tools = [...]` configured, which routes through the single-shot code path (`cmd/run.go:282`). No code change is required for this item.

---

## Non-Goals

- No changes to the multi-turn conversation loop.
- No changes to the provider implementations.
- No changes to tool registration or dispatch.
- No new CLI flags.
- No new external dependencies.
- No changes to `SilenceUsage: true` — stdout must remain safe to pipe per AGENTS.md design philosophy.

---

## Scope

Two work areas:

1. **Helpful argument validation errors** — Replace `cobra.ExactArgs(1)` with a custom validator that produces actionable error messages.
2. **Smart skill path resolution** — Enhance `resolve.Skill()` with a fallback chain so bare skill names and directory paths auto-resolve to `SKILL.md`.

---

## 1. Helpful Argument Validation Errors

### Current State

Four commands use `cobra.ExactArgs(1)`:

| Command | File | Line | `Use` string |
|---------|------|------|-------------|
| `run` | `cmd/run.go` | 35 | `"run"` (missing `<agent>` placeholder) |
| `agents show` | `cmd/agents.go` | 51 | `"show <agent>"` |
| `agents init` | `cmd/agents.go` | 117 | `"init <agent>"` |
| `agents edit` | `cmd/agents.go` | 154 | `"edit <agent>"` |

The root command sets `SilenceErrors: true` and `SilenceUsage: true` (`cmd/root.go:24-25`). When `ExactArgs` fails, cobra returns the error to `Execute()` which prints only the raw error string to stderr (`cmd/root.go:39-41`). Users see:

```
accepts 1 arg(s), received 0
```

This violates the AGENTS.md principle: "Errors that help the user fix the problem, not just describe it."

### Required Changes

#### 1a. Fix `run` command `Use` string

Change `Use: "run"` to `Use: "run <agent>"` in `cmd/run.go:30`.

#### 1b. Create a custom argument validator

Create a function `exactArgs` in a new file `cmd/args.go` with this signature:

```
func exactArgs(n int, usage string) cobra.PositionalArgs
```

This function returns a `cobra.PositionalArgs` function. When the number of arguments does not match `n`, it returns an error with this exact format:

**Too few arguments (0 provided, 1 expected):**
```
missing required argument: <agent>
Usage: axe <usage>
```

Where `<agent>` is extracted from the `usage` string — specifically, the first `<...>` token found in the usage string. If no `<...>` token is found, use `"argument"` as the fallback.

**Too many arguments (e.g. 3 provided, 1 expected):**
```
expected 1 argument, got 3
Usage: axe <usage>
```

**Correct number of arguments:** Return `nil` (no error).

#### 1c. Apply to all four commands

Replace `cobra.ExactArgs(1)` with `exactArgs(1, "<use-string>")` for:

| Command | `exactArgs` call |
|---------|-----------------|
| `run` | `exactArgs(1, "run <agent>")` |
| `agents show` | `exactArgs(1, "show <agent>")` |
| `agents init` | `exactArgs(1, "init <agent>")` |
| `agents edit` | `exactArgs(1, "edit <agent>")` |

### Error Message Format Rules

- The error message is a single string containing embedded newlines. Cobra's `SilenceUsage: true` remains — the `Usage:` line is part of the error string itself, not cobra's built-in usage output.
- The `Usage:` line includes the full subcommand path. For `agents show`, the usage line is `Usage: axe agents show <agent>`. The `usage` parameter to `exactArgs` is only the leaf portion (e.g. `"show <agent>"`); the function must prepend the parent command path. To achieve this, the function uses `cmd.CommandPath()` which cobra provides (returns e.g. `"axe agents show"`). The function signature changes to accept the `<arg-placeholder>` instead of the full usage string:

Revised function: The `cobra.PositionalArgs` signature is `func(cmd *cobra.Command, args []string) error`. The validator has access to `cmd.UseLine()` (which returns the `Use` string with the full parent path). So the actual approach is:

```
func exactArgs(n int) cobra.PositionalArgs
```

The function extracts the argument placeholder(s) from `cmd.Use` (e.g. from `"run <agent>"` it extracts `"<agent>"`). The full usage line uses `cmd.CommandPath()` + the argument portion of `cmd.Use`.

This is simpler — no extra parameter beyond `n`. The `Use` field on each command already contains the information needed.

### Revised Error Formats

**Too few arguments:**
```
missing required argument: <agent>
Usage: axe run <agent>
```

**Too many arguments:**
```
expected 1 argument, got 3
Usage: axe run <agent>
```

### Edge Cases

- If `cmd.Use` has no `<...>` token (defensive case), use `"argument"` as the placeholder name.
- If `n` is 0 (no arguments expected) and arguments are provided: `"expected no arguments, got 3\nUsage: axe version"`.
- The function only needs to handle `n = 0` and `n = 1` for the current codebase. It does not need to handle arbitrary `n > 1`.

### Affected Files

| File | Change |
|------|--------|
| `cmd/args.go` | New file: `exactArgs(n int) cobra.PositionalArgs` |
| `cmd/run.go` | Change `Use: "run"` to `Use: "run <agent>"`; replace `cobra.ExactArgs(1)` with `exactArgs(1)` |
| `cmd/agents.go` | Replace `cobra.ExactArgs(1)` with `exactArgs(1)` on `agentsShowCmd`, `agentsInitCmd`, `agentsEditCmd` |

### Tests

All tests go in `cmd/args_test.go`.

#### Test: `TestExactArgs_MissingArg_IncludesUsageHint`

Create a minimal `cobra.Command` with `Use: "test <thing>"`. Call the validator returned by `exactArgs(1)` with 0 args. Assert:
- Error is not nil.
- Error string contains `"missing required argument: <thing>"`.
- Error string contains `"Usage:"`.

#### Test: `TestExactArgs_TooManyArgs_IncludesUsageHint`

Create a minimal `cobra.Command` with `Use: "test <thing>"`. Call the validator with 3 args. Assert:
- Error is not nil.
- Error string contains `"expected 1 argument, got 3"`.
- Error string contains `"Usage:"`.

#### Test: `TestExactArgs_CorrectArgs_NoError`

Create a minimal `cobra.Command` with `Use: "test <thing>"`. Call the validator with 1 arg. Assert:
- Error is nil.

#### Test: `TestExactArgs_NoPlaceholder_FallbackName`

Create a minimal `cobra.Command` with `Use: "test"` (no `<...>` token). Call the validator returned by `exactArgs(1)` with 0 args. Assert:
- Error string contains `"missing required argument: argument"`.

#### Test: `TestExactArgs_RunCommand_MissingArg`

Run through `rootCmd` end-to-end. Set `rootCmd.SetArgs([]string{"run"})`. Execute. Assert:
- Error is not nil.
- Error string contains `"missing required argument: <agent>"`.
- Error string contains `"Usage:"`.

#### Test: `TestExactArgs_AgentsShowCommand_MissingArg`

Run through `rootCmd` end-to-end. Set `rootCmd.SetArgs([]string{"agents", "show"})`. Execute. Assert:
- Error is not nil.
- Error string contains `"missing required argument: <agent>"`.
- Error string contains `"Usage:"`.

### Existing Test Updates

The following existing tests assert on arg validation errors. They must be updated to check for the new error format instead of the old `"accepts 1 arg(s)"` format:

| Test | File | Current assertion | New assertion |
|------|------|-------------------|---------------|
| `TestRunE_ErrorNotPrintedByCobra` | `cmd/root_test.go:105-122` | Checks stderr does not contain `"accepts 1 arg(s)"` | Check stderr does not contain the new error message (because `SilenceErrors: true` means cobra itself does not write to stderr — only `Execute()` in `root.go:40` does) |
| `TestAgentsShow_NoArgs` | `cmd/agents_test.go:339-350` | Only checks `err != nil` | Additionally check `err.Error()` contains `"missing required argument: <agent>"` |
| `TestAgentsInit_NoArgs` | `cmd/agents_test.go:446-457` | Only checks `err != nil` | Additionally check `err.Error()` contains `"missing required argument: <agent>"` |
| `TestAgentsEdit_NoArgs` | `cmd/agents_test.go:500-511` | Only checks `err != nil` | Additionally check `err.Error()` contains `"missing required argument: <agent>"` |

Note: `TestRunE_ErrorNotPrintedByCobra` (`cmd/root_test.go:105-122`) specifically tests that with `SilenceErrors: true`, cobra does NOT write the error to stderr itself. The assertion checks that stderr does not contain the error string. This test's assertion on line 119 checks for `"accepts 1 arg(s)"` — update the check to look for the new error substring `"missing required argument"`. The test logic (verifying cobra silence behavior) is unchanged.

`TestRunE_ErrorDoesNotPrintUsage` (`cmd/root_test.go:65-103`) tests that stderr does NOT contain `"Usage:"` or `"Available Commands:"`. Since the new error message embeds `"Usage:"` within the error string itself, and `Execute()` prints the error to stderr, this test WILL now find `"Usage:"` in stderr for the `args_validation_error` case. This is acceptable behavior — the embedded usage hint is not cobra's usage dump (no `"Available Commands:"` block). Update this test's `args_validation_error` case to no longer assert that stderr lacks `"Usage:"`, but keep the assertion that stderr lacks `"Available Commands:"`.

---

## 2. Smart Skill Path Resolution

### Current State

`resolve.Skill()` in `internal/resolve/resolve.go:317-339` handles skill paths with this logic:

1. If `skillPath` is empty, return empty string.
2. If `skillPath` is not absolute, join it with `configDir`.
3. If the resolved path does not exist (`os.Stat` returns `os.IsNotExist`), return `"skill not found"` error.
4. Call `os.ReadFile(resolved)` — fails with `"is a directory"` if the path is a directory.

The `config init` command (`cmd/config.go:57-64`) establishes the convention: `$XDG_CONFIG_HOME/axe/skills/<skill-name>/SKILL.md`. Users are expected to write `skill = "skills/yti/SKILL.md"` in their TOML, but this is unintuitive. Users naturally try `skill = "yti"` or `skill = "skills/yti"`.

### Required Change

Rewrite `resolve.Skill()` to use a fallback chain that resolves the skill path to a readable file. The function signature does not change: `func Skill(skillPath, configDir string) (string, error)`.

### Resolution Fallback Chain

Given `skillPath` and `configDir`, try these paths in order. Return the content of the first path that exists **and is a regular file**:

1. **Direct resolution** (existing behavior for absolute or relative paths):
   - If `skillPath` is absolute, `resolved = skillPath`.
   - If `skillPath` is relative, `resolved = filepath.Join(configDir, skillPath)`.
   - If `resolved` is a regular file: read and return its content.

2. **Directory with SKILL.md** (handles `skill = "skills/yti"` or absolute directory paths):
   - If `resolved` (from step 1) is a directory: try `filepath.Join(resolved, "SKILL.md")`.
   - If that file exists and is a regular file: read and return its content.

3. **Bare name in skills directory** (handles `skill = "yti"`):
   - If `skillPath` contains no path separators (`/` or `filepath.Separator`): try `filepath.Join(configDir, "skills", skillPath, "SKILL.md")`.
   - If that file exists and is a regular file: read and return its content.

4. **Not found**: Return an error.

### Error Format

When no resolution succeeds, return an error listing what was tried. The error format:

```
skill not found: tried <path1>, <path2>, ...
```

Where `<pathN>` are the absolute paths that were checked. Only include paths that were actually tried (not all 3 if a bare name check was skipped because the path contains separators).

### Edge Cases

| Input | Behavior |
|-------|----------|
| `skillPath = ""` | Return `("", nil)` immediately. No resolution attempted. |
| `skillPath = "/abs/path/to/SKILL.md"` | Step 1 succeeds (file). |
| `skillPath = "/abs/path/to/skill-dir"` | Step 1 fails (directory). Step 2 tries `/abs/path/to/skill-dir/SKILL.md`. |
| `skillPath = "skills/review/SKILL.md"` | Step 1 succeeds (relative file path joined with configDir). |
| `skillPath = "skills/review"` | Step 1 fails (directory). Step 2 tries `configDir/skills/review/SKILL.md`. |
| `skillPath = "yti"` | Step 1 fails (either `configDir/yti` doesn't exist or is a directory). Step 2 tries `configDir/yti/SKILL.md` (fails if configDir/yti doesn't exist). Step 3 tries `configDir/skills/yti/SKILL.md`. |
| `skillPath = "nonexistent"` | All steps fail. Error lists all tried paths. |
| `skillPath = "yti"` where `configDir/yti` is a file | Step 1 succeeds — reads the file `configDir/yti`. The bare-name-to-skills-dir fallback is never reached. |
| `skillPath = "skills/yti"` where `configDir/skills/yti/SKILL.md` does not exist | Step 2 tries `SKILL.md` in the directory. Fails if missing. Step 3 is skipped (path contains `/`). Error reports tried paths. |

### Detecting Bare Names

A "bare name" is a `skillPath` that contains no `/` characters and no `filepath.Separator` characters. On Unix, these are the same; on Windows, `filepath.Separator` is `\`. Use `!strings.Contains(skillPath, "/") && !strings.Contains(skillPath, string(filepath.Separator))` for the check.

### Detecting Files vs. Directories

Use `os.Stat()` on the resolved path. If `err == nil` and `info.IsDir()` is false, it is a regular file. If `info.IsDir()` is true, it is a directory. If `os.IsNotExist(err)`, the path does not exist.

### Affected Files

| File | Change |
|------|--------|
| `internal/resolve/resolve.go` | Rewrite `Skill()` function with fallback chain |

No call-site changes — the function signature is unchanged.

### Tests

All tests go in `internal/resolve/resolve_test.go`.

#### Test: `TestSkill_DirectoryAutoResolvesSKILLMD`

Setup:
- Create `configDir/skills/review/` directory.
- Write `configDir/skills/review/SKILL.md` with content `"review skill content"`.

Call `Skill("skills/review", configDir)`.

Assert:
- No error.
- Result equals `"review skill content"`.

#### Test: `TestSkill_BareNameResolvesToSkillsDir`

Setup:
- Create `configDir/skills/yti/` directory.
- Write `configDir/skills/yti/SKILL.md` with content `"yti skill content"`.

Call `Skill("yti", configDir)`.

Assert:
- No error.
- Result equals `"yti skill content"`.

#### Test: `TestSkill_AbsoluteDirectoryAutoResolvesSKILLMD`

Setup:
- Create a temp directory `dir/my-skill/`.
- Write `dir/my-skill/SKILL.md` with content `"abs dir skill"`.

Call `Skill("<absolute-path-to-dir/my-skill>", "/irrelevant")`.

Assert:
- No error.
- Result equals `"abs dir skill"`.

#### Test: `TestSkill_BareNameNotFound`

Setup:
- Create an empty `configDir` (no skills directory).

Call `Skill("nonexistent", configDir)`.

Assert:
- Error is not nil.
- Error string contains `"skill not found"`.

#### Test: `TestSkill_BareNameFileInConfigDir`

Setup:
- Write a regular file `configDir/myskill` with content `"direct file"`.

Call `Skill("myskill", configDir)`.

Assert:
- No error.
- Result equals `"direct file"`.

This verifies that step 1 (direct resolution) takes priority over step 3 (bare name in skills dir), so existing configs that use non-standard file names at the config root still work.

### Existing Tests That Must Still Pass

| Test | Current Input | Expected Behavior |
|------|--------------|-------------------|
| `TestSkill_EmptyPath` | `""` | Returns `("", nil)` |
| `TestSkill_AbsolutePath` | Absolute file path | Returns file content |
| `TestSkill_RelativePath` | `"skills/test.md"` | Returns file content (step 1 — relative file) |
| `TestSkill_NotFound` | `"/nonexistent/SKILL.md"` | Returns error. Note: the error message format changes from `"skill not found: /nonexistent/SKILL.md"` to `"skill not found: tried /nonexistent/SKILL.md"`. Update this test's expected error string. |

---

## 3. Multi-Turn Issue — No Code Change

### Investigation Summary

The multi-turn conversation loop in `cmd/run.go:301-366` is correctly implemented:

- When `len(req.Tools) > 0`, the code enters a `for` loop (up to `maxConversationTurns = 50`).
- Each iteration calls `prov.Send()`, checks for tool calls in the response, executes them via `executeToolCalls()`, appends results to the message history, and loops.
- The loop breaks when the LLM returns a response with no tool calls (`len(resp.ToolCalls) == 0`).

When `len(req.Tools) == 0` (line 282), the code takes a single-shot path — one `prov.Send()` call with no loop. Since no tool definitions are sent to the LLM, the model has no formal tool schema and can only describe tool usage as prose text.

### Root Cause

The `yti` agent TOML did not include `tools = [...]`. Without tool definitions, the single-shot path is used.

### Resolution

No code change. The user should add `tools = ["run_command"]` (or whichever tools the agent needs) to the agent's TOML configuration.

---

## Acceptance Criteria

1. Running `axe run` (no args) prints an error containing `"missing required argument: <agent>"` and a `"Usage:"` line.
2. Running `axe agents show` (no args) prints an error containing `"missing required argument: <agent>"` and a `"Usage:"` line.
3. Running `axe agents init` (no args) prints an error containing `"missing required argument: <agent>"` and a `"Usage:"` line.
4. Running `axe agents edit` (no args) prints an error containing `"missing required argument: <agent>"` and a `"Usage:"` line.
5. An agent TOML with `skill = "yti"` resolves to `$XDG_CONFIG_HOME/axe/skills/yti/SKILL.md` and loads the skill content.
6. An agent TOML with `skill = "skills/yti"` resolves to `$XDG_CONFIG_HOME/axe/skills/yti/SKILL.md` and loads the skill content.
7. An agent TOML with `skill = "skills/yti/SKILL.md"` continues to work (no regression).
8. Absolute skill file paths continue to work (no regression).
9. `make test` passes with zero failures.

---

## Test Summary

| # | Test Name | File | Section |
|---|-----------|------|---------|
| 1 | `TestExactArgs_MissingArg_IncludesUsageHint` | `cmd/args_test.go` | 1 |
| 2 | `TestExactArgs_TooManyArgs_IncludesUsageHint` | `cmd/args_test.go` | 1 |
| 3 | `TestExactArgs_CorrectArgs_NoError` | `cmd/args_test.go` | 1 |
| 4 | `TestExactArgs_NoPlaceholder_FallbackName` | `cmd/args_test.go` | 1 |
| 5 | `TestExactArgs_RunCommand_MissingArg` | `cmd/args_test.go` | 1 |
| 6 | `TestExactArgs_AgentsShowCommand_MissingArg` | `cmd/args_test.go` | 1 |
| 7 | `TestSkill_DirectoryAutoResolvesSKILLMD` | `internal/resolve/resolve_test.go` | 2 |
| 8 | `TestSkill_BareNameResolvesToSkillsDir` | `internal/resolve/resolve_test.go` | 2 |
| 9 | `TestSkill_AbsoluteDirectoryAutoResolvesSKILLMD` | `internal/resolve/resolve_test.go` | 2 |
| 10 | `TestSkill_BareNameNotFound` | `internal/resolve/resolve_test.go` | 2 |
| 11 | `TestSkill_BareNameFileInConfigDir` | `internal/resolve/resolve_test.go` | 2 |

---

## File Change Summary

| File | Type | Description |
|------|------|-------------|
| `cmd/args.go` | Create | `exactArgs(n int) cobra.PositionalArgs` helper |
| `cmd/args_test.go` | Create | Tests for `exactArgs` |
| `cmd/run.go` | Modify | Fix `Use` string; replace `cobra.ExactArgs(1)` |
| `cmd/agents.go` | Modify | Replace `cobra.ExactArgs(1)` on 3 commands |
| `cmd/root_test.go` | Modify | Update arg validation error assertions |
| `cmd/agents_test.go` | Modify | Add error message assertions to `NoArgs` tests |
| `internal/resolve/resolve.go` | Modify | Rewrite `Skill()` with fallback chain |
| `internal/resolve/resolve_test.go` | Modify | Add 5 new skill resolution tests; update `TestSkill_NotFound` error string |

---

## Constraints

- No new external dependencies. All changes use Go stdlib, cobra, and existing project code.
- `SilenceUsage: true` and `SilenceErrors: true` remain on the root command. The `Usage:` hint is embedded in the error string, not printed by cobra's usage system.
- Stdout remains clean and pipeable — errors go to stderr only.
- The `resolve.Skill()` function signature does not change.
- Tests call real code — no mocks.
- All test helpers call `t.Helper()`.
- `make test` must pass with zero failures after all changes.
