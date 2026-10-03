# importer — the import chain

`importer` turns a library directory into items in the DB. It is a chain built on
[`perceplib/chain`](../../../perceplib/chain/README.md) (every step its own
goroutine, steps connected by typed pipes that carry values and a flush) with one rule for its shape (roadmap
"Chains"): **the top has only linear stages**, each named by what it yields; every
stage is a sub-chain in its own package with one constructor (`New`) listing all
its steps; the top knows none of the tools — the file system, the providers,
exiftool, the plugins belong to the stage that uses them. One step does one thing.

```
discover → identify → core → plugins → commit
```

[`entry.go`](entry.go) (`NewImporterService`) wires the stages and holds what
spans them: the pipes between the stages (typed: a stage's in/out types are
checked at compile time), the walk's progress, one error channel, the end of the
chain (a `Sink`: an item's waiters hear it; the walk's flush means its work is
done), `Refresh` (one asset again, without a walk).

```
importer/                  the top: the stages, Refresh
importer/discover/         walk → group → gate: the groups that need work, the files table up to date; the walk's progress
importer/discover/group/   the grouping sub-chain: the providers' switch and their groupers
importer/identify/         read → classify → merge → fingerprint → validate → embedded → sizes → pick → yield: the item known
importer/core/             the core perceptors (built in): they write into the item
importer/plugins/          the external perceptors (.so): they only read it
importer/commit/           keep (the perceptors' values) → close (the item): the item published
```

Stage packages export their steps' logic (`discover.NewGate`, `identify.Steps`,
`commit.NewKeep`, `commit.NewCloser`, …) — their `New` runs it between pipes;
the tests of the whole import run it one group at a time (`identify.Steps.Run`).
**A type belongs to the package that produces it** (there is no shared package of
messages): see [Types](#types-who-owns-what). Every step declares the DB methods it calls as its own small
interface (`GateStore`, `SweepStore`, `ValidatorStore`, `SizesStore`, `KindsStore`,
`CloserStore`); a stage's `Store` embeds its steps'; the top passes the proxy.
**No step knows the plugin registry**: the top (`entry.go`) is the only one that calls
`pkg/plugins` and hands each stage what it needs — `core` and `plugins` their lists
of perceptors, the gate and the sweep `discover.Perceptors{Unprocessed, Prune}`,
`keep` `plugins.SaveValues` — functions, so a step sees exactly one call. The sources are providers (`pkg/providers`: Apple Photos, the plain
folder last); the transcoders (`pkg/transcode`, not wired yet) are a chain of their
own later (fed from the DB).

Diagrams: [`Import chain.puml`](../../../_sb/puml/Import%20chain.puml) (the whole
chain), [`Walker.puml`](../../../_sb/puml/Walker.puml) (files gate and validator in
detail).

```
[discover: walk → group (switch → a grouper per provider) → gate]
  → [identify: read (one exiftool call per group, N) → classify → merge → fingerprint
             → validate → embedded → sizes → pick → yield]
  → [core: open → date, size, … → release]
  → [plugins: (to read-only → a .so perceptor → back) each, or pass]
  → [commit: keep (the perceptors' values) → close (Visible | Waiting)]
```

## Stages and their steps

| Stage / step | File | In -> out | What it does |
|---|---|---|---|
| **discover** | `discover/entry.go` | root -> `discover.Group` | The groups that need work; the files table up to date; one asset's group again (`Regroup`). |
| walk | `discover/walker.go` | root -> `providers.Found` | A `chain.Source`: reports every file (path + stat), records the walk (`discover.Walk`) and flushes the chain. Unreadable subdirectories are skipped and recorded. |
| group | `discover/group/switch.go` | `providers.Found` -> `providers.Group` | A sub-chain: a switch sends a file to the grouper of the first enabled provider that claims it (the plain folder last: everything else), the walk's flush to every grouper (a `chain.Route`); each grouper is a step of it. |
| (plain folder grouper) | `pkg/providers/folder` | `providers.Found` -> `providers.Group` | Sidecars by name, next to each other: one open group; a complete group goes out, the last one on the walk's flush. |
| (Apple Photos grouper) | `pkg/providers/apple` | `providers.Found` -> `providers.Group` | The first file of a library loads the assets from a copy of `Photos.sqlite` and forms the groups (files that exist, per the naming layout); a group goes out when its last file arrives. Key = asset UUID; the main file = the source; `Show` = the edit, the original, then Apple's derivatives. Trashed / hidden assets are not sent; incomplete groups' files are `Held`, given on the walk's flush. Video renditions Photos downloads on request (`_2_3_o.mp4`, `_2_4_o.mp4`, `_2_201_o.mov`, `_2_101_o.mov`; `_a` instead of `_o` for an edit, preferred) are `motion`, after the stills; `apple.Local` finds the best file for an on-demand want; `Regroup` forms one asset again (the last load's DB rows + the disk now). |
| gate | `discover/gate.go` | `providers.Group` -> `discover.Group` | The files table (rows, stat, `CheckTime`); drops groups that need no work (and tells a keyed one's waiters); turns a provider's asset into our format (stored rows); on the walk's flush (once every grouper has flushed) runs the deletions (`discover/sweep.go`: files not stamped, their items, the perceptors' rows). |
| **identify** | `identify/entry.go` | `discover.Group` -> `*identify.Item` | The item known: identity, metadata, kinds and roles, what to show now. exiftool lives here; its working item (`draft`: every file, its exif and kind) never leaves it. |
| read | `identify/read.go` | `discover.Group` -> draft | One `exiftool -j -n` call for the whole group (a keyed group: the main file only), only the declared tags (see [exif](#exif-what-is-read-and-who-gets-it)), a map per file; N steps in parallel on the same channels, one pool. |
| classify | `identify/classify.go` | draft -> draft | The kind of every file; the main file (the source) first; roles. |
| merge | `identify/merge.go` | draft -> draft | The asset's metadata package: a tag from the source's metadata, else the metadata sidecars (.xmp), the main file, the derivatives; only the perceptors' tags. |
| fingerprint | `identify/fingerprint.go` | draft -> draft | The main file's identity across paths: its size and sha256 of its first and last 64 KB (no exiftool). |
| validate | `identify/validate.go` | draft -> draft | Links the group, same / changed / moved / duplicate / broken -> the item. |
| embedded | `identify/embedded.go` | draft -> draft | Only when no file of the group shows (see pick): the main file's embedded preview (JpgFromRaw, PreviewImage, ThumbnailImage — the biggest first) extracted by exiftool into `cache/previews/<guid>/`, the RAW's Orientation copied onto it. After validate: needs the main file and the GUID. |
| sizes | `identify/sizes.go` | draft -> draft | Pixels and codec of every file the client may show (the original's from the metadata, images from their header), written to the files table. |
| pick | `identify/pick.go` | draft -> draft | What the browser shows now, no transcode: the source's `Show`, the main file (JPEG, PNG, …; H.264 video), the biggest viewable derivative, else the embedded one. Any size counts. |
| yield | `identify/item.go` | draft -> `*identify.Item` | What leaves the stage: the item and its metadata package. |
| **core** | `core/entry.go` | `*identify.Item` -> `*identify.Item` | The built-in perceptors (date + zone, size, length), a step each, between `open` (skips a non-item; read-write view) and `release`. |
| **plugins** | `plugins/entry.go` | `*identify.Item` -> `*identify.Item` | The external `.so` perceptors, a step each with read-only adapters around it; none loaded: one pass step. |
| **commit** | `commit/entry.go` | `*identify.Item` -> `*dto.ItemDto` | The item published. |
| keep | `commit/keep.go` | `*identify.Item` -> `*identify.Item` | A row in every import perceptor's storage: its value, or "processed, nothing found". Before close: an item published without them would be taken as done. |
| close | `commit/close.go` | `*identify.Item` -> `*dto.ItemDto` | The item saved `Visible` (a preview) or `Waiting` (none). |

## Types: who owns what

- **The provider contract** ([`pkg/providers/asset.go`](../providers/asset.go)), seen
  only by discover: `Found` (a found file), `Asset` (one whole asset as its source
  describes it: files with their stat, `Key`, `Show`, `Meta`, `MetaHash`, `Kind`),
  `Group` (what a grouper sends: an `Asset`; on the walk's flush the files it held
  back, `Held`). The end of a walk is not a value: the chain flushes.
- **`discover.Group`** — what discover yields, in our format: the asset's files as
  rows of the files table (GUIDs), what the source said about it. identify reads it
  and never sees a provider type. `discover.Walk` (the walk's result: root, start,
  complete or not, unreadable directories) and `discover.Progress` (the last walk,
  and whether its flush reached the end of the chain).
- **`identify.Item`** — what identify yields: the item and its metadata package. The
  perceptors read it (`api.RawItemR`), the core ones write into it
  (`exif_core.RawItemRW`), commit publishes `Item.Item`. identify's working `draft`
  (`Files`, `Exif`, `Kinds` aligned) is private to it.
- **`transcode.Item`** — the transcoders' input (an item and its files with roles),
  fed from the DB later, not by the import.

## exif: what is read and who gets it

- **Declared, never `-all`.** A perceptor declares the tags it reads
  (`api.ExifTagger`, required of every EXIF perceptor; `exif.CoordinateTags` for
  perceplib's `exif.Coordinates`); `plugins.ExifTags` is their union. identify adds its own
  (`read.go` `ownTags`: MIME type, errors, sizes, codec, whether embedded previews
  are there). A perceptor that reads an undeclared tag gets "" — each core
  perceptor's test checks it reads only what it declares (`exif_coretest`).
- **One call per group**: `exiftool -j -n -<tag>… file1 file2 …` — every file of
  the group, the sidecars too (a keyed group: the main file). **`-n`**: no print
  conversion, numbers as numbers — the composite `GPSLatitude` / `GPSLongitude`
  signed by their Ref (the EXIF ones would be unsigned: never asked), `Orientation`
  1–8, `Duration` in seconds, `ImageSize` "W H", QuickTime `GPSCoordinates` "lat
  lon [alt]"; dates, offsets, MIME, codecs and binary markers are unchanged.
  `Rotation` is degrees in QuickTime but quarter turns in HEIC (whose turn comes
  from `Orientation`): only a video's 90 / 270 turns it. Pinned by a probe test with
  a real exiftool (`identify/read_test.go`). A JSON value is kept as its text (a
  number's literal, a list joined by ", "). The Apple provider writes its record the
  same way (numbers).
- **The package** (merge): a tag from the source's metadata (the Photos DB: what the
  user corrected there), else the metadata sidecars (.xmp: they override the main
  file without changing it), the main file, then the derivatives (a fallback: a
  JPEG's size or orientation never overrides its RAW's). Only the perceptors' tags;
  nothing is stored — it travels with the item.
- **Plugins never run exiftool.** Only identify does: `read`, and `embedded` (the
  bytes of an embedded preview, only for a group with nothing to show).

## Rules that are easy to break

- **Groups are whole assets.** Groupers are plain decorators with their own buffer
  of open groups; a group goes out when it is complete, so each file closes at most
  one group.
- **The walk's flush passes through the groupers**, not around them: each gives
  what it holds (its last group, the files held back) before passing it on, and the
  gate's input has a writer per grouper — the chain's barrier passes the flush to
  the gate only once every grouper has flushed (all files stamped); then the gate
  runs the deletions.
- **Deletions are conservative**: only after a complete walk that found files, never
  under an unreadable directory, only under the configured root. A main file gone
  -> the item is soft-deleted; a sidecar gone -> the item is `Dirty`.
- **The walk repeats** (`rescan`, default 1 min) — counted from the moment the
  previous walk's flush reached the end of the chain (every group of it went through
  every step: a flush passes a step only after the values before it, `Parallel`
  waits for the ones in flight), not from the end of the walk: walks never overlap,
  no group is in the chain twice (`discover/progress.go`). Nothing is counted: a
  group dropped (a skip) or failed on the way is simply not there.
- **Moves race with deletions** (the chain is asynchronous): the validator also finds
  soft-deleted items by fingerprint and restores them, so a moved file keeps its GUID.
- **Item == asset**: a derivative never becomes an item of its own; in an Apple
  library every file of an asset links to its key.
- **Keyed groups** (`Key`, Apple): the key is the item's GUID, so a main
  file that changes (a derivative, then the downloaded original) keeps the item;
  classify does not re-rank them; read reads only the main file; the cheap preview
  follows `Show`. An item with no files left is deleted.
- **The asset contract**: every file has a role (`original`, `edit`, `still`,
  `motion`, `frames`, `meta`) and a size; `/items` sends the asset by roles and
  the client decides what to show when (`pkg/client/routes/asset.go`). Roles come
  from the Apple grouper, else from mime; sizes from the header (images) or the
  metadata (the original only).
- **The source's metadata wins** (`Meta`, Apple: date + zone, oriented
  size, GPS from `Photos.sqlite` with exiftool's tag names): merge takes it before
  the files' EXIF — the user may have corrected it in Photos, and a
  cloud-only asset has nothing else. `MetaHash` makes the gate reprocess a group
  whose DB metadata changed while its files did not.
- **States**: `Visible` (a cheap preview), `Waiting` (nothing viewable yet — HEIC,
  HEVC, a RAW without previews), `Ready` (the expensive stage, later). The client
  gets `Visible` and `Ready` only; `/assets/:guid` serves the preview.
- **The main file is the source**: RAW > video > image. The JPEG of RAW+JPEG and
  the photo of a Live Photo are derivatives (sidecars), future ready previews.
- **A link outside the group** (a file linked to a GUID that is not in its group:
  the main file is gone, e.g. a RAW deleted and its JPEG left) makes the gate pass
  the group, so the survivor becomes the item in the same walk.
- **Changing the kind detection** (`identify/classify.go`: the extension table, the fallbacks) needs a new
  `mimeVersion`: on start, a new version clears every "ignored" mark, so groups an
  older detection dropped are classified once more (stored in the `meta` table).
- **Changing the fingerprint** (`identify/fingerprint.go`) needs a new `hashVersion`:
  on start every item forgets its fingerprint (`ClearHashes`), the gate passes a
  group whose item has none, validate gives it the new one (same path, same GUID).
  Not `Dirty`: the client shows only `Visible` and `Ready`, the library would vanish
  for the whole pass. Without it an untouched file keeps the old fingerprint and its
  next move looks like a new item.

## Tests

- `discover/walker_test.go`, `discover/progress_test.go` — walk results (complete,
  unreadable dir, missing root, cancel); walks repeat only after the walk's flush
  reached the end.
- `discover/group/…_test.go`, `pkg/providers/folder/…_test.go` — the switch and the
  plain folder's grouper (names, directories, the flush).
- `pkg/providers/apple/grouper_test.go` — a fixture library: edit, cloud-only, Live
  Photo, trashed, Photos' own files, a file vanishing mid-walk (held, complete next
  walk).
- `identify/…_test.go` — kinds and the main file, roles, sizes, the cheap preview's
  pick (embedded → pick), an embedded preview's orientation and the group read
  (real exiftool), merge's priority, the fingerprint, the kinds' and the
  fingerprint's versions.
- `pkg/plugins/exif_core/*/tags_test.go` — every core perceptor reads only the tags
  it declares.
- `apple_test.go` — a fixture library through the whole import: keys, previews,
  nothing to do on the next walk, a downloaded original, an asset moved to the trash,
  the gate telling a dropped asset.
- `steps_test.go` — a source that appears later, moved -> no reprocessing, not media
  remembered, a former main file gone, a new fingerprint re-identifies every group
  once.
- `validator_test.go` — whole walks through the stages' real steps on a temp library
  and a temp sqlite, exiftool replaced by `fakeExif` (the file content is its
  metadata): new / same / changed / moved / duplicate / deleted main and sidecar /
  unreadable / missing root / deleted then back / broken files.
