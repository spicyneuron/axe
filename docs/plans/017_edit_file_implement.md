# Implementation Checklist: Tool Call M6 — `edit_file` Tool

**Spec:** `docs/plans/017_edit_file_spec.md`
**Status:** Complete

---

## Phase 1: Tests (Red)

Write all tests first. They must fail (compile errors or test failures) until the implementation exists.

- [x] Create `internal/tool/edit_file_test.go` with `package tool` and required imports (`context`, `os`, `path/filepath`, `strings`, `testing`, `github.com/jrswab/axe/internal/provider`)
- [x] `TestEditFile_SingleReplace` — file `hello.txt` with `"hello world"`, replace `"hello"` → `"goodbye"`, verify file is `"goodbye world"`, Content is `"replaced 1 occurrence(s) in hello.txt"`, CallID matches
- [x] `TestEditFile_ReplaceAll` — file `repeat.txt` with `"aaa"`, replace `"a"` → `"b"` with `replace_all: "true"`, verify file is `"bbb"`, Content is `"replaced 3 occurrence(s) in repeat.txt"`
- [x] `TestEditFile_NotFoundError` — file `target.txt` with `"hello world"`, old_string `"xyz"`, verify IsError true, Content contains `"old_string not found in file"`, file unchanged
- [x] `TestEditFile_MultipleMatchesWithoutReplaceAll` — file `multi.txt` with `"abab"`, old_string `"ab"`, verify IsError true, Content contains `"found 2 times"`, file unchanged
- [x] `TestEditFile_PathTraversalRejected` — path `"../../escape.txt"`, verify IsError true, Content contains `"path escapes workdir"`
- [x] `TestEditFile_AbsolutePathRejected` — path `"/etc/passwd"`, verify IsError true, Content contains `"absolute paths"`
- [x] `TestEditFile_MissingPathArgument` — Arguments has no `path` key, verify IsError true, Content contains `"path is required"`
- [x] `TestEditFile_MissingOldStringArgument` — file `target.txt` exists, Arguments has no `old_string` key, verify IsError true, Content contains `"old_string is required"`
- [x] `TestEditFile_EmptyNewStringDeletesText` — file `delete.txt` with `"remove me please"`, old_string `"remove "`, new_string `""`, verify file is `"me please"`, Content is `"replaced 1 occurrence(s) in delete.txt"`
- [x] `TestEditFile_SymlinkEscapeRejected` — symlink `workdir/link.txt` → `outsideDir/secret.txt`, verify IsError true, outside file unchanged
- [x] `TestEditFile_CallIDPassthrough` — call.ID `"ef-unique-42"`, verify ToolResult.CallID matches
- [x] `TestEditFile_NonexistentFileError` — path `"missing.txt"` (does not exist), verify IsError true
- [x] `TestEditFile_DirectoryPathRejected` — path points to a subdirectory, verify IsError true, Content contains `"path is a directory"`
- [x] `TestEditFile_InvalidReplaceAllValue` — replace_all `"banana"`, verify IsError true, Content contains `"invalid replace_all value"`, file unchanged
- [x] `TestEditFile_MultilineMatch` — file with `"line1\nline2\nline3"`, old_string `"line1\nline2"`, new_string `"replaced"`, verify file is `"replaced\nline3"`
- [x] Add `TestRegisterAll_RegistersEditFile` to `internal/tool/registry_test.go` — verify `r.Has(toolname.EditFile)` after `RegisterAll`
- [x] Add `TestRegisterAll_ResolvesEditFile` to `internal/tool/registry_test.go` — verify resolved tool has Name `toolname.EditFile` and 4 parameters (`path`, `old_string`, `new_string`, `replace_all`)
- [x] Run `make test` — confirm new tests fail (compile errors expected since `editFileEntry` does not exist yet)

## Phase 2: Implementation (Green)

Minimal code to make all tests pass.

- [x] Create `internal/tool/edit_file.go` with `package tool`
- [x] Implement `editFileEntry() ToolEntry` returning `ToolEntry{Definition: editFileDefinition, Execute: editFileExecute}`
- [x] Implement `editFileDefinition() provider.Tool` with Name `toolname.EditFile`, Description, and 4 Parameters (`path`, `old_string`, `new_string`, `replace_all`)
- [x] Implement `editFileExecute(ctx, call, ec) provider.ToolResult`:
  - [x] Extract `path`, call `validatePath(ec.Workdir, path)`, return error ToolResult on failure (Req 2.2, 2.3)
  - [x] `os.Stat` on resolved path, reject directories with `"path is a directory, not a file"` (Req 2.4)
  - [x] Extract `old_string`, return error if empty `"old_string is required"` (Req 2.5)
  - [x] Extract `new_string` (empty is valid) (Req 2.6)
  - [x] `os.ReadFile` the resolved path, return error on failure (Req 2.7)
  - [x] Parse `replace_all` via `strconv.ParseBool`, default false, error on invalid value (Req 2.8)
  - [x] `strings.Count` for occurrences, error if 0 `"old_string not found in file"` (Req 2.9, 2.10)
  - [x] Error if count > 1 and !replaceAll `"old_string found %d times; ..."` (Req 2.11)
  - [x] Perform replacement: `strings.ReplaceAll` or `strings.Replace(..., 1)` (Req 2.12)
  - [x] `os.WriteFile` modified content with `0o644`, return error on failure (Req 2.13)
  - [x] Return success `"replaced %d occurrence(s) in %s"` with correct count (Req 2.14, 2.15)
- [x] Add `r.Register(toolname.EditFile, editFileEntry())` to `RegisterAll` in `internal/tool/registry.go` (Req 3.1)
- [x] Run `make test` — confirm all new tests pass and all existing tests still pass

## Phase 3: Verification

- [x] Run `go build ./internal/tool/` — confirm compilation
- [x] Run `make test` — confirm all tests pass (exit 0)
- [x] Verify no changes to: `go.mod`, `go.sum`, `internal/provider/`, `internal/agent/`, `internal/toolname/`, `cmd/run.go`, `internal/tool/tool.go`, `internal/tool/path_validation.go`
