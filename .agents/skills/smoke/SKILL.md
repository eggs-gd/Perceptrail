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
     against it), so it lands in the scratch directory;
   - `plugins:` as absolute paths to plugins built **into the scratch directory**
     (step 3) — the example's `../.build/plugins/…` resolves against the scratch
     directory, and a plugin that does not open is only logged: the server runs
     without it.
2. **A copy of the database**, when real data matters — through SQLite, not `cp`
   (the WAL may hold the last writes):

   ```bash
   sqlite3 <owner's media_library.db> ".backup '<scratch>/media_library.db'"
   ```

3. **The plugins and the server, into the scratch directory** — never `make
   build-plugins`: it rewrites `gontroller/.build/plugins/*.so`, which the owner's
   running server has loaded:

   ```bash
   for m in exif_geo ml_color; do (cd perceptors/$m && go build -buildmode=plugin -o <scratch>/plugins/$m.so .); done
   (cd gontroller && go build -o <scratch>/gontroller .)
   <scratch>/gontroller --config <scratch>/config.yml > <scratch>/server.log 2>&1 &
   ```

   A perceptor's store (`perceptors/*.db`) is copied with `.backup` too.

4. **The gallery**, pointed at :1329 (the variables are in `svebapp/.env`):

   ```bash
   cd svebapp && SVEBAPP_SERVER_PORT=1329 PUBLIC_API_PATH=http://localhost:1329 npx vite dev --port 5174 --strictPort
   ```

5. **Check** the log first — `grep -i "failed to load plugin" <scratch>/server.log`
   says nothing, and `GET /perceptors` lists the external ones — then through the HTTP API (`gontroller/readme.md` lists it) and the
   browser; numbers come from the log or a bench, with the command that made them.
6. **Stop both** when done — by the scratch config's path
   (`pkill -f -- "--config <scratch>/config.yml"`), never by name: the owner's
   server is a `gontroller` too.
