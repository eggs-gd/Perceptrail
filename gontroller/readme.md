# gontroller

The Perceptrail Go backend: scans the library, extracts metadata with ExifTool, runs
it through EXIF plugins, stores it in SQLite and serves it to the client over HTTP.

The core implements the necessary minimum (date, size); core plugins live here, in
`internal/perceptor/date`, `size`, `duration`. Extended features are external perceptors
([`../perceptors`](../perceptors/readme.md)).

Design: [Import chain](../_sb/puml/Import%20chain.puml),
[Walker](../_sb/puml/Walker.puml), [Protocol](../_sb/puml/Protocol.puml).

## Running

Requirements:

- Go 1.27.1 (`go` downloads the toolchain itself);
- `exiftool` in `PATH` (or a distribution from the
  [`eggs-gd/go-exiftool` `dist-*` releases](https://github.com/eggs-gd/go-exiftool/releases),
  set with `exiftool:` in the config);
- `vipsthumbnail` (libvips: `brew install vips`, `apt install libvips-tools`) for
  render — or `render: {enabled: false}`.

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
packages — see [findings](../_sb/docs/findings.md#go-and-the-toolchain).

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
| GET | `/assets/:guid/r/:name` | One of our renditions (`<size>.<format>`), as the asset's `stills` list them — only the ones the database lists for that item |

## Import

A chain of stages run pass after pass — walk → group → gate → identify → exif →
commit: [`internal/importer/README.md`](internal/importer/README.md). The libraries
it reads (Apple Photos, the plain folder): [`internal/library/README.md`](internal/library/README.md).

## Layout

`main` ([`gontroller.go`](gontroller.go)) reads as the server's modules in the
order they start: the config, the logger, the model, the perceptors, the
libraries, then the services (the libraries' background work, the import, HTTP).
Each module is set up the same way: `New(cfg, deps…, logger)` / `Load` / `Enable`,
reading its own `Config` interface of the whole config.

The packages live in `internal/` (Go's own rule: nothing outside this module may
import them — gontroller is an application, not a library). `main` stays at the
module's root while there is one binary (`cmd/<name>/` when a second one comes).

`test/` holds the integration tests, by the path of what they test, through the
public API only (`test/fake` stands in for what CI cannot have: Photos); a
package's own unit tests stay next to it.

| Package | What |
|---|---|
| `internal/app` | the server as a whole: its services run together (`Services`), the version |
| `internal/cache` | where an item's files live in the data's cache: a tree by the GUID (`<part>/<ab>/<cd>/<guid>`) |
| `internal/config` | the config file, read once (`Load`, `Read`); a leaf — a module declares the getters it reads as its own `Config` interface |
| `internal/importer` | the import chain ([README](internal/importer/README.md)) |
| `internal/render` | the expensive stage: woken by `ItemPublished`, takes what is due from the work queue, renders photos with libvips (`vipsthumbnail`, a process each) on N workers; config `render` |
| `internal/perceptor` | the perceptors' registry (built in + `.so`, their storages); `perceptor/builtin`: the built-ins' contract (`builtin.Item`, `builtin.Perceptor`, `OrderByValue`); the built-in EXIF perceptors `perceptor/date`, `size`, `duration` |
| `internal/model` | the library's data and its rules: one writer, a read pool, every write a command ([README](internal/model/README.md)) |
| `internal/web` | the HTTP service (Echo); `web/route`: `/items`, `/assets`, `/perceptors`, `/p/:view/order`, renditions |
| `internal/library` | the libraries the import reads and the web service asks on demand: Apple Photos, the plain folder ([README](internal/library/README.md)) |

## Worth knowing

- SQLite: one writer connection and a read-only pool; every write is a command — [`internal/model/README.md`](internal/model/README.md).
- Known issues and plans — [roadmap](../_sb/docs/roadmap.md).
