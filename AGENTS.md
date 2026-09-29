# Agent Instructions — Perceptrail

This file is the canonical agent instruction file for the **Perceptrail**
repository (`gontroller` Go backend, `svebapp` Svelte gallery, `perceptors`
plugins, `perceplib`). Codex reads this file from the repository root.
`CLAUDE.md`, `GEMINI.md`, `.gemini/settings.json`, and
`.cursor/rules/agents.mdc` all point here instead of duplicating these rules —
keep durable rules in this file only.

Canonical MCP server config: [`.mcp.json`](.mcp.json). Cursor / Gemini / Codex
need format-local mirrors (`.cursor/mcp.json`, `.gemini/settings.json`,
`.codex/config.toml`) — edit `.mcp.json` first, then keep those mirrors in sync.
No symlinks (Windows).

## Project knowledge — read before changing things

- [`_sb/docs/findings.md`](_sb/docs/findings.md) — what was already tried, what
  broke and why, decisions taken. Read the relevant section BEFORE working on the
  gallery layout / resize / streaming, Go plugins, exiftool, or the import
  pipeline. Do not re-propose approaches it lists as rejected without new reasons.
- [`_sb/docs/roadmap.md`](_sb/docs/roadmap.md) — status, next steps, and what
  belongs in the core vs. in perceptors.
- [`_sb/puml`](_sb/puml) — design diagrams (target architecture: import chain,
  file validation, client/ML event flow, workers, protocol). Check them before
  redesigning a flow; if the code deviates from a diagram, say so in findings.
- Module READMEs: `gontroller/readme.md`, `perceplib/README.md`,
  `perceptors/readme.md`, `svebapp/README.md`.
- When you learn something non-obvious (a root cause, a dead end, a decision),
  add a dated entry to `findings.md` and update the roadmap.

## Git workflow (git flow)

- Long-lived branches: `master` (releases) and `develop` (integration). Never
  commit to them directly.
- Work happens in `feature/<name>` branches created from `develop`.
- `feature/*` → `develop`: a pull request, merged with **squash**. Each merge into
  `develop` is a patch increment `0.0.x`. Several PRs accumulate in `develop`.
- `develop` → `master`: a pull request, merged with **rebase** (no merge commit).
  Each merge into `master` is a minor increment `0.x.0`.
- Open PRs against `develop` unless the task is the `develop` → `master` release.
- Version: one for the whole monorepo, in the root [`VERSION`](VERSION), synced into
  `svebapp/package.json` (+ lock) and `gontroller/pkg/app/version.go`. Bump it in the
  PR itself: `scripts/version.sh patch` in every feature PR, `scripts/version.sh minor`
  in the `develop` → `master` PR. CI fails if the files are out of sync
  (`scripts/version.sh check`). `perceplib` is versioned separately by its own tags.
- `develop` stays linear (squash merges only), which is what makes the rebase into
  `master` possible — GitHub cannot rebase a PR that contains merge commits.
  Operations that create merge commits (e.g. `git subtree pull` for `perceplib`)
  are done in a feature branch and get squashed on the way into `develop`.
- Enforced by GitHub **rulesets** only (no classic branch protection): per branch —
  PR with 1 approval and resolved threads, allowed merge method (`develop`: squash,
  `master`: rebase), required checks `go` + `svebapp` (branch up to date), linear
  history, no force-push, no deletion. Repository admins may bypass only when
  merging a PR; direct pushes to `master`/`develop` are rejected for everyone.


---

# MCP Tools

You have access to MCP servers for Svelte, Go, and design-pattern research.
Use them proactively, not only when the user explicitly asks.

### Svelte MCP

#### 1. list-sections
Use this FIRST to discover all available documentation sections. Returns a
structured list with titles, use_cases, and paths. When asked about Svelte
or SvelteKit topics, ALWAYS use this tool at the start of the chat to find
relevant sections.

#### 2. get-documentation
Retrieves full documentation content for specific sections. Accepts single
or multiple sections. After calling list-sections, you MUST analyze the
returned sections (especially use_cases) and then use get-documentation to
fetch ALL sections relevant to the user's task.

#### 3. svelte-autofixer
Analyzes Svelte code and returns issues and suggestions. You MUST use this
tool whenever writing Svelte code before sending it to the user. Keep
calling it until no issues or suggestions are returned.

#### 4. playground-link
Generates a Svelte Playground link with the provided code. After completing
the code, ask the user if they want a playground link. Only call this tool
after user confirmation and NEVER if code was written to files in their
project.

### Go MCP (gopls)

#### 5. go_diagnostics
Runs compile-time and static analysis checks on Go files. Returns errors
and warnings. You MUST call this tool on every Go file you create or
modify, BEFORE marking the task complete. Fix all reported issues and
re-run until clean.

#### 6. go_vulncheck
Scans dependencies for known vulnerabilities affecting the code paths you
touched. You MUST call this tool after any change to go.mod or go.sum, or
when adding a new import from an external package.

#### 7. go_references / go_symbol_references
Finds all usages of a function, type, or symbol across the workspace. You
MUST use this tool BEFORE modifying or removing any exported symbol, to
confirm you understand every call site. Also use it BEFORE writing new
code: search for existing symbols that solve a similar problem — if
something similar exists, extend it instead of writing a duplicate.

#### 8. go_package_api
Shows the public API surface of a package without reading all its source
files. Use this when working in an unfamiliar package to understand its
existing abstractions before adding new ones.

### Design Patterns MCP

Installed at `$AGENTS_TOOLS_DIR/design_patterns_mcp` (`AGENTS_TOOLS_DIR`
defaults to `$HOME/.agents`). This tool is OPTIONAL — it may not be
available on every machine that runs this agent. If it is NOT in your tool
list, fall back to the neighboring-files check from the "Code Style"
section below — do not treat its absence as a task blocker, and do not
report it as an error to the user. If it IS present, using it before
writing a non-trivial new abstraction is REQUIRED, not optional — see below.

#### 9. find_patterns
Hybrid search (semantic + keyword) over 700+ documented design patterns
given a natural-language problem description. If this tool is present in
your tool list, you MUST use it BEFORE writing any non-trivial new
abstraction (a new interface, a new coordination mechanism, a new
cross-cutting concern) to check whether a well-known pattern already fits.
This tool suggests options; it does not mandate using one. Prefer the
simplest option it returns, and only reach for a heavier pattern (e.g.
Strategy, Observer) when simpler options clearly don't fit — this repo's
own "Go — write it like Go" rules (no interface-just-in-case, no
Manager/Factory/Builder by default, no wrapper structs) still override
anything this tool suggests.

#### 10. get_pattern_details
Retrieves full detail and code examples for a specific pattern by name. Use
after `find_patterns` has narrowed down a candidate, before implementing
it. Same optionality as `find_patterns` above.

#### 11. search_patterns
Keyword/semantic/hybrid search across the catalog by name or concept (e.g.
"circuit breaker", "event sourcing") when you already know roughly what
you're looking for. Use it to look up a pattern the user or a doc mentions
by name, or to compare a few related patterns before picking one. Same
optionality as `find_patterns` above.

`count_patterns` and `get_health_status` are introspection-only — skip them
during normal work.

---