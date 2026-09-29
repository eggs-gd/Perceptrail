# perceptors

Perceptors are plugins that add new navigation and analysis axes to Perceptrail
(geo, faces, objects, colour, …). The idea: a perceptor may bring not only data but
also its own UI to the client (a map, face management) — see the
[roadmap](../_sb/docs/roadmap.md#later-the-perceptor-platform) and
[ML Flow](../_sb/puml/ML%20Flow.puml).

The basics (date, size) are not here but in the core:
`gontroller/pkg/plugins/exif_core`. What lives here can be implemented differently
(e.g. someone may write another geo plugin).

## Contract

A plugin is a Go plugin (`-buildmode=plugin`), `package main`, exporting a variable
`Perceptor` of type `api.Perceptor` from [`perceplib/api`](../perceplib/README.md):

```go
type geoPerceptor struct{}

func (p *geoPerceptor) Name() string                       { return "exif_geo" }
func (p *geoPerceptor) DataProvider() api.DataProviderType { return api.ExifDataProvider }
func (p *geoPerceptor) ProcessingMode() api.ProcessingMode { return api.SingleItem }
func (p *geoPerceptor) NewProcessor(chin <-chan api.RawItemR, chout chan<- api.RawItemR,
    logger *l.Logger) chain.Processor { … }

var Perceptor api.Perceptor = &geoPerceptor{}

func main() {}
```

- `DataProvider`: where the data comes from — `ExifDataProvider`, `RawDataProvider`,
  `MetadataProvider`.
- `ProcessingMode`: `SingleItem` or `ItemGroup` (groups are not implemented on the
  server side yet).
- `NewProcessor` returns a `perceplib/chain` step (usually `chain.NewDecorator`).

## Building

From the repo root:

```bash
make build-plugins
```

Output: `gontroller/.build/plugins/<name>.so`, enabled in `config.yml` (`plugins:`).
Host and plugin must be built with **the same Go** and **the same versions** of
`perceplib`, zap, multierr — update all modules together.

## Status

| Plugin | Status |
|---|---|
| `exif_geo` | loads; `Decorate` writes nothing yet (nowhere to — see perceptor data storage in the roadmap) |
| `ml_color` | loads; `NewProcessor` returns `nil` |
| `ml_faces` | empty `main.go`, no `Perceptor` symbol |
| `ml_objects` | empty `main.go`, no `Perceptor` symbol |

EXIF plugins may implement either `api.ExifPerceptor` (read-only `RawItemR`, e.g.
`exif_geo`) or the core `exif_core.ExifCorePerceptor` (`RawItemRW`); the server wires
both. A plugin whose `NewProcessor` returns `nil` is skipped.
