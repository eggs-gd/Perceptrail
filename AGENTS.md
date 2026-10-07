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

- [`_sb/docs/findings.md`](_sb/docs/findings.md) — the why: decisions not obvious
  from the code, approaches rejected, traps. Read the relevant section BEFORE
  working on the gallery, sync, perceptors, the import, Apple Photos, Go plugins or
  exiftool. Do not re-propose approaches it lists as rejected without new reasons.
- [`_sb/docs/roadmap.md`](_sb/docs/roadmap.md) — what is open and the designs not
  built yet (done work is one line per PR), what belongs in the core vs. in
  perceptors.
- [`_sb/puml`](_sb/puml) — design diagrams (target architecture: import chain,
  file validation, client/ML event flow, workers, protocol). Check them before
  redesigning a flow; if the code deviates from a diagram, say so in findings.
- Module READMEs — how things work now: `gontroller/readme.md`,
  `gontroller/internal/importer/README.md`, `gontroller/internal/library/README.md`,
  `gontroller/internal/model/README.md`,
  `perceplib/README.md`, `perceptors/readme.md`, `svebapp/README.md`.

### Hard limits

- The owner's running gontroller (:1323) and Vite (:5173) are never stopped,
  restarted or written to — read them at most. To run things, a test pair on other
  ports over a copy of the data (the `smoke` skill).
- The Apple Photos library is read only, always.

### Procedures — skills

How to do the recurring work is in [`.agents/skills/`](.agents/skills/) (canonical;
`.claude/skills/` only points there): `pr-flow` (branch → commit → push → PR → after
merge), `review-reply` (answering a review), `self-review` (the owner's corrections
as a checklist, before "done"), `smoke` (a test pair). This file holds the rules;
the skills hold the order of the hands and link back here.

### Documentation contract — one fact, one place

A fact is written once, by its owner; everywhere else it is a link, never a
retelling (a retold fact goes stale on the next change and widens every PR).

| What | Where |
|---|---|
| how a package works: its API, the non-obvious lines | godoc in the code |
| a module's design: its parts, the rules across them, the types it owns | that module's README (`internal/importer`, `internal/library`, `internal/model`, `svebapp`, `perceplib`, `perceptors`) |
| running, config, the HTTP API, the map of packages (one line each + a link) | the top README of the program (`gontroller/readme.md`) |
| target flows: steps and who does what | `_sb/puml`, rendered to `_sb/diagrams/*.svg` (the READMEs show them: re-render with every `.puml` change) |
| why it is so, what was rejected, traps | `_sb/docs/findings.md` |
| what is open, designs not built yet | `_sb/docs/roadmap.md` |

- **A contract is described by the side that defines it**: the chain's messages
  (walk → group → gate → …) by the importer's README, the library contract
  (`provider`) by the library README; the other side links.
- **Diagrams name steps and responsibilities, not types or fields**: a rename must
  not touch them.
- **No lists of test files** in READMEs: one sentence on where the tests are and what
  they run against (real files, a real exiftool, the public API).
- Once a design is built, its description moves from the roadmap to the owner's
  README; a finding that became plain architecture moves there too.

## Git workflow (git flow)

- Long-lived branches: `master` (releases) and `develop` (integration). Never
  commit to them directly.
- Work happens in `feature/<name>` branches created from `develop`.
- `feature/*` → `develop`: a pull request, merged with **squash**. Each merge into
  `develop` is a patch increment `0.0.x`. Several PRs accumulate in `develop`.
- Release `develop` → `master` = a minor increment `0.x.0`:
  1. a feature PR into `develop` that runs `scripts/version.sh minor` (changes only
     `VERSION`);
  2. a PR `develop` → `master` for CI and review;
  3. once CI on `develop` has tagged the bump commit (`v0.x.0`), merged as a
     **fast-forward**: `git push origin develop:master` (admin bypass on `master`). Not with the GitHub rebase button — it rewrites every commit (new
     hashes), `develop` stops being an ancestor of `master` and the branches diverge
     (this happened with PR #2).
- Open PRs against `develop` unless the task is the `develop` → `master` release.
- One feature = one branch = one PR; its steps are commits on that branch (no PR
  per step, no branches stacked on each other).
- Open the PR as a **draft** as soon as the branch has its first commit (the work
  is visible, CI runs on every push); mark it **ready for review** only when the
  feature is done.
- Docs go in the same PR as the feature, never in a follow-up: before the PR is
  ready, the module READMEs and the diagrams match the code, `_sb/docs/roadmap.md`
  has the PR as one line in Done and only what is still open, and
  `_sb/docs/findings.md` has what was learned (the why, not the how).
- Version: one for the whole monorepo, **derived from git history** — nothing to bump
  or commit per PR, so parallel PRs never race. The root [`VERSION`](VERSION) holds
  only `MAJOR.MINOR`; `PATCH` = first-parent commits since `VERSION` last changed
  (one squash merge = +1). `scripts/version.sh` prints it; builds inject it
  (gontroller: `-ldflags -X .../app.Version`, see `gontroller/Makefile`; svebapp:
  `__APP_VERSION__` via `vite.config.ts`). `package.json` `version` is a placeholder
  (`0.0.0`, private). Needs full history (CI uses `fetch-depth: 0`). `perceplib` is
  versioned separately by its own tags.
- Tags: the CI job `tag` puts `vX.Y.Z` on every commit pushed to `develop` once
  `go` and `svebapp` pass. `master` gets no tags of its own — it only fast-forwards
  to already tagged `develop` commits; CI fails on an untagged `master` head.
- `develop` stays linear (squash merges only), which is what makes the rebase into
  `master` possible — GitHub cannot rebase a PR that contains merge commits.
  Operations that create merge commits (e.g. `git subtree pull` for `perceplib`)
  are done in a feature branch and get squashed on the way into `develop`.
- Only the owner (the sole account with write access) merges; that is enforced by
  repository permissions, not by a ruleset (a "restrict updates" rule would force a
  full bypass on every merge and switch off the checks below).
- Enforced by GitHub **rulesets** only (no classic branch protection): per branch —
  PR (no approvals required) with resolved threads, allowed merge method (`develop`: squash,
  `master`: rebase — do not use it, see the release steps), required checks `go` +
  `svebapp` (branch up to date), linear history, no force-push, no deletion.
  `develop`: admins may bypass only when merging a PR. `master`: admins may push
  directly — only for the fast-forward release push.


## Code Style

The rules in this section are for any code, in any language; the language
sections below hold only what is about that language itself. A rule found while
working on Go is general unless it is about a Go feature.

Before writing new code, read the neighbouring files of the package and follow
them: their naming, comment density, error handling, how they declare their
dependencies. Comments: one line per step (what it means for the product), plus a
short "not obvious" list where there is something non-obvious.

**A boolean is a getter of real state, not a field set by hand** — in any language.
`closed()` is "the channel is closed", `changed()` is "the stat differs from the
validated one", "is it a rule" is "is it the rule's type". A bool someone sets and
someone else clears duplicates a state and keeps the copy in sync by hand: at least
a smell, nearly always an architecture problem (one type playing two roles, a
queue kept as a flag, a "done once" instead of the thing that is done). The rare
exceptions are data, not state: a field on the wire (`removed` in a JSON line), an
input option (`hevc` from a request).

**A file reads top-down, public first** — in any language: public interfaces,
then public declarations (types, constants, variables), private declarations,
public implementations (functions and methods of exported names), private
implementations. A reader sees what the file offers before how. CI checks it in
every Go module (`scripts/declorder`; tests, `testdata` and generated files aside).

### Go — write it like Go

- **Names: length follows distance.** Short where the whole use is on one screen —
  a loop variable over a few lines (`for i, f := range files`), a method receiver
  (`func (w *Walker)`), the common idioms (`ctx`, `err`, `db`, `cfg`). Telling where
  the name lives longer or far from its declaration — struct fields, parameters,
  variables used over half a screen, anything at package level: `libraries`, not
  `ps`; `provider`, not `p`, when the loop body is long. No invented abbreviations
  (`ps`, `gw`, `st`) unless the context makes them obvious. Readability comes
  first; the Go convention does not excuse a cryptic name.
- **Package names are singular, short, lower case, no underscores** (`library`,
  `perceptor`, `route`, not `providers`, `exif_date`) — plural only where the
  singular collides with a builtin (as `strings`, `bytes` do); never the name of a
  standard library package (`plugin`). A repeated name for the package's main type
  is fine (`provider.Provider`, as `time.Time`).
- **The package name is part of the name**: `library.Enable`, not
  `library.EnableLibraries`; `walk.New`, not `walk.NewWalker`; `identify.Item`,
  not `identify.IdentifyItem`.
- **An abstraction and its instance are named apart**: the package and the type
  say what it is (`provider.Provider`), a value says which one (`library`,
  `libraries`).
- **Interfaces are declared by their consumer**, with only the methods it calls
  (a step's `Store`, a module's `Config`); the producer passes its whole value.
  No interface just in case: one implementation and no test fake — no interface.
- **No Manager / Factory / Builder by default, no wrapper structs** that only
  carry arguments (`Deps`, `Config` structs bundling a constructor's parameters
  hide the coupling instead of cutting it).
- **Constructors take, in one order**: what the module reads of the config (its
  own `Config` interface), the dependencies, the logger, then (for a chain step)
  `in`, `out`. A service gets its `context.Context` in `Start`, not in `New`.
- **A registry (one per process) is package functions** (`perceptor.Load`,
  `library.Enable`); instances where they hold logic (steps, providers,
  groupers).
- **Module layout** (Go's conventions, not `golang-standards/project-layout`'s):
  an application's packages in `internal/` (the toolchain enforces it); `main` at
  the module's root while there is one binary, `cmd/<name>/` from the second one;
  integration tests in `test/`; fixture files in `testdata/` next to their test.
  No `pkg/`, no `lib/`.
- **Tests**: a package's unit tests stay next to it (they may reach unexported
  code). Integration tests — a module through its public API, the server's own
  start, real files and tools — live in `gontroller/test/`, by the path of what
  they test (`test/importer`, `test/perceptor`). No test hooks in the code: a test
  that needs one is either a unit test or uses the public API.

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

The server runs from the repo root, which is no module: its MCP command takes the
Go that `gontroller/go.mod` asks for and the workspace [`gopls.work`](gopls.work)
(every Go module of the repo; not `go.work`, so the go command never sees it). A new
Go module goes into `gopls.work` too.

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