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

| tool | for | config |
|---|---|---|
| Go 1.27.1 | building (`go` downloads the toolchain itself) | |
| `exiftool` | the import: metadata, embedded previews | `exiftool:` (default: `PATH`) |
| `vipsthumbnail` (libvips) | render: photos' renditions | `render.vipsthumbnail:` (default: `PATH`) |
| `ffmpeg`, `ffprobe` | render: videos' renditions, posters; HDR → SDR needs a build with `zscale` (zimg) | `render.ffmpeg:` (default: `PATH`; `ffprobe` beside it) |

Render does not start without libvips or ffmpeg (the log says why; the import runs
anyway) — or turn it off: `render: {enabled: false}`.

**macOS** (Homebrew):

```bash
brew install exiftool vips ffmpeg-full
```

`ffmpeg-full` has `zscale`; it is keg-only — set
`render.ffmpeg: /opt/homebrew/opt/ffmpeg-full/bin/ffmpeg`. The plain `ffmpeg`
formula works too, HDR videos kept as they are.

**Debian / Ubuntu**:

```bash
sudo apt install libimage-exiftool-perl libvips-tools ffmpeg
```

**Fedora**: `ffmpeg` from [RPM Fusion](https://rpmfusion.org) (Fedora's
`ffmpeg-free` has no H.264 encoder):

```bash
sudo dnf install perl-Image-ExifTool vips-tools ffmpeg
```

**Windows**: not natively (the perceptors are Go plugins, `.so`) — WSL 2 with the
Debian / Ubuntu line above. A Docker image with everything inside comes with 0.2.0.

`exiftool` may also be a distribution from the
[`eggs-gd/go-exiftool` `dist-*` releases](https://github.com/eggs-gd/go-exiftool/releases),
set with `exiftool:` in the config.

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
[`config.example.yml`](config.example.yml) for the fields (`paths` — the older
`path` still read, `plugins`, `data_dir`, `exiftool`, `server` with CORS
`allowed_origins`, `database`, `render`, `providers`).

Host and plugins must be built with the same Go and the same versions of shared
packages — see [findings](../_sb/docs/findings.md#go-and-the-toolchain).

### Docker

Two images, one [`compose.yml`](../compose.yml) at the repository root:
`ghcr.io/eggs-gd/perceptrail-server` (gontroller, its plugins, jellyfin-ffmpeg,
libvips, exiftool) and `ghcr.io/eggs-gd/perceptrail-web` (the gallery, served by
Caddy, which sends `/api` to the server — one address for the browser).

```bash
LIBRARY=/path/to/photos docker compose up -d    # then http://<host>:8080
```

- The library is mounted read only at `/library`; the data (database, caches) is
  `./data` → `/data`. `PORT` changes the published port.
- The server's config in the image is [`docker/config.yml`](../docker/config.yml);
  mount your own over `/etc/perceptrail/config.yml` to change it.
- Published to GHCR on every release (`latest` and the version); `docker compose
  build` builds them from the checkout.
- A Mac with Apple Photos runs the native binary: PhotoKit (on demand) and
  VideoToolbox are not in Docker.

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
| `internal/render` | the expensive stage: woken by `ItemPublished`, takes what is due from the work queue, on N workers renders photos with libvips (`vipsthumbnail`) and videos with ffmpeg (H.264 + a poster; Live Photos' motion), a process each; config `render` |
| `internal/perceptor` | the perceptors' registry (built in + `.so`, their storages); `perceptor/builtin`: the built-ins' contract (`builtin.Item`, `builtin.Perceptor`, `OrderByValue`); the built-in EXIF perceptors `perceptor/date`, `size`, `duration` |
| `internal/model` | the library's data and its rules: one writer, a read pool, every write a command ([README](internal/model/README.md)) |
| `internal/web` | the HTTP service (Echo); `web/route`: `/items`, `/assets`, `/perceptors`, `/p/:view/order`, renditions |
| `internal/library` | the libraries the import reads and the web service asks on demand: Apple Photos, Immich (its API), the plain folder ([README](internal/library/README.md)) |

## Worth knowing

- SQLite: one writer connection and a read-only pool; every write is a command — [`internal/model/README.md`](internal/model/README.md).
- Known issues and plans — [roadmap](../_sb/docs/roadmap.md).
