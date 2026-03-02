# Implementation Checklist: Tool Call M3 — `list_directory` Tool

**Spec:** `docs/plans/014_list_directory_spec.md`
**Status:** Complete

---

## Phase 1: `validatePath` — Path Security (Spec §3.2)

Write tests first (red), then implement (green).

- [x] Create `internal/tool/path_validation_test.go` with all `validatePath` tests:
  - `TestValidatePath_ValidRelativePath` — tmpdir with subdirectory, verify resolved path
  - `TestValidatePath_DotPath` — `"."` resolves to workdir itself
  - `TestValidatePath_NestedPath` — `tmpdir/a/b/c`, verify success
  - `TestValidatePath_EmptyPath` — `""` returns error containing `path is required`
  - `TestValidatePath_AbsolutePath` — `"/etc"` returns error containing `absolute paths are not allowed`
  - `TestValidatePath_ParentTraversalEscape` — `"../../etc"` returns error containing `path escapes workdir`
  - `TestValidatePath_ParentTraversalWithinWorkdir` — `"a/b/../b"` stays within workdir, verify success
  - `TestValidatePath_SymlinkWithinWorkdir` — symlink inside tmpdir → inside tmpdir, verify success
  - `TestValidatePath_SymlinkEscapingWorkdir` — symlink inside tmpdir → outside tmpdir, error containing `path escapes workdir`
  - `TestValidatePath_NonexistentPath` — `"no_such_dir"` returns error (from `EvalSymlinks`)
- [x] Run tests, confirm all fail (red phase)
- [x] Create `internal/tool/path_validation.go` with `validatePath(workdir, relPath string) (string, error)`:
  - Return error `path is required` if `relPath` is empty
  - Return error `absolute paths are not allowed` if `filepath.IsAbs(relPath)`
  - Compute resolved: `filepath.Clean(filepath.Join(workdir, relPath))`
  - `filepath.EvalSymlinks` on resolved path; return error on failure
  - `filepath.EvalSymlinks` on `filepath.Clean(workdir)`; return error on failure
  - `strings.HasPrefix(evalResolved, evalWorkdir)` check; return error `path escapes workdir` on failure
  - Return the evaluated resolved path on success
- [x] Run tests, confirm all pass (green phase)

---

## Phase 2: `list_directory` Tool — Definition & Executor (Spec §3.3, §3.4)

Write tests first (red), then implement (green).

- [x] Create `internal/tool/list_directory_test.go` with all `list_directory` tests:
  - `TestListDirectory_ExistingDir` — tmpdir with `a.txt`, `b.txt`, `sub/`; verify output has entries one per line, `sub/` has `/` suffix
  - `TestListDirectory_NestedPath` — `tmpdir/sub/` with file inside; `path: "sub"` lists only sub's contents
  - `TestListDirectory_EmptyDir` — empty tmpdir; `Content` is `""`, `IsError` is false
  - `TestListDirectory_NonexistentPath` — `path: "no_such_dir"`; `IsError` is true
  - `TestListDirectory_AbsolutePathRejected` — `path: "/etc"`; `IsError` is true, content mentions absolute paths
  - `TestListDirectory_ParentTraversalRejected` — `path: "../../etc"`; `IsError` is true, content mentions path escaping
  - `TestListDirectory_SymlinkEscapeRejected` — symlink inside tmpdir → outside; `IsError` is true
  - `TestListDirectory_MissingPathArgument` — empty `Arguments` map; `IsError` is true, content mentions path required
  - `TestListDirectory_CallIDPassthrough` — specific `call.ID`; verify `ToolResult.CallID` matches
- [x] Run tests, confirm all fail (red phase)
- [x] Create `internal/tool/list_directory.go`:
  - Package-private `listDirectoryEntry() ToolEntry` returning `ToolEntry{Definition: ..., Execute: ...}`
  - `Definition` returns `provider.Tool` with `Name: toolname.ListDirectory`, `Description`, `Parameters` with `path` (type `string`, required)
  - `Execute` extracts `call.Arguments["path"]`, calls `validatePath(ec.Workdir, path)`, calls `os.ReadDir`, formats entries (dirs get `/` suffix), returns `ToolResult`
  - Error cases return `ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}`
  - Empty dir returns `ToolResult{CallID: call.ID, Content: "", IsError: false}`
- [x] Run tests, confirm all pass (green phase)

---

## Phase 3: `RegisterAll` Function (Spec §3.1, §3.5)

Write tests first (red), then implement (green).

- [x] Add `RegisterAll` tests to `internal/tool/registry_test.go` (or a new file if preferred):
  - `TestRegisterAll_RegistersListDirectory` — `NewRegistry()` + `RegisterAll(r)`, verify `r.Has(toolname.ListDirectory)` is true
  - `TestRegisterAll_ResolvesListDirectory` — `RegisterAll(r)` + `r.Resolve([]string{toolname.ListDirectory})`, verify returned tool has correct `Name` and `path` parameter
  - `TestRegisterAll_Idempotent` — call `RegisterAll(r)` twice, no panic, `Has` still true
- [x] Run tests, confirm all fail (red phase)
- [x] Add `RegisterAll(r *Registry)` to `internal/tool/registry.go`:
  - Import `internal/toolname`
  - Call `r.Register(toolname.ListDirectory, listDirectoryEntry())`
- [x] Run tests, confirm all pass (green phase)

---

## Phase 4: Wiring — `cmd/run.go` (Spec §3.6)

- [x] In `cmd/run.go`, after `registry := tool.NewRegistry()` (line 214), add `tool.RegisterAll(registry)`
- [x] Run `make test`, confirm all existing tests still pass

---

## Phase 5: Wiring — `internal/tool/tool.go` `ExecuteCallAgent` (Spec §3.7)

- [x] In `ExecuteCallAgent`, replace `NewRegistry()` (line 245) with `NewRegistry()` + `RegisterAll(registry)`:
  - Create registry: `registry := NewRegistry()`
  - Register tools: `RegisterAll(registry)`
  - Pass `registry` to `runConversationLoop` (replacing inline `NewRegistry()`)
- [x] After the existing `call_agent` tool injection (lines 228–232), add sub-agent `cfg.Tools` resolution:
  - If `len(cfg.Tools) > 0`, call `registry.Resolve(cfg.Tools)`
  - If resolve error, return error `ToolResult{CallID: call.ID, Content: error message, IsError: true}`
  - Append resolved tools to `req.Tools`
  - Injection order: `call_agent` first (if applicable), then `cfg.Tools`
- [x] Run `make test`, confirm all existing and new tests pass

---

## Phase 6: Final Verification

- [x] Run `make test` — all tests pass (zero failures)
- [x] Run `go build ./...` — compiles clean
- [x] Run `go vet ./...` — no warnings
- [x] Verify no changes to `go.mod`, `go.sum`, `internal/provider/`, `internal/agent/`, `internal/toolname/`
