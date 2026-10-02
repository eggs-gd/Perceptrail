# scan — the import chain

`scan` turns a library directory into items in the DB. It is one chain of small
steps built on [`perceplib/chain`](../../../perceplib/chain/README.md): every step
is its own goroutine, steps are connected by channels. The chain is assembled in
[`entry.go`](entry.go) (`NewImporterService`); the comment there describes every
step, the comments at the channels say what each carries. One step = one file,
named after its constructor (`NewFilesGate` -> `filesgate.go`); groups of steps
with branches are sub-packages:

```
scan/                     fswalker, files gate, exif, mime, validator, plugins; the wiring
scan/flow/                what flows between the steps (shared by everything below)
scan/groups/              the grouping sub-chain: the providers' switch and their groupers
scan/transcode/           the switch by the kind of the asset (not wired yet)
scan/transcode/photo/     thumbnails (stub)
scan/transcode/video/     poster, previews, playable video (stub)
scan/transcode/livephoto/ the video with its photo (stub)
```

Every source is a provider (`pkg/providers`): Apple Photos (`providers/apple`), the
plain folder (`providers/folder`, last — it claims what nobody else did). Grouping
is a sub-chain (`groups.NewGrouping`), as processing is: files in, groups out;
inside, one switch sends a file to the grouper of the first provider that claims
it, each provider's grouper a step of its own. A new transcoder is a `transcode/<name>` package
+ a branch in `transcode.Switch`. Sub-packages cannot import `scan` (it imports
them): the types they share live in `flow`.

Diagrams: [`Import chain.puml`](../../../_sb/puml/Import%20chain.puml) (the whole
chain), [`Walker.puml`](../../../_sb/puml/Walker.puml) (files gate and validator in
detail).

```
fswalker -> [grouping: switch ─┬─ Apple Photos grouper ─┬─] -> files gate -> [processing: exif (N) -> mime -> validator
                               ├─ … (other providers)  │
                               └─ plain folder grouper ┘
         -> cheap preview -> plugins -> closer (Visible | Waiting)]

later, its own chain:  feeder (DB) -> transcode switch (photo | video | Live Photo) -> Ready
```

## Steps

| Step | File | In -> out | What it does |
|---|---|---|---|
| fswalker | `fswalker.go` | root -> `FileEvent` | Reports every file (path + stat), then the end-of-walk marker. Unreadable subdirectories are skipped and recorded. |
| grouping | `groups/switch.go` | `FileEvent` -> `FileGroup` | A sub-chain: a switch sends a file to the grouper of the first enabled provider that claims it (the plain folder last: everything else), the marker to every grouper; each grouper is a step of it. |
| plain folder grouper | `pkg/providers/folder` | `FileEvent` -> `FileGroup` | Sidecars by name, next to each other: one open group; a complete group goes out, the last one with the marker. |
| Apple Photos grouper | `pkg/providers/apple` | `FileEvent` -> `FileGroup` | The first file of a library loads the assets from a copy of `Photos.sqlite` and forms the groups (files that exist, per the naming layout); a group goes out when its last file arrives. Key = asset UUID; the main file = the source; `Show` = the edit, the original, then Apple's derivatives. Trashed / hidden assets are not sent; incomplete groups are `Held` with the marker. Video renditions Photos downloads on request (`_2_3_o.mp4`, `_2_4_o.mp4`, `_2_201_o.mov`, `_2_101_o.mov`; `_a` instead of `_o` for an edit, preferred) are `motion`, after the stills (the main file does not change); `apple.Local` finds the best file for an on-demand want; `Regroup` forms one asset's group again (the last load's DB rows + the disk now) for `importerService.Refresh`: after Photos made a file local, that asset is processed again through the gate without a walk. |
| files gate | `filesgate.go` | `FileGroup` -> `FileGroup` | The files table (rows, stat, `CheckTime`); drops groups that need no work; deletions after every grouper's marker. |
| exif | `exifextractor.go` | `FileGroup` -> `*RawItem` | `exiftool -all` for every file; N steps in parallel on the same channels. |
| mime | `mimeranker.go` | `*RawItem` -> `*RawItem` | The kind of every file; the main file (the source) first. |
| validator | `validator.go` | `*RawItem` -> `*RawItem` | Links the group, same / changed / moved / duplicate -> the item. |
| cheap preview | `cheappreview.go` | `*RawItem` -> `*RawItem` | What the browser shows now, no transcode: the main file (JPEG, PNG, …; H.264 video), else the biggest viewable derivative, else an embedded preview extracted into `cache/previews/<guid>/` (with the RAW's Orientation copied onto it). Any size counts. |
| plugins, closer | `exifpluginprocessor.go` | `*RawItem` -> `*dto.ItemDto` | Core plugins (date, size), external perceptors, then the item is saved `Visible` (a preview) or `Waiting` (none). |
| transcode switch + transcoders | `transcode/…` | `*RawItem` -> `*RawItem` | Not wired: the expensive chain (fed from the DB) comes with thumbnails and sets `Ready`. |

## Types (package [`flow`](flow/flow.go))

- `FileEvent` — one found file, or the end-of-walk marker.
- `FileGroup` — **one whole asset**: all its files (main
  file, sidecars, derivatives), or the marker (`Done`). Before the gate the files
  carry only their stat, after it they are rows of the files table (GUIDs).
- `RawItem` — the asset from exif to the closer: `Files`,
  `Exif`, `Kinds` are aligned. exif fills `Files` + `Exif`, mime fills `Kinds` and
  puts the main file first, the validator sets `Item`. Plugins see it through
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
  overlap, no group is in the chain twice. The steps after the gate report to their
  own error channel; each passed group ends as an item or one error there
  (`progress.go`).
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
- **Changing the kind detection** (`mimeranker.go`: the extension table, the fallbacks) needs a new
  `mimeVersion`: on start, a new version clears every "ignored" mark, so groups an
  older detection dropped are classified once more (stored in the `meta` table).
- **exiftool args are part of the short hash**: changing `allTags` changes every hash
  (every item becomes "changed").

## Tests

- `fswalker_test.go` — walk results (complete, unreadable dir, missing root, cancel).
- `groups/…_test.go`, `pkg/providers/folder/…_test.go`, `transcode/…_test.go` — the
  switches and the plain folder's grouper (names, directories, the marker).
- `pkg/providers/apple/grouper_test.go` — a fixture library: edit, cloud-only, Live Photo,
  trashed, Photos' own files, a file vanishing mid-walk (held, complete next walk).
- `apple_test.go` — a fixture library through the chain: keys, previews, nothing
  to do on the next walk, a downloaded original, an asset moved to the trash.
- `steps_test.go` — ranking, a source that appears later, moved -> no
  reprocessing, not media remembered.
- `validator_test.go` — whole walks through the real steps on a temp library and a
  temp sqlite, exiftool replaced by `fakeExif` (the file content is the metadata):
  new / same / changed / moved / duplicate / deleted main and sidecar / unreadable /
  missing root / deleted then back.
