---
name: pr-flow
description: Branch, commit, push, open, finish and clean up a Perceptrail pull request (or one in an eggs-gd library repo) — git flow, docs in the same PR, after-merge tags. Use when starting a change, before a commit or a push, when opening or updating a PR, marking it ready, or after the owner merged one.
---

# A pull request, start to after merge

The rules are in [AGENTS.md](../../../AGENTS.md) ("Git workflow"); this is the order
of the hands. Only the owner merges.

## Start

```bash
git checkout develop && git pull
git checkout -b feature/<name>
```

One feature = one branch = one PR. Another open PR touching the same files: wait for
its merge rather than stack a branch on it.

The owner's servers run from the main checkout (AGENTS.md "Hard limits"): a change
to svebapp's config or `.env` there restarts their Vite. Such work goes into a
worktree beside it — `git worktree add -b feature/<name> ../Perceptrail-<name>
develop` — removed after the merge (`git worktree remove`).

## Before every commit

1. `git status --short` — only the files you meant. A plain `go build` inside a
   module writes an executable next to the sources; scratch files, logs, `.db`
   copies stay out. `git add` by path when in doubt, never a blind `git add -A`.
2. What changed is checked: `gofmt`, `go vet`, tests (`go test -race ./...` in
   gontroller), gopls diagnostics on every Go file touched; `go_vulncheck` after a
   `go.mod` change. A plugin touched: `go build -buildmode=plugin` in its module.
3. The message: the subject says what it is for the product; the body says why.
   End with the attribution lines the session gives.

## Push

By URL, through the gh credential helper — then point the branch at `origin`, so a
merged branch shows `[gone]`:

```bash
git -c credential.helper= -c "credential.helper=!gh auth git-credential" \
  push -q https://github.com/eggs-gd/<repo>.git <branch>
git fetch -q origin <branch> && git branch -q --set-upstream-to=origin/<branch>
```

## The PR

- **Draft at the first push**: `gh pr create --draft --base develop` (a library repo:
  `--base main`). The body: what changed, why, how it was verified (commands), its
  width when wide and why, what is left for after the merge; the attribution line.
- **Before ready** — the docs are in this PR, never a follow-up:
  - the module READMEs match the code; a changed `.puml` is re-rendered to
    `_sb/diagrams/*.svg`;
  - `_sb/docs/roadmap.md`: this PR as one line in "Done, by PR" (the real number:
    `gh pr view --json number`), only what is still open elsewhere;
  - `_sb/docs/findings.md`: what was learned — the why and the rejected, not the how.
- Run the `self-review` skill on the diff, then `gh pr ready <n>`.
- Review comments: the `review-reply` skill.

## After the owner merged

```bash
git checkout develop && git pull && git fetch --prune
git branch -D feature/<name>
```

- `perceplib/` changed: push the subtree and tag it ([perceplib
  README](../../../perceplib/README.md) "Versioning"); the `require` lines name that
  tag already.
- A library repo (`go-pub-sub`, `go-chain`, `go-zap-decor`, `go-exiftool`): tag the
  merge commit on `main` (`vX.Y.Z`, minor for a breaking change before 1.0), push the
  tag, check `GOPROXY=proxy.golang.org go list -m <module>@<tag>`.
