# perceplib

Shared library for the Perceptrail server (`gontroller`) and plugins (`perceptors`).
Module: `github.com/eggs-gd/perceplib`. Standalone public repo
([eggs-gd/perceplib](https://github.com/eggs-gd/perceplib)), vendored into Perceptrail
as a **git subtree** under `perceplib/`.

## Packages

| Package | What |
|---|---|
| `api` | perceptor contract (`Perceptor`, `ExifPerceptor` (+ `ExifTagger`: the tags it reads — only declared tags are read, `GetExif` of another is ""; + `Decorator(logger)`: its logic over one item, the host runs it as a step), every perceptor also navigates: `View` (its button) + `Order` (the gallery's sheet in its order, with sections); what it knows about an item: `Info` (facts for the info panel); its data: `Schema` + a typed `Store[T]` (`NewStore`, `Put`, `Get` — the host keeps the storage)), data types (`RawExif`, `Size`), item access interfaces (`RawItemR`, `ItemDataProvider/Editor`), `GetRatio` |
| `chain` | steps connected by typed pipes that carry values and a flush (a batch done): `Decorate`, `Parallel`, `Route`, `Entry`, `Sink`, `Series`, `New`; `ErrSkippedItem` (a skip, not an error) — [README](chain/README.md) |
| `exif` | helpers for the values the host reads with exiftool `-n` (numbers as numbers): `Coordinates` (signed decimal degrees) + `CoordinateTags` to declare |
| `logger` | zap wrapper with a custom console encoder |
| `logger/decorators` | `GontrollerDecorator` — tree-style fields, SQL highlighting |

### chain

Each step is a goroutine reading one typed pipe and writing another; its logic is
plain Go (`Decorator`, `Router`, `Source`). A source flushes after a batch; every
step passes the flush on after the values before it, and where branches join it
passes once every branch has flushed — so a flush at the end means the batch is
done. Errors go to the chain's error channel (a skip never does); a chain is a step
too, so chains nest. Details: [`chain/README.md`](chain/README.md).

## Working from Perceptrail (subtree)

In Perceptrail `perceplib/` is plain files: edit and commit together with the code
that needs the change — one commit, no submodule pointer. Consumers use
`replace github.com/eggs-gd/perceplib => ../perceplib` locally.

Publishing changes back to this repo (from the Perceptrail root):

```bash
git subtree push --prefix=perceplib git@github.com:eggs-gd/perceplib.git master
# then tag in this repo (clone or `git ls-remote`), e.g.:
git tag vX.Y.Z <pushed commit> && git push git@github.com:eggs-gd/perceplib.git vX.Y.Z
```

Pulling changes made directly in this repo:

```bash
git subtree pull --prefix=perceplib git@github.com:eggs-gd/perceplib.git master
```

## Versioning

- Tags `v0.0.x`. After a change: tag it and bump `require` in `gontroller` and every
  `perceptors/*` (in Perceptrail they use `replace => ../perceplib`, but `require`
  must match).
- **Go plugins:** host and `.so` must have identical versions of this module and its
  dependencies (zap, multierr) and the same Go. Update all modules together.
- exiftool is no longer here — it lives in `github.com/eggs-gd/go-exiftool`.
