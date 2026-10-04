# library

The libraries of this run, read through their own means — Apple Photos now, Immich and others later
(roadmap "Providers"). A library that keeps renditions of its own is asked for them:
we render and store nothing it keeps.

- **The contract** ([`provider.go`](provider/provider.go)) has a part per consumer:
  `Grouping` for the import (`Claims`, `Grouper`), `Renditions` for the web
  service (`Levels`, `Rendition`); `Provider` is the whole library as the registry
  keeps it (+ `Owns`, `Start`).
- **Grouping**: the import's switch asks the enabled libraries in order and a file
  goes to the grouper of the first that claims it; the plain folder
  (`library/folder`) is last and claims the rest. A library not enabled is not
  asked: its files are a plain folder's. A grouper turns the walk's files into whole
  assets (the messages: [importer README](../importer/README.md#the-chains-messages))
  and, when its input closes, gives what it holds (`chain.Flusher`). A missing file
  it passes through, or makes something of it; it may also say a file still on disk
  is gone for the library (Apple: an asset trashed or hidden in Photos). A role it
  gives goes through `SetRole` (a new role is new work); its `Meta` uses exiftool's
  tag names and `-n` values.
- **On demand**: the web service asks the item's provider (`Of`, `Owns`) for a
  level (`Levels`: medium, hover, original) and serves what it gets — a file, or
  bytes it drew (`Rendition`). After its library made a file local, the provider
  marks the item for the next pass (`MarkRework`); it does not wait —
  the client guesses meanwhile.
- **Background**: `Start` — access to the library, assets nothing shows yet; run
  with the server's services (`library.Service()`).

`library.Enable(cfg, db, logger)` (in `main`) builds them from the config
(`providers: {apple: {enabled: …}}`, enabled when not listed); the plain folder
always, last. `Enabled` gives the import their `Grouping`, in order; `Of` the
`Renditions` of the item's library (the web service gets `library.Of` passed in:
the routes do not reach the registry).

| package | what |
|---|---|
| `library` | the libraries of this run: `Enable`, `Enabled`, `Of`, `Service` |
| `library/provider` | the contract a library implements: `Grouping` (the import), `Renditions` (the web service), `Provider` (both + `Owns`, `Start`); `Grouper`, `Rendition`, `Options` |
| `library/apple` | Apple Photos: the grouper (the library's DB, the naming layout), on demand (`ondemand.go`) |
| `library/apple/photokit` | PhotoKit through cgo (macOS; a stub elsewhere) |
| `library/folder` | the plain folder: sidecars by name; nothing on demand (the transcode renders for it) |

## Apple Photos

Measured on the dev library (6 457 assets, Optimize Mac Storage) in two spikes
(`_sb/spikes/photokit`). We only read the library; Photos writes into it.

**The grouper.** The first file of a library loads its assets from a copy of
`Photos.sqlite` and forms the groups up front (the files that exist, by the naming
layout below); a group goes out when its last file arrives, one that did not
complete waits for the next walk. The key is the asset UUID (the item's GUID: the
main file may change — a derivative, then the downloaded original — the item
stays); the main file is the source (a Live Photo's video before its photo); `Show`
is the edit, the original, then Photos' derivatives; video renditions go after the
stills. The DB's date + zone, oriented size, GPS, length and kind are the asset's
`Meta` (they win over the files' EXIF). Trashed or hidden assets: their files are
missing for us.

**The bundle.** The DB stores only the original's path,
`originals/<ZDIRECTORY>/<ZFILENAME>`; everything else follows a naming layout
(`<X>` = the UUID's first character; `_o` of the original, `_a` of the edit — the
edit's wins):

| file | what | long side |
|---|---|---|
| `originals/<X>/<UUID>.<ext>` | the original (the source) | full |
| `resources/renders/<X>/<UUID>_1_201_a.jpeg\|heic` | the user's edit (`.plist` beside it: edit data) | full |
| `resources/derivatives/<X>/<UUID>_1_101_o`, `_1_102_o.jpeg` | previews of the original (`_1_102_o`: recipe 65741, what PhotoKit downloads for ≤ 2048 px) | ~2000–2600 |
| `resources/derivatives/<X>/<UUID>_1_102_a.jpeg` | preview of the edit | ~2000 |
| `resources/derivatives/<X>/<UUID>_1_105_c`, `_1_106_c.jpeg` | medium previews (65747) | ~1000 |
| `resources/derivatives/masters/<X>/<UUID>_4_5005_c.jpeg` | the small thumbnail — local for nearly every asset, cloud-only too; not in the DB | ~360×480 |
| `resources/derivatives/<X>/<UUID>.THM` | a video's "poster" | 32×32 |
| `resources/derivatives/cvt/<X>/<UUID>/…_cvt_tNNNN.jpeg` | video scrubbing frames (0–10, Photos' own analysis; no request makes them; a third of videos have none, Live Photos never) | 400×600 |
| `_2_4_o.mp4` / `_2_201_o.mov`, `_2_3_o.mp4` / `_2_101_o.mov` | video renditions PhotoKit makes local: fast 360p H.264 / medium (iPhone: HEVC 720p; others: H.264) / a Live Photo's motion | |

Everything outside `originals/` and `resources/` (Messages backdrops in
`internal/`, iCloud sharing in `scopes/`) is not the user's and is skipped.

**The DB** (`Photos.sqlite`, read from a copy):

- `ZDATECREATED` is Core Data seconds since 2001 UTC, **declared `TIMESTAMP`**:
  SQLite keeps whole seconds as integers and the Go driver turns them into
  `time.Time` — read it as `CAST(… AS REAL)`. `ZTIMEZONEOFFSET` is known for nearly
  every asset.
- `ZWIDTH` / `ZHEIGHT` are already oriented (the record says Orientation 1); GPS
  `-180` = none.
- `ZKIND` 1 = video; `ZPLAYBACKSTYLE` 3 = a Live Photo with live on (`ZKINDSUBTYPE`
  2 with style 1: live off — a still). `ZKINDSUBTYPE` 101 is not slo-mo (Android
  screen recordings carry it).
- `ZINTERNALRESOURCE` knows every rendition: recipe, size, local or in iCloud. Under
  Optimize Mac Storage most originals and nearly every video are only in iCloud;
  a derivative can show 6 420 assets where the originals alone show 1 781; 7 had
  nothing local at all.

**PhotoKit** (`apple/photokit`, cgo):

- **Photos normalises the library**: asked for an image ≤ 2048 px with network
  allowed it downloads recipe 65741 into `_1_102_o.jpeg` (~0.6–0.9 s, ~1 MB) and
  marks it local; the original stays in iCloud. Videos and Live Photos the same: the
  file comes as a `file://` URL in `resources/derivatives/`.
- Video `deliveryMode`: `fast` → H.264 360p (~0.7 MB, 0.9 s); `medium` → HEVC 720p
  for iPhone videos (no mode hands over iCloud's H.264 720p); `automatic` / `high`
  download the original — only for the Original button.
- Without network a cloud-only asset fails at once (`PHPhotosErrorDomain` 3164): a
  free "is it local". An image of a local HEIC is drawn from the original and leaves
  no file: the bytes PhotoKit hands over are served (JPEG), not kept.
- `PHAssetResource` lists only the original, the edit and its adjustments — whether
  a derivative is local is read from the DB / disk.
- **Asynchronous results come on the main queue**, which a Go program does not run:
  `photokit` locks `main` to the main thread (`LockOSThread` in `init`) and
  `RunMain` turns its run loop in place of waiting for a stop. Images are
  synchronous, videos come on any queue, Live Photos on the main queue. `NSImage`
  sizes are points: the pixels come from its `CGImage`. The `.m` file is
  `photokit_darwin.m` (a Linux build refuses a bare `.m`).
- **The permission goes to the terminal** that starts the binary (its "responsible"
  app), not to the binary: a rebuild does not drop it. Started by launchd — still
  open (roadmap). Access is asked only when a `*.photoslibrary` is under the root.
