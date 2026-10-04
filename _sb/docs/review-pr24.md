# Review of PR #24 (import stages), 2026-10-04

A strict review of the import refactor before closing it: dependencies, duplicates,
missed code, docs. The goal of the work: prepare for the expensive stages
(transcodes, ML) and cut responsibilities so that a change does not touch 50 files.

Status: **[PR24]** — done in PR #24; **[PR26]** — done in PR #26; **[next]** — a separate PR before the
transcodes (also in the roadmap).

## What PR #24 did

1. The import is cut into stages, `walk → group → gate → identify → exif →
   commit`, each in its own package; `pkg/scan` and the `flow` bag are gone.
2. A type belongs to its producer (`dto.Asset`, `identify.Item`, `transcode.Item`);
   every step declares the small `Store` it uses.
3. The model decides, the steps gather facts (`NeedsWork`, `Gone`, `Ignore`,
   `Publish`, `MarkRework` in `model/itemslife.go`).
4. exif: declared tags only, one exiftool call per group, `-n`, the metadata
   package (source > .xmp > main > derivatives), one exif step, the plugin API is a
   `Decorator`.
5. The fingerprint from the file's bytes, with a `hashVersion` migration.
6. The walk writes the rows; gone files flow (providers see them); Refresh is a
   rework mark; the client guesses the cloud.
7. `chain`: channels, a `WaitGroup` per output, one runner, `…N` and `…Decorator`
   variants; a pass is a new chain.
8. A clean stop: exiftool and the databases closed.
9. The import tests are integration tests: a real exiftool, real files.
10. `transcode`: stubs for the next chain.

## 1. Dependencies, cross-dependencies, inversions

- **1.1 [next] `app` imports `client`, `model`, `plugins/settings`** (`app.Config`
  aggregates their config types), and `importer` takes `app.AppContext`: so
  `importer → app → client → routes → providers/model` — the import depends on the
  HTTP layer. Fix: the importer service takes plain values (root, cache dir,
  rescan, loggers) from `main`; only `main` aggregates the config.
- **1.2 [PR24] Refresh is a needless middleman**: provider → `Refresher` callback →
  `importer.Refresh` → `db.MarkRework`. The Apple provider has its own `Items`
  store; it marks the rework itself. `providers.Refresher`, `importer.Refresh` and
  the wiring in `main` go.
- **1.3 [PR26] `providers.Provider` mixes three roles**: import (`Claims`,
  `Grouper`), renditions (`Owns`, `Levels`, `Rendition`), background (`Start`).
  `group` and `client/routes` depend on the whole. Split: `Grouping` for the import,
  `Renditions` for the routes; a provider implements both.
- **1.4 [next] The core perceptors' contract lives in the registry package**:
  `exif_date` / `exif_size` / `exif_duration` import `pkg/plugins` for
  `RawItemRW` / `ExifCorePerceptor`. Move the contract to a small package of its
  own; the registry depends on it, not the other way round.
- **1.5 [PR26] Steps import `model` for small things**: `walk` (`ErrNotFound`),
  `identify` (`Outcome`), `exif` (`*PerceptorStore`). Move `ErrNotFound` and
  `Outcome` to `dto`; exif needs a storage interface (`Name`, `Save`, `Guids`,
  `Prune`).
- **1.6 [next] Global state**: `model.db` (lazy init in `NewProxy`, unsynchronised),
  `providers.Enable`, the `plugins` registry, five package-level DB proxies in the
  routes with the same lazy init. Registries as package functions is a decision;
  the routes' proxies and `model.db` are not (pass the proxy to `NewWebService`,
  open the DB in `Configure`). Already deferred: "the HTTP routes' DB proxies".
- **1.7 [PR26] `identify` imports `plugins` for `ExifTags()`** (decided in
  `fc46706`). With a chain built per pass, the service can pass the tags in.

## 2. Duplicated code and logic

- **2.1 [PR24] Four steps write the file rows**: walk (stat, `CheckTime`), gate
  (roles on `Changed`), validate (links, through the model), show (sizes). The
  gate's write is redundant: a changed group passes, and show writes the same rows;
  an identify error re-detects the role next pass. The gate's `UpdateFiles` goes.
- **2.2 [PR24] The `plugins.All()` filter by `DataProvider` three times**
  (`corePerceptors`, `externalPerceptors`, `importStores`), and `Prune` walks every
  storage unfiltered. One helper; prune the import's storages.
- **2.3 [PR24] Three things called "Gone"**: `walk.Gone` (which unseen rows count as
  deleted), `model.Gone` (the rules), `dto.FileDto.Gone` (the flag). The walk's
  filter is renamed.
- **2.4** Two `Switch` types (`group`, `transcode`) — different packages, fine.
- **2.5** "Kind" in four places: `identify.mediaKind` (a file), `dto.AssetKind`
  (the one asset rule), `apple.assetRow.kind()`, `transcode.Item.kind()` (a wrapper
  of `AssetKind` — drop it when transcode is wired).

## 3. Missed in the code

- **3.1 [next] N+1 every pass**: the walk does `GetFileByPath` + `UpdateFiles` per
  file, every minute — ~20k queries for 10k files, a WAL write each. Batch: read
  the rows under the root in one query, stamp `CheckTime` with one `UPDATE … WHERE
  id IN` per page, create in batches.
- **3.2 [next] Photos' own files become rows** (its database, caches inside the
  bundle); before, only grouped files had rows. Not measured (the library is not
  readable from the agent's sandbox). Either the walk skips what a provider says is
  not its media, or measure and accept.
- **3.3 [PR26] `dto.FileDto` carries the pass's state** (`Changed`, `Gone` with
  `gorm:"-"`): a DB DTO is also a chain message. A message type
  (`dto.SeenFile{*FileDto; Changed, Gone}`) for walk → group.
- **3.4 [PR26] `Prune` every pass** reads every GUID and every storage's GUIDs, even
  when nothing was deleted. Run it when the gate deleted something.
- **3.5** `exif_coretest/recorder.go`: a test helper in a plain package file — fine
  as a test-support package (like `httptest`); keep.
- **3.6 [PR24] Stale comments** still describe the flush as a message
  (`importer/entry.go`, `exif/entry.go`, `exif/perceptors.go`, `walk/walker.go`,
  `group/switch.go`, `providers/providers.go`, `folder/grouper.go`).
- **3.7 [next]** The exiftool pool panics when it cannot start — now lazily, in the
  middle of a pass. Return an error instead.
- **3.8** CI with exiftool on Linux not seen yet; `pkg/plugins` fails under `-race`
  (the `.so` are built without it) — not written down anywhere.

## 4. Docs: lagging, and what to delete

Lagging:

- Roadmap "Chains": `New(deps, in, out, errch)`, `Chain.Run`, `discover.Group`,
  "pipes with a flush", "API providers… discover".
- Roadmap "perceplib chain — what the import taught" describes the intermediate
  model (pipes, flush, `chain.Parallel`, `Route`).
- Roadmap: "Core (exif_core)"; `fswalker` and the marker in Done.
- `_sb/docs/readme.md` and `Item flow.puml`: "fswalker → metaprocessor →
  transcoder" — a design that never existed in code.

Delete from `findings.md` (states inside this PR, not needed for the next
decisions):

- "chain: pipes with a flush" (replaced by the WaitGroup version);
- "discover cut into walk, group, gate; the walk cycle is the top's" (no discover;
  the cycle is the service's);
- the history of tries in the newer entries — one line "rejected: X, because Y" is
  enough;
- from before the PR: "Import chain as small steps" (C1–C7, fswalker, the marker),
  "exiftool → own package" — they describe code that no longer exists: two or three
  lines of "why" each.

Diagrams: `Item flow.puml` — redo or delete; `Walker.puml` partly repeats
`Import chain.puml` — merge.
