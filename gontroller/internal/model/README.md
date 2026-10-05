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
| `query` | the reads (`GetItemByGuid`, `NeedsWork`, `StreamItemsSince`…), written once | a connection: the readers' pool, or a rule's transaction |
| `tx` | what a rule runs on: the reads and the rules themselves (unexported methods: `createFile`, `gone`, `validateGroup`…) — no public writes | the writer's transaction |
| `Proxy` | the model as others see it: the reads over the pool, the public writes, their topics | the pool and the writer |

A rule calls other rules directly, in the same transaction; it cannot call a public
write (`tx` has none) — that would wait for the writer that runs it.

## One writer, a read pool

- **The writer** (`writer.go`) owns the one write connection, in one goroutine. It
  takes a job (the `Now` lane first, then `Frame`, then `Idle`), takes whatever else
  is queued already (up to 512), runs them in one transaction — each rule under its
  own savepoint: an error or a panic rolls back that rule alone — commits, then ends
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

## Every write rule is a topic

`writes.go`: each rule is a [go-pub-sub](https://github.com/eggs-gd/go-pub-sub) `Op`
run by the writer, with two faces, in pairs:

```go
func (p *Proxy) CreateFile(entry dto.ItemEntry) (*dto.FileDto, error)  // submit, wait for its result
func (p *Proxy) CreateFileTopic() pubsub.Topic[dto.ItemEntry, *dto.FileDto] // submit, go on, results by subscription
```

- A rule takes one argument and gives one result (`pubsub.None` where it has none;
  several travel as one message: `ValidateGroupArgs`, `GoneResult`…), so its method
  on `tx` is its topic's function as it is: `rule(p, pubsub.Frame, (*tx).createFile)`.
- A topic comes in the view of its shape: `Message[A]` without a result, `Signal[R]`
  without an argument — `None` never reaches a caller. Without a result a
  subscriber still gets one result per operation: its ID and its error.
- **Who may write what is who holds what**: the executor is the model's own, so every
  rule is the model's; a `Job` only an `Op` makes (sealed); others get topics — an
  argument for a given rule, no transaction, no code of their own.

## Tests

Unit tests next to the package, over a real SQLite file: the writer (a read after a
write, a failing rule alone among many writers, a rule calling a rule, topics and
their shapes, closing, no journal left) and the stores. The rules as the import uses
them are tested through the import, in [`gontroller/test/importer`](../../test/importer).
