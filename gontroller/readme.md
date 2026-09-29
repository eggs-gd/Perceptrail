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
  [`eggs-gd/go-exiftool` `dist-*` releases](https://github.com/eggs-gd/go-exiftool/releases));
- `.env` with `GONTROLLER_CONFIG=<path to config.yml>`.

```bash
go run .
```

Plugins are built from the repo root: `make build-plugins` → `build/plugins/*.so`.
Host and plugins must be built with the same Go and the same versions of shared
packages — see [findings](../_sb/docs/findings.md#go-plugins-2026-09-28).

`config.yml` (only these fields are used):

```yaml
path: "/Users/me/Pictures/"      # library root
plugins:                          # external perceptors
  - build/plugins/exif_geo.so
```

`server`, `database`, `allowed_hosts`, `api_keys` are not read yet: HTTP listens on
`:1323`, the database is `media_library.db` in the working directory.

## HTTP API

| Method | Path | Returns |
|---|---|---|
| GET | `/items` | All items as an NDJSON stream (`id, guid, date, mimeType, width, height`; `width/height` is the reduced aspect ratio) |
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
| `pkg/client` | Echo, `/items` and `/assets` routes |
| `pkg/transcoder` | thumbnail stub (needs libvips) |

## Worth knowing

- SQLite: WAL, **a single connection**. Never query through `db` inside a `tx`
  transaction — deadlock.
- `pkg/transcoder/images` does not build without libvips (`pkg-config vips`); it is
  not imported by `main`, so the server is unaffected. Run vet/tests without it:
  `go test $(go list ./... | grep -v transcoder/images)`.
- Known issues and plans — [roadmap](../_sb/docs/roadmap.md).
