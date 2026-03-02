# Implementation Checklist: Tool Call M5 — `write_file` Tool

**Spec:** `docs/plans/016_write_file_spec.md`
**Status:** Complete

---

## Phase 1: Tests (RED — all tests must fail before implementation)

- [x] Create `internal/tool/write_file_test.go` with all 11 tool tests:
  - [x] `TestWriteFile_CreateNewFile` — write new file, verify disk contents and success message (Req 2.9, 2.10)
  - [x] `TestWriteFile_OverwriteExisting` — overwrite existing file, verify old content replaced (Req 2.9)
  - [x] `TestWriteFile_CreateWithNestedDirs` — write `a/b/c/deep.txt`, verify dirs created (Req 2.6)
  - [x] `TestWriteFile_PathTraversalRejected` — `../../escape.txt` returns error (Req 2.5)
  - [x] `TestWriteFile_AbsolutePathRejected` — `/tmp/absolute.txt` returns error (Req 2.4)
  - [x] `TestWriteFile_EmptyContent` — empty string content creates 0-byte file (Req 2.8)
  - [x] `TestWriteFile_MissingContentArgument` — absent content key creates 0-byte file (Req 2.8)
  - [x] `TestWriteFile_MissingPathArgument` — empty Arguments returns `path is required` (Req 2.3)
  - [x] `TestWriteFile_SymlinkEscapeRejected` — symlink parent to outside workdir (Req 2.7)
  - [x] `TestWriteFile_CallIDPassthrough` — verify CallID propagation (Req 2.1)
  - [x] `TestWriteFile_ByteCountAccurate` — UTF-8 multi-byte `"日本語"` = 9 bytes (Req 2.10)

- [x] Add 2 registry tests to `internal/tool/registry_test.go`:
  - [x] `TestRegisterAll_RegistersWriteFile` — `r.Has(toolname.WriteFile)` returns true (Req 4.1)
  - [x] `TestRegisterAll_ResolvesWriteFile` — resolve returns tool with `path` and `content` params (Req 4.2)

## Phase 2: Implementation (GREEN — make all tests pass)

- [x] Create `internal/tool/write_file.go`:
  - [x] `writeFileEntry() ToolEntry` — returns ToolEntry with Definition and Execute (Req 1.2)
  - [x] `writeFileDefinition() provider.Tool` — name: `toolname.WriteFile`, params: `path` (required), `content` (not required) (Req 1.3, 1.4)
  - [x] `writeFileExecute(ctx, call, ec) provider.ToolResult` — full executor logic (Req 2.1–2.10):
    - [x] Empty path check (Req 2.3)
    - [x] Absolute path check (Req 2.4)
    - [x] Traversal fast-path via `isWithinDir` (Req 2.5)
    - [x] Parent directory creation via `os.MkdirAll` (Req 2.6)
    - [x] Symlink escape check on parent via `filepath.EvalSymlinks` (Req 2.7)
    - [x] Extract content — missing/empty key writes 0 bytes (Req 2.8)
    - [x] Write file via `os.WriteFile` with `0o644` (Req 2.9)
    - [x] Success response: `"wrote %d bytes to %s"` (Req 2.10)

- [x] Modify `internal/tool/registry.go`:
  - [x] Add `r.Register(toolname.WriteFile, writeFileEntry())` to `RegisterAll` (Req 3.1)

## Phase 3: Verification

- [x] Run `go build ./internal/tool/` — compiles without errors
- [x] Run `go test ./internal/tool/ -run TestWriteFile` — all 11 write_file tests pass
- [x] Run `go test ./internal/tool/ -run TestRegisterAll` — all registry tests pass (including new ones)
- [x] Run `make test` — full suite passes, no regressions

## Constraints Checklist

- [x] No changes to `go.mod` or `go.sum`
- [x] No changes to `internal/provider/`, `internal/agent/`, or `internal/toolname/`
- [x] No changes to `cmd/run.go` or `internal/tool/tool.go`
- [x] No changes to `path_validation.go`
- [x] `validatePath` not called from `write_file.go` — inline validation only
- [x] `isWithinDir` reused from `path_validation.go` (package-private access)
- [x] Tool name uses `toolname.WriteFile` constant, not hardcoded string
