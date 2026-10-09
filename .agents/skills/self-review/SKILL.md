---
name: self-review
description: Review your own diff the way the Perceptrail owner does, before showing the work or marking a PR ready — duplicated state, hand-set booleans, misleading names, files by subject, interfaces, how general a rule is, width, scope, docs, evidence. Use before every "done", every PR ready, every summary of a change.
---

# Before you say it is done

Run on `git diff develop...HEAD` (and what is not committed yet). Each point is a
correction the owner has had to make; the rules behind them are in
[AGENTS.md](../../../AGENTS.md) "Code Style" — this list is how to catch them.

- **State held twice.** Two fields reaching the same thing (`Proxy.writes` and
  `writer.db` were one connection), a flag mirroring a state, a cache nobody
  invalidates. Keep one, derive the other.
- **A boolean set by hand.** What real state does it mirror? Make it a getter, a nil
  check, a closed channel, a separate type for the other role.
- **Names.** Would a reader of this code know what it is? "rule" read as a policy
  where it was a write; a name that needs the comment to be understood is the wrong
  name. Package names are part of the name; length follows distance.
- **Files.** One subject per file, whole (its reads, writes, faces); public first —
  CI checks the order (`scripts/declorder`). A file that is only a role (`writes.go`
  next to `*api.go`) means the cut is by role, not by subject.
- **Interfaces.** Declared by the consumer with only what it calls; none without a
  second implementation or a test fake; a catalogue of every method is a smell
  anywhere (Interface Segregation), whatever the language.
- **How general a rule is.** When you write a rule into AGENTS.md or the docs: is it
  about this language's syntax, or about code? The owner's rules are for any code
  unless they name a language feature — they go under "Code Style", not "Go".
- **Width.** `git diff --stat develop...HEAD`: ~50 files that are not one mechanical
  change mean coupling — propose the cut instead of editing everything.
- **Scope.** A refactor is not mixed with new behaviour. What you found on the way
  goes to the owner and the roadmap, not into this diff.
- **Docs.** One fact, one place: godoc for how, the module README for the design,
  findings for why and the rejected, roadmap for what is open. A retold fact is a
  link instead.
- **Evidence.** Every "works", "passes", "faster" has its command and its output in
  your report; what was not checked is said so.
- **The owner's decisions.** Names, public API, module boundaries, anything wide:
  a proposal with a recommendation and a question — not a silent choice.
