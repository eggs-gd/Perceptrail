# model — the library's data and its rules

`model` keeps the library's data (items, their files, a few settings) in SQLite and
holds the rules about it: which item a group of files is, what a file gone means,
when an item is shown. Every chain that touches items goes by the same rules; a
step only gathers facts (see the [importer README](../importer/README.md#the-model-decides-the-steps-gather-facts)).

`model.Open(cfg, logger)` connects, migrates the schema and returns the `Proxy`;
`Close` ends it.

## Three types, one per role

| type | what | over |
|---|---|---|
| `query` | the reads (`GetItemByGUID`, `NeedsWork`, `StreamItemsSince`…), written once | a connection: the readers' pool, or a write's transaction |
| `tx` | what a write runs on: the reads and the writes themselves (unexported methods: `createFile`, `gone`, `validateGroup`…) — no public writes | the writer's transaction |
| `Proxy` | the model as others see it: the reads over the pool, the public writes, their commands | the pool and the writer |

A write calls other writes directly, in the same transaction; it cannot call a public
write (`tx` has none) — that would wait for the writer that runs it.

## A file per subject

Each subject is whole in its file — its reads (`query`), its writes (`tx`), its
commands and their public faces:

| file | subject |
|---|---|
| `items.go` | items: what the library shows |
| `identity.go` | which item a group of files is (by path and fingerprint, or by a key) |
| `flow.go` | an item's flow through its states: needs work, gone, ignored, published |
| `files.go` | the files' rows and their links to items |
| `meta.go` | the library's own settings |
| `work.go` | the work queue: what the expensive stage still has to do per item |
| `events.go` | the domain events, published after the commit |
| `percepstore.go` | a perceptor's values, in its own file |

Around them: `proxy.go` (the three types, `Open`, a write's savepoint, `Close`),
`writer.go` (the executor), `command.go` (how a write becomes a command).

## One writer, a read pool

- **The writer** (`writer.go`) owns the one write connection, in one goroutine. It
  takes a job (the `Now` lane first, then `Frame`, then `Idle`), takes whatever else
  is queued already (up to 512), runs them in one transaction — each write under its
  own savepoint: an error or a panic rolls back that write alone — commits, then ends
  them: **a result is delivered only after the commit**, so whatever its caller
  reads next is there.
- **The readers**: a pool of read-only connections (WAL lets them read beside the
  writer).
- `synchronous=NORMAL`: with WAL a power loss may lose the last transactions, never
  corrupts; the library comes back from the disk anyway.
- **Closing**: the writer writes what is queued and stops (a write after that
  fails); the readers close first, the writer's connection last — the last
  connection to close merges the journal into the file and removes it, and a
  read-only one cannot.
- Nothing waits for a batch to fill yet: batches come from callers writing at once.

## Every write is a command

Each write (a method on `tx`) is submitted as a [go-pub-sub](https://github.com/eggs-gd/go-pub-sub) `Op`
run by the writer (`command.go`), with two faces, in pairs:

```go
func (p *Proxy) CreateFile(entry dto.ItemEntry) (*dto.FileDto, error)        // submit, wait for its result
func (p *Proxy) CreateFileCommand() pubsub.Command[dto.ItemEntry, *dto.FileDto] // submit and go on
```

- A write takes one argument and gives one result (`pubsub.None` where it has none;
  several travel as one message: `ValidateGroupArgs`, `GoneResult`…), so its method
  on `tx` is its command's function as it is: `command(p, pubsub.Frame, (*tx).createFile)`.
- A command comes in the view of its shape: `Message[A]` without a result,
  `Signal[R]` without an argument — `None` never reaches a caller.
- An async caller picks how it gets results (the library's README has the
  patterns): fire and forget, its own through a `Client` (room counted at submit),
  or every result of the write through `Done`.
- **Who may write what is who holds what**: the executor is the model's own, so every
  write is the model's; a `Job` only an `Op` makes (sealed); others get commands — an
  argument for a given write, no transaction, no code of their own.

## The work queue

`work(guid, slug, version, input, …)` and `renditions` — the expensive stage's work
per item and slug (`render`; later the pixel perceptors). **What is needed is
derived, never recorded**: an item needs a slug's work when it has no row, or its row
was done with another `version` (the code or config that made it) or for another
`input` (the item's fingerprint). A new item enters by itself, a deleted one leaves
by the join, a new version makes everything due — nobody enqueues.

- **`Due`**: a page of what is due, Waiting items first (nothing shows them yet), then
  the shown ones (Visible, Ready) newest first, by a keyset cursor on the date
  (`dto.Cursor`) — never a sort of everything.
- **`Take`**: a lease of 15 minutes, not a mark — a crash lets it expire.
- **`Finish`**: the row done and the version's renditions replaced — unless the
  item's fingerprint changed meanwhile: the result is dropped, the item stays due.
- **`Fail`**: the error kept (`Work` reads it), the item backs off (1 min, 10 min,
  1 h, 1 day); after five in a row with the same version and input it waits for a
  new one.

Its messages (`dto.WorkDone`, `dto.WorkFailed`, `dto.Cursor`, `dto.Taken`) are data
in `dto`: a consumer (render) depends on `dto` and its own `Store`, not on the model.

**Renditions make an item `Ready`** — what the browser shows is ours now:

- `Finish` with renditions sets `Ready` (`updated_at` moves: the client's delta
  brings it, a Waiting HEIC finally shows);
- `publish` keeps it `Ready` while its renditions are for its fingerprint — another
  pass of the cheap stage (a perceptor's rework) would hide it otherwise, and render
  would not come back (its work is done); a new fingerprint sends it back to Waiting
  or Visible, and render's work is due again;
- `NeedsWork` counts such an item through the cheap stage, preview or not.

A stream of items (`StreamAllItems`, `StreamItemsSince`) carries `dto.StoredItem` —
the row, its files, its renditions, a query of each per page: a new fact about an
item is a field there, not a parameter changed in every caller.

## Domain events

A write emits a fact of the library into its transaction's outbox
(`emit(t, &t.events.published, dto.ItemPublished{…})`); the writer publishes the
batch's events only after the commit, in order — no one hears of a write that is not
there. A write rolled back to its savepoint takes its events with it. Listeners are
called on the writer's goroutine: they hand the work off and return (the database is
the truth, an event only says "look"). `Published()`: items through the cheap stage
— render wakes on it.

## Tests

Unit tests next to the package, over a real SQLite file: the writer (a read after a
write, a failing write alone among many writers, a write calling a write, commands and
their shapes, closing, no journal left), the queue's rules, the events (after the
commit, none from a write rolled back) and the stores. The writes as the import uses
them are tested through the import, in [`gontroller/test/importer`](../../test/importer).
