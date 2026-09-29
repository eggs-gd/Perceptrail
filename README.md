# perceplib

Shared library for the Perceptrail server (`gontroller`) and plugins (`perceptors`).
Module: `github.com/dukobpa3/perceplib` (a git submodule of Perceptrail).

## Packages

| Package | What |
|---|---|
| `api` | perceptor contract (`Perceptor`, `ExifPerceptor`), data types (`RawExif`, `Size`), item access interfaces (`RawItemR`, `ItemDataProvider/Editor`), `GetRatio` |
| `chain` | channel-based pipeline: `NewChainProcessor`, `NewEntryPoint`, `NewDecorator`, `NewSwitch`; `ErrSkippedItem` |
| `logger` | zap wrapper with a custom console encoder |
| `logger/decorators` | `GontrollerDecorator` — tree-style fields, SQL highlighting |

### chain

Each step is a `Processor` with its own goroutine: reads from its input channel,
writes to its output channel. `Decorator[Ti, To]` transforms an item; an error goes
to the chain's shared error channel (`ErrSkippedItem` is a regular skip, not a
failure). A chain is itself a `Processor`, so chains nest (that is how EXIF plugins
are run).

## Versioning

- Tags `v0.0.x`. After a change: tag it and bump `require` in `gontroller` and every
  `perceptors/*` (in Perceptrail they use `replace => ../perceplib`, but `require`
  must match).
- **Go plugins:** host and `.so` must have identical versions of this module and its
  dependencies (zap, multierr) and the same Go. Update all modules together.
- exiftool is no longer here — it lives in `github.com/eggs-gd/go-exiftool`.
