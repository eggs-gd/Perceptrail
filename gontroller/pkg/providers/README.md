# providers

Libraries read through their own means — Apple Photos now, Immich and others later
(roadmap "Providers"). A library that keeps renditions of its own is asked for them:
we render and store nothing it keeps.

- **The import chain**: every enabled provider is a step, one after another. A
  found file it claims (`Claims`) goes to its own grouper; one it does not, to the
  next step; what nobody claims is a plain folder's (the generic grouper, last). A
  provider not enabled is not in the chain: its files are a plain folder's.
- **On demand**: the web service asks the item's provider (`Of`, `Owns`) for a
  level (`Levels`: medium, hover, original) and serves what it gets — a file, or
  bytes it drew (`Rendition`). After its library made a file local, the provider
  has the importer process that one asset again (`Regroup` → `Refresh`).
- **Background**: `Start` — access to the library, assets nothing shows yet.

Enabled in `main` by the config (`providers: {apple: {enabled: …}}`, enabled when
not listed).

| package | what |
|---|---|
| `providers` | the interface, the enabled list |
| `providers/apple` | Apple Photos: the grouper (the library's DB, the naming layout), on demand (`ondemand.go`), `Regroup` |
| `providers/apple/photokit` | PhotoKit through cgo (macOS; a stub elsewhere) |
