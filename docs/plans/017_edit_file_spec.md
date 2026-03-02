# Specification: Tool Call M6 — `edit_file` Tool

**Status:** Draft
**Version:** 1.0
**Created:** 2026-03-02
**Scope:** Find-and-replace within existing files, with optional replace-all, path security via `validatePath`, and replacement count confirmation

---

## 1. Purpose

Implement the `edit_file` tool — a find-and-replace tool for existing files, registered in the `Registry`. This tool performs exact string replacement within a file relative to the agent's workdir. It supports single replacement (default) and replace-all mode.

This milestone builds on the infrastructure established in M3–M5:

- **`validatePath`** — reused directly (unlike `write_file` which uses inline validation). Since `edit_file` operates on files that must already exist, `validatePath`'s call to `filepath.EvalSymlinks` on the full path will succeed.
- **`isWithinDir`** — indirectly reused via `validatePath`
- **`RegisterAll`** — extended with one additional `r.Register(...)` call
- **`toolname.EditFile`** — constant already declared in `internal/toolname/toolname.go`

---

## 2. Design Decisions

The following decisions were made during planning and are binding for implementation:

1. **`validatePath` IS called directly.** Unlike `write_file`, the `edit_file` tool only operates on files that already exist (it reads, modifies, and writes back). The existing `validatePath` function handles all path security checks (empty, absolute, traversal, symlink escape) and returns the evaluated resolved path. This matches the `read_file` and `list_directory` pattern.

2. **Exact string matching only.** The `old_string` parameter is matched as a literal string, not a regular expression. `strings.Count` and `strings.Replace`/`strings.ReplaceAll` are used for matching and replacement.

3. **Single replacement is the default.** When `replace_all` is absent or `"false"`, only the first occurrence of `old_string` is replaced. If multiple occurrences exist and `replace_all` is not `"true"`, the tool returns an error prompting the user to either provide a more unique string or set `replace_all` to `"true"`.

4. **`old_string` must not be empty.** An empty `old_string` is rejected with an error. This prevents nonsensical replacements (every position in a string matches the empty string).

5. **`new_string` may be empty.** An empty `new_string` (or absent key) performs a deletion of the matched `old_string` text. This is a valid and useful operation.

6. **File is read, modified in memory, and written back.** The entire file is read into memory, the replacement is performed on the string, and the result is written back with `os.WriteFile`. No streaming or line-by-line processing.

7. **File permissions on write-back are `0o644`.** Consistent with `write_file`. The original file permissions are not preserved.

8. **Confirmation message includes replacement count and relative path.** On success, the tool returns `"replaced N occurrence(s) in <relPath>"` where `N` is the number of replacements performed and `<relPath>` is the original relative path from the arguments.

9. **`replace_all` is parsed with `strconv.ParseBool`.** This accepts `"true"`, `"false"`, `"1"`, `"0"`, `"t"`, `"f"`, `"T"`, `"F"`, `"TRUE"`, `"FALSE"` — all standard Go boolean string representations. An invalid value (e.g., `"banana"`) returns an error.

10. **`call_agent` remains outside the registry.** Same as M2–M5.

11. **No new external dependencies.** Uses only Go stdlib.

---

## 3. Requirements

### 3.1 `edit_file` Tool Definition

**Requirement 1.1:** Create a file `internal/tool/edit_file.go`.

**Requirement 1.2:** Define an unexported function `editFileEntry() ToolEntry` that returns a `ToolEntry` with both `Definition` and `Execute` set to non-nil functions.

**Requirement 1.3:** The tool definition returned by the `Definition` function must have:
- `Name`: the value of `toolname.EditFile` (i.e., `"edit_file"`)
- `Description`: a clear description for the LLM explaining the tool performs find-and-replace on an existing file relative to the working directory
- `Parameters`: four parameters:
  - `path`:
    - `Type`: `"string"`
    - `Description`: a description stating it is a relative path to the file to edit
    - `Required`: `true`
  - `old_string`:
    - `Type`: `"string"`
    - `Description`: a description stating it is the exact text to find in the file
    - `Required`: `true`
  - `new_string`:
    - `Type`: `"string"`
    - `Description`: a description stating it is the text to replace old_string with
    - `Required`: `true`
  - `replace_all`:
    - `Type`: `"string"`
    - `Description`: a description stating when set to "true", all occurrences are replaced; defaults to false (single replacement)
    - `Required`: `false`

**Requirement 1.4:** The tool name in the definition must use `toolname.EditFile`, not a hardcoded string.

**Requirement 1.5:** `ToolCall.Arguments` is `map[string]string`. All parameters are strings. `replace_all` is parsed from string to bool within the executor.

### 3.2 `edit_file` Tool Executor

**Requirement 2.1:** The `Execute` function must have the signature: `func(ctx context.Context, call provider.ToolCall, ec ExecContext) provider.ToolResult`.

**Requirement 2.2:** Extract the `path` argument from `call.Arguments["path"]`.

**Requirement 2.3 (Path Validation):** Call `validatePath(ec.Workdir, path)`. If `validatePath` returns an error, return a `ToolResult` with `CallID: call.ID`, `Content: err.Error()`, `IsError: true`. This handles all path security checks: empty path, absolute path, `..` traversal, nonexistent path, and symlink escape.

**Requirement 2.4 (Directory Rejection):** After `validatePath` succeeds, call `os.Stat` on the resolved path. If the path is a directory, return a `ToolResult` with `CallID: call.ID`, `Content: "path is a directory, not a file"`, `IsError: true`.

**Requirement 2.5 (Extract old_string):** Extract the `old_string` argument from `call.Arguments["old_string"]`. If `old_string` is an empty string (including when the key is absent from the map), return a `ToolResult` with `CallID: call.ID`, `Content: "old_string is required"`, `IsError: true`.

**Requirement 2.6 (Extract new_string):** Extract the `new_string` argument from `call.Arguments["new_string"]`. An empty string is valid (it means "delete the matched text"). A missing key is treated as an empty string.

**Requirement 2.7 (Read File):** Read the file contents using `os.ReadFile` on the resolved path. If `os.ReadFile` returns an error, return a `ToolResult` with `CallID: call.ID`, `Content: err.Error()`, `IsError: true`.

**Requirement 2.8 (Parse replace_all):** Extract the `replace_all` argument from `call.Arguments["replace_all"]`. If the value is empty or the key is absent, default to `false`. If the value is non-empty, parse it with `strconv.ParseBool`. If parsing fails, return a `ToolResult` with `CallID: call.ID`, `Content` in the format `"invalid replace_all value %q: %s"` (where the first value is the raw string and the second is the parse error message), `IsError: true`.

**Requirement 2.9 (Count Occurrences):** Count occurrences of `old_string` in the file content using `strings.Count(content, old_string)`.

**Requirement 2.10 (Not Found Error):** If the count is 0, return a `ToolResult` with `CallID: call.ID`, `Content: "old_string not found in file"`, `IsError: true`.

**Requirement 2.11 (Multiple Matches Without replace_all Error):** If the count is greater than 1 and `replace_all` is `false`, return a `ToolResult` with `CallID: call.ID`, `Content` in the format `"old_string found %d times; set replace_all to true or provide a more unique string"` (where the value is the count), `IsError: true`.

**Requirement 2.12 (Perform Replacement):** If `replace_all` is `true`, use `strings.ReplaceAll(content, old_string, new_string)`. If `replace_all` is `false` (and count is exactly 1), use `strings.Replace(content, old_string, new_string, 1)`.

**Requirement 2.13 (Write File):** Write the modified content to the resolved path using `os.WriteFile(resolved, []byte(newContent), 0o644)`. If `os.WriteFile` returns an error, return a `ToolResult` with `CallID: call.ID`, `Content: err.Error()`, `IsError: true`.

**Requirement 2.14 (Success Response):** Return a `ToolResult` with `CallID: call.ID`, `Content` in the format `"replaced %d occurrence(s) in %s"` (where the first value is the number of replacements performed and the second is the original `path` argument as provided by the caller), `IsError: false`.

**Requirement 2.15 (Replacement Count):** When `replace_all` is `true`, the count in the success message equals the value from `strings.Count`. When `replace_all` is `false`, the count is always `1`.

### 3.3 Registration in Registry

**Requirement 3.1:** Add `r.Register(toolname.EditFile, editFileEntry())` to the `RegisterAll` function in `internal/tool/registry.go`.

**Requirement 3.2:** This is the only change to `registry.go`. No call-site changes are needed in `cmd/run.go` or `internal/tool/tool.go`.

### 3.4 Registry Tests

**Requirement 4.1:** Add tests to `internal/tool/registry_test.go` that verify `edit_file` is registered by `RegisterAll`.

**Requirement 4.2:** Add a test that resolves `edit_file` and verifies the tool definition has the correct name and parameters (`path`, `old_string`, `new_string`, `replace_all`).

---

## 4. Project Structure

After completion, the following files will be added or modified:

```
axe/
├── internal/
│   └── tool/
│       ├── edit_file.go                # NEW: Definition, Execute, helper entry func
│       ├── edit_file_test.go           # NEW: tests
│       ├── registry.go                 # MODIFIED: add one line to RegisterAll
│       ├── registry_test.go            # MODIFIED: add edit_file registration tests
│       ├── write_file.go               # UNCHANGED
│       ├── write_file_test.go          # UNCHANGED
│       ├── read_file.go                # UNCHANGED
│       ├── read_file_test.go           # UNCHANGED
│       ├── list_directory.go           # UNCHANGED
│       ├── list_directory_test.go      # UNCHANGED
│       ├── path_validation.go          # UNCHANGED
│       ├── path_validation_test.go     # UNCHANGED
│       ├── tool.go                     # UNCHANGED
│       └── tool_test.go               # UNCHANGED
├── go.mod                              # UNCHANGED
├── go.sum                              # UNCHANGED
└── ...                                 # all other files UNCHANGED
```

---

## 5. Edge Cases

### 5.1 Path Validation (reuses `validatePath`)

Since `edit_file` operates on existing files, the shared `validatePath` function handles all path validation. This is the same pattern used by `read_file` and `list_directory`.

| Scenario | Input `path` | Behavior |
|----------|-------------|----------|
| Empty path | `""` | `validatePath` returns error: `"path is required"` |
| Absolute path | `"/etc/passwd"` | `validatePath` returns error: `"absolute paths are not allowed"` |
| Parent traversal escaping workdir | `"../../escape.txt"` | Fast-path `isWithinDir` check fails. Error: `"path escapes workdir"` |
| Deep parent traversal | `"a/b/../../../../etc/file"` | After `filepath.Clean`, resolves outside workdir. Error: `"path escapes workdir"` |
| Simple relative path (existing file) | `"file.txt"` | `validatePath` succeeds. Edit proceeds. |
| Nested relative path (existing file) | `"a/b/file.txt"` | `validatePath` succeeds. Edit proceeds. |
| Nonexistent file | `"missing.txt"` | `filepath.EvalSymlinks` in `validatePath` fails with ENOENT. Error returned. |
| Path is a directory | `"somedir"` | `validatePath` succeeds (directory exists), but `os.Stat` + `IsDir()` check rejects it. Error: `"path is a directory, not a file"` |
| Symlink in path escaping workdir | Symlink `workdir/link` → `/tmp/outside/file.txt` | `filepath.EvalSymlinks` resolves to `/tmp/outside/file.txt`. `isWithinDir` check fails. Error: `"path escapes workdir"` |
| Symlink in path within workdir | Symlink `workdir/link` → `workdir/real/file.txt` | `filepath.EvalSymlinks` resolves within workdir. `isWithinDir` check passes. Edit proceeds. |

### 5.2 old_string Matching

| Scenario | File content | `old_string` | `replace_all` | Behavior |
|----------|-------------|-------------|---------------|----------|
| Exact single match | `"hello world"` | `"hello"` | absent/false | Success: `"replaced 1 occurrence(s) in <path>"`. File becomes `"<new_string> world"`. |
| Exact single match (with replace_all true) | `"hello world"` | `"hello"` | `"true"` | Success: `"replaced 1 occurrence(s) in <path>"`. Same result as without replace_all. |
| Multiple matches with replace_all | `"aaa"` | `"a"` | `"true"` | Success: `"replaced 3 occurrence(s) in <path>"`. File becomes `"<new_string><new_string><new_string>"`. |
| Multiple matches without replace_all | `"aaa"` | `"a"` | absent/false | Error: `"old_string found 3 times; set replace_all to true or provide a more unique string"` |
| Not found | `"hello world"` | `"xyz"` | any | Error: `"old_string not found in file"` |
| old_string matches entire file | `"hello"` | `"hello"` | absent/false | Success. File becomes `"<new_string>"`. |
| old_string with newlines | `"line1\nline2"` | `"line1\nline2"` | absent/false | Success. Multi-line matching works (exact string match). |
| old_string with special regex chars | `"price: $10.00"` | `"$10.00"` | absent/false | Success. No regex interpretation — literal string match. |
| Empty old_string | N/A | `""` | any | Error: `"old_string is required"`. Checked before file read. |

### 5.3 new_string Behavior

| Scenario | `old_string` | `new_string` | Behavior |
|----------|-------------|-------------|----------|
| Normal replacement | `"old"` | `"new"` | `"old"` replaced with `"new"` |
| Deletion (empty new_string) | `"remove"` | `""` | `"remove"` deleted from file |
| Missing new_string key in map | `"old"` | (key absent) | `call.Arguments["new_string"]` returns `""`. Same as deletion. |
| new_string same as old_string | `"same"` | `"same"` | Success. File content unchanged. Count is still reported. |
| new_string contains old_string | `"a"` | `"ab"` | Replacement is performed on the original content. No recursive expansion. `strings.Replace` / `strings.ReplaceAll` handles this correctly. |
| new_string longer than old_string | `"x"` | `"xyz"` | File grows. Written as-is. |
| new_string shorter than old_string | `"xyz"` | `"x"` | File shrinks. Written as-is. |

### 5.4 replace_all Parameter

| Scenario | `replace_all` value | Behavior |
|----------|-------------------|----------|
| Key absent from map | (missing) | Default: `false` |
| Empty string | `""` | Default: `false` |
| `"true"` | `"true"` | All occurrences replaced |
| `"false"` | `"false"` | Only first occurrence replaced (if exactly 1 match) |
| `"1"` | `"1"` | Parsed as `true` by `strconv.ParseBool` |
| `"0"` | `"0"` | Parsed as `false` by `strconv.ParseBool` |
| `"TRUE"` | `"TRUE"` | Parsed as `true` by `strconv.ParseBool` |
| `"banana"` | `"banana"` | Error: `"invalid replace_all value \"banana\": ..."` |

### 5.5 File Content Edge Cases

| Scenario | File content | Behavior |
|----------|-------------|----------|
| Empty file | `""` (0 bytes) | `strings.Count` returns 0 for any non-empty old_string. Error: `"old_string not found in file"` |
| Binary file | Binary content with NUL bytes | No binary detection. The tool performs string replacement on the byte content as UTF-8. If old_string is found, replacement occurs. This is consistent with the milestone spec which does not mention binary detection for edit_file. |
| Very large file | 10MB+ file | Read entirely into memory. Replacement performed. Written back. No size limit enforced. |
| File with Windows line endings | `"line1\r\nline2\r\n"` | Matched and replaced as-is. No CRLF normalization. |
| UTF-8 multi-byte content | `"hello 世界"` | Matched as exact UTF-8 string. old_string `"世界"` matches and is replaced. |

### 5.6 Argument Handling

| Scenario | Behavior |
|----------|----------|
| `path` argument present | Normal operation via `validatePath`. |
| `path` argument missing from `call.Arguments` | `call.Arguments["path"]` returns `""` → `validatePath` returns error: `"path is required"`. |
| `old_string` argument missing from map | `call.Arguments["old_string"]` returns `""` → Error: `"old_string is required"`. |
| `new_string` key absent from map | `call.Arguments["new_string"]` returns `""` → treated as deletion (empty replacement). |
| `new_string` key present with empty value | Same as absent: empty replacement (deletion). |
| `replace_all` absent | Default `false`. |

---

## 6. Constraints

**Constraint 1:** No new external dependencies. `go.mod` must remain unchanged.

**Constraint 2:** No changes to `internal/provider/`, `internal/agent/`, or `internal/toolname/` packages.

**Constraint 3:** No changes to `cmd/run.go` or `internal/tool/tool.go`. The only modifications outside the new files are adding one line to `RegisterAll` in `registry.go` and adding tests to `registry_test.go`.

**Constraint 4:** `call_agent` must NOT be registered in the registry. It remains special-cased.

**Constraint 5:** `validatePath` must NOT be modified. It is called as-is from `edit_file`.

**Constraint 6:** `path_validation.go` must NOT be modified. No new functions are added to that file.

**Constraint 7:** Cross-platform compatibility: must build and run on Linux, macOS, and Windows. Path separator handling must use `filepath` (not `path`).

**Constraint 8:** `ToolCall.Arguments` is `map[string]string`. All parameter values are strings. `replace_all` must be parsed with `strconv.ParseBool`.

**Constraint 9:** No content transformation beyond the specified replacement. No CRLF normalization, no encoding conversion, no trailing newline insertion.

**Constraint 10:** File permission mode on write-back is `0o644`. This is not configurable. Original file permissions are not preserved.

---

## 7. Testing Requirements

### 7.1 Test Conventions

Tests must follow the patterns established in M3–M5:

- **Package-level tests:** Tests live in the same package (`package tool`)
- **Standard library only:** Use `testing` package. No test frameworks.
- **Temp directories:** Use `t.TempDir()` for filesystem isolation.
- **Descriptive names:** `TestEditFile_Scenario` with underscores.
- **Test real code, not mocks.** Tests must call actual functions with real filesystem operations. Each test must fail if the code under test is deleted.
- **Red/green TDD:** Write failing tests first, then implement code to make them pass.
- **Run tests with:** `make test`

### 7.2 `internal/tool/edit_file_test.go` Tests

**Test: `TestEditFile_SingleReplace`** — Create a tmpdir with a file `hello.txt` containing `"hello world"`. Call `Execute` with `Arguments: {"path": "hello.txt", "old_string": "hello", "new_string": "goodbye"}`. Verify `IsError` is false. Read the file from disk and verify contents equal `"goodbye world"`. Verify `Content` equals `"replaced 1 occurrence(s) in hello.txt"`. Verify `CallID` matches.

**Test: `TestEditFile_ReplaceAll`** — Create a tmpdir with a file `repeat.txt` containing `"aaa"`. Call `Execute` with `Arguments: {"path": "repeat.txt", "old_string": "a", "new_string": "b", "replace_all": "true"}`. Verify `IsError` is false. Read the file from disk and verify contents equal `"bbb"`. Verify `Content` equals `"replaced 3 occurrence(s) in repeat.txt"`.

**Test: `TestEditFile_NotFoundError`** — Create a tmpdir with a file `target.txt` containing `"hello world"`. Call `Execute` with `Arguments: {"path": "target.txt", "old_string": "xyz", "new_string": "abc"}`. Verify `IsError` is true. Verify `Content` contains `"old_string not found in file"`. Read the file from disk and verify contents are unchanged (`"hello world"`).

**Test: `TestEditFile_MultipleMatchesWithoutReplaceAll`** — Create a tmpdir with a file `multi.txt` containing `"abab"`. Call `Execute` with `Arguments: {"path": "multi.txt", "old_string": "ab", "new_string": "cd"}`. Verify `IsError` is true. Verify `Content` contains `"found 2 times"`. Read the file from disk and verify contents are unchanged (`"abab"`).

**Test: `TestEditFile_PathTraversalRejected`** — Create a tmpdir. Call `Execute` with `Arguments: {"path": "../../escape.txt", "old_string": "a", "new_string": "b"}`. Verify `IsError` is true. Verify `Content` contains `"path escapes workdir"`.

**Test: `TestEditFile_AbsolutePathRejected`** — Create a tmpdir. Call `Execute` with `Arguments: {"path": "/etc/passwd", "old_string": "a", "new_string": "b"}`. Verify `IsError` is true. Verify `Content` contains `"absolute paths"`.

**Test: `TestEditFile_MissingPathArgument`** — Create a tmpdir. Call `Execute` with `Arguments: {"old_string": "a", "new_string": "b"}`. Verify `IsError` is true. Verify `Content` contains `"path is required"`.

**Test: `TestEditFile_MissingOldStringArgument`** — Create a tmpdir with a file `target.txt` containing `"hello"`. Call `Execute` with `Arguments: {"path": "target.txt", "new_string": "b"}`. Verify `IsError` is true. Verify `Content` contains `"old_string is required"`.

**Test: `TestEditFile_EmptyNewStringDeletesText`** — Create a tmpdir with a file `delete.txt` containing `"remove me please"`. Call `Execute` with `Arguments: {"path": "delete.txt", "old_string": "remove ", "new_string": ""}`. Verify `IsError` is false. Read the file from disk and verify contents equal `"me please"`. Verify `Content` equals `"replaced 1 occurrence(s) in delete.txt"`.

**Test: `TestEditFile_SymlinkEscapeRejected`** — Create a tmpdir. Create a second tmpdir (outside) with `t.TempDir()`. Write a file `secret.txt` with content `"original"` in the outside dir. Create a symlink inside the first tmpdir pointing to the file in the outside dir (e.g., `workdir/link.txt` → `outsideDir/secret.txt`). Call `Execute` with `Arguments: {"path": "link.txt", "old_string": "original", "new_string": "modified"}`. Verify `IsError` is true. Read the file in the outside directory and verify contents are unchanged (`"original"`).

**Test: `TestEditFile_CallIDPassthrough`** — Create a tmpdir with a file `test.txt` containing `"abc"`. Call `Execute` with a specific `call.ID` value (e.g., `"ef-unique-42"`). Verify the returned `ToolResult.CallID` matches.

**Test: `TestEditFile_NonexistentFileError`** — Create a tmpdir with no files. Call `Execute` with `Arguments: {"path": "missing.txt", "old_string": "a", "new_string": "b"}`. Verify `IsError` is true. Verify `Content` contains an error message (from `validatePath`'s `EvalSymlinks` failure).

**Test: `TestEditFile_DirectoryPathRejected`** — Create a tmpdir. Create a subdirectory `subdir` inside it. Call `Execute` with `Arguments: {"path": "subdir", "old_string": "a", "new_string": "b"}`. Verify `IsError` is true. Verify `Content` contains `"path is a directory"`.

**Test: `TestEditFile_InvalidReplaceAllValue`** — Create a tmpdir with a file `target.txt` containing `"hello"`. Call `Execute` with `Arguments: {"path": "target.txt", "old_string": "hello", "new_string": "hi", "replace_all": "banana"}`. Verify `IsError` is true. Verify `Content` contains `"invalid replace_all value"`. Read the file from disk and verify contents are unchanged (`"hello"`).

**Test: `TestEditFile_MultilineMatch`** — Create a tmpdir with a file `multi.txt` containing `"line1\nline2\nline3"`. Call `Execute` with `Arguments: {"path": "multi.txt", "old_string": "line1\nline2", "new_string": "replaced"}`. Verify `IsError` is false. Read the file from disk and verify contents equal `"replaced\nline3"`.

### 7.3 `RegisterAll` Tests (additions to `registry_test.go`)

**Test: `TestRegisterAll_RegistersEditFile`** — Call `NewRegistry()`, then `RegisterAll(r)`. Verify `r.Has(toolname.EditFile)` returns true.

**Test: `TestRegisterAll_ResolvesEditFile`** — Call `RegisterAll(r)`, then `r.Resolve([]string{toolname.EditFile})`. Verify the returned tool has `Name` equal to `toolname.EditFile` and has `path`, `old_string`, `new_string`, and `replace_all` parameters.

### 7.4 Existing Tests

All existing tests must continue to pass without modification:

- `internal/tool/tool_test.go`
- `internal/tool/registry_test.go` (existing tests)
- `internal/tool/write_file_test.go`
- `internal/tool/read_file_test.go`
- `internal/tool/list_directory_test.go`
- `internal/tool/path_validation_test.go`
- All other test files in the project.

### 7.5 Running Tests

All tests must pass when run with:

```bash
make test
```

---

## 8. Acceptance Criteria

| Criterion | Verification |
|-----------|-------------|
| `internal/tool/edit_file.go` exists and compiles | `go build ./internal/tool/` succeeds |
| `RegisterAll` registers `edit_file` | `TestRegisterAll_RegistersEditFile` passes |
| Tool definition has correct name | `toolname.EditFile` constant used, not hardcoded string |
| Tool definition has 4 parameters | `path` (required), `old_string` (required), `new_string` (required), `replace_all` (not required) |
| Single replacement works | `TestEditFile_SingleReplace` passes |
| Replace all works | `TestEditFile_ReplaceAll` passes |
| Not found returns error | `TestEditFile_NotFoundError` passes |
| Multiple matches without replace_all returns error | `TestEditFile_MultipleMatchesWithoutReplaceAll` passes |
| Path traversal rejected | `TestEditFile_PathTraversalRejected` passes |
| Absolute path rejected | `TestEditFile_AbsolutePathRejected` passes |
| Missing path argument rejected | `TestEditFile_MissingPathArgument` passes |
| Missing old_string argument rejected | `TestEditFile_MissingOldStringArgument` passes |
| Empty new_string deletes text | `TestEditFile_EmptyNewStringDeletesText` passes |
| Symlink escape rejected | `TestEditFile_SymlinkEscapeRejected` passes |
| CallID propagated | `TestEditFile_CallIDPassthrough` passes |
| Nonexistent file returns error | `TestEditFile_NonexistentFileError` passes |
| Directory path rejected | `TestEditFile_DirectoryPathRejected` passes |
| Invalid replace_all value returns error | `TestEditFile_InvalidReplaceAllValue` passes |
| Multi-line matching works | `TestEditFile_MultilineMatch` passes |
| File unchanged on error | `TestEditFile_NotFoundError`, `TestEditFile_MultipleMatchesWithoutReplaceAll`, `TestEditFile_InvalidReplaceAllValue` all verify file contents unchanged |
| Registry registers edit_file | `TestRegisterAll_RegistersEditFile` passes |
| Registry resolves edit_file with correct params | `TestRegisterAll_ResolvesEditFile` passes |
| All existing tests pass | `make test` exits 0 |

---

## 9. Out of Scope

The following items are explicitly **not** included in this milestone:

1. Other tool executors (`run_command`) — M7 scope
2. Regex-based matching (literal string matching only)
3. Line-based replacement (operates on raw string content, not line numbers)
4. Binary file detection (unlike `read_file`, no NUL-byte check is performed)
5. File permission preservation (always writes back with `0o644`)
6. Backup or undo of modified files
7. Confirmation prompts before edit
8. Atomic writes (write to temp file + rename)
9. File size limits
10. Content encoding detection or conversion
11. Windows CRLF normalization
12. Diff output (only reports count, not what changed)
13. Creating new files (file must already exist; `validatePath` enforces this)
14. Creating parent directories (file must already exist; unlike `write_file`)
15. Changes to `--dry-run`, `--json`, or `--verbose` output — M8 scope
16. Changes to `internal/agent/`, `internal/provider/`, or `internal/toolname/` packages
17. Changes to `cmd/run.go` or `internal/tool/tool.go`

---

## 10. References

- Milestone Definition: `docs/plans/000_tool_call_milestones.md` (M6 section, lines 91–103)
- M5 Spec: `docs/plans/016_write_file_spec.md`
- M4 Spec: `docs/plans/015_read_file_spec.md`
- M3 Spec: `docs/plans/014_list_directory_spec.md`
- Existing `write_file` implementation: `internal/tool/write_file.go`
- Existing `read_file` implementation: `internal/tool/read_file.go`
- `validatePath` function: `internal/tool/path_validation.go:23`
- `isWithinDir` helper: `internal/tool/path_validation.go:13`
- `RegisterAll` function: `internal/tool/registry.go:92`
- `toolname.EditFile` constant: `internal/toolname/toolname.go:11`
- `provider.Tool` type: `internal/provider/provider.go:34`
- `provider.ToolCall` type: `internal/provider/provider.go:41`
- `provider.ToolResult` type: `internal/provider/provider.go:48`
- `ToolEntry` type: `internal/tool/registry.go:20`
- `ExecContext` type: `internal/tool/registry.go:13`

---

## 11. Notes

- **`validatePath` CAN be reused for edit operations.** Unlike `write_file` which creates new files (where the target path may not exist), `edit_file` only operates on existing files. `validatePath` calls `filepath.EvalSymlinks` on the full resolved path, which succeeds for existing files and returns ENOENT for nonexistent files — both are correct behaviors for `edit_file`.
- **No binary detection.** The milestone spec does not require binary detection for `edit_file` (unlike `read_file` which has it). If an LLM requests editing a binary file and the `old_string` happens to match, the replacement will occur. This is acceptable because: (a) the LLM would need to explicitly provide the exact bytes to match, (b) `edit_file` is a mutation tool — the LLM is expected to know what it's doing, and (c) adding binary detection is a future enhancement if needed.
- **`strings.Count` counts non-overlapping occurrences.** For `old_string="aa"` and content `"aaa"`, `strings.Count` returns 1 (not 2). `strings.Replace`/`strings.ReplaceAll` behave consistently with this. This is the standard Go library behavior and does not need special handling.
- **`os.WriteFile` atomicity:** Same note as `write_file` — not atomic. If the write fails partway, the file may contain partial content. Atomic writes are out of scope.
- **The `RegisterAll` pattern continues to scale.** M6 adds one line. M7 will add one line. No call-site changes needed.
- **Execution order in the executor matters.** Path validation and `old_string` validation happen before the file is read. `replace_all` parsing happens after the file is read (but before any modification). The file is only modified if all validations pass and at least one match is found.
