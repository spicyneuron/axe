# 021 — Path Expansion Implementation Checklist

Spec: `021_path_expansion_spec.md`

---

## Phase 1: Core Function (TDD)

- [x] **1.1** Write `TestExpandPath` in `internal/resolve/resolve_test.go` — table-driven, 12 cases per spec section 4a: `empty`, `tilde_slash`, `bare_tilde`, `env_var`, `braced_env_var`, `tilde_and_env`, `absolute`, `relative`, `tilde_user`, `mid_tilde`, `unset_var`, `no_vars`. Run tests — confirm all 12 fail.
- [x] **1.2** Implement `ExpandPath(path string) (string, error)` in `internal/resolve/resolve.go`. Tilde expansion first (`os.UserHomeDir`), then `os.ExpandEnv`. Run tests — confirm all 12 pass.

## Phase 2: Update `Workdir` Signature (TDD)

- [x] **2.1** Update existing `TestWorkdir_FlagOverride`, `TestWorkdir_TOMLFallback`, `TestWorkdir_CWDFallback` in `internal/resolve/resolve_test.go` to expect `(string, error)` return. Write new `TestWorkdir_TildeExpansion` and `TestWorkdir_EnvVarExpansion`. Run tests — confirm new tests fail, updated tests fail to compile.
- [x] **2.2** Change `Workdir` signature in `internal/resolve/resolve.go` from `func Workdir(flagValue, tomlValue string) string` to `func Workdir(flagValue, tomlValue string) (string, error)`. Call `ExpandPath` on the selected raw path before returning. Run resolve tests — confirm all pass.

## Phase 3: Update `Workdir` Callers

- [x] **3.1** Update `cmd/run.go:104` — change `workdir := resolve.Workdir(...)` to `workdir, err := resolve.Workdir(...)` with error handling returning `&ExitError{Code: 2, Err: err}`.
- [x] **3.2** Update `internal/tool/tool.go:142` (`ExecuteCallAgent`) — change `workdir := resolve.Workdir(...)` to `workdir, err := resolve.Workdir(...)` with error handling returning an error result via `errorResult`.
- [x] **3.3** Update `internal/tool/tool.go:302` (`runConversationLoop`) — change `toolWorkdir := resolve.Workdir(...)` to `toolWorkdir, err := resolve.Workdir(...)` with error handling returning `nil, err`.
- [x] **3.4** Run `make test` — confirm all existing tests compile and pass.

## Phase 4: Integrate into `Skill` (TDD)

- [x] **4.1** Write `TestSkill_EnvVarInPath` (env var expansion approach per spec section 4c) in `internal/resolve/resolve_test.go`. Run tests — confirm it fails.
- [x] **4.2** Add `ExpandPath(skillPath)` call at the top of `Skill()` in `internal/resolve/resolve.go`, before the `filepath.IsAbs` check. Run tests — confirm new test passes and all existing skill tests still pass.

## Phase 5: Integrate into `Files`

- [x] **5.1** Add `ExpandPath` call on each pattern inside the `for` loop in `Files()` (`internal/resolve/resolve.go`), before the `strings.Contains(pattern, "**")` check. Run tests — confirm all existing file tests still pass (patterns like `"*.txt"` have no expansion triggers).

## Phase 6: Integrate into `memory.FilePath` (TDD)

- [x] **6.1** Write `TestFilePath_TildeExpansion` and `TestFilePath_EnvVarExpansion` in `internal/memory/memory_test.go`. Run tests — confirm both fail.
- [x] **6.2** In `memory.FilePath()` (`internal/memory/memory.go`), call `resolve.ExpandPath(customPath)` when `customPath` is non-empty. Add `resolve` import. Run tests — confirm new tests pass and all existing memory tests still pass.

## Phase 7: Final Verification

- [x] **7.1** Run `make test` — all tests pass with zero failures.
- [x] **7.2** Verify acceptance criteria from spec section 5 (spot-check key behaviors).
