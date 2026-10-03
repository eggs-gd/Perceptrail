# providers

Libraries read through their own means — Apple Photos now, Immich and others later
(roadmap "Providers"). A library that keeps renditions of its own is asked for them:
we render and store nothing it keeps.

- **The import chain**: grouping is a sub-chain (`importer/discover/group.NewGrouping`) — found
  files in, whole assets out; inside, one switch asks the enabled providers in order
  and a file goes to the grouper of the first that claims it (`Claims`) — each
  grouper is a step of its own. The plain folder (`providers/folder`) is a provider too, the
  last: it claims what nobody else did. A provider not enabled is not asked: its
  files are a plain folder's.
- **The contract** ([`asset.go`](asset.go)): a grouper takes `Found` (a found file)
  and gives `Group` (a whole `Asset` — its files with their stat, `Key`, `Show`, the
  source's `Meta` / `MetaHash`, `Kind`); on the walk's flush (`chain.Flusher`) it
  gives what it holds: its last group, the files it `Held` back. `Regroup` gives one
  `Asset`. The source's `Meta` uses exiftool's tag names and `-n` values (numbers as
  numbers). Only
  the importer's discover stage sees these: its gate turns an `Asset` into its own
  `discover.Group`.
- **On demand**: the web service asks the item's provider (`Of`, `Owns`) for a
  level (`Levels`: medium, hover, original) and serves what it gets — a file, or
  bytes it drew (`Rendition`). After its library made a file local, the provider
  has the importer process that one asset again (`Regroup` → `Refresh`).
- **Background**: `Start` — access to the library, assets nothing shows yet.

Enabled in `main` by the config (`providers: {apple: {enabled: …}}`, enabled when
not listed); the plain folder always, last.

| package | what |
|---|---|
| `providers` | the interface, the enabled list |
| `providers/apple` | Apple Photos: the grouper (the library's DB, the naming layout), on demand (`ondemand.go`), `Regroup` |
| `providers/apple/photokit` | PhotoKit through cgo (macOS; a stub elsewhere) |
| `providers/folder` | the plain folder: sidecars by name; nothing on demand (the transcode renders for it) |
