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
scan/groups/              the source switch
scan/groups/generic/      plain folders: sidecars by name
scan/groups/apple/        Apple Photos library (stub)
scan/transcode/           the switch by the kind of the asset
scan/transcode/photo/     thumbnails (stub)
scan/transcode/video/     poster, previews, playable video (stub)
scan/transcode/livephoto/ the video with its photo (stub)
```

A new source is a new `groups/<name>` package + a branch in `groups.SourceSwitch`
(+ `groups.Branches`); a new transcoder is a `transcode/<name>` package + a branch
in `transcode.Switch`. Sub-packages cannot import `scan` (it imports them): the
types they share live in `flow`.

Diagrams: [`Import chain.puml`](../../../_sb/puml/Import%20chain.puml) (the whole
chain), [`Walker.puml`](../../../_sb/puml/Walker.puml) (files gate and validator in
detail).

```
fswalker -> source switch ─┬─ generic grouper ───┬─> files gate -> exif (N) -> mime -> validator
                           └─ Apple Photos (stub)┘
         -> transcode switch ─┬─ photo ──────┬─> plugins -> closer
                              ├─ video ──────┤
                              └─ Live Photo ─┘
```

## Steps

| Step | File | In -> out | What it does |
|---|---|---|---|
| fswalker | `fswalker.go` | root -> `FileEvent` | Reports every file (path + stat), then the end-of-walk marker. Unreadable subdirectories are skipped and recorded. |
| source switch | `groups/sourceswitch.go` | `FileEvent` -> `FileEvent` | Routes a file to the grouper of its source; the marker goes to every grouper. |
| generic grouper | `groups/generic` | `FileEvent` -> `FileGroup` | Sidecars by name, next to each other: one open group; a complete group goes out, the last one with the marker. |
| Apple Photos grouper | `groups/apple` | `FileEvent` -> `FileGroup` | Stub: passes the marker on. `appleEnabled = false` sends the library to generic, which reads only its `originals/`. |
| files gate | `filesgate.go` | `FileGroup` -> `FileGroup` | The files table (rows, stat, `CheckTime`); drops groups that need no work; deletions after every grouper's marker. |
| exif | `exifextractor.go` | `FileGroup` -> `*RawItem` | `exiftool -all` for every file; N steps in parallel on the same channels. |
| mime | `mimeranker.go` | `*RawItem` -> `*RawItem` | The kind of every file; the main file (the source) first. |
| validator | `validator.go` | `*RawItem` -> `*RawItem` | Links the group, same / changed / moved / duplicate -> the item. |
| transcode switch | `transcode/switch.go` | `*RawItem` -> `*RawItem` | Routes the asset by kind. |
| photo / video / Live Photo transcoders | `transcode/photo`, `transcode/video`, `transcode/livephoto` | `*RawItem` -> `*RawItem` | Stubs for now: pass the asset on. |
| plugins, closer | `exifpluginprocessor.go` | `*RawItem` -> `*dto.ItemDto` | Core plugins (date, size), external perceptors, then the item is saved `Ready`. |

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
  It counts markers (`groups.Branches`).
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
- **Item == asset**: a derivative never becomes an item of its own. Inside an
  Apple Photos library only `originals/` are read (`generic.shouldSkipPath`) until its
  grouper links derivatives from the library's DB.
- **The main file is the source**: RAW > video > image. The JPEG of RAW+JPEG and
  the photo of a Live Photo are derivatives (sidecars), future ready previews.
- **A link outside the group** (a file linked to a GUID that is not in its group:
  the main file is gone, e.g. a RAW deleted and its JPEG left) makes the gate pass
  the group, so the survivor becomes the item in the same walk.
- **Changing the kind detection** (`mimeRanker`, the extension table) needs a new
  `mimeVersion`: on start, a new version clears every "ignored" mark, so groups an
  older detection dropped are classified once more (stored in the `meta` table).
- **exiftool args are part of the short hash**: changing `allTags` changes every hash
  (every item becomes "changed").

## Tests

- `fswalker_test.go` — walk results (complete, unreadable dir, missing root, cancel).
- `groups/…_test.go`, `groups/generic/…_test.go`, `transcode/…_test.go` — the
  switches and the generic grouper (names, directories, the marker, a Photos
  library's `originals/` only).
- `steps_test.go` — ranking, a source that appears later, moved -> no
  reprocessing, not media remembered.
- `validator_test.go` — whole walks through the real steps on a temp library and a
  temp sqlite, exiftool replaced by `fakeExif` (the file content is the metadata):
  new / same / changed / moved / duplicate / deleted main and sidecar / unreadable /
  missing root / deleted then back.
