# importer — the import chain

`importer` turns a library directory into items in the DB. It is a chain on
[go-chain](https://github.com/eggs-gd/go-chain): every step its own
goroutine, steps connected by channels, a pass ends from its input.

```
walk → group → gate → identify → exif → commit
```

![Import chain](../../../_sb/diagrams/Import%20chain.svg)

**The shape** (the contract for every chain, roadmap "The next chains"): the top has
only linear stages, each named by what it yields; every stage is a sub-chain in its
own package with one `New` listing all its steps; the top knows none of the tools —
the file system, the libraries, exiftool, the plugins belong to the stage that uses
them. One step does one thing; no callbacks between steps. A constructor takes, in
order: the model, the step's own dependencies, the logger, `in`, `out`.

**The service** ([`entry.go`](entry.go), `importer.New(cfg, db, logger)`): `Pass`
builds a new chain and runs it to its end (the walk returns, its output closes, each
step ends after its input); `Start` first does what changed since the last run
(`identify.Migrate`, `exif.Reconcile`), then pass after pass with the `rescan`
pause. Passes never overlap. A step's errors are logged and returned by `Pass`; a
skip is never an error.

## Stages

| stage | in → out | what it does |
|---|---|---|
| **walk** | the root → `dto.WalkedFile` | The chain's entry: reads the files table once, skips the directories a library says hold none of its media, then sends every file as its row, page by page (a new file's row created, a changed stat saved with `Changed`, one transaction a page; an unchanged file is not written); after a complete walk, the rows it did not see sent as `Missing`. |
| **group** | `dto.WalkedFile` → `dto.Asset` | A switch sends each file to the grouper of the first library that claims it (the plain folder last); each grouper is a step. The groupers are the libraries': [library README](../library/README.md). |
| **gate** | `dto.Asset` → `dto.Asset` | The asset's `Missing` applied by the model (`Gone`); an asset passes if a file changed or the model says it needs work (`NeedsWork`). |
| **identify** | `dto.Asset` → `*identify.Item` | read (in parallel, no DB: one exiftool call per group, kinds and roles, the metadata package, the fingerprint) → validate (one at a time: which item it is, or ignored) → show (sizes, what the browser shows now, an embedded preview as the last resort). exiftool lives only here. |
| **exif** | `*identify.Item` → `*identify.Item` | The EXIF perceptors, a step each (the built-in ones write into the item, the `.so` ones only read it), then keep: a row in every import perceptor's storage. |
| **commit** | `*identify.Item` → end | The model publishes the item: `Visible` with a preview, else `Waiting`. |

## The chain's messages

A type belongs to the package that produces it; there is no shared bag of messages.

- **`dto.WalkedFile`** (walk → group): the file's row (`*dto.FileDto`, `Changed`
  stored: set by the walk or a grouper's `SetRole`, cleared only by validate, so a
  pass that fails before deciding the group leaves it for the next) and `Missing` —
  a fact of this pass, not stored.
- **`dto.Asset`** (group → gate → identify): one whole asset before it is identified —
  its files' rows, `Key` (a library's identity: the item's GUID), `Show`, the
  source's `Meta` / `MetaHash`, `Kind`, and `Missing`: the files gone for the
  library (the walk's, or the library's own say). The end of a walk is not a value:
  the walk's output closes.
- **`identify.Item`** (identify → exif → commit): the item and its metadata package;
  the built-in perceptors write into it (`builtin.Item`), the plugins read it
  (`api.RawItemR`). identify's working `draft` never leaves it.

Every step declares the DB methods it calls as its own small `Store`; a sub-chain's
`Store` embeds its steps', the service's embeds the stages'.

## exif: what is read

- **Declared tags only, never `-all`**: each perceptor declares what it reads
  (`api.ExifTagger`); the service passes their union to identify, which adds its own
  (MIME type, errors, sizes, codec, embedded previews). An undeclared tag reads "".
- **One `exiftool -j -n` call per group** (a keyed group: the main file only). `-n`:
  numbers as numbers — signed decimal GPS (the composite tags), `Orientation` 1–8,
  `Duration` in seconds, `ImageSize` "W H". Pinned by a probe test with a real
  exiftool (`identify/read_test.go`). A library's own `Meta` is written the same way.
- **The metadata package** (merge): the source's `Meta` (what the user may have
  corrected in the library), else the `.xmp` sidecars, the main file, the
  derivatives. Not stored: it travels with the item.
- **Plugins never run exiftool**; only identify does.

## The model decides, the steps gather facts

The rules about the library's data live in the model
([`model`](../model/README.md#a-file-per-subject): `flow.go`, `identity.go`), so every chain that touches items
goes by the same ones: which item a group is (`ValidateGroup`, `ValidateAsset`),
whether an unchanged group needs work (`NeedsWork`), what a file gone means
(`Gone`), a group that is no item (`Ignore`), publishing (`Publish`), what an asset
is (`dto.AssetKind`). A step gathers stat, exif, kinds, the fingerprint.

## Rules that are easy to break

- **Groups are whole assets**: a grouper sends a group when it is complete, so each
  file closes at most one group. **The end of the walk passes through the groupers**:
  each gives what it holds when its input closes; the gate's input closes after the
  last of them returned.
- **Seen is what the walk visited**, not what reached the gate: a file of a group
  that did not complete, or of a library whose DB did not load, is seen — not
  missing. The walk keeps it in memory (the rows read at its start, minus the ones
  it visited); nothing is stamped on the rows. Before they are sent, the missing
  rows are read again: the chain worked meanwhile (a moved file's old row may be
  gone, its item moved to the new path).
- **Deletions are conservative**: only after a complete walk that found files, never
  under an unreadable directory, only under the root. A main file gone → the item is
  soft-deleted; a sidecar gone → the item is `Dirty`.
- **Moves**: a moved file's old path may be deleted before the new one is
  validated; validate restores the soft-deleted item by fingerprint, the GUID stays.
- **On demand does not wait**: a library marks the item (`MarkRework`, `updated_at`
  untouched so no delta brings the old state back); the next pass processes it.
- **The perceptors' rows** are reconciled at start (`exif.Reconcile`): an item
  without a row is reworked, a row of a gone item dropped — not per pass.
- **Item == asset**: a derivative never becomes an item of its own. **The main file
  is the source**: RAW > video > image (a keyed group's main file is the library's
  choice). **A link outside the group** (its main file gone) makes the gate pass the
  group, so the survivor becomes the item in the same pass.
- **Changing the kind detection** (`identify/classify.go`) needs a new
  `mimeVersion` (on start every "ignored" mark is cleared). **Changing the
  fingerprint** (`identify/fingerprint.go`) needs a new `hashVersion` (on start
  every item forgets its fingerprint; not `Dirty`, or the library would vanish from
  the client for the pass).

## Tests

A step's unit tests sit in its package. The import as a whole is tested in
[`gontroller/test/importer`](../../test/importer) through the public API: the
server's own start and the service's `Pass`, over real files and a real exiftool
(CI installs it), what a pass published read from the DB.
