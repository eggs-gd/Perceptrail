# importer — the import chain

`importer` turns a library directory into items in the DB. It is a chain built on
[`perceplib/chain`](../../../perceplib/chain/README.md) (every step its own
goroutine, steps connected by channels; a pass ends from its input) with one rule for its shape (roadmap
"Chains"): **the top has only linear stages**, each named by what it yields; every
stage is a sub-chain in its own package with one constructor (`New`) listing all
its steps; the top knows none of the tools — the file system, the providers,
exiftool, the plugins belong to the stage that uses them. One step does one thing.

```
walk → group → gate → identify → exif → commit
```

A step's constructor takes what it really depends on, then its channels — always in
one order: the model, the step's own dependencies, the logger, `in`, `out`.
`walk` the model (the files table) and the root,
`group` the providers, `gate` the model, `identify` the model and the cache directory (its exiftool is its own; the tags it
reads it asks the plugin registry: `perceptor.ExifTags`), `exif` the model (rework, pruning; it reads the plugin
registry itself), `commit` the model. No callbacks between the steps.

[`entry.go`](entry.go) is the service: `importChain` wires one pass — the channels
between the steps (typed: a step's in/out types are checked at compile time), one
error channel; `Start` first does what changed since the last run
(`identify.Migrate`: the kinds' table, the fingerprint; `exif.MarkUnprocessed`: new
perceptors), then pass after pass: a new chain, `Process` to its end (`walk`, the
chain's one input, returns and its output closes; each step ends after its input;
`commit` last), the pause (`rescan`). `Refresh` (one asset again) only marks its
item (`MarkRework`): the next pass processes it. `walk` writes the rows of what it
sees and, after the walk, sends the rows it says are gone; the gate has the model
delete those; `exif` prunes the perceptors' rows of gone items at the end of a
pass.

```
importer/                  the top: the steps wired, the passes, Refresh
importer/walk/             the chain's entry: the library's files as rows, then what the walk says is gone (Gone)
importer/group/            whole assets: the providers' switch and their groupers (a sub-chain)
importer/gate/             only the groups that need work pass; gone files deleted (the model's Gone)
importer/identify/         read (exiftool, classify, merge, fingerprint) → validate → show (sizes, pick): the item known
importer/exif/             the EXIF perceptors (built in, then .so), their values kept; their tags, rework, pruning
importer/commit/           the item published
```

A step's `New` is its declaration: the chain of its steps, once. A step's own tests
live in its package (and may give it a fake exiftool there). The tests of the whole
import are integration tests: the service's own chain (`importChain`), one pass,
with a real exiftool (identify's pool, started by the first read, closed when its
steps stop) over real files what a pass processed is read from the DB (the items
it published) (`fixtures_test.go`: a JPEG, a TIFF as a RAW,
QuickTime, HEIF, XMP — each with its content inside; a broken JPEG, a cut one). No
test hook in the code; CI installs exiftool.
**A type belongs to the package that produces it** (there is no shared package of
messages): see [Types](#types-who-owns-what). Every step declares the DB methods it calls as its own small
interface (`walk.Store`, `gate.Store`, `ValidatorStore`, `SizesStore`, `KindsStore`,
`exif.Items`, `commit.Store`); a sub-chain's `Store` embeds its steps'; the top's
`store` embeds the stages' and holds the proxy.
**What a perceptor means to the import is the exif step's**
([`exif/`](exif/)), not the registry's and not scattered over the steps: which ones
run (EXIF data: the built-in ones, then the external), their rows written (its last step, `keep`), and two bits of
bookkeeping the top calls — **at start** (`MarkUnprocessed`) an item an import
perceptor has no row for (the perceptor is new, or its schema changed) is marked
for rework (`MarkRework`: the gate sends its group once more; publishing clears the
mark); **after each walk** (`Prune`) the rows of gone items go. It reads the
registry itself (`perceptor.All`, `perceptor.Store`); walk, group, gate and commit know
nothing of perceptors. The sources are providers (`pkg/library`: Apple Photos, the plain
folder last); the transcoders (`pkg/transcode`, not wired yet) are a chain of their
own later (fed from the DB).

Diagrams: [`Import chain.puml`](../../../_sb/puml/Import%20chain.puml) (the whole
chain), [`Walker.puml`](../../../_sb/puml/Walker.puml) (files gate and validator in
detail).

```
[walk] → [group: switch → a grouper per provider] → [gate]
  → [identify: read (N: one exiftool call per group, classify, merge, fingerprint)
             → validate → show (sizes, pick, an embedded preview)]
  → [exif: exif_date → exif_size → exif_duration → each .so (read-only) → keep]
  → [commit: close (Visible | Waiting)]
```

## Stages and their steps

| Stage / step | File | In -> out | What it does |
|---|---|---|---|
| **walk** | `walk/walker.go` | the root -> `*dto.FileDto` | The chain's entry point, a walk per pass: every file's row written as it goes (created, or its stat refreshed; `CheckTime`; `Changed`: new or its stat changed — stored, cleared only by identify's validate, so a pass that fails before deciding the group leaves it for the next) and sent; after a complete walk the rows it did not stamp that it says are gone (`walk.Missing`: under the root, not under an unreadable directory) sent with `Gone`; then it returns (its output closes). Unreadable subdirectories are skipped and recorded. |
| **group** | `group/switch.go` | `*dto.FileDto` -> `dto.Asset` | A sub-chain: a switch sends a file (a gone one too) to the grouper of the first enabled provider that claims it (the plain folder last: everything else), its end reaches every grouper (a `chain.NewSwitch`: its outputs close when it returns); each grouper is a step of it. |
| (plain folder grouper) | `pkg/library/folder` | `*dto.FileDto` -> `dto.Asset` | Sidecars by name, next to each other: one open group; a complete group goes out, the last one when its input ends. A gone file passes through (an asset of its own). |
| (Apple Photos grouper) | `pkg/library/apple` | `*dto.FileDto` -> `dto.Asset` | The first file of a library loads the assets from a copy of `Photos.sqlite` and forms the groups (files that exist, per the naming layout); a group goes out when its last file arrives. Key = asset UUID; the main file = the source; `Show` = the edit, the original, then Apple's derivatives. Trashed / hidden assets: their files are sent as gone (their items go); a gone file passes through; a group that did not complete waits for the next walk. Video renditions Photos downloads on request (`_2_3_o.mp4`, `_2_4_o.mp4`, `_2_201_o.mov`, `_2_101_o.mov`; `_a` instead of `_o` for an edit, preferred) are `motion`, after the stills; `apple.Local` finds the best file for an on-demand want. |
| **gate** | `gate/gate.go` | `dto.Asset` -> `dto.Asset` | Gone files: the model's `Gone` (a main file's item deleted, a sidecar's item `Dirty`). A group passes if a file changed (new, its stat or the role its grouper gave: stored here) or the model says it needs work (`NeedsWork`: links, state, fingerprint, `MetaHash`, `Rework`). |
| **identify** | `identify/entry.go` | `dto.Asset` -> `*identify.Item` | The item known: identity, metadata, kinds and roles, what to show now. exiftool lives here; its working item (`draft`: every file, its exif and kind) never leaves it. |
| read | `identify/read.go` | `dto.Asset` -> draft | Everything known without the DB, groups in parallel (N workers, one exiftool pool): one `exiftool -j -n` call for the whole group (a keyed group: the main file only), only the declared tags (see [exif](#exif-what-is-read-and-who-gets-it)); the kind of every file, the main file (the source) first, roles (`classify.go`); the metadata package — a tag from the source's metadata, else the .xmp sidecars, the main file, the derivatives; only the perceptors' tags (`merge.go`); the main file's fingerprint — its size and sha256 of its first and last 64 KB (`fingerprint.go`). |
| validate | `identify/validate.go` | draft -> draft | One at a time: not media or broken, the model ignores the group (`Ignore`); else the model says which item it is (`ValidateGroup` / `ValidateAsset`: links, superseded items, same / changed / moved / duplicate); the kind saved with it (`dto.AssetKind`). |
| show | `identify/show.go` | draft -> `*identify.Item` | Pixels and codec of every file the client may show (the original's from the metadata, images from their header), written to the files table (`sizes.go`); what the browser shows now, no transcode: the source's `Show`, the main file (JPEG, PNG, …; H.264 video), the biggest viewable derivative, else the main file's embedded preview (JpgFromRaw, PreviewImage, ThumbnailImage — the biggest first) extracted by exiftool into `cache/previews/<guid>/`, the RAW's Orientation copied onto it (`pick.go`); then the item leaves the stage. It closes the exiftool pool when it stops. |
| **exif** | `exif/entry.go` | `*identify.Item` -> `*identify.Item` | The EXIF perceptors, a step each: the built-in ones (date + zone, size, length; read-write `builtin.Item`), then the external `.so` ones (read-only `api.RawItemR`); then keep — a row in every import perceptor's storage (its value, or "processed, nothing found"), before the item is published; at start the items a perceptor has not processed are marked for rework (`MarkUnprocessed`, at the service's start), at the end of every pass the rows of gone items pruned. |
| **commit** | `commit/commit.go` | `*identify.Item` -> (the chain's end) | The model publishes the item (`Publish`: `Visible` with a preview, else `Waiting`). |

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
  it. The end of a walk is not a value: the walk's output closes.
- **`identify.Item`** — what identify yields: the item and its metadata package. The
  perceptors read it (`api.RawItemR`), the core ones write into it
  (`builtin.Item`), commit publishes `Item.Item`. identify's working `draft`
  (`Files`, `Exif`, `Kinds` aligned) is private to it.
- **`transcode.Item`** — the transcoders' input (an item and its files with roles),
  fed from the DB later, not by the import.

## exif: what is read and who gets it

- **Declared, never `-all`.** A perceptor declares the tags it reads
  (`api.ExifTagger`, required of every EXIF perceptor; `exif.CoordinateTags` for
  perceplib's `exif.Coordinates`); the registry's `perceptor.ExifTags` is their union (identify asks it). identify adds its own
  (`read.go` `ownTags`: MIME type, errors, sizes, codec, whether embedded previews
  are there). A perceptor that reads an undeclared tag gets "" — each core
  perceptor's test checks it reads only what it declares (`builtintest`).
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
- **Plugins never run exiftool.** Only identify does: `read`, and `show` (the bytes
  of an embedded preview, only for a group with nothing else to show).

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
- **The end of the walk passes through the groupers**, not around them: when its
  input closes, each gives what it holds (its last group) and returns; the gate's
  input has a writer per grouper and closes only after the last of them returned.
- **Seen is stamped by the walk**, not by the gate: a file of a group that did not
  complete, of a library whose DB did not load, or that no grouper wants is seen —
  not gone. Gone is what the walk did not see (or what a provider says is gone).
- **Deletions are conservative**: only after a complete walk that found files, never
  under an unreadable directory, only under the configured root. A main file gone
  -> the item is soft-deleted; a sidecar gone -> the item is `Dirty`.
- **The pass repeats** (`rescan`, default 1 min) — counted from the end of the
  previous pass (`Process` returns: every step has, so every group of the walk went
  through every step), not from the end of the walk: passes never overlap, no group
  is in the chain twice. A pass is a new chain: channels close once. The service owns the passes;
  the walk is the chain's entry. Nothing is counted per group: one dropped (a skip)
  or failed on the way is simply not there.
- **Refresh does not wait** (Apple on demand: a rendition made local): it marks the
  item (`Rework`, not touching `updated_at`, so no delta brings the item back in
  its old state); the client guesses meanwhile (the tile's cloud goes once it got
  the original), the next pass makes it true or takes it back.
- **The perceptors' rows of gone items** are pruned at the end of a pass (the exif
  step's input ended: the gate has done the walk's deletions by then).
- **Moves and deletions**: the gone files come after every file the walk saw, but
  a moved file's old path may be deleted before identify has validated the new one;
  the validator finds soft-deleted items by fingerprint and restores them, so a
  moved file keeps its GUID. A sidecar gone makes its item `Dirty` before the
  grouper gives the main file's group at the end of its input: processed in the same
  pass.
- **Item == asset**: a derivative never becomes an item of its own; in an Apple
  library every file of an asset links to its key.
- **Keyed groups** (`Key`, Apple): the key is the item's GUID, so a main
  file that changes (a derivative, then the downloaded original) keeps the item;
  classify does not re-rank them; read reads only the main file; the cheap preview
  follows `Show`. An item with no files left is deleted.
- **The asset contract**: every file has a role (`original`, `edit`, `still`,
  `motion`, `frames`, `meta`) and a size; `/items` sends the asset by roles and
  the client decides what to show when (`pkg/web/route/asset.go`). Roles come
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
  cancel); a pass: the rows, then the gone ones; what a walk says is gone.
- `group/…_test.go`, `pkg/library/folder/…_test.go` — the switch and the plain
  folder's grouper (names, directories, the end of its input).
- `pkg/library/apple/grouper_test.go` — a fixture library: edit, cloud-only, Live
  Photo, trashed (sent as gone), Photos' own files, a file vanishing mid-walk (its
  asset complete next walk).
- `identify/…_test.go` — kinds and the main file, roles, sizes, the cheap preview's
  pick (an embedded preview last), an embedded preview's orientation and the group read
  (real exiftool), merge's priority, the fingerprint, the kinds' and the
  fingerprint's versions.
- `pkg/perceptor/exif_{date,size,duration}/tags_test.go` — every core perceptor reads
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
