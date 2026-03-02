# 020 — Manual Testing Fixes Implementation

Reference: `020_manual_testing_fixes_spec.md`

---

## Section 1: Helpful Argument Validation Errors

### RED — Write failing tests first

- [x] Create `cmd/args_test.go` with `TestExactArgs_MissingArg_IncludesUsageHint`
- [x] Add `TestExactArgs_TooManyArgs_IncludesUsageHint` to `cmd/args_test.go`
- [x] Add `TestExactArgs_CorrectArgs_NoError` to `cmd/args_test.go`
- [x] Add `TestExactArgs_NoPlaceholder_FallbackName` to `cmd/args_test.go`
- [x] Add `TestExactArgs_RunCommand_MissingArg` to `cmd/args_test.go`
- [x] Add `TestExactArgs_AgentsShowCommand_MissingArg` to `cmd/args_test.go`
- [x] Run tests — confirm all 6 new tests fail (RED)

### GREEN — Implement the validator

- [x] Create `cmd/args.go` with `exactArgs(n int) cobra.PositionalArgs`
- [x] Change `Use: "run"` to `Use: "run <agent>"` in `cmd/run.go`
- [x] Replace `cobra.ExactArgs(1)` with `exactArgs(1)` in `cmd/run.go`
- [x] Replace `cobra.ExactArgs(1)` with `exactArgs(1)` on `agentsShowCmd` in `cmd/agents.go`
- [x] Replace `cobra.ExactArgs(1)` with `exactArgs(1)` on `agentsInitCmd` in `cmd/agents.go`
- [x] Replace `cobra.ExactArgs(1)` with `exactArgs(1)` on `agentsEditCmd` in `cmd/agents.go`
- [x] Run tests — confirm all 6 new tests pass (GREEN)

### Update existing tests

- [x] Update `TestRunE_ErrorNotPrintedByCobra` in `cmd/root_test.go` — change `"accepts 1 arg(s)"` assertion to `"missing required argument"`
- [x] `TestRunE_ErrorDoesNotPrintUsage` — no update needed (stderr is empty due to SilenceErrors; test passes as-is)
- [x] Add error message assertion to `TestAgentsShow_NoArgs` in `cmd/agents_test.go`
- [x] Add error message assertion to `TestAgentsInit_NoArgs` in `cmd/agents_test.go`
- [x] Add error message assertion to `TestAgentsEdit_NoArgs` in `cmd/agents_test.go`
- [x] Run `make test` — confirm all existing + new tests pass

---

## Section 2: Smart Skill Path Resolution

### RED — Write failing tests first

- [x] Add `TestSkill_DirectoryAutoResolvesSKILLMD` to `internal/resolve/resolve_test.go`
- [x] Add `TestSkill_BareNameResolvesToSkillsDir` to `internal/resolve/resolve_test.go`
- [x] Add `TestSkill_AbsoluteDirectoryAutoResolvesSKILLMD` to `internal/resolve/resolve_test.go`
- [x] Add `TestSkill_BareNameNotFound` to `internal/resolve/resolve_test.go`
- [x] Add `TestSkill_BareNameFileInConfigDir` to `internal/resolve/resolve_test.go`
- [x] Run tests — confirm 3 of 5 new tests fail (RED); 2 already pass with existing code

### GREEN — Implement the fallback chain

- [x] Rewrite `Skill()` in `internal/resolve/resolve.go` with 3-step fallback chain
- [x] Run new skill tests — confirm all 5 pass (GREEN)

### Update existing tests

- [x] Update `TestSkill_NotFound` expected error string from `"skill not found: ..."` to `"skill not found: tried ..."`
- [x] Run `make test` — confirm all existing + new tests pass

---

## Final Verification

- [x] Run `make test` — full suite passes with zero failures
