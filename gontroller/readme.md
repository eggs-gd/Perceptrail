# gontroller

The Perceptrail Go backend: scans the library, extracts metadata with ExifTool, runs
it through EXIF plugins, stores it in SQLite and serves it to the client over HTTP.

The core implements the necessary minimum (date, size); core plugins live here, in
`pkg/plugins/exif_core`. Extended features are external perceptors
([`../perceptors`](../perceptors/readme.md)).

Design: [Item flow](../_sb/puml/Item%20flow.puml),
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
| GET | `/items` | All shown items as an NDJSON stream, newest first (`id, guid, date, mimeType, width, height, asset`; `width/height` is the reduced aspect ratio) |
| GET | `/perceptors` | The perceptors given to the client (config `perceptors.<name>.client`): `name, title, icon (SVG), help, relative` — a button each |
| GET | `/p/:name/order?anchor=` | The sheet in that perceptor's order, NDJSON `{guid, sections?: [{level, label}]}` — the sections this photo starts, coarsest first (a path or one tag) |
| GET | `/assets/:guid` | The original file of an item |

## Import pipeline

Services (`pkg/app/services.go`) start in parallel: `ImporterService` and
`WebService`. Import is a chain of steps over channels (`perceplib/chain`):

```
FsWalker          files → groups (main file + sidecars by name)
                  → files table (GUID, LinkedTo, CheckTime)
ExifExtractor     5 long-lived exiftool processes (-stay_open)
                  → RawItem {Item, []RawExif}; ValidateFile creates/finds the item
ExifPluginProcessor
                  opener → exif_core/date → exif_core/size → [external] → closer
                  closer writes the item to the items table
(Transcoder)      disabled for now: thumbnails/transcoding (bimg + libvips)
```

Item states (`dto.ItemState`): `New → Dirty → Processing → Ready`, `Deleted`.

## Layout

| Package | What |
|---|---|
| `pkg/app` | app context, config, logger categories, services |
| `pkg/scan` | import steps: `fswalker`, `exifextractor`, `exifpluginprocessor` |
| `pkg/plugins` | plugin manager (core + `.so`), `exif_core/{date,size}` |
| `pkg/model` | SQLite via GORM, `ItemsApi`/`FilesApi`, DTOs |
| `pkg/client` | Echo, `/items`, `/assets`, `/perceptors` and `/p/:name/order` routes |
| `pkg/transcoder` | thumbnail stub (needs libvips) |

## Worth knowing

- SQLite: WAL, **a single connection**. Never query through `db` inside a `tx`
  transaction — deadlock.
- `pkg/transcoder/images` does not build without libvips (`pkg-config vips`); it is
  not imported by `main`, so the server is unaffected. Run vet/tests without it:
  `go test $(go list ./... | grep -v transcoder/images)`.
- Known issues and plans — [roadmap](../_sb/docs/roadmap.md).
