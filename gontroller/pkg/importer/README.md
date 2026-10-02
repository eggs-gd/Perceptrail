# importer — the import chain

`importer` turns a library directory into items in the DB. It is a chain built on
[`perceplib/chain`](../../../perceplib/chain/README.md) (every step its own
goroutine, steps connected by channels) with one rule for its shape (roadmap
"Chains"): **the top has only linear stages**, each named by what it yields; every
stage is a sub-chain in its own package with one constructor (`New`) listing all
its steps; the top knows none of the tools — the file system, the providers,
exiftool, the plugins belong to the stage that uses them. One step does one thing.

```
discover → identify → core → plugins → commit
```

[`entry.go`](entry.go) (`NewImporterService`) wires the stages and holds what
spans them: the walk's progress, the two error channels, `Refresh` (one asset
again, without a walk).

```
importer/                  the top: the stages, Refresh
importer/flow/             what flows between the stages and steps; the walk's progress
importer/discover/         walk → group → gate: the groups that need work, the files table up to date
importer/discover/group/   the grouping sub-chain: the providers' switch and their groupers
importer/identify/         read (exiftool) → classify → validate → embedded → sizes → pick: the item known
importer/core/             the core perceptors (built in): they write into the item
importer/plugins/          the external perceptors (.so): they only read it
importer/commit/           keep (the perceptors' values) → close (the item): the item published
```

Stage packages export their steps' logic (`discover.NewGate`, `identify.NewReader`,
`identify.Classifier`, `identify.NewValidator`, `identify.NewEmbedded`,
`identify.NewSizes`, `identify.Pick`, `commit.NewKeep`, `commit.NewCloser`, …) — their
`New` runs it between channels; the tests of the whole import run it one group at a
time. Sub-packages cannot import `importer` (it imports them): what they share
lives in `flow`. Every step declares the DB methods it calls as its own small
interface (`GateStore`, `SweepStore`, `ValidatorStore`, `SizesStore`, `KindsStore`,
`CloserStore`); a stage's `Store` embeds its steps'; the top passes the proxy.
**No step knows the plugin manager**: the top (`entry.go`) is the only one that reads
`plugins.Pm` and hands each stage what it needs — `core` and `plugins` their lists of
perceptors, the gate `Unprocessed`, the sweep `Prune`, `keep` `SaveValues`, each as a
small interface. The sources are providers (`pkg/providers`: Apple Photos, the plain
folder last); the transcoders (`pkg/transcode`, not wired yet) are a chain of their
own later (fed from the DB).

Diagrams: [`Import chain.puml`](../../../_sb/puml/Import%20chain.puml) (the whole
chain), [`Walker.puml`](../../../_sb/puml/Walker.puml) (files gate and validator in
detail).

```
[discover: walk → group (switch → a grouper per provider) → gate]
  → [identify: read (exiftool, N) → classify → validate → embedded → sizes → pick]
  → [core: open → date, size, … → release]
  → [plugins: (to read-only → a .so perceptor → back) each, or pass]
  → [commit: keep (the perceptors' values) → close (Visible | Waiting)]
```

## Stages and their steps

| Stage / step | File | In -> out | What it does |
|---|---|---|---|
| **discover** | `discover/entry.go` | root -> `FileGroup` | The groups that need work; the files table up to date; one asset's group again (`Regroup`). |
| walk | `discover/walker.go` | root -> `FileEvent` | Reports every file (path + stat), then the end-of-walk marker. Unreadable subdirectories are skipped and recorded. |
| group | `discover/group/switch.go` | `FileEvent` -> `FileGroup` | A sub-chain: a switch sends a file to the grouper of the first enabled provider that claims it (the plain folder last: everything else), the marker to every grouper; each grouper is a step of it. |
| (plain folder grouper) | `pkg/providers/folder` | `FileEvent` -> `FileGroup` | Sidecars by name, next to each other: one open group; a complete group goes out, the last one with the marker. |
| (Apple Photos grouper) | `pkg/providers/apple` | `FileEvent` -> `FileGroup` | The first file of a library loads the assets from a copy of `Photos.sqlite` and forms the groups (files that exist, per the naming layout); a group goes out when its last file arrives. Key = asset UUID; the main file = the source; `Show` = the edit, the original, then Apple's derivatives. Trashed / hidden assets are not sent; incomplete groups are `Held` with the marker. Video renditions Photos downloads on request (`_2_3_o.mp4`, `_2_4_o.mp4`, `_2_201_o.mov`, `_2_101_o.mov`; `_a` instead of `_o` for an edit, preferred) are `motion`, after the stills; `apple.Local` finds the best file for an on-demand want; `Regroup` forms one asset's group again (the last load's DB rows + the disk now). |
| gate | `discover/gate.go` | `FileGroup` -> `FileGroup` | The files table (rows, stat, `CheckTime`); drops groups that need no work (and tells a keyed one's waiters); after every grouper's marker runs the deletions (`discover/sweep.go`: files not stamped, their items, the perceptors' rows). |
| **identify** | `identify/entry.go` | `FileGroup` -> `*RawItem` | The item known: identity, metadata, kinds and roles, what to show now. exiftool lives here. |
| read | `identify/read.go` | `FileGroup` -> `*RawItem` | `exiftool -all` for every file; N steps in parallel on the same channels, one pool. |
| classify | `identify/classify.go` | `*RawItem` -> `*RawItem` | The kind of every file; the main file (the source) first; roles. |
| validate | `identify/validate.go` | `*RawItem` -> `*RawItem` | Links the group, same / changed / moved / duplicate / broken -> the item. |
| embedded | `identify/embedded.go` | `*RawItem` -> `*RawItem` | Only when no file of the group shows (see pick): the main file's embedded preview (JpgFromRaw, PreviewImage, ThumbnailImage — the biggest first) extracted by exiftool into `cache/previews/<guid>/`, the RAW's Orientation copied onto it; `RawItem.Embedded`. After validate: needs the main file and the GUID. |
| sizes | `identify/sizes.go` | `*RawItem` -> `*RawItem` | Pixels and codec of every file the client may show (the original's from the metadata, images from their header), written to the files table. |
| pick | `identify/pick.go` | `*RawItem` -> `*RawItem` | What the browser shows now, no transcode: the source's `Show`, the main file (JPEG, PNG, …; H.264 video), the biggest viewable derivative, else the embedded one. Any size counts. |
| **core** | `core/entry.go` | `*RawItem` -> `*RawItem` | The built-in perceptors (date + zone, size, length), a step each, between `open` (skips a non-item; read-write view) and `release`. |
| **plugins** | `plugins/entry.go` | `*RawItem` -> `*RawItem` | The external `.so` perceptors, a step each with read-only adapters around it; none loaded: one pass step. |
| **commit** | `commit/entry.go` | `*RawItem` -> `*dto.ItemDto` | The item published. |
| keep | `commit/keep.go` | `*RawItem` -> `*RawItem` | A row in every import perceptor's storage: its value, or "processed, nothing found". Before close: an item published without them would be taken as done. |
| close | `commit/close.go` | `*RawItem` -> `*dto.ItemDto` | The item saved `Visible` (a preview) or `Waiting` (none). |

## Types (package [`flow`](flow/flow.go))

- `FileEvent` — one found file, or the end-of-walk marker.
- `FileGroup` — **one whole asset**: all its files (main
  file, sidecars, derivatives), or the marker (`Done`). Before the gate the files
  carry only their stat, after it they are rows of the files table (GUIDs).
- `RawItem` — the asset from exif to the closer: `Files`,
  `Exif`, `Kinds` are aligned. read fills `Files` + `Exif`, classify fills `Kinds`
  and puts the main file first, validate sets `Item`, embedded `Embedded`. Plugins see it through
  `exif_core.RawItemRW` / `api.RawItemR`.
- `WalkResult` — rides in the marker: root, start time,
  complete or not, unreadable directories.

## Rules that are easy to break

- **Groups are whole assets.** Groupers are plain decorators with their own buffer
  of open groups; a group goes out when it is complete, so each file closes at most
  one group.
- **The marker passes through the groupers**, not around them: the gate may derive
  deletions only after every grouper has sent its last group (all files stamped).
  It counts markers: one per provider's grouper.
- **Deletions are conservative**: only after a complete walk that found files, never
  under an unreadable directory, only under the configured root. A main file gone
  -> the item is soft-deleted; a sidecar gone -> the item is `Dirty`.
- **The walk repeats** (`rescan`, default 1 min) — counted from the moment the last
  group of the previous walk is done, not from the end of the walk: walks never
  overlap, no group is in the chain twice. The stages after the gate report to their
  own error channel; each passed group ends as an item or one error there
  (`flow/progress.go`).
- **Moves race with deletions** (the chain is asynchronous): the validator also finds
  soft-deleted items by hash and restores them, so a moved file keeps its GUID.
- **Item == asset**: a derivative never becomes an item of its own; in an Apple
  library every file of an asset links to its key.
- **Keyed groups** (`FileGroup.Key`, Apple): the key is the item's GUID, so a main
  file that changes (a derivative, then the downloaded original) keeps the item;
  mime does not re-rank them; exif reads only the main file; the cheap preview
  follows `Show`. An item with no files left is deleted.
- **The asset contract**: every file has a role (`original`, `edit`, `still`,
  `motion`, `frames`, `meta`) and a size; `/items` sends the asset by roles and
  the client decides what to show when (`pkg/client/routes/asset.go`). Roles come
  from the Apple grouper, else from mime; sizes from the header (images) or the
  metadata (the original only).
- **The source's metadata wins** (`FileGroup.Meta`, Apple: date + zone, oriented
  size, GPS from `Photos.sqlite` with exiftool's tag names): `RawItem.GetExif` reads
  it before the files' EXIF — the user may have corrected it in Photos, and a
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
- **exiftool args are part of the short hash**: changing `allTags` changes every hash
  (every item becomes "changed").

## Tests

- `discover/walker_test.go`, `discover/progress_test.go` — walk results (complete,
  unreadable dir, missing root, cancel); walks repeat only after the work is done.
- `discover/group/…_test.go`, `pkg/providers/folder/…_test.go` — the switch and the
  plain folder's grouper (names, directories, the marker).
- `pkg/providers/apple/grouper_test.go` — a fixture library: edit, cloud-only, Live
  Photo, trashed, Photos' own files, a file vanishing mid-walk (held, complete next
  walk).
- `identify/…_test.go` — kinds and the main file, roles, sizes, the cheap preview's
  pick (embedded → pick), an embedded preview's orientation (real exiftool), the kinds' table version.
- `apple_test.go` — a fixture library through the whole import: keys, previews,
  nothing to do on the next walk, a downloaded original, an asset moved to the trash,
  the gate telling a dropped asset.
- `steps_test.go` — a source that appears later, moved -> no reprocessing, not media
  remembered, a former main file gone.
- `validator_test.go` — whole walks through the stages' real steps on a temp library
  and a temp sqlite, exiftool replaced by `fakeExif` (the file content is the
  metadata): new / same / changed / moved / duplicate / deleted main and sidecar /
  unreadable / missing root / deleted then back / broken files.
