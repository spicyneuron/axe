# Specification: Tool Call M5 — `write_file` Tool

**Status:** Draft
**Version:** 1.0
**Created:** 2026-03-02
**Scope:** File creation and overwrite tool with parent directory creation, path security for non-existent paths, and byte-count confirmation

---

## 1. Purpose

Implement the `write_file` tool — the first mutation tool registered in the `Registry`. This tool writes content to a file relative to the agent's workdir, creating parent directories as needed. It overwrites existing files and creates new ones.

This milestone builds on the infrastructure established in M3 and M4:

- **`isWithinDir`** — reused as-is for path boundary checking
- **`RegisterAll`** — extended with one additional `r.Register(...)` call
- **`toolname.WriteFile`** — constant already declared in `internal/toolname/toolname.go`

This milestone does **not** reuse `validatePath` directly because `validatePath` calls `filepath.EvalSymlinks` on the target path, which requires the path to exist. `write_file` must handle non-existent target files and parent directories. Instead, path validation is performed inline within the executor using the same security checks (empty, absolute, traversal, symlink escape) adapted for write operations.

---

## 2. Design Decisions

The following decisions were made during planning and are binding for implementation:

1. **`validatePath` is NOT called directly.** The existing `validatePath` function calls `filepath.EvalSymlinks` on the full resolved path, which fails with ENOENT when the target file does not yet exist. The `write_file` executor performs equivalent security checks inline: reject empty paths, reject absolute paths, reject `..` traversal via `isWithinDir` on the cleaned path, create parent directories, then validate the parent directory via `filepath.EvalSymlinks` to catch symlink escapes.

2. **Parent directories are created automatically.** If the parent directory of the target path does not exist, it is created using `os.MkdirAll` with permissions `0o755`. This happens before symlink evaluation on the parent.

3. **Missing or empty `content` argument writes a 0-byte file.** The `content` key absent from `call.Arguments` or present with an empty string value both result in a 0-byte file being created. This is not an error.

4. **File permissions are `0o644`.** Files are written using `os.WriteFile` with permission mode `0o644`. This is the standard permission for non-executable files.

5. **Overwrite is unconditional.** If the target file already exists, it is overwritten in its entirety. There is no confirmation, backup, or diff. This matches the milestone specification.

6. **Confirmation message includes byte count and relative path.** On success, the tool returns a message in the format: `wrote N bytes to <relPath>` where `N` is the number of bytes written (i.e., `len(content)` on the byte representation) and `<relPath>` is the original relative path from the arguments (not the resolved absolute path).

7. **Symlink escape detection is performed on the parent directory.** After `os.MkdirAll` creates the parent directory (or confirms it exists), `filepath.EvalSymlinks` is called on the parent directory. The evaluated parent is checked against the evaluated workdir using `isWithinDir`. This catches cases where a symlink in the parent chain points outside the workdir.

8. **`call_agent` remains outside the registry.** Same as M2–M4.

9. **No new external dependencies.** Uses only Go stdlib.

---

## 3. Requirements

### 3.1 `write_file` Tool Definition

**Requirement 1.1:** Create a file `internal/tool/write_file.go`.

**Requirement 1.2:** Define an unexported function `writeFileEntry() ToolEntry` that returns a `ToolEntry` with both `Definition` and `Execute` set to non-nil functions.

**Requirement 1.3:** The tool definition returned by the `Definition` function must have:
- `Name`: the value of `toolname.WriteFile` (i.e., `"write_file"`)
- `Description`: a clear description for the LLM explaining the tool creates or overwrites a file relative to the working directory, creating parent directories as needed
- `Parameters`: two parameters:
  - `path`:
    - `Type`: `"string"`
    - `Description`: a description stating it is a relative path to the file to write
    - `Required`: `true`
  - `content`:
    - `Type`: `"string"`
    - `Description`: a description stating it is the content to write to the file
    - `Required`: `false`

**Requirement 1.4:** The tool name in the definition must use `toolname.WriteFile`, not a hardcoded string.

**Requirement 1.5:** `ToolCall.Arguments` is `map[string]string`. Both parameters are strings. No type conversion is needed within the executor.

### 3.2 `write_file` Tool Executor

**Requirement 2.1:** The `Execute` function must have the signature: `func(ctx context.Context, call provider.ToolCall, ec ExecContext) provider.ToolResult`.

**Requirement 2.2:** Extract the `path` argument from `call.Arguments["path"]`.

**Requirement 2.3 (Empty Path Check):** If `path` is an empty string (including when the key is absent from the map), return a `ToolResult` with `CallID: call.ID`, `Content: "path is required"`, `IsError: true`.

**Requirement 2.4 (Absolute Path Check):** If `path` is an absolute path (as determined by `filepath.IsAbs`), return a `ToolResult` with `CallID: call.ID`, `Content: "absolute paths are not allowed"`, `IsError: true`.

**Requirement 2.5 (Traversal Fast-Path Check):** Compute the resolved path: `filepath.Clean(filepath.Join(filepath.Clean(ec.Workdir), path))`. Check that the resolved path is within the workdir using `isWithinDir(resolved, cleanWorkdir)`. If not, return a `ToolResult` with `CallID: call.ID`, `Content: "path escapes workdir"`, `IsError: true`.

**Requirement 2.6 (Parent Directory Creation):** Compute the parent directory of the resolved path using `filepath.Dir(resolved)`. Call `os.MkdirAll(parent, 0o755)`. If `os.MkdirAll` returns an error, return a `ToolResult` with `CallID: call.ID`, `Content: err.Error()`, `IsError: true`.

**Requirement 2.7 (Symlink Escape Check on Parent):** After the parent directory exists, call `filepath.EvalSymlinks` on the parent directory. If `EvalSymlinks` fails, return a `ToolResult` with `CallID: call.ID`, `Content: err.Error()`, `IsError: true`. Then call `filepath.EvalSymlinks` on the workdir. If `EvalSymlinks` fails, return a `ToolResult` with `CallID: call.ID`, `Content: err.Error()`, `IsError: true`. Check that the evaluated parent is within the evaluated workdir using `isWithinDir(evalParent, evalWorkdir)`. If not, return a `ToolResult` with `CallID: call.ID`, `Content: "path escapes workdir"`, `IsError: true`.

**Requirement 2.8 (Extract Content):** Extract the `content` argument from `call.Arguments["content"]`. If the key is absent from the map or the value is an empty string, treat content as an empty string (write a 0-byte file).

**Requirement 2.9 (Write File):** Write the content to the resolved path using `os.WriteFile(resolved, []byte(content), 0o644)`. If `os.WriteFile` returns an error, return a `ToolResult` with `CallID: call.ID`, `Content: err.Error()`, `IsError: true`.

**Requirement 2.10 (Success Response):** Return a `ToolResult` with `CallID: call.ID`, `Content` in the format `"wrote %d bytes to %s"` (where the first value is `len([]byte(content))` and the second value is the original `path` argument as provided by the caller), `IsError: false`.

### 3.3 Registration in Registry

**Requirement 3.1:** Add `r.Register(toolname.WriteFile, writeFileEntry())` to the `RegisterAll` function in `internal/tool/registry.go`.

**Requirement 3.2:** This is the only change to `registry.go`. No call-site changes are needed in `cmd/run.go` or `internal/tool/tool.go`.

### 3.4 Registry Tests

**Requirement 4.1:** Add tests to `internal/tool/registry_test.go` that verify `write_file` is registered by `RegisterAll`.

**Requirement 4.2:** Add a test that resolves `write_file` and verifies the tool definition has the correct name and parameters (`path` and `content`).

---

## 4. Project Structure

After completion, the following files will be added or modified:

```
axe/
├── internal/
│   └── tool/
│       ├── write_file.go               # NEW: Definition, Execute, helper entry func
│       ├── write_file_test.go           # NEW: tests
│       ├── registry.go                 # MODIFIED: add one line to RegisterAll
│       ├── registry_test.go            # MODIFIED: add write_file registration tests
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

### 5.1 Path Validation (adapted for write — no `validatePath` reuse)

The `write_file` executor performs its own path validation because `validatePath` requires the target path to exist (it calls `filepath.EvalSymlinks` on the full path). The security checks are equivalent but adapted for non-existent targets.

| Scenario | Input `path` | Behavior |
|----------|-------------|----------|
| Empty path | `""` | Error: `path is required` |
| Absolute path | `"/etc/passwd"` | Error: `absolute paths are not allowed` |
| Parent traversal escaping workdir | `"../../escape.txt"` | Fast-path `isWithinDir` check fails. Error: `path escapes workdir` |
| Deep parent traversal | `"a/b/../../../../etc/file"` | After `filepath.Clean`, resolves outside workdir. Error: `path escapes workdir` |
| Simple relative path (new file) | `"output.txt"` | Parent is workdir itself. Validation passes. File created. |
| Nested relative path (new dirs) | `"a/b/c/file.txt"` | Parent dirs `a/b/c/` created via `MkdirAll`. File created. |
| Dot path | `"."` | Resolves to workdir (a directory). `os.WriteFile` will fail (EISDIR). Error returned. |
| Symlink in parent chain escaping workdir | Symlink `workdir/link/` → `/tmp/outside/` | `MkdirAll` succeeds (follows symlink). `EvalSymlinks` on parent resolves to `/tmp/outside/`. `isWithinDir` check fails. Error: `path escapes workdir` |
| Symlink in parent chain within workdir | Symlink `workdir/link/` → `workdir/real/` | `EvalSymlinks` resolves to `workdir/real/`. `isWithinDir` check passes. File created. |
| Path with trailing slash | `"dir/"` | `filepath.Clean` removes trailing slash. If `dir` is a directory, `os.WriteFile` fails (EISDIR). If `dir` does not exist, a file named `dir` is created (no trailing slash after Clean). |

### 5.2 Content Handling

| Scenario | `content` argument | Behavior |
|----------|--------------------|----------|
| Normal text content | `"hello world\n"` | File contains `hello world\n`. Returns `wrote 12 bytes to <path>`. |
| Empty string content | `""` | File created with 0 bytes. Returns `wrote 0 bytes to <path>`. |
| Missing `content` key in Arguments map | Key absent | `call.Arguments["content"]` returns `""`. File created with 0 bytes. Returns `wrote 0 bytes to <path>`. |
| Multi-line content | `"line1\nline2\nline3\n"` | File contains the exact string. Newlines preserved as-is. |
| Content with special characters | `"tab\there\nnull?"` | Written as-is. No escaping or transformation. |
| Large content (>1MB) | Large string | Written as-is. No size limit enforced. |
| Content with Windows line endings | `"line1\r\nline2\r\n"` | Written as-is. No CRLF normalization. |
| Unicode content | `"hello 世界"` | Written as UTF-8 bytes. `len([]byte(content))` used for byte count (13 bytes, not 8 runes). |

### 5.3 File Operations

| Scenario | Behavior |
|----------|----------|
| Target file does not exist | File created with specified content. |
| Target file already exists | File overwritten entirely with new content. Old content is lost. |
| Target file already exists and is larger than new content | File is overwritten (not truncated then written). `os.WriteFile` handles this correctly — it truncates and writes. |
| Parent directory does not exist | Parent directories created via `os.MkdirAll(parent, 0o755)`. |
| Parent directory already exists | `os.MkdirAll` is a no-op. No error. |
| Deeply nested new path (e.g., `a/b/c/d/e/file.txt`) | All intermediate directories created. File written. |
| Target is an existing directory | `os.WriteFile` returns error (EISDIR on Linux/macOS). Error result returned. |
| Permission denied on parent directory | `os.MkdirAll` or `os.WriteFile` returns error. Error result returned. |
| Disk full | `os.WriteFile` returns error. Error result returned. |

### 5.4 Byte Count in Success Message

| Scenario | Content | Message |
|----------|---------|---------|
| ASCII text | `"hello"` | `wrote 5 bytes to <path>` |
| Empty content | `""` | `wrote 0 bytes to <path>` |
| UTF-8 multi-byte | `"日本語"` | `wrote 9 bytes to <path>` (3 chars × 3 bytes each) |
| Content with newlines | `"a\nb\n"` | `wrote 4 bytes to <path>` |

### 5.5 Argument Handling

| Scenario | Behavior |
|----------|----------|
| `path` argument present | Normal operation. |
| `path` argument missing from `call.Arguments` | `call.Arguments["path"]` returns `""` → Error: `path is required`. |
| `content` key absent from map | `call.Arguments["content"]` returns `""` → 0-byte file created. |
| `content` key present with empty string value | 0-byte file created. |

---

## 6. Constraints

**Constraint 1:** No new external dependencies. `go.mod` must remain unchanged.

**Constraint 2:** No changes to `internal/provider/`, `internal/agent/`, or `internal/toolname/` packages.

**Constraint 3:** No changes to `cmd/run.go` or `internal/tool/tool.go`. The only modifications outside the new files are adding one line to `RegisterAll` in `registry.go` and adding tests to `registry_test.go`.

**Constraint 4:** `call_agent` must NOT be registered in the registry. It remains special-cased.

**Constraint 5:** `validatePath` must NOT be modified. The `write_file` executor handles its own path validation inline.

**Constraint 6:** `path_validation.go` must NOT be modified. The `isWithinDir` helper is reused (it is package-private and accessible within `package tool`), but no new functions are added to that file.

**Constraint 7:** Cross-platform compatibility: must build and run on Linux, macOS, and Windows. Path separator handling must use `filepath` (not `path`).

**Constraint 8:** `ToolCall.Arguments` is `map[string]string`. Both `path` and `content` values are strings used directly — no type conversion needed.

**Constraint 9:** No content transformation. Content is written byte-for-byte as provided. No CRLF normalization, no encoding conversion, no trailing newline insertion.

**Constraint 10:** File permission mode is `0o644`. Directory permission mode is `0o755`. These are not configurable.

---

## 7. Testing Requirements

### 7.1 Test Conventions

Tests must follow the patterns established in M3–M4:

- **Package-level tests:** Tests live in the same package (`package tool`)
- **Standard library only:** Use `testing` package. No test frameworks.
- **Temp directories:** Use `t.TempDir()` for filesystem isolation.
- **Descriptive names:** `TestWriteFile_Scenario` with underscores.
- **Test real code, not mocks.** Tests must call actual functions with real filesystem operations. Each test must fail if the code under test is deleted.
- **Red/green TDD:** Write failing tests first, then implement code to make them pass.
- **Run tests with:** `make test`

### 7.2 `internal/tool/write_file_test.go` Tests

**Test: `TestWriteFile_CreateNewFile`** — Create a tmpdir (no pre-existing files). Call `Execute` with `Arguments: {"path": "output.txt", "content": "hello world"}`. Verify `IsError` is false. Read the file from disk using `os.ReadFile` and verify contents equal `"hello world"`. Verify `Content` contains `"wrote 11 bytes to output.txt"`. Verify `CallID` matches.

**Test: `TestWriteFile_OverwriteExisting`** — Create a tmpdir with an existing file `existing.txt` containing `"old content"`. Call `Execute` with `Arguments: {"path": "existing.txt", "content": "new content"}`. Verify `IsError` is false. Read the file from disk and verify contents equal `"new content"` (old content completely replaced). Verify `Content` contains `"wrote 11 bytes to existing.txt"`.

**Test: `TestWriteFile_CreateWithNestedDirs`** — Create a tmpdir with no subdirectories. Call `Execute` with `Arguments: {"path": "a/b/c/deep.txt", "content": "nested"}`. Verify `IsError` is false. Verify the file `a/b/c/deep.txt` exists on disk with contents `"nested"`. Verify directories `a/`, `a/b/`, `a/b/c/` were created.

**Test: `TestWriteFile_PathTraversalRejected`** — Create a tmpdir. Call `Execute` with `Arguments: {"path": "../../escape.txt", "content": "bad"}`. Verify `IsError` is true. Verify `Content` contains `"path escapes workdir"`. Verify no file was created outside the tmpdir.

**Test: `TestWriteFile_AbsolutePathRejected`** — Create a tmpdir. Call `Execute` with `Arguments: {"path": "/tmp/absolute.txt", "content": "bad"}`. Verify `IsError` is true. Verify `Content` contains `"absolute paths"`.

**Test: `TestWriteFile_EmptyContent`** — Create a tmpdir. Call `Execute` with `Arguments: {"path": "empty.txt", "content": ""}`. Verify `IsError` is false. Read the file from disk and verify it exists with 0 bytes. Verify `Content` contains `"wrote 0 bytes to empty.txt"`.

**Test: `TestWriteFile_MissingContentArgument`** — Create a tmpdir. Call `Execute` with `Arguments: {"path": "nokey.txt"}` (content key absent from map). Verify `IsError` is false. Read the file from disk and verify it exists with 0 bytes. Verify `Content` contains `"wrote 0 bytes to nokey.txt"`.

**Test: `TestWriteFile_MissingPathArgument`** — Create a tmpdir. Call `Execute` with empty `Arguments` map `{}`. Verify `IsError` is true. Verify `Content` contains `"path is required"`.

**Test: `TestWriteFile_SymlinkEscapeRejected`** — Create a tmpdir. Create a second tmpdir (outside) with `t.TempDir()`. Create a symlink inside the first tmpdir pointing to the second tmpdir (e.g., `workdir/link` → `outsideDir`). Call `Execute` with `Arguments: {"path": "link/escape.txt", "content": "bad"}`. Verify `IsError` is true. Verify that no file was created in the outside directory.

**Test: `TestWriteFile_CallIDPassthrough`** — Create a tmpdir. Call `Execute` with a specific `call.ID` value (e.g., `"wf-unique-99"`). Verify the returned `ToolResult.CallID` matches.

**Test: `TestWriteFile_ByteCountAccurate`** — Create a tmpdir. Call `Execute` with `Arguments: {"path": "unicode.txt", "content": "日本語"}`. Verify `IsError` is false. Verify `Content` contains `"wrote 9 bytes to unicode.txt"` (3 characters × 3 bytes each in UTF-8). Read the file from disk and verify length is 9 bytes.

### 7.3 `RegisterAll` Tests (additions to `registry_test.go`)

**Test: `TestRegisterAll_RegistersWriteFile`** — Call `NewRegistry()`, then `RegisterAll(r)`. Verify `r.Has(toolname.WriteFile)` returns true.

**Test: `TestRegisterAll_ResolvesWriteFile`** — Call `RegisterAll(r)`, then `r.Resolve([]string{toolname.WriteFile})`. Verify the returned tool has `Name` equal to `toolname.WriteFile` and has `path` and `content` parameters.

### 7.4 Existing Tests

All existing tests must continue to pass without modification:

- `internal/tool/tool_test.go`
- `internal/tool/registry_test.go` (existing tests)
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
| `internal/tool/write_file.go` exists and compiles | `go build ./internal/tool/` succeeds |
| `RegisterAll` registers `write_file` | `TestRegisterAll_RegistersWriteFile` passes |
| Tool definition has correct name | `toolname.WriteFile` constant used, not hardcoded string |
| Tool definition has 2 parameters | `path` (required), `content` (not required) |
| Create new file | `TestWriteFile_CreateNewFile` passes |
| Overwrite existing file | `TestWriteFile_OverwriteExisting` passes |
| Create file with nested directories | `TestWriteFile_CreateWithNestedDirs` passes |
| Path traversal rejected | `TestWriteFile_PathTraversalRejected` passes |
| Absolute path rejected | `TestWriteFile_AbsolutePathRejected` passes |
| Empty content creates 0-byte file | `TestWriteFile_EmptyContent` passes |
| Missing content key creates 0-byte file | `TestWriteFile_MissingContentArgument` passes |
| Missing path argument rejected | `TestWriteFile_MissingPathArgument` passes |
| Symlink escape rejected | `TestWriteFile_SymlinkEscapeRejected` passes |
| CallID propagated | `TestWriteFile_CallIDPassthrough` passes |
| Byte count accurate for multi-byte content | `TestWriteFile_ByteCountAccurate` passes |
| Registry registers write_file | `TestRegisterAll_RegistersWriteFile` passes |
| Registry resolves write_file with correct params | `TestRegisterAll_ResolvesWriteFile` passes |
| All existing tests pass | `make test` exits 0 |

---

## 9. Out of Scope

The following items are explicitly **not** included in this milestone:

1. Other tool executors (`edit_file`, `run_command`) — M6–M7 scope
2. File permission configuration (always `0o644` for files, `0o755` for directories)
3. Backup or undo of overwritten files
4. Confirmation prompts before overwrite
5. Atomic writes (write to temp file + rename)
6. File size limits
7. Content encoding detection or conversion
8. Windows CRLF normalization
9. Append mode (always overwrite)
10. Creating a shared `validatePathForWrite` helper in `path_validation.go` — deferred until needed by M6
11. Changes to `--dry-run`, `--json`, or `--verbose` output — M8 scope
12. Changes to `internal/agent/`, `internal/provider/`, or `internal/toolname/` packages
13. Changes to `cmd/run.go` or `internal/tool/tool.go`

---

## 10. References

- Milestone Definition: `docs/plans/000_tool_call_milestones.md` (M5 section, lines 76–88)
- M4 Spec: `docs/plans/015_read_file_spec.md`
- M3 Spec: `docs/plans/014_list_directory_spec.md`
- Existing `read_file` implementation: `internal/tool/read_file.go`
- Existing `list_directory` implementation: `internal/tool/list_directory.go`
- `validatePath` function: `internal/tool/path_validation.go`
- `isWithinDir` helper: `internal/tool/path_validation.go:13`
- `RegisterAll` function: `internal/tool/registry.go:92`
- `toolname.WriteFile` constant: `internal/toolname/toolname.go:10`
- `provider.Tool` type: `internal/provider/provider.go:34`
- `provider.ToolCall` type: `internal/provider/provider.go:41`
- `provider.ToolResult` type: `internal/provider/provider.go:48`
- `ToolEntry` type: `internal/tool/registry.go:20`
- `ExecContext` type: `internal/tool/registry.go:13`

---

## 11. Notes

- **`validatePath` cannot be reused for write operations.** It calls `filepath.EvalSymlinks` on the full target path (line 41 of `path_validation.go`), which returns ENOENT for non-existent files. Since `write_file` creates new files, the target path will not exist in the common case. The executor performs equivalent checks: empty/absolute/traversal fast-path using the same `isWithinDir` helper, then symlink escape detection on the parent directory (which must exist after `MkdirAll`).
- **Symlink escape detection on the parent directory is sufficient.** The only way a write can escape the workdir via symlinks is if a directory in the parent chain is a symlink pointing outside. The file itself cannot be a symlink escape because `os.WriteFile` on a non-existent path creates a regular file (it does not follow symlinks). If the file already exists and is a symlink, `os.WriteFile` follows the symlink — but the parent directory check catches the case where the symlink points outside. If the file is a symlink to another location within the workdir, the write is allowed (same security model as `validatePath`).
- **`os.WriteFile` atomicity:** `os.WriteFile` is not atomic. If the write fails partway (e.g., disk full), the file may contain partial content. Atomic writes (temp file + rename) are explicitly out of scope for this milestone. The tool reports the error from `os.WriteFile`; the LLM can retry.
- **The `RegisterAll` pattern continues to scale.** M5 adds one line. M6–M7 will each add one line. No call-site changes needed.
- **Byte count uses `len([]byte(content))`, not `len(content)`.** In Go, `len(string)` returns the byte count (not the rune count), so these are equivalent. The spec is explicit about this to prevent confusion: the confirmation message reports bytes, not characters.
- **Existing file symlink edge case:** If `workdir/file.txt` is a symlink to `workdir/other/real.txt` (both within workdir), writing to `file.txt` follows the symlink and writes to `real.txt`. This is consistent with the M3/M4 security model: intra-workdir symlinks are allowed. The parent directory check validates `workdir/` (the parent of `file.txt`), which passes.
