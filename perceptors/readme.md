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
// The tags it reads (api.ExifTagger): only declared tags are read from the files
func (p *geoPerceptor) ExifTags() []string { return api.CoordinateTags }
func (p *geoPerceptor) NewProcessor(chin <-chan api.RawItemR, chout chan<- api.RawItemR,
    logger *l.Logger) chain.Processor { … }

// Navigation — every perceptor is a view of the gallery's sheet
func (p *geoPerceptor) View() api.View { return api.View{Title: "Place", Icon: geoIcon, Help: "…"} }
func (p *geoPerceptor) Order(ctx context.Context, anchor string,
    items []api.ItemDataProvider) ([]api.Entry, error) { … }

var Perceptor api.Perceptor = &geoPerceptor{}

func main() {}
```

- `DataProvider`: where the data comes from — `ExifDataProvider`, `RawDataProvider`,
  `MetadataProvider`.
- `ProcessingMode`: `SingleItem` or `ItemGroup` (groups are not implemented on the
  server side yet).
- `NewProcessor` returns a `perceplib/chain` step (usually `chain.NewDecorator`).
- `Schema`: what the perceptor keeps per item — the `Schema()` of a typed
  `api.NewStore[T]` (T is a struct; `Put(item, T)` in the processor, `Get(item)` in
  `Order`), or the zero `api.Schema{}` for nothing. The core keeps the storage (SQLite:
  `data_dir/perceptors/<store>.db`). Put T in a package of its own so other
  perceptors can read it (see `exif_geo/places`). The plugin's dependencies must match
  the host's versions exactly (`go list -m all` in both).
- `Info(item)`: what the perceptor knows about one item, for the viewer's info panel —
  facts `{Label, Value}` (its stored values are loaded, as for `Order`); none: it says
  nothing about this item.
- `View` + `Order`: **navigation is a base requirement** — the perceptor's button in
  the gallery (title, an SVG icon drawn as a mask, help) and every item in its order
  (`items` come newest first), with sections for the side panel. Absolute (date,
  size, place) or relative (`View.Relative`: a two-sided trail from `anchor` — faces,
  colour). Config `perceptors.<name>.client: false` keeps the button away, `enabled:
  false` does not run it. See [`Perceptors.puml`](../_sb/puml/Perceptors.puml).

## Building

From the repo root:

```bash
make build-plugins
```

Output: `gontroller/.build/plugins/<name>.so`, enabled in `config.yml` (`plugins:`).
Host and plugin must be built with **the same Go** and **the same versions** of
`perceplib`, zap, multierr — update all modules together.
`TestLoadExternalPlugins` (gontroller, `pkg/plugins`) builds every perceptor here and
loads it into the test process — a mismatch fails it. After changing a plugin run it
with `-count=1` (the test cache does not see other modules change).

## Status

| Plugin | Status |
|---|---|
| `exif_geo` | keeps the coordinates (`places.Places`, a `Store[Location]` — importable by other perceptors); its view: the sheet on a Hilbert curve, sections region → city from the time zone of the place |
| `ml_color` | loads; `NewProcessor` returns `nil`; its view (relative): "not analysed yet" |
| `ml_faces` | empty `main.go`, no `Perceptor` symbol |
| `ml_objects` | empty `main.go`, no `Perceptor` symbol |

EXIF plugins may implement either `api.ExifPerceptor` (read-only `RawItemR`, e.g.
`exif_geo`) or the core `exif_core.ExifCorePerceptor` (`RawItemRW`); the server wires
both. A plugin whose `NewProcessor` returns `nil` is skipped. Every EXIF plugin
declares the tags it reads (`ExifTags`, `api.ExifTagger`): the server reads only
declared tags, from the whole asset (the source's metadata first, then the .xmp
sidecars, the main file, the derivatives) — `GetExif` of an undeclared tag is "".
Plugins never run exiftool themselves.
