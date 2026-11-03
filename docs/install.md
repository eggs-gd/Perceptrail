# Install

## Docker

Two images on GHCR, one [`compose.yml`](../compose.yml):
`ghcr.io/eggs-gd/perceptrail-server` (the server, its plugins, jellyfin-ffmpeg,
libvips, exiftool) and `ghcr.io/eggs-gd/perceptrail-web` (the gallery, served by
Caddy, which sends `/api` to the server — one address for the browser). No
checkout on the host, in a directory of its own:

```bash
curl -fsSLO https://raw.githubusercontent.com/eggs-gd/Perceptrail/master/compose.yml
docker run --rm --entrypoint cat ghcr.io/eggs-gd/perceptrail-server /etc/perceptrail/config.yml > config.yml
docker compose up -d    # then http://<host>:8080
```

- **The config decides**: compose mounts `./config.yml` over the image's own (the
  template, [`docker/config.yml`](../docker/config.yml)); without it compose stops
  with an error. Its paths are the container's. What to set: [config.md](config.md).
- **The libraries are mounts**, read only, one per host directory (`./library` →
  `/library` by default); `paths` in the config says which are walked — `paths: []`
  for providers only (an Immich, its key a file under `./data`). The data
  (database, caches) is `./data` → `/data`. `PORT` changes the published port.
- **Your own settings in `compose.override.yml`** beside it (compose reads it by
  itself; an update replaces `compose.yml`, not that): more mounts, a user
  (`user: 1000:1000` — `./data` must be theirs), `/dev/dri` with the `render`
  group for Intel QSV.
- **Tags**: `latest` and `MAJOR.MINOR` (releases), `develop` (every merge, what is
  being tested), the version (`0.1.35`).
- From a checkout: `docker compose -f compose.yml -f compose.build.yml up -d
  --build` ([`compose.build.yml`](../compose.build.yml)).

An example `compose.override.yml` — a second library and a user:

```yaml
services:
  server:
    user: 1000:1000
    volumes:
      - /mnt/photos/old:/old:ro   # and `- /old` under paths in config.yml
```

## Updating

```bash
docker compose pull && docker compose up -d
```

The database's tables are brought up to date at start; renditions whose recipe
changed (sizes, format) are made again in the background.

## A Mac with Apple Photos

Apple Photos needs the native server: PhotoKit (what is only in iCloud) and
VideoToolbox are not in Docker. Build and run it from source:
[`gontroller/readme.md`](../gontroller/readme.md#running) (Homebrew: exiftool,
vips, ffmpeg). macOS asks once for access to Photos, naming the terminal that
started the server — so start it from a terminal; a service run by launchd needs
a signed build of its own (not published yet).

The gallery for it: the web image with `API_UPSTREAM=host.docker.internal:1323`,
or `npm run build` in [`svebapp`](../svebapp/README.md) behind any server that
sends `/api` to the server.

## Windows

Not natively (the perceptors are Go plugins, `.so`): Docker, or WSL 2.
