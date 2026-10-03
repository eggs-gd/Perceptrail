# importer — the import chain

`importer` turns a library directory into items in the DB. It is a chain built on
[`perceplib/chain`](../../../perceplib/chain/README.md) (every step its own
goroutine, steps connected by typed pipes that carry values and a flush) with one rule for its shape (roadmap
"Chains"): **the top has only linear stages**, each named by what it yields; every
stage is a sub-chain in its own package with one constructor (`New`) listing all
its steps; the top knows none of the tools — the file system, the providers,
exiftool, the plugins belong to the stage that uses them. One step does one thing.

```
walk → group → gate → identify → exif → commit
```

A step's constructor takes what it really depends on and its pipes: `walk` the root,
`group` the providers, `gate` the model, `identify` the model and the cache directory (its exiftool is its own; the tags it
reads it asks the plugin registry: `plugins.ExifTags`), `exif` the logger (it reads the plugin
registry itself), `commit` the model. No callbacks between the steps.

[`entry.go`](entry.go) (`NewImporterService`) wires the stages and holds what
spans them: the pipes between the steps (typed: a step's in/out types are checked
at compile time), one error channel, the end of the chain (a `Sink`: an item's
waiters hear it; the walk's flush means its work is done), the **walk cycle**
([`cycle.go`](cycle.go): after a walk's flush reached the end — its deletions, the
perceptors' rows of gone items, the rescan pause, the next walk), `Refresh` (one
asset again, without a walk); at start and after a walk it calls the exif step's
bookkeeping (`exif.MarkUnprocessed`, `exif.Prune`).

```
importer/                  the top: the steps, the walk cycle, Refresh
importer/walk/             the library's files, one walk when asked; what a walk says is gone (Gone)
importer/group/            whole assets: the providers' switch and their groupers (a sub-chain)
importer/gate/             the files table up to date; only the groups that need work pass
importer/identify/         read → classify → merge → fingerprint → validate → embedded → sizes → pick → yield: the item known
importer/exif/             the EXIF perceptors (built in, then .so), their values kept; their tags, rework, pruning
importer/commit/           the item published
```

A step's `New` is its declaration: the chain of its steps, once. Step packages export
their logic (`gate.NewGate`, `identify.NewReader`, …) for unit tests; the tests of the
whole import drive the real chains (`identify.New` between pipes: `Send` a group,
`Flush`, read what came out), with exiftool replaced: identify runs its own (a pool
of processes, closed when its steps stop), a test gives a fake
(`identify.WithExiftool`).
**A type belongs to the package that produces it** (there is no shared package of
messages): see [Types](#types-who-owns-what). Every step declares the DB methods it calls as its own small
interface (`gate.Store`, `ValidatorStore`, `SizesStore`, `KindsStore`, `CloserStore`,
the cycle's `CycleStore`); a sub-chain's `Store` embeds its steps'; the top passes
the proxy.
**What a perceptor means to the import is the exif step's**
([`exif/`](exif/)), not the registry's and not scattered over the steps: which ones
run (EXIF data: the built-in ones, then the external), their rows written (its last step, `keep`), and two bits of
bookkeeping the top calls — **at start** (`MarkUnprocessed`) an item an import
perceptor has no row for (the perceptor is new, or its schema changed) is marked
for rework (`MarkRework`: the gate sends its group once more; publishing clears the
mark); **after each walk** (`Prune`) the rows of gone items go. It reads the
registry itself (`plugins.All`, `plugins.Store`); walk, group, gate and commit know
nothing of perceptors. The sources are providers (`pkg/providers`: Apple Photos, the plain
folder last); the transcoders (`pkg/transcode`, not wired yet) are a chain of their
own later (fed from the DB).

Diagrams: [`Import chain.puml`](../../../_sb/puml/Import%20chain.puml) (the whole
chain), [`Walker.puml`](../../../_sb/puml/Walker.puml) (files gate and validator in
detail).

```
[walk] → [group: switch → a grouper per provider] → [gate]
  → [identify: read (one exiftool call per group, N) → classify → merge → fingerprint
             → validate → embedded → sizes → pick → yield]
  → [exif: exif_date → exif_size → exif_duration → each .so (read-only) → keep]
  → [commit: close (Visible | Waiting)]
```

## Stages and their steps

| Stage / step | File | In -> out | What it does |
|---|---|---|---|
| **walk** | `walk/walker.go` | a request (`Next`) -> `dto.ItemEntry` | A `chain.Source`: on each request walks the root — every file (path + stat) — records the walk (`walk.Result`, `Last`) and flushes the chain. Unreadable subdirectories are skipped and recorded. `walk.Gone`: of the files a walk did not stamp, the ones it says are deleted. |
| **group** | `group/switch.go` | `dto.ItemEntry` -> `providers.Group` | A sub-chain: a switch sends a file to the grouper of the first enabled provider that claims it (the plain folder last: everything else), the walk's flush to every grouper (a `chain.Route`); each grouper is a step of it. |
| (plain folder grouper) | `pkg/providers/folder` | `dto.ItemEntry` -> `providers.Group` | Sidecars by name, next to each other: one open group; a complete group goes out, the last one on the walk's flush. |
| (Apple Photos grouper) | `pkg/providers/apple` | `dto.ItemEntry` -> `providers.Group` | The first file of a library loads the assets from a copy of `Photos.sqlite` and forms the groups (files that exist, per the naming layout); a group goes out when its last file arrives. Key = asset UUID; the main file = the source; `Show` = the edit, the original, then Apple's derivatives. Trashed / hidden assets are not sent; incomplete groups' files are `Held`, given on the walk's flush. Video renditions Photos downloads on request (`_2_3_o.mp4`, `_2_4_o.mp4`, `_2_201_o.mov`, `_2_101_o.mov`; `_a` instead of `_o` for an edit, preferred) are `motion`, after the stills; `apple.Local` finds the best file for an on-demand want; `Regroup` forms one asset again (the last load's DB rows + the disk now). |
| **gate** | `gate/gate.go` | `providers.Group` -> `gate.Group` | The files table (rows, stat, `CheckTime`; the files a grouper held back stamped too); a group whose files did not change passes only if the model says it needs work (`NeedsWork`) or it was asked for (`Requested`); turns a provider's asset into our format (stored rows). Knows nothing of walks or deletions. |
| **identify** | `identify/entry.go` | `gate.Group` -> `*identify.Item` | The item known: identity, metadata, kinds and roles, what to show now. exiftool lives here; its working item (`draft`: every file, its exif and kind) never leaves it. |
| read | `identify/read.go` | `gate.Group` -> draft | One `exiftool -j -n` call for the whole group (a keyed group: the main file only), only the declared tags (see [exif](#exif-what-is-read-and-who-gets-it)), a map per file; N steps in parallel on the same channels, one pool. |
| classify | `identify/classify.go` | draft -> draft | The kind of every file; the main file (the source) first; roles. |
| merge | `identify/merge.go` | draft -> draft | The asset's metadata package: a tag from the source's metadata, else the metadata sidecars (.xmp), the main file, the derivatives; only the perceptors' tags. |
| fingerprint | `identify/fingerprint.go` | draft -> draft | The main file's identity across paths: its size and sha256 of its first and last 64 KB (no exiftool). |
| validate | `identify/validate.go` | draft -> draft | Not media or broken: the model ignores the group (`Ignore`); else the model says which item it is (`ValidateGroup` / `ValidateAsset`: links, superseded items, same / changed / moved / duplicate); the kind saved with it (`dto.AssetKind`). |
| embedded | `identify/embedded.go` | draft -> draft | Only when no file of the group shows (see pick): the main file's embedded preview (JpgFromRaw, PreviewImage, ThumbnailImage — the biggest first) extracted by exiftool into `cache/previews/<guid>/`, the RAW's Orientation copied onto it. After validate: needs the main file and the GUID. |
| sizes | `identify/sizes.go` | draft -> draft | Pixels and codec of every file the client may show (the original's from the metadata, images from their header), written to the files table. |
| pick | `identify/pick.go` | draft -> draft | What the browser shows now, no transcode: the source's `Show`, the main file (JPEG, PNG, …; H.264 video), the biggest viewable derivative, else the embedded one. Any size counts. |
| yield | `identify/item.go` | draft -> `*identify.Item` | What leaves the stage: the item and its metadata package. |
| **exif** | `exif/entry.go` | `*identify.Item` -> `*identify.Item` | The EXIF perceptors, a step each: the built-in ones (date + zone, size, length; read-write `plugins.RawItemRW`), then the external `.so` ones (read-only `api.RawItemR`); then keep — a row in every import perceptor's storage (its value, or "processed, nothing found"), before the item is published. |
| **commit** | `commit/entry.go`, `close.go` | `*identify.Item` -> `*dto.ItemDto` | The model publishes the item (`Publish`: `Visible` with a preview, else `Waiting`). |

## Types: who owns what

- **walk** yields `dto.ItemEntry` (a file: path, stat) and, to the top, `walk.Result`
  (root, start, complete or not, files, unreadable directories) — the deletions'
  input.
- **The provider contract** ([`pkg/providers/asset.go`](../providers/asset.go)),
  between group and gate: a grouper takes `dto.ItemEntry`, gives `Group` — an
  `Asset` (one whole asset as its source describes it: files with their stat, `Key`,
  `Show`, `Meta`, `MetaHash`, `Kind`); on the walk's flush the files it held back
  (`Held`: the gate stamps them, they are not gone); `Requested` (Refresh: processed
  even if nothing changed). The end of a walk is not a value: the chain flushes.
- **`gate.Group`** — what the gate yields, in our format: the asset's files as rows
  of the files table (GUIDs), what the source said about it. identify reads it and
  never sees a provider type.
- **`identify.Item`** — what identify yields: the item and its metadata package. The
  perceptors read it (`api.RawItemR`), the core ones write into it
  (`plugins.RawItemRW`), commit publishes `Item.Item`. identify's working `draft`
  (`Files`, `Exif`, `Kinds` aligned) is private to it.
- **`transcode.Item`** — the transcoders' input (an item and its files with roles),
  fed from the DB later, not by the import.

## exif: what is read and who gets it

- **Declared, never `-all`.** A perceptor declares the tags it reads
  (`api.ExifTagger`, required of every EXIF perceptor; `exif.CoordinateTags` for
  perceplib's `exif.Coordinates`); the registry's `plugins.ExifTags` is their union (identify asks it). identify adds its own
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

## The model decides, the steps gather facts

The rules about the library's data live in the model ([`pkg/model/itemslife.go`](../model/itemslife.go)),
not in the steps: every chain that touches items (the import, an asset again on
demand, later the API providers and the maintenance) goes by the same ones. A step
gathers the facts — stat, exif, kinds, the fingerprint — and the model decides and
keeps: which item a group is (`ValidateGroup`, `ValidateAsset`), whether an unchanged
group needs work (`NeedsWork`), what a file gone means (`Gone`), a group that is no
item (`Ignore`), an item published (`Publish`), what nothing can show yet
(`Unshown`), what an asset is (`dto.AssetKind`, saved with the item; the client and
the transcoders read the same rule). Each step still sees only the model's methods
it calls (its own small interface).

## Rules that are easy to break

- **Groups are whole assets.** Groupers are plain decorators with their own buffer
  of open groups; a group goes out when it is complete, so each file closes at most
  one group.
- **The walk's flush passes through the groupers**, not around them: each gives
  what it holds (its last group, the files held back) before passing it on, and the
  gate's input has a writer per grouper — the chain's barrier passes the flush on
  only once every grouper has flushed. The deletions come when the flush reached
  the end of the chain (the cycle): every file the walk saw is stamped by then.
- **Deletions are conservative**: only after a complete walk that found files, never
  under an unreadable directory, only under the configured root. A main file gone
  -> the item is soft-deleted; a sidecar gone -> the item is `Dirty`.
- **The walk repeats** (`rescan`, default 1 min) — counted from the moment the
  previous walk's flush reached the end of the chain (every group of it went through
  every step: a flush passes a step only after the values before it, `Parallel`
  waits for the ones in flight), not from the end of the walk: walks never overlap,
  no group is in the chain twice (`cycle.go`: the walker walks only when the cycle
  asks). Nothing is counted: a group dropped (a skip) or failed on the way is simply
  not there.
- **Moves and deletions**: the deletions come after the walk's groups went through
  the whole chain, so a moved file is validated before its old path is deleted; the
  validator also finds soft-deleted items by fingerprint and restores them (a file
  moved back later), so a moved file keeps its GUID.
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

- `walk/walker_test.go` — walk results (complete, unreadable dir, missing root,
  cancel); a walk only when asked; what a walk says is gone.
- `group/…_test.go`, `pkg/providers/folder/…_test.go` — the switch and the plain
  folder's grouper (names, directories, the flush).
- `pkg/providers/apple/grouper_test.go` — a fixture library: edit, cloud-only, Live
  Photo, trashed, Photos' own files, a file vanishing mid-walk (held, complete next
  walk).
- `identify/…_test.go` — kinds and the main file, roles, sizes, the cheap preview's
  pick (embedded → pick), an embedded preview's orientation and the group read
  (real exiftool), merge's priority, the fingerprint, the kinds' and the
  fingerprint's versions.
- `pkg/plugins/exif_{date,size,duration}/tags_test.go` — every core perceptor reads
  only the tags it declares.
- `apple_test.go` — a fixture library through the whole import: keys, previews,
  nothing to do on the next walk, a downloaded original, an asset moved to the trash,
  one asset asked again on demand (Refresh: processed though nothing changed).
- `steps_test.go` — a source that appears later, moved -> no reprocessing, not media
  remembered, a former main file gone, a new fingerprint re-identifies every group
  once.
- `validator_test.go` — whole walks through the stages' real steps on a temp library
  and a temp sqlite, exiftool replaced by `fakeTool` (the file content is its
  metadata): new / same / changed / moved / duplicate / deleted main and sidecar /
  unreadable / missing root / deleted then back / broken files.
