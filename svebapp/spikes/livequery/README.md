# liveQuery spike (2026-09-29)

Does a Dexie write in a **worker** reach a `liveQuery` subscription on the **page**,
how fast, and does a window subscription ignore writes outside the window?
Standalone page, own database `spike-livequery`; not part of the SvelteKit app.

```bash
cd svebapp && npx vite --config spikes/livequery/vite.config.ts   # http://localhost:5175
```

Result (Dexie 4.4.6, Chromium): see `_sb/docs/findings.md` → "liveQuery across threads".
