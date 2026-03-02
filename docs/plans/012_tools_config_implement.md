# Implementation: Tool Call M1 — Agent Config `tools` Field

**Spec:** `docs/plans/012_tools_config_spec.md`
**Status:** Complete
**Created:** 2026-03-01

---

## Implementation Tasks

Tasks are ordered by dependency. Each task is completable in one TDD iteration (write failing test, then implement).

### Phase 1: Shared Tool Name Package (`internal/toolname/`)

- [x] **1.1** Create `internal/toolname/toolname_test.go` with `TestConstants_Values` — assert each constant (`CallAgent`, `ListDirectory`, `ReadFile`, `WriteFile`, `EditFile`, `RunCommand`) equals its expected string value. Test will not compile yet.
- [x] **1.2** Create `internal/toolname/toolname.go` with package declaration, all six exported string constants. Verify `TestConstants_Values` passes.
- [x] **1.3** Add tests `TestValidNames_ReturnsExpectedCount`, `TestValidNames_ContainsAllExpectedNames`, `TestValidNames_ExcludesCallAgent`, `TestValidNames_ReturnsNewMapEachCall` to `toolname_test.go`. Tests fail (function doesn't exist).
- [x] **1.4** Implement `ValidNames() map[string]bool` in `toolname.go` returning a fresh map of the five configurable names. Verify all `toolname_test.go` tests pass.

### Phase 2: Update `internal/tool/tool.go` Constant

- [x] **2.1** Update `CallAgentToolName` in `internal/tool/tool.go` to `toolname.CallAgent` and add the import. Run existing `internal/tool/tool_test.go` tests to verify no regression.

### Phase 3: Agent Config `Tools` Field & Validation

- [x] **3.1** Add tests `TestLoad_WithTools`, `TestLoad_WithoutTools`, `TestLoad_WithEmptyTools` to `internal/agent/agent_test.go`. Tests fail (field doesn't exist).
- [x] **3.2** Add `Tools []string` field (with `toml:"tools"` tag) to `AgentConfig` struct in `internal/agent/agent.go`, placed after `Workdir` and before `SubAgents`. Verify the three Load tests pass.
- [x] **3.3** Add validation tests `TestValidate_ValidTools`, `TestValidate_UnknownTool`, `TestValidate_CallAgentInTools`, `TestValidate_EmptyStringTool`, `TestValidate_EmptyToolsSlice`, `TestValidate_NilToolsSlice`, `TestValidate_AllFiveTools` to `internal/agent/agent_test.go`. Invalid-tool tests fail (no validation yet).
- [x] **3.4** Add tools validation logic to `Validate()` in `internal/agent/agent.go` — iterate `cfg.Tools`, check each against `toolname.ValidNames()`, return `fmt.Errorf("unknown tool %q in tools config", name)` on first unknown. Add `internal/toolname` import. Verify all validation tests pass.

### Phase 4: Scaffold Template

- [x] **4.1** Add tests `TestScaffold_ContainsToolsComment`, `TestScaffold_ContainsValidToolNames`, `TestScaffold_ToolsBeforeSubAgents` to `internal/agent/agent_test.go`. Tests fail (scaffold doesn't include tools section).
- [x] **4.2** Update `Scaffold()` in `internal/agent/agent.go` to include the tools comment block after the `# workdir` section and before the `# sub_agents` section. Verify scaffold tests pass.

### Phase 5: `agents show` Display

- [x] **5.1** Add tests `TestAgentsShow_WithTools`, `TestAgentsShow_WithoutTools`, `TestAgentsShow_ToolsDisplayOrder` to `cmd/agents_test.go`. Tests fail (no tools display).
- [x] **5.2** Update `agentsShowCmd` in `cmd/agents.go` to display `Tools:` line (using `%-16s` format and `strings.Join`) after `Workdir:` and before `Sub-Agents:`, only when `len(cfg.Tools) > 0`. Verify agents show tests pass.

### Phase 6: Test Fixture

- [x] **6.1** Create `cmd/testdata/agents/with_tools.toml` with content: `name = "with_tools"`, `model = "openai/gpt-4o"`, `tools = ["read_file", "list_directory"]`.
- [x] **6.2** Add test `TestFixtureAgents_WithToolsConfig` to `cmd/fixture_test.go` — read and decode `with_tools.toml`, verify `cfg.Tools` has exactly 2 entries (`"read_file"`, `"list_directory"`). Verify test passes.
- [x] **6.3** Run existing `TestFixtureAgents_AllParseAndValidate` to verify the new fixture is picked up and passes.

### Phase 7: Final Verification

- [x] **7.1** Run `make test` — all tests pass (zero failures).
- [x] **7.2** Run `go build ./internal/toolname/` — confirm package compiles.
- [x] **7.3** Verify `go.mod` and `go.sum` are unchanged (no new dependencies).
