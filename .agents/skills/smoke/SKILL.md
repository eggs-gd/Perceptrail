---
name: smoke
description: Run Perceptrail for real without touching the owner's instance — a test pair (gontroller on :1329 over a copy of the database, svebapp on :5174) to check a change end to end. Use when a change must be seen working in the server or the gallery, or to measure an import on real data.
---

# A smoke run on a test pair

**Never touch** the owner's gontroller (:1323) and Vite (:5173) — read them at
most. The Apple Photos library is read only, always.

1. **A scratch directory** (the session's scratchpad), with a config copied from
   `gontroller/config.example.yml`:
   - `path:` the library to import — a copy or a small sample, unless the run only
     reads (a Photos library is read only anyway);
   - `server.port: 1329`;
   - the database relative to the config's directory (relative paths resolve
     against it), so it lands in the scratch directory.
2. **A copy of the database**, when real data matters — through SQLite, not `cp`
   (the WAL may hold the last writes):

   ```bash
   sqlite3 <owner's media_library.db> ".backup '<scratch>/media_library.db'"
   ```

3. **The server**: `make build-plugins` once, then from `gontroller/`:

   ```bash
   go build -o <scratch>/gontroller . && <scratch>/gontroller --config <scratch>/config.yml > <scratch>/server.log 2>&1 &
   ```

4. **The gallery**, pointed at :1329 (the variables are in `svebapp/.env`):

   ```bash
   cd svebapp && SVEBAPP_SERVER_PORT=1329 PUBLIC_API_PATH=http://localhost:1329 npx vite dev --port 5174 --strictPort
   ```

5. **Check** through the HTTP API (`gontroller/readme.md` lists it) and the
   browser; numbers come from the log or a bench, with the command that made them.
6. **Stop both** when done; the scratch directory goes with the session.
