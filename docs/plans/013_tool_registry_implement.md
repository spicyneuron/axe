# Implementation: Tool Call M2 — Tool Registry

**Spec:** `docs/plans/013_tool_registry_spec.md`
**Status:** Complete
**Created:** 2026-03-01

---

## Implementation Tasks

Tasks are ordered by dependency. Each task is completable in one TDD iteration (write failing test, then implement). All tests run with `make test`.

### Phase 1: Registry Core Types & Constructor (`internal/tool/registry.go`)

- [x] **1.1** Create `internal/tool/registry_test.go` with `TestNewRegistry` — call `NewRegistry()`, verify the returned value is non-nil. Test will not compile yet.
- [x] **1.2** Create `internal/tool/registry.go` with `ExecContext` struct (`Workdir string`, `Stderr io.Writer`, `Verbose bool`), `ToolEntry` struct (`Definition func() provider.Tool`, `Execute func(ctx context.Context, call provider.ToolCall, ec ExecContext) provider.ToolResult`), `Registry` struct (unexported `entries map[string]ToolEntry`), and `NewRegistry() *Registry` returning a registry with an empty initialized map. Verify `TestNewRegistry` passes.

### Phase 2: `Register` and `Has` Methods

- [x] **2.1** Add `TestRegistry_Register_And_Has` to `registry_test.go` — register a tool entry, verify `Has` returns `true` for the registered name and `false` for an unregistered name. Verify `Has` returns `false` before registration. Test fails (methods don't exist).
- [x] **2.2** Implement `Register(name string, entry ToolEntry)` and `Has(name string) bool` on `*Registry`. Verify `TestRegistry_Register_And_Has` passes.

### Phase 3: `Resolve` Method

- [x] **3.1** Add `TestRegistry_Resolve_KnownTools` to `registry_test.go` — register two tools with known `Definition` functions returning `provider.Tool` with distinct `Name` fields. Call `Resolve` with both names. Verify the returned slice has 2 entries with correct `Name` fields in input order. Test fails.
- [x] **3.2** Add `TestRegistry_Resolve_UnknownTool` to `registry_test.go` — register one tool. Call `Resolve` with that tool plus an unknown name. Verify an error is returned containing `unknown tool`. Test fails.
- [x] **3.3** Add `TestRegistry_Resolve_Empty` to `registry_test.go` — call `Resolve` with an empty slice, verify empty non-nil slice and nil error. Repeat with nil, same result. Test fails.
- [x] **3.4** Implement `Resolve(names []string) ([]provider.Tool, error)` on `*Registry`. For each name, look up entry and call `entry.Definition()`. Return error `unknown tool "<name>"` for first unrecognized name. Return empty non-nil slice for nil/empty input. Verify all Resolve tests pass.

### Phase 4: `Dispatch` Method

- [x] **4.1** Add `TestRegistry_Dispatch_KnownTool` to `registry_test.go` — register a tool with a working `Execute` that reads `call.Arguments` and returns a `ToolResult` with specific `CallID` and `Content`. Call `Dispatch` with a matching `ToolCall`. Verify returned `ToolResult` has correct `CallID` and `Content`, and error is nil. Test fails.
- [x] **4.2** Add `TestRegistry_Dispatch_UnknownTool` to `registry_test.go` — call `Dispatch` on an empty registry. Verify a non-nil error is returned containing `unknown tool`. Test fails.
- [x] **4.3** Add `TestRegistry_Dispatch_NilExecutor` to `registry_test.go` — register a tool with `Execute: nil`. Call `Dispatch`. Verify a non-nil error is returned containing `has no executor`. Test fails.
- [x] **4.4** Add `TestRegistry_Dispatch_PassesExecContext` to `registry_test.go` — register a tool whose `Execute` captures the `ExecContext` it receives. Call `Dispatch` with specific `ExecContext` values (`Workdir`, `Verbose`). Verify the executor received the correct values. Test fails.
- [x] **4.5** Implement `Dispatch(ctx context.Context, call provider.ToolCall, ec ExecContext) (provider.ToolResult, error)` on `*Registry`. Look up `call.Name`; if not found return error `unknown tool "<name>"`; if `Execute` is nil return error `tool "<name>" has no executor`; otherwise call `entry.Execute(ctx, call, ec)` and return result with nil error. Verify all Dispatch tests pass.

### Phase 5: Verify Registry in Isolation

- [x] **5.1** Run `go build ./internal/tool/` — verify the package compiles with no errors.
- [x] **5.2** Run `make test` — verify all existing tests still pass alongside the new registry tests.

### Phase 6: Refactor `internal/tool/tool.go` — `runConversationLoop` Signature

- [x] **6.1** Update `runConversationLoop` signature to add `registry *Registry` parameter after `cfg` and before `depth`: `func runConversationLoop(ctx context.Context, prov provider.Provider, req *provider.Request, cfg *agent.AgentConfig, registry *Registry, depth int, opts ExecuteOptions) (*provider.Response, error)`. Code will not compile yet.
- [x] **6.2** Update the call to `runConversationLoop` in `ExecuteCallAgent` (line ~245) to pass a new `NewRegistry()` as the registry argument. Code compiles again.
- [x] **6.3** Run `make test` — verify all existing `tool_test.go` tests still pass (they exercise `ExecuteCallAgent` which internally calls `runConversationLoop`).

### Phase 7: Refactor `internal/tool/tool.go` — `runConversationLoop` Dispatch Logic

- [x] **7.1** In `runConversationLoop`'s tool dispatch loop, change the `else` branch from "Unknown tool" error to `registry.Dispatch(ctx, tc, execContext)`. Construct `ExecContext` with `Workdir: resolve.Workdir("", cfg.Workdir)`, `Stderr: opts.Stderr`, `Verbose: opts.Verbose`. If `Dispatch` returns an error, produce a `ToolResult` with `CallID: tc.ID`, `Content: err.Error()`, `IsError: true`. The `call_agent` branch remains unchanged.
- [x] **7.2** Run `make test` — verify all existing tests still pass. No behavioral change since registry is empty and `call_agent` is special-cased.

### Phase 8: Refactor `cmd/run.go` — `executeToolCalls` Signature

- [x] **8.1** Update `executeToolCalls` signature to add `registry *tool.Registry` and `workdir string` parameters. New signature: `func executeToolCalls(ctx context.Context, toolCalls []provider.ToolCall, cfg *agent.AgentConfig, globalCfg *config.GlobalConfig, registry *tool.Registry, depth, maxDepth int, parallel, verbose bool, stderr io.Writer, workdir string) []provider.ToolResult`. Code will not compile yet.
- [x] **8.2** Update the call site in `runAgent` (the conversation loop, line ~332) to pass the registry and the resolved `workdir` to `executeToolCalls`. Registry and workdir creation will be added in Phase 9; for now, pass `tool.NewRegistry()` and `workdir` (already resolved at line 104). Code compiles again.
- [x] **8.3** Run `make test` — verify all existing tests still pass.

### Phase 9: Refactor `cmd/run.go` — Tool Injection via Registry

- [x] **9.1** In `runAgent`, after loading the agent config and resolving the workdir, create a registry via `tool.NewRegistry()`. If `cfg.Tools` is non-empty, call `registry.Resolve(cfg.Tools)` to get tool definitions. If `Resolve` returns an error, return `ExitError{Code: 1}` with message `fmt.Errorf("failed to resolve tools: %w", err)`.
- [x] **9.2** Append the resolved `[]provider.Tool` to `req.Tools` before the existing `call_agent` injection logic. Ensure tool injection order: configured tools first, then `call_agent` (if applicable).
- [x] **9.3** Update the `executeToolCalls` call site (if not already done in 8.2) to pass the registry created in 9.1 instead of `tool.NewRegistry()`.
- [x] **9.4** Run `make test` — verify all existing tests still pass.

### Phase 10: Refactor `cmd/run.go` — `executeToolCalls` Dispatch Logic

- [x] **10.1** In the sequential branch of `executeToolCalls`, change the `else` (non-`call_agent`) branch from "Unknown tool" error to `registry.Dispatch(ctx, tc, tool.ExecContext{Workdir: workdir, Stderr: stderr, Verbose: verbose})`. If `Dispatch` returns an error, produce `ToolResult{CallID: tc.ID, Content: err.Error(), IsError: true}`.
- [x] **10.2** In the parallel branch of `executeToolCalls`, apply the same change: replace "Unknown tool" error with `registry.Dispatch` + error-to-`ToolResult` conversion.
- [x] **10.3** Run `make test` — verify all existing tests still pass. No behavioral change since the registry is empty and `call_agent` is still special-cased.

### Phase 11: Final Verification

- [x] **11.1** Run `make test` — all tests pass (zero failures).
- [x] **11.2** Run `go build ./...` — full project compiles with no errors.
- [x] **11.3** Verify `go.mod` and `go.sum` are unchanged (no new dependencies).
- [x] **11.4** Verify `call_agent` dispatch is unchanged at all three sites (still special-cased, not in registry).
