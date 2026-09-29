# scan — the import chain

`scan` turns a library directory into items in the DB. It is one chain of small
steps built on [`perceplib/chain`](../../../perceplib/chain/README.md): every step
is its own goroutine, steps are connected by channels. The chain is assembled in
[`entry.go`](entry.go) (`NewImporterService`); the comment there describes every
step, the comments at the channels say what each carries.

Diagrams: [`Import chain.puml`](../../../_sb/puml/Import%20chain.puml) (the whole
chain), [`Walker.puml`](../../../_sb/puml/Walker.puml) (files gate and validator in
detail).

```
fswalker -> source switch ─┬─ generic grouper ──┬─> files gate -> exif (N) -> mime -> validator
                           └─ Apple Photos (stub)┘
         -> transcode switch ─┬─ photo ──────┬─> plugins -> closer
                              ├─ video ──────┤
                              └─ Live Photo ─┘
```

## Steps

| Step | File | In -> out | What it does |
|---|---|---|---|
| fswalker | `fswalker.go` | root -> `fileEvent` | Reports every file (path + stat), then the end-of-walk marker. Unreadable subdirectories are skipped and recorded. |
| source switch | `groups.go` | `fileEvent` -> `fileEvent` | Routes a file to the grouper of its source; the marker goes to every grouper. |
| generic grouper | `groups.go` | `fileEvent` -> `FileGroup` | Sidecars by name, next to each other: one open group; a complete group goes out, the last one with the marker. |
| Apple Photos grouper | `groups.go` | `fileEvent` -> `FileGroup` | Stub: passes the marker on. `photosLibraryEnabled = false` sends the library to generic. |
| files gate | `gate.go` | `FileGroup` -> `FileGroup` | The files table (rows, stat, `CheckTime`); drops groups that need no work; deletions after every grouper's marker. |
| exif | `exifextractor.go` | `FileGroup` -> `*RawItem` | `exiftool -all` for every file; N steps in parallel on the same channels. |
| mime | `mime.go` | `*RawItem` -> `*RawItem` | The kind of every file; the main file (the source) first. |
| validator | `validator.go` | `*RawItem` -> `*RawItem` | Links the group, same / changed / moved / duplicate -> the item. |
| transcode switch | `transcoder.go` | `*RawItem` -> `*RawItem` | Routes the asset by kind; the transcoders are stubs for now. |
| plugins, closer | `exifpluginprocessor.go` | `*RawItem` -> `*dto.ItemDto` | Core plugins (date, size), external perceptors, then the item is saved `Ready`. |

## Types

- `fileEvent` ([`events.go`](events.go)) — one found file, or the end-of-walk marker.
- `FileGroup` ([`events.go`](events.go)) — **one whole asset**: all its files (main
  file, sidecars, derivatives), or the marker (`Done`). Before the gate the files
  carry only their stat, after it they are rows of the files table (GUIDs).
- `RawItem` ([`types.go`](types.go)) — the asset from exif to the closer: `Files`,
  `Exif`, `Kinds` are aligned. exif fills `Files` + `Exif`, mime fills `Kinds` and
  puts the main file first, the validator sets `Item`. Plugins see it through
  `exif_core.RawItemRW` / `api.RawItemR`.
- `walkResult` ([`events.go`](events.go)) — rides in the marker: root, start time,
  complete or not, unreadable directories.

## Rules that are easy to break

- **Groups are whole assets.** Groupers are plain decorators with their own buffer
  of open groups; a group goes out when it is complete, so each file closes at most
  one group.
- **The marker passes through the groupers**, not around them: the gate may derive
  deletions only after every grouper has sent its last group (all files stamped).
  It counts markers (`groupBranches`).
- **Deletions are conservative**: only after a complete walk that found files, never
  under an unreadable directory, only under the configured root. A main file gone
  -> the item is soft-deleted; a sidecar gone -> the item is `Dirty`.
- **Moves race with deletions** (the chain is asynchronous): the validator also finds
  soft-deleted items by hash and restores them, so a moved file keeps its GUID.
- **The main file is the source**: RAW > video > image. The JPEG of RAW+JPEG and
  the photo of a Live Photo are derivatives (sidecars), future ready previews.
- **exiftool args are part of the short hash**: changing `allTags` changes every hash
  (every item becomes "changed").
- Adding a source: a grouper step + a branch in `sourceSwitch` + `groupBranches`.
  Adding a transcoder: a branch in `transcodeSwitch` + a step writing to the plugins
  channel.

## Tests

- `fswalker_test.go` — walk results (complete, unreadable dir, missing root, cancel).
- `steps_test.go` — groupers, switches, ranking, transcode routing, a source that
  appears later, moved -> no reprocessing, not media remembered.
- `validator_test.go` — whole walks through the real steps on a temp library and a
  temp sqlite, exiftool replaced by `fakeExif` (the file content is the metadata):
  new / same / changed / moved / duplicate / deleted main and sidecar / unreadable /
  missing root / deleted then back.
