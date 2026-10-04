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

A step's constructor takes what it really depends on, then its pipes — always in
one order: the model, the step's own dependencies, the logger, `in`, `out`.
`walk` the model (the files table) and the root,
`group` the providers, `gate` the model, `identify` the model and the cache directory (its exiftool is its own; the tags it
reads it asks the plugin registry: `plugins.ExifTags`), `exif` the model (rework, pruning; it reads the plugin
registry itself), `commit` the model. No callbacks between the steps.

[`entry.go`](entry.go) (`NewImporterService`) only wires the steps: the pipes
between them (typed: a step's in/out types are checked at compile time), one error
channel, and the passes: `Chain.Run` starts `walk` (the chain's one input, an
`Entry`) and returns once the walk's flush has reached `commit` (the chain's end);
then the pause (`rescan`), the next pass. `Refresh` (one asset again) only marks its
item (`MarkRework`): the next pass processes it. `walk` writes the rows of what it
sees and, after the walk, sends the rows it says are gone; the gate has the model
delete those; `exif` checks the perceptors' rows at start and prunes them on every
flush.

```
importer/                  the top: the steps wired, the passes, Refresh
importer/walk/             the chain's entry: the library's files as rows, then what the walk says is gone (Gone)
importer/group/            whole assets: the providers' switch and their groupers (a sub-chain)
importer/gate/             only the groups that need work pass; gone files deleted (the model's Gone)
importer/identify/         read → classify → merge → fingerprint → validate → embedded → sizes → pick → yield: the item known
importer/exif/             the EXIF perceptors (built in, then .so), their values kept; their tags, rework, pruning
importer/commit/           the item published
```

A step's `New` is its declaration: the chain of its steps, once. Step packages export
their logic (`gate.NewGate`, `identify.NewReader`, …) for unit tests (a step's own
tests may give it a fake exiftool, inside its package). The tests of the whole
import are integration tests: the real chain, one pass (`walk` → `group` → `gate` →
`identify`, a test end), with a real exiftool (identify runs its own pool, closed
when its steps stop) over real files (`fixtures_test.go`: a JPEG, a TIFF as a RAW,
QuickTime, HEIF, XMP — each with its content inside; a broken JPEG, a cut one). No
test hook in the code; CI installs exiftool.
**A type belongs to the package that produces it** (there is no shared package of
messages): see [Types](#types-who-owns-what). Every step declares the DB methods it calls as its own small
interface (`gate.Store`, `ValidatorStore`, `SizesStore`, `KindsStore`, `CloserStore`,
`walk.Store`, `exif.Items`); a sub-chain's `Store` embeds its steps'; the top passes
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
| **walk** | `walk/walker.go` | the root -> `*dto.FileDto` | The chain's `Entry`, a walk per pass: every file's row written as it goes (created, or its stat refreshed; `CheckTime`; `Changed`: new or its stat changed) and sent; after a complete walk the rows it did not stamp that it says are gone (`walk.Gone`: under the root, not under an unreadable directory) sent with `Gone`; then the flush. Unreadable subdirectories are skipped and recorded. |
| **group** | `group/switch.go` | `*dto.FileDto` -> `dto.Asset` | A sub-chain: a switch sends a file (a gone one too) to the grouper of the first enabled provider that claims it (the plain folder last: everything else), the walk's flush to every grouper (a `chain.Route`); each grouper is a step of it. |
| (plain folder grouper) | `pkg/providers/folder` | `*dto.FileDto` -> `dto.Asset` | Sidecars by name, next to each other: one open group; a complete group goes out, the last one on the walk's flush. A gone file passes through (an asset of its own). |
| (Apple Photos grouper) | `pkg/providers/apple` | `*dto.FileDto` -> `dto.Asset` | The first file of a library loads the assets from a copy of `Photos.sqlite` and forms the groups (files that exist, per the naming layout); a group goes out when its last file arrives. Key = asset UUID; the main file = the source; `Show` = the edit, the original, then Apple's derivatives. Trashed / hidden assets: their files are sent as gone (their items go); a gone file passes through; a group that did not complete waits for the next walk. Video renditions Photos downloads on request (`_2_3_o.mp4`, `_2_4_o.mp4`, `_2_201_o.mov`, `_2_101_o.mov`; `_a` instead of `_o` for an edit, preferred) are `motion`, after the stills; `apple.Local` finds the best file for an on-demand want. |
| **gate** | `gate/gate.go` | `dto.Asset` -> `dto.Asset` | Gone files: the model's `Gone` (a main file's item deleted, a sidecar's item `Dirty`). A group passes if a file changed (new, its stat or the role its grouper gave: stored here) or the model says it needs work (`NeedsWork`: links, state, fingerprint, `MetaHash`, `Rework`). |
| **identify** | `identify/entry.go` | `dto.Asset` -> `*identify.Item` | The item known: identity, metadata, kinds and roles, what to show now. exiftool lives here; its working item (`draft`: every file, its exif and kind) never leaves it. |
| read | `identify/read.go` | `dto.Asset` -> draft | One `exiftool -j -n` call for the whole group (a keyed group: the main file only), only the declared tags (see [exif](#exif-what-is-read-and-who-gets-it)), a map per file; N steps in parallel on the same channels, one pool. |
| classify | `identify/classify.go` | draft -> draft | The kind of every file; the main file (the source) first; roles. |
| merge | `identify/merge.go` | draft -> draft | The asset's metadata package: a tag from the source's metadata, else the metadata sidecars (.xmp), the main file, the derivatives; only the perceptors' tags. |
| fingerprint | `identify/fingerprint.go` | draft -> draft | The main file's identity across paths: its size and sha256 of its first and last 64 KB (no exiftool). |
| validate | `identify/validate.go` | draft -> draft | Not media or broken: the model ignores the group (`Ignore`); else the model says which item it is (`ValidateGroup` / `ValidateAsset`: links, superseded items, same / changed / moved / duplicate); the kind saved with it (`dto.AssetKind`). |
| embedded | `identify/embedded.go` | draft -> draft | Only when no file of the group shows (see pick): the main file's embedded preview (JpgFromRaw, PreviewImage, ThumbnailImage — the biggest first) extracted by exiftool into `cache/previews/<guid>/`, the RAW's Orientation copied onto it. After validate: needs the main file and the GUID. |
| sizes | `identify/sizes.go` | draft -> draft | Pixels and codec of every file the client may show (the original's from the metadata, images from their header), written to the files table. |
| pick | `identify/pick.go` | draft -> draft | What the browser shows now, no transcode: the source's `Show`, the main file (JPEG, PNG, …; H.264 video), the biggest viewable derivative, else the embedded one. Any size counts. |
| yield | `identify/item.go` | draft -> `*identify.Item` | What leaves the stage: the item and its metadata package. |
| **exif** | `exif/entry.go` | `*identify.Item` -> `*identify.Item` | The EXIF perceptors, a step each: the built-in ones (date + zone, size, length; read-write `plugins.RawItemRW`), then the external `.so` ones (read-only `api.RawItemR`); then keep — a row in every import perceptor's storage (its value, or "processed, nothing found"), before the item is published; at start the items a perceptor has not processed are marked for rework, on every flush the rows of gone items pruned. |
| **commit** | `commit/entry.go`, `close.go` | `*identify.Item` -> (the chain's end) | The model publishes the item (`Publish`: `Visible` with a preview, else `Waiting`); who waits for it hears (`published`, Refresh). |

## Types: who owns what

- **walk** yields `*dto.FileDto` — a row of the files table, with what this walk
  found (not stored: `Changed`, `Gone`). `walk.Result` (root, start, complete or not,
  files, unreadable directories) stays inside it: what it says is gone.
- **`dto.Asset`** ([`pkg/model/dto/asset.go`](../model/dto/asset.go)) — one whole
  asset before it is identified, the system's unit next to the item it becomes
  (`ItemDto`: item == asset): its files' rows, `Key`, `Show`, the source's `Meta` /
  `MetaHash`, `Kind`. A provider's grouper makes it from the walk's rows (a gone
  file it passes through, or makes something of it: an Apple asset trashed in Photos
  — its files sent as gone); the gate passes the ones that need work; identify reads
  it. The end of a walk is not a value: the chain flushes.
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
  what it holds (its last group) before passing it on, and the gate's input has a
  writer per grouper — the chain's barrier passes the flush on only once every
  grouper has flushed.
- **Seen is stamped by the walk**, not by the gate: a file of a group that did not
  complete, of a library whose DB did not load, or that no grouper wants is seen —
  not gone. Gone is what the walk did not see (or what a provider says is gone).
- **Deletions are conservative**: only after a complete walk that found files, never
  under an unreadable directory, only under the configured root. A main file gone
  -> the item is soft-deleted; a sidecar gone -> the item is `Dirty`.
- **The pass repeats** (`rescan`, default 1 min) — counted from the moment the
  previous walk's flush reached the end of the chain (`Chain.Run` returns: every
  group of it went through every step — a flush passes a step only after the values
  before it, `Parallel` waits for the ones in flight), not from the end of the walk:
  passes never overlap, no group is in the chain twice. The service owns the passes;
  the walk is the chain's entry. Nothing is counted per group: one dropped (a skip)
  or failed on the way is simply not there.
- **Refresh does not wait** (Apple on demand: a rendition made local): it marks the
  item (`Rework`, not touching `updated_at`, so no delta brings the item back in
  its old state); the client guesses meanwhile (the tile's cloud goes once it got
  the original), the next pass makes it true or takes it back.
- **The perceptors' rows of gone items** are pruned on a walk's flush at the exif
  step.
- **Moves and deletions**: the gone files come after every file the walk saw, but
  a moved file's old path may be deleted before identify has validated the new one;
  the validator finds soft-deleted items by fingerprint and restores them, so a
  moved file keeps its GUID. A sidecar gone makes its item `Dirty` before the
  grouper's flush gives the main file's group: processed in the same pass.
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
