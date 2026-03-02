# Specification: Tool Call M1 — Agent Config `tools` Field

**Status:** Draft
**Version:** 1.0
**Created:** 2026-03-01
**Scope:** Add a `Tools []string` field to `AgentConfig`, validate entries against a known set, update scaffold template and `agents show` output

---

## 1. Purpose

Enable agents to opt into specific tool capabilities via a `tools` field in their TOML configuration. This is the first milestone of the tool-calling feature set (defined in `docs/plans/000_tool_call_milestones.md`). It establishes the configuration plumbing that subsequent milestones (M2–M8) will build upon.

This milestone does NOT implement tool execution, tool definitions for the LLM, or any changes to `cmd/run.go` dispatch logic. It is purely agent configuration infrastructure.

---

## 2. Design Decisions

The following decisions were made during planning and are binding for implementation:

1. **New shared package `internal/toolname/`:** Tool name constants and the valid-names set live in a new `internal/toolname/` package. This avoids a circular import between `internal/agent` (which needs valid names for validation) and `internal/tool` (which imports `internal/agent` for `agent.Load`).

2. **`call_agent` excluded from `tools` field:** The `call_agent` tool is controlled by the `sub_agents` field, not the `tools` field. The `tools` field governs file/directory/command tools only. This preserves existing behavior and avoids configuration confusion.

3. **Five valid tool names:** The valid tool names for the `tools` field are: `list_directory`, `read_file`, `write_file`, `edit_file`, `run_command`. These correspond to milestones M3–M7. No other names are accepted.

4. **No duplicate validation:** Duplicate entries in `tools` (e.g., `tools = ["read_file", "read_file"]`) are permitted. They are harmless and keeping validation simple is preferred.

5. **No new external dependencies:** This milestone uses only the Go standard library and existing dependencies (`BurntSushi/toml`, `spf13/cobra`).

6. **Field placement:** The `Tools` field is placed after `Workdir` and before `SubAgents` in the `AgentConfig` struct, grouping tool access near working directory configuration.

---

## 3. Requirements

### 3.1 Shared Tool Name Package (`internal/toolname/`)

**Requirement 1.1:** Create a new package `internal/toolname/` with a single file `toolname.go`.

**Requirement 1.2:** Define the following exported string constants:

| Constant | Value |
|----------|-------|
| `CallAgent` | `"call_agent"` |
| `ListDirectory` | `"list_directory"` |
| `ReadFile` | `"read_file"` |
| `WriteFile` | `"write_file"` |
| `EditFile` | `"edit_file"` |
| `RunCommand` | `"run_command"` |

**Requirement 1.3:** Define an exported function:

```go
func ValidNames() map[string]bool
```

This function returns a map containing exactly the five configurable tool names (`list_directory`, `read_file`, `write_file`, `edit_file`, `run_command`) as keys with `true` values. `call_agent` is NOT included in this map.

**Requirement 1.4:** The `internal/toolname/` package must have zero imports from other `internal/` packages. It must be a leaf dependency.

**Requirement 1.5:** Each call to `ValidNames()` must return a new map instance. Callers must be able to modify the returned map without affecting other callers.

### 3.2 Update `internal/tool/tool.go` Constants

**Requirement 2.1:** The existing `CallAgentToolName` constant in `internal/tool/tool.go` (line 19) must be updated to reference the shared constant:

```go
const CallAgentToolName = toolname.CallAgent
```

**Requirement 2.2:** The `internal/tool/` package must add `internal/toolname` to its imports.

**Requirement 2.3:** No other changes to `internal/tool/tool.go` are permitted in this milestone. All existing behavior must remain identical.

### 3.3 Agent Config: `Tools` Field

**Requirement 3.1:** Add a `Tools` field to the `AgentConfig` struct in `internal/agent/agent.go`:

| Go Field | TOML Key | Go Type | Required | Description |
|----------|----------|---------|----------|-------------|
| `Tools` | `tools` | `[]string` | no | Tool names this agent is allowed to use |

**Requirement 3.2:** The field must have the TOML struct tag `` `toml:"tools"` ``.

**Requirement 3.3:** The field must be placed after `Workdir` and before `SubAgents` in the struct definition.

**Requirement 3.4:** When the `tools` key is absent from a TOML file, `cfg.Tools` must be `nil` (Go zero value for a slice).

**Requirement 3.5:** When the `tools` key is present but empty (`tools = []`), `cfg.Tools` must be a non-nil empty slice.

### 3.4 Validation

**Requirement 4.1:** The `Validate` function in `internal/agent/agent.go` must validate the `Tools` field.

**Requirement 4.2:** For each entry in `cfg.Tools`, if the entry is not present in `toolname.ValidNames()`, the `Validate` function must return an error with the message:

```
unknown tool "<name>" in tools config
```

Where `<name>` is the unrecognized tool name string.

**Requirement 4.3:** Validation of tools must occur after the existing validation checks (name, model, sub_agents_config, memory). This maintains the existing fail-fast order: name → model → sub_agents_config → memory → tools.

**Requirement 4.4:** If multiple unknown tools are present, the error must be for the first unknown tool encountered (iteration order of the slice).

**Requirement 4.5:** Empty string entries in the tools list (`tools = [""]`) must be rejected as unknown tools.

**Requirement 4.6:** Duplicate entries are NOT validated. `tools = ["read_file", "read_file"]` is valid.

**Requirement 4.7:** The `internal/agent/` package must add `internal/toolname` to its imports.

### 3.5 Scaffold Template

**Requirement 5.1:** The `Scaffold` function in `internal/agent/agent.go` must include a commented-out `tools` section in the generated TOML template.

**Requirement 5.2:** The tools section must appear after the `# workdir` section and before the `# sub_agents` section.

**Requirement 5.3:** The tools section must contain the following exact lines:

```
# Tools this agent can use (optional)
# Valid: list_directory, read_file, write_file, edit_file, run_command
# tools = []
```

**Requirement 5.4:** The scaffold output must remain valid TOML when all comment lines are removed and placeholder values are replaced.

### 3.6 `agents show` Display

**Requirement 6.1:** The `agentsShowCmd` in `cmd/agents.go` must display the `Tools` field when it is non-empty (length > 0).

**Requirement 6.2:** The display format must be:

```
Tools:          <tool1>, <tool2>, ...
```

Using the same `%-16s` format string and `strings.Join(cfg.Tools, ", ")` pattern as `Files` and `Sub-Agents`.

**Requirement 6.3:** The `Tools:` line must appear after the `Workdir:` line and before the `Sub-Agents:` line.

**Requirement 6.4:** When `cfg.Tools` is nil or empty, the `Tools:` line must NOT appear in the output.

### 3.7 Test Fixture

**Requirement 7.1:** Create a new test fixture file at `cmd/testdata/agents/with_tools.toml` with the following content:

```toml
name = "with_tools"
model = "openai/gpt-4o"
tools = ["read_file", "list_directory"]
```

**Requirement 7.2:** The fixture must parse and validate successfully. The existing `TestFixtureAgents_AllParseAndValidate` test in `cmd/fixture_test.go` must pass with this new fixture included.

---

## 4. Project Structure

After completion, the following files will be added or modified:

```
axe/
├── cmd/
│   ├── agents.go                   # MODIFIED: Tools display in agents show
│   ├── agents_test.go              # MODIFIED: tests for Tools display
│   ├── fixture_test.go             # MODIFIED: test for with_tools fixture
│   └── testdata/
│       └── agents/
│           └── with_tools.toml     # NEW: fixture with tools field
├── internal/
│   ├── toolname/
│   │   ├── toolname.go             # NEW: tool name constants and ValidNames()
│   │   └── toolname_test.go        # NEW: tests for constants and ValidNames()
│   ├── agent/
│   │   ├── agent.go                # MODIFIED: Tools field, validation, scaffold
│   │   └── agent_test.go           # MODIFIED: tests for Tools parsing, validation, scaffold
│   └── tool/
│       ├── tool.go                 # MODIFIED: use toolname.CallAgent constant
│       └── tool_test.go            # UNCHANGED (existing tests must still pass)
├── go.mod                          # UNCHANGED
├── go.sum                          # UNCHANGED
└── ...                             # all other files UNCHANGED
```

---

## 5. Edge Cases

### 5.1 Tools Field Parsing

| Scenario | Behavior |
|----------|----------|
| `tools` key absent from TOML | `cfg.Tools` is `nil` |
| `tools = []` (empty array) | `cfg.Tools` is non-nil empty slice (`[]string{}`) |
| `tools = ["read_file"]` (single entry) | `cfg.Tools` has one element |
| `tools = ["read_file", "write_file", "edit_file", "list_directory", "run_command"]` (all five) | `cfg.Tools` has five elements; all valid |
| `tools = ["read_file", "read_file"]` (duplicates) | Valid; no error |
| `tools = [""]` (empty string entry) | Validation error: `unknown tool "" in tools config` |
| `tools = ["READ_FILE"]` (wrong case) | Validation error: `unknown tool "READ_FILE" in tools config` |
| `tools = ["call_agent"]` | Validation error: `unknown tool "call_agent" in tools config` |
| `tools = ["read_file", "bogus", "write_file"]` | Validation error: `unknown tool "bogus" in tools config` (first unknown) |
| `tools = ["read_file "]` (trailing whitespace) | Validation error: `unknown tool "read_file " in tools config` (no trimming) |

### 5.2 Interaction with Other Fields

| Scenario | Behavior |
|----------|----------|
| Agent has `tools` and `sub_agents` | Both are valid; `tools` controls file/command tools, `sub_agents` controls `call_agent`. No conflict. |
| Agent has `tools` but no `workdir` | Valid; workdir is resolved at runtime (M2+), not at config time. |
| Agent has only `tools`, no other optional fields | Valid; `agents show` displays only `Name:`, `Model:`, and `Tools:`. |

### 5.3 ValidNames() Function

| Scenario | Behavior |
|----------|----------|
| Two callers modify the returned map | Each gets an independent copy; no shared state corruption. |
| `CallAgent` queried in `ValidNames()` | Returns `false`; `call_agent` is not in the valid names set. |

---

## 6. Constraints

**Constraint 1:** No new external dependencies. `go.mod` must remain unchanged.

**Constraint 2:** No changes to `cmd/run.go`. Tool injection and dispatch changes are deferred to M2 (Tool Registry).

**Constraint 3:** No changes to `internal/tool/tool.go` beyond updating the `CallAgentToolName` constant to reference `toolname.CallAgent`. No new functions, types, or exports in `internal/tool/` for this milestone.

**Constraint 4:** No tool definitions for the provider (`provider.Tool` structs). No tool execution logic.

**Constraint 5:** No changes to `--dry-run`, `--json`, or `--verbose` output. Those are M8 scope.

**Constraint 6:** The `internal/toolname/` package must have zero dependencies on other internal packages. This is a hard requirement to prevent circular imports.

**Constraint 7:** All existing tests must continue to pass without modification. New tests must be additive.

**Constraint 8:** Cross-platform compatibility: must build and run on Linux, macOS, and Windows.

---

## 7. Testing Requirements

### 7.1 Test Conventions

Tests must follow the patterns established in prior milestones:

- **Package-level tests:** Tests live in the same package (e.g., `package toolname`, `package agent`, `package cmd`)
- **Standard library only:** Use `testing` package. No test frameworks (no testify, no gomock).
- **Temp directories:** Use `t.TempDir()` for filesystem isolation.
- **Env overrides:** Use `t.Setenv("XDG_CONFIG_HOME", tmpDir)` for XDG path control.
- **Cobra output capture:** Use `rootCmd.SetOut(buf)` / `rootCmd.SetArgs([]string{...})` pattern.
- **Descriptive names:** `TestFunctionName_Scenario` with underscores.
- **Test real code, not mocks.** Tests must call actual functions with real TOML files on disk. Each test must fail if the code under test is deleted.
- **Red/green TDD:** Write failing tests first, then implement code to make them pass.
- **Run tests with:** `make test`

### 7.2 `internal/toolname/toolname_test.go` Tests

**Test: `TestValidNames_ReturnsExpectedCount`** — Call `ValidNames()`, verify the returned map has exactly 5 entries.

**Test: `TestValidNames_ContainsAllExpectedNames`** — Call `ValidNames()`, verify each of the five tool names (`list_directory`, `read_file`, `write_file`, `edit_file`, `run_command`) is present with value `true`.

**Test: `TestValidNames_ExcludesCallAgent`** — Call `ValidNames()`, verify `call_agent` is NOT present in the map.

**Test: `TestValidNames_ReturnsNewMapEachCall`** — Call `ValidNames()` twice, modify the first map, verify the second map is unaffected.

**Test: `TestConstants_Values`** — Verify each constant (`CallAgent`, `ListDirectory`, `ReadFile`, `WriteFile`, `EditFile`, `RunCommand`) has the expected string value.

### 7.3 `internal/agent/agent_test.go` Tests (New)

**Test: `TestLoad_WithTools`** — Write a TOML with `tools = ["read_file", "list_directory"]` to a temp dir, call `Load`, verify `cfg.Tools` equals `[]string{"read_file", "list_directory"}`.

**Test: `TestLoad_WithoutTools`** — Write a TOML with no `tools` key, call `Load`, verify `cfg.Tools` is `nil`.

**Test: `TestLoad_WithEmptyTools`** — Write a TOML with `tools = []`, call `Load`, verify `cfg.Tools` is non-nil and has length 0.

**Test: `TestValidate_ValidTools`** — Create an `AgentConfig` with `Tools: []string{"read_file", "write_file"}`, call `Validate`, verify no error.

**Test: `TestValidate_UnknownTool`** — Create an `AgentConfig` with `Tools: []string{"read_file", "bogus"}`, call `Validate`, verify error message is `unknown tool "bogus" in tools config`.

**Test: `TestValidate_CallAgentInTools`** — Create an `AgentConfig` with `Tools: []string{"call_agent"}`, call `Validate`, verify error message is `unknown tool "call_agent" in tools config`.

**Test: `TestValidate_EmptyStringTool`** — Create an `AgentConfig` with `Tools: []string{""}`, call `Validate`, verify error message is `unknown tool "" in tools config`.

**Test: `TestValidate_EmptyToolsSlice`** — Create an `AgentConfig` with `Tools: []string{}`, call `Validate`, verify no error.

**Test: `TestValidate_NilToolsSlice`** — Create an `AgentConfig` with `Tools: nil`, call `Validate`, verify no error.

**Test: `TestValidate_AllFiveTools`** — Create an `AgentConfig` with all five valid tool names, call `Validate`, verify no error.

**Test: `TestScaffold_ContainsToolsComment`** — Call `Scaffold("test")`, verify the output contains `# tools = []`.

**Test: `TestScaffold_ContainsValidToolNames`** — Call `Scaffold("test")`, verify the output contains `# Valid: list_directory, read_file, write_file, edit_file, run_command`.

**Test: `TestScaffold_ToolsBeforeSubAgents`** — Call `Scaffold("test")`, verify the `# tools = []` line appears before the `# sub_agents = []` line in the output.

### 7.4 `cmd/agents_test.go` Tests (New)

**Test: `TestAgentsShow_WithTools`** — Write an agent TOML with `tools = ["read_file", "write_file"]`, run `axe agents show <name>`, verify output contains `Tools:` and `read_file, write_file`.

**Test: `TestAgentsShow_WithoutTools`** — Write an agent TOML without `tools`, run `axe agents show <name>`, verify output does NOT contain `Tools:`.

**Test: `TestAgentsShow_ToolsDisplayOrder`** — Write an agent TOML with `tools`, `workdir`, and `sub_agents`. Run `axe agents show <name>`, verify `Tools:` appears after `Workdir:` and before `Sub-Agents:` in the output.

### 7.5 `cmd/fixture_test.go` Tests (New)

**Test: `TestFixtureAgents_WithToolsConfig`** — Read `testdata/agents/with_tools.toml`, decode and validate. Verify `cfg.Tools` has exactly 2 entries: `"read_file"` and `"list_directory"`.

### 7.6 Existing Tests

All existing tests in the following files must continue to pass without modification:

- `internal/tool/tool_test.go`
- `internal/agent/agent_test.go` (existing tests)
- `cmd/agents_test.go` (existing tests)
- `cmd/fixture_test.go` (existing test `TestFixtureAgents_AllParseAndValidate` must pass with the new fixture added)
- All other test files in the project

### 7.7 Running Tests

All tests must pass when run with:

```bash
make test
```

---

## 8. Acceptance Criteria

| Criterion | Verification |
|-----------|-------------|
| `internal/toolname/` package exists | `go build ./internal/toolname/` succeeds |
| Constants have correct values | `TestConstants_Values` passes |
| `ValidNames()` returns exactly 5 entries | `TestValidNames_ReturnsExpectedCount` passes |
| `ValidNames()` excludes `call_agent` | `TestValidNames_ExcludesCallAgent` passes |
| `ValidNames()` returns independent maps | `TestValidNames_ReturnsNewMapEachCall` passes |
| `tool.CallAgentToolName` uses shared constant | `internal/tool/tool_test.go` all pass (no behavior change) |
| `AgentConfig.Tools` field exists with TOML tag | `TestLoad_WithTools` passes |
| Absent `tools` key yields `nil` | `TestLoad_WithoutTools` passes |
| Empty `tools` key yields empty slice | `TestLoad_WithEmptyTools` passes |
| Valid tool names pass validation | `TestValidate_ValidTools` passes |
| Unknown tool names fail validation | `TestValidate_UnknownTool` passes |
| `call_agent` rejected in `tools` | `TestValidate_CallAgentInTools` passes |
| Empty string rejected in `tools` | `TestValidate_EmptyStringTool` passes |
| Scaffold includes tools comment | `TestScaffold_ContainsToolsComment` passes |
| Scaffold lists valid tool names | `TestScaffold_ContainsValidToolNames` passes |
| `agents show` displays tools | `TestAgentsShow_WithTools` passes |
| `agents show` hides tools when empty | `TestAgentsShow_WithoutTools` passes |
| Fixture file parses and validates | `TestFixtureAgents_AllParseAndValidate` passes |
| Fixture has expected tools | `TestFixtureAgents_WithToolsConfig` passes |
| All existing tests pass | `make test` exits 0 |

---

## 9. Out of Scope

The following items are explicitly **not** included in this milestone:

1. Tool registry (`internal/tool/registry.go`) — M2 scope
2. Tool execution logic — M3–M7 scope
3. Tool definitions for the provider (`provider.Tool` structs) — M2 scope
4. Changes to `cmd/run.go` tool injection or dispatch — M2 scope
5. Changes to `--dry-run`, `--json`, or `--verbose` output — M8 scope
6. Validation that tool names correspond to implemented tools (all five are valid even though M3–M7 are not yet built)
7. Runtime behavior changes when `tools` is populated (no tools are actually executed)
8. Any changes to the `Provider` interface or provider implementations
9. Validation of duplicate tool entries
10. Trimming whitespace from tool name strings

---

## 10. References

- Milestone Definition: `docs/plans/000_tool_call_milestones.md` (M1 section)
- Agent Config Spec: `docs/plans/002_agent_config_spec.md`
- Sub-Agents Spec: `docs/plans/005_sub_agents_spec.md`
- Current `AgentConfig` struct: `internal/agent/agent.go:36-48`
- Current `Validate` function: `internal/agent/agent.go:53-76`
- Current `Scaffold` function: `internal/agent/agent.go:149-187`
- Current `agents show` command: `cmd/agents.go:48-109`
- Current `CallAgentToolName` constant: `internal/tool/tool.go:19`
- Circular import: `internal/tool/tool.go:10` imports `internal/agent`

---

## 11. Notes

- The `internal/toolname/` package is intentionally minimal. It exists solely to break the circular import between `internal/agent` and `internal/tool`. It contains no logic beyond returning a map of string constants.
- The five tool names are defined now even though their implementations (M3–M7) do not exist yet. This allows users to configure agents ahead of time. When a tool is referenced in config but not yet implemented, it will be resolved and dispatched in M2 (which will surface appropriate errors for unimplemented tools).
- The `call_agent` constant is included in `internal/toolname/` for reference by `internal/tool/tool.go`, but it is explicitly excluded from `ValidNames()` because it is not user-configurable via the `tools` field.
- Validation does not trim whitespace from tool name strings. `"read_file "` (with trailing space) is rejected as unknown. This is intentional: TOML string parsing already handles surrounding whitespace, so any whitespace in the parsed value is part of the string and indicates a user error.
- The `Parallel` field on `SubAgentsConfig` is `*bool` to distinguish "not set" from "set to false". The `Tools` field does not need this pattern because `nil` vs empty slice is sufficient to distinguish "not set" from "set to empty".
