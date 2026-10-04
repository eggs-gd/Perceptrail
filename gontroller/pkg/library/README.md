# providers

Libraries read through their own means — Apple Photos now, Immich and others later
(roadmap "Providers"). A library that keeps renditions of its own is asked for them:
we render and store nothing it keeps.

- **The import chain**: grouping is a step of its own, a sub-chain (`importer/group.New`) — found
  files in, whole assets out; inside, one switch asks the enabled providers in order
  and a file goes to the grouper of the first that claims it (`Claims`) — each
  grouper is a step of its own. The plain folder (`library/folder`) is a provider too, the
  last: it claims what nobody else did. A provider not enabled is not asked: its
  files are a plain folder's.
- **The contract** ([`providers.go`](providers.go) `Grouper`): a grouper takes the
  walk's rows (`*dto.FileDto`: path, stat; `Changed`, `Gone`) and gives a whole
  `dto.Asset` (its
  files' rows, `Key`, `Show`, the source's `Meta` / `MetaHash`, `Kind`); on the
  walk's end — its input closes — (`chain.Flusher`) it gives what it holds (its last group). A file the
  walk says is gone comes too: the grouper passes it through (an asset of its own)
  or makes something of it (Apple: the files of an asset trashed or hidden in Photos
  are sent as gone). A grouper that gives a file a role uses `SetRole` (a new role
  is new work). The source's `Meta` uses exiftool's tag names and `-n` values
  (numbers as numbers).
- **On demand**: the web service asks the item's provider (`Of`, `Owns`) for a
  level (`Levels`: medium, hover, original) and serves what it gets — a file, or
  bytes it drew (`Rendition`). After its library made a file local, the provider
  marks the item for the next pass (`Refresh`: `MarkRework`); it does not wait —
  the client guesses meanwhile.
- **Background**: `Start` — access to the library, assets nothing shows yet.

Enabled in `main` by the config (`providers: {apple: {enabled: …}}`, enabled when
not listed); the plain folder always, last.

| package | what |
|---|---|
| `providers` | the interface, the enabled list |
| `library/apple` | Apple Photos: the grouper (the library's DB, the naming layout), on demand (`ondemand.go`) |
| `library/apple/photokit` | PhotoKit through cgo (macOS; a stub elsewhere) |
| `library/folder` | the plain folder: sidecars by name; nothing on demand (the transcode renders for it) |
