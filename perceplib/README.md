# perceplib

Shared library for the Perceptrail server (`gontroller`) and plugins (`perceptors`).
Module: `github.com/eggs-gd/perceplib`. Standalone public repo
([eggs-gd/perceplib](https://github.com/eggs-gd/perceplib)), vendored into Perceptrail
as a **git subtree** under `perceplib/`.

## Packages

| Package | What |
|---|---|
| `api` | perceptor contract (`Perceptor`, `ExifPerceptor`, every perceptor also navigates: `View` (its button) + `Order` (the gallery's sheet in its order, with sections)), data types (`RawExif`, `Size`), item access interfaces (`RawItemR`, `ItemDataProvider/Editor`), `GetRatio` |
| `chain` | channel-based pipeline: `NewChainProcessor`, `NewEntryPoint`, `NewDecorator`, `NewSwitch`; `ErrSkippedItem` |
| `logger` | zap wrapper with a custom console encoder |
| `logger/decorators` | `GontrollerDecorator` — tree-style fields, SQL highlighting |

### chain

Each step is a `Processor` with its own goroutine: reads from its input channel,
writes to its output channel. `Decorator[Ti, To]` transforms an item; an error goes
to the chain's shared error channel (`ErrSkippedItem` is a regular skip, not a
failure). A chain is itself a `Processor`, so chains nest (that is how EXIF plugins
are run).

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
