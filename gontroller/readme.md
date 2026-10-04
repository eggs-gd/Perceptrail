# gontroller

The Perceptrail Go backend: scans the library, extracts metadata with ExifTool, runs
it through EXIF plugins, stores it in SQLite and serves it to the client over HTTP.

The core implements the necessary minimum (date, size); core plugins live here, in
`pkg/perceptor/date`, `size`, `duration`. Extended features are external perceptors
([`../perceptors`](../perceptors/readme.md)).

Design: [Import chain](../_sb/puml/Import%20chain.puml),
[Walker](../_sb/puml/Walker.puml), [Protocol](../_sb/puml/Protocol.puml).

## Running

Requirements:

- Go 1.27.1 (`go` downloads the toolchain itself);
- `exiftool` in `PATH` (or a distribution from the
  [`eggs-gd/go-exiftool` `dist-*` releases](https://github.com/eggs-gd/go-exiftool/releases),
  set with `exiftool:` in the config).

```bash
mkdir -p .var && cp config.example.yml .var/config.yml   # once, then edit path:
make run
```

`make run` builds the binary and the plugins, then runs
`./.build/gontroller --config .var/config.yml`. Everything is under `gontroller/`
(both directories are git-ignored):

```
.build/           build artifacts
  gontroller
  plugins/*.so    make build-plugins (repo root)
.var/             runtime data
  config.yml
  media_library.db*
  cache/          generated files (thumbnails)
```

Config lookup: `--config <file>`, then `$GONTROLLER_CONFIG`, then `./config.yml`.
Relative paths in the config are resolved against the config's directory, so the
same config gives the same database and caches from any working directory. See
[`config.example.yml`](config.example.yml) for the fields (`path`, `plugins`,
`data_dir`, `exiftool`, `server` with CORS `allowed_origins`, `database`).

Host and plugins must be built with the same Go and the same versions of shared
packages — see [findings](../_sb/docs/findings.md#go-plugins-2026-09-28).

## HTTP API

| Method | Path | Returns |
|---|---|---|
| GET | `/items` | All shown items as an NDJSON stream, newest first (`id, guid, date, mimeType, width, height, asset`; `width/height` is the reduced aspect ratio). Header `X-Sync-Epoch` (this database); the last line is `{cursor, total}` — only a complete stream has it, `total` = shown items (the client checks its copy against it); `?since=<cursor>`: only the changes, a removed item as `{guid, removed}` |
| GET | `/items?since=<cursor>` | The delta: changed shown items as above, `{guid, removed: true}` for deleted or hidden ones |
| GET | `/perceptors` | The perceptors given to the client (config `perceptors.<name>.client`): `slug (the view in URLs), title, icon (SVG), help, relative` — a button each |
| GET | `/items/:guid/rendition/:level` | Apple Photos on demand: `medium` (the viewer: the image ~2048 px or the edit; a video's 720p, `?hevc=0` H.264 only), `hover` (a video's 360p, a Live Photo's motion), `original` (the biggest of what is seen: a photo's current version — the edit — at full resolution as JPEG; a video's original file). Serves the file from the library; asks Photos (PhotoKit, macOS) when it is not local; 404 when nothing is there |
| GET | `/items/:guid/files` | Every file of the item's group (original, edits, derivatives, motion, frames, sidecars): `[{name, role, mime, size, w, h, url}]`; `url` + `?download=1` saves it under its name |
| GET | `/items/:guid/info` | What each perceptor knows about the item (the viewer's info panel): `[{slug, title, icon, facts: [{label, value}]}]` |
| GET | `/app` | The server's `version` and `mode` (debug / release) |
| GET | `/p/:view/order?anchor=` | The sheet in that perceptor's order, NDJSON `{guid, sections?: [{level, label}]}` — the sections this photo starts, coarsest first (a path or one tag) |
| GET | `/assets/:guid` | The original file of an item |

## Import pipeline

Services (`pkg/app/services.go`) start in parallel: `ImporterService` and
`WebService`. Import is a chain of steps over typed pipes (`perceplib/chain`):

```
walk       the chain's entry: the library's files as rows (stat, seen), then the
           ones it says are gone
group      whole assets: the providers' groupers (the plain folder last)
gate       only the assets that need work pass (dto.Asset); gone files deleted
identify   one exiftool call per group (the declared tags only) → kinds → the
           metadata package → fingerprint → the item → the cheap preview
           (identify.Item)
exif       the EXIF perceptors: built in (date, size, length), then the .so ones;
           their values kept
commit     the item published (Visible / Waiting)
```

A pass is a new chain run to its end (`Process`: the walk returns, each step ends
after its input); the importer service pauses (`rescan`) and runs the next. Details, the types
and the rules: [`pkg/importer/README.md`](pkg/importer/README.md).
The transcoders (`pkg/transcode`) are not wired yet: a chain of their own, fed from
the DB.

Item states (`dto.ItemState`): `New → Dirty → Processing → Ready`, `Deleted`.

## Layout

`main` ([`gontroller.go`](gontroller.go)) reads as the server's modules in the
order they start: the config, the logger, the model, the perceptors, the
libraries, then the services (the libraries' background work, the import, HTTP).
Each module is set up the same way: `New(cfg, deps…, logger)` / `Load` / `Enable`,
reading its own `Config` interface of the whole config.

`test/` holds the integration tests, by the path of what they test, through the
public API only: `test/importer` (the server's own start, the import's passes),
`test/perceptor` (the built-ins' declared tags, loading the `.so` plugins),
`test/web/route` (the HTTP API over a real model, read as the client reads its
JSON), `test/library` (the Apple library's background work); `test/fake` stands in
for what CI cannot have (Photos). A package's own unit tests stay next to it.

| Package | What |
|---|---|
| `pkg/app` | the server as a whole: its services run together (`Services`), the version |
| `pkg/config` | the config file, read once (`Load`, `Read`); a leaf — a module declares the getters it reads as its own `Config` interface |
| `pkg/importer` | the import chain: linear stages, each a sub-chain of its own — `walk`, `group`, `gate`, `identify` (exiftool, kinds, the item, sizes, the cheap preview), `exif` (the EXIF perceptors, built in and external, their values kept), `commit` (the item published); see its README |
| `pkg/transcode` | the transcoders' switch and stubs (a chain of its own later) |
| `pkg/perceptor` | the perceptors' registry (built in + `.so`, their storages); `perceptor/builtin`: the built-ins' contract (`builtin.Item`, `builtin.Perceptor`, `OrderByValue`); the built-in EXIF perceptors `perceptor/date`, `size`, `duration` |
| `pkg/model` | the model over GORM (`Open`: SQLite), `ItemsApi`/`FilesApi`/`MetaApi` and the import's rules, DTOs |
| `pkg/web` | the HTTP service (Echo); `web/route`: `/items`, `/assets`, `/perceptors`, `/p/:view/order`, renditions |
| `pkg/library` | the libraries of this run (`Enable`, `Enabled`, `Of`, `Service`): one switch sends a file to the grouper of the first that claims it, and on-demand renditions come from the item's library; `library/provider`: the contract they implement; `library/folder`: the plain folder (last, takes the rest); `library/apple`: Apple Photos (its DB, the grouper, on demand), `library/apple/photokit`: PhotoKit (cgo, macOS only; a stub elsewhere; the main thread serves its main queue) |
| `pkg/transcoder` | thumbnail stub (needs libvips) |

## Worth knowing

- SQLite: WAL, **a single connection**. Never query through `db` inside a `tx`
  transaction — deadlock.
- `pkg/transcoder/images` does not build without libvips (`pkg-config vips`); it is
  not imported by `main`, so the server is unaffected. Run vet/tests without it:
  `go test $(go list ./... | grep -v transcoder/images)`.
- Known issues and plans — [roadmap](../_sb/docs/roadmap.md).
