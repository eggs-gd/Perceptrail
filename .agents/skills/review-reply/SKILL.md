---
name: review-reply
description: Read and answer review comments on a PR (Codex or a person) — every comment found, each claim checked, a bug reproduced by a failing test before the fix, each thread answered with the fixing commit. Use when the owner says there are comments, or a PR got a review.
---

# Answering a review

1. **Read everything** — a review has three places, and a comment on a binary file
   is only in the review's body:

   ```bash
   gh pr view <n> --comments
   gh api repos/eggs-gd/<repo>/pulls/<n>/comments --jq '.[] | "\(.id) \(.path):\(.line)\n\(.body)\n---"'
   gh api repos/eggs-gd/<repo>/pulls/<n>/reviews  --jq '.[] | "\(.id) \(.user.login) \(.state)\n\(.body)"'
   ```

2. **Check each claim against the code.** Right: fix it. Wrong or out of scope: say
   why in the thread — no silent skip, no fix for show.
3. **A bug: a test first** that fails on the current code — run it, see it fail —
   then the fix, then green. The test stays. (go-pub-sub's `TestClientCloseRace`
   failed on the old client before the RLock went in.)
4. **One commit per finding**, through the `pr-flow` skill (checks, push).
5. **Answer in the thread** — the commit hash, what changed, how it was verified:

   ```bash
   # an inline comment
   gh api -X POST repos/eggs-gd/<repo>/pulls/<n>/comments/<id>/replies -f body="Fixed in <sha>: …"
   # a review body, or a comment without a thread
   gh pr comment <n> --body "Fixed in <sha>: …"
   ```

   The owner resolves the threads (the ruleset requires them resolved to merge).
6. **Tell the owner** in a few lines: each finding, fixed or not and why.
