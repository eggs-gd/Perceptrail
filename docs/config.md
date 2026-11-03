# Config

One YAML file. Every field, with its default:
[`gontroller/config.example.yml`](../gontroller/config.example.yml); the Docker
template: [`docker/config.yml`](../docker/config.yml). Relative paths are read
against the config's own directory. This page is about the choices.

## Libraries

- **Plain folders** — `paths`: directories, each walked (none inside another). In
  Docker each is a read-only mount in `compose.yml` (or your
  `compose.override.yml`) and `paths` names the container's side.
- **Apple Photos** — a `*.photoslibrary` under one of the `paths`, on a Mac with
  the native server: read from its database, its files, and PhotoKit for what is
  only in iCloud. `providers.apple.enabled: false` walks it as a plain folder.
- **Immich** — `providers.immich.url` and a key in a file of its own
  (`api_key_file`) or the env `IMMICH_API_KEY`; never in the config. The key needs
  only `asset.read`, `asset.view`, `asset.download`. Its timeline is shown (not the
  archive, the hidden or the trash); its previews and videos are served through
  our server, nothing of it is rendered again. Only an Immich: `paths: []`.

Nothing is ever written to a library.

## Render

What makes a plain folder's photos and videos fit for a browser (an Apple or an
Immich library brings its own).

- `render.workers` × `render.threads` is render's whole load, at nice 10 (below
  the server). By default both come from the CPUs this process may use. **Set them
  in an LXC, or on a box shared with something heavy** (an Immich): an LXC shows
  the host's CPUs, not its share.
- `sizes` (`[400, 1600]`: a tile and the viewer), `format` (`webp`, or `jpg`),
  `video` (a video's long side, `1280`). Changing them makes everything again, in
  the background.
- HDR videos are tone mapped when ffmpeg has `zscale` (the Docker image has it).
- Hardware encoding: not yet; measured, it takes ~25× less CPU for video, so it
  comes for archives of long videos and shared boxes.

## Views

Every perceptor is a view of the sheet, a button in the gallery.
`perceptors.<name>.client: false` hides the button (it still runs);
`enabled: false` switches it off. Plugins are listed under `plugins`.

## Database

SQLite in `data_dir` (`database.name`). The storage is database-agnostic (GORM
behind the model); `driver: postgres` is planned, not implemented yet.

## Mode

`mode: debug` logs every SQL and request and marks the gallery's tiles;
`release` (the default) is quiet.
