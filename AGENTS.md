# Axe — AGENTS.md

## What This Is

A lightweight CLI for running single-purpose LLM agents. Think `make` for AI — define agents in TOML, give them focused context via SKILL.md files, and trigger them from anywhere (cron, git hooks, pipes, file watchers).

## Design Philosophy

- **Small context windows by design.** Each agent gets only what it needs. Sub-agents return results, not conversations. This is the core value prop — fight context bloat at every level.
- **Unix citizen.** Stdout is clean and pipeable. Debug goes to stderr. Exit codes are meaningful. Stdin is always accepted. Axe composes with existing tools, it doesn't replace them.
- **Executor, not scheduler.** Axe runs agents. Triggering is the user's job (cron, entr, git hooks, webhooks). No built-in daemon, no watch mode, no event loop.
- **Single binary, zero runtime.** Go. No interpreters, no node_modules, no Docker required. `go build` and ship.
- **Config over code.** Agent definitions are TOML, not Go. Users should never touch source to define or modify agents.

## Non-Obvious Constraints

- **SKILL.md is a community format** — not invented here. Treat it as an external standard. Don't add axe-specific extensions to the format itself; put axe-specific config in TOML.
- **models.dev format for model strings** — always `provider/model-name`. Don't invent a custom format.
- **XDG Base Directory spec** — config, data, and cache go where XDG says. No dotfiles in $HOME.
- **Sub-agents are opaque to parents.** A parent never sees a sub-agent's internal turns, tool calls, or files. Only the final text result crosses the boundary. This is intentional — don't leak internals upward.
- **Depth limits are safety rails, not features.** Default 3, hard max 5. If someone needs more depth, the design is wrong, not the limit.

## What Good Contributions Look Like

- Tests for every new package (table-driven, no mocks when avoidable — use the real thing or `testutil` helpers)
- Errors that help the user fix the problem, not just describe it
- No global state — pass dependencies explicitly
- Flags override TOML overrides defaults. Resolution order matters everywhere.
- When in doubt, print nothing. Axe output should be safe to pipe.

## Project Docs

Design docs live in `../projects/axe/docs/` — check milestones, config schema, sub-agent patterns, and CLI structure there before making architectural changes.
