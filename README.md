# Perceptrail

**Perceptrail** is a self-hosted next-generation photo gallery that allows users to interact with large photo libraries through dynamic and flexible navigation. Instead of traditional albums and directories, **Perceptrail** offers an endless journey through your photos, enabling you to view them from various perspectives, such as people, geolocations, objects, cameras, and other metadata powered by AI.

The core concept is to provide an infinite way to explore content. By navigating through similar photos, users can see their collection from new angles, choosing different points of view through the plugin system we call **Perceptors**.

**Perceptors** is a modular system built from scratch, allowing the integration of various plugins for analyzing, searching, and interacting with photos. These plugins can introduce new navigation criteria like filters by geotags, faces, objects, camera models, and more.

## Key Features

- Infinite browsing of photos based on similarity or metadata.
- Navigation by people, geolocations, objects, cameras, and other criteria.
- Powerful **Perceptors** plugin system for extended navigation and analysis capabilities.
- AI/ML integration for recognizing and analyzing content.

## Tech Stack

- **Frontend**: Built with [Svelte](https://svelte.dev/), using **IndexDB** for local storage and fast interactions.
- **Backend**: Developed with [Go](https://go.dev/), using **PostgreSQL** for data storage.
- **ML Backend**: (in development) Likely to be implemented in Go or Python with a custom data structure for storing machine learning results.

## Capabilities

- Interactive display and editing of metadata for each item.
- Filtering based on various criteria depending on installed **Perceptors**.
- Support for batch operations for bulk metadata editing.

## Development

Git hooks (fast local checks before CI) — enable once per clone:

```bash
git config core.hooksPath .githooks
```

- `pre-commit`: refuses commits on `master`/`develop`, requires `gofmt` for staged Go files.
- `pre-push`: runs the CI checks (Go vet/tests + plugin build, `svelte-check`) for the
  parts changed since `develop`.

Skip in an emergency with `--no-verify`.

## Documentation

- [Roadmap](_sb/docs/roadmap.md) and [findings & decisions](_sb/docs/findings.md)
- [Design diagrams](_sb/puml) (PlantUML; rendered in [_sb/diagrams](_sb/diagrams))
- Modules: [gontroller](gontroller/readme.md) · [perceplib](perceplib/README.md) ·
  [perceptors](perceptors/readme.md) · [svebapp](svebapp/README.md)

## Big Flow

![Alt text](./_sb/diagrams/Item%20Flow.svg)

## Items watching and validating process

![Alt text](./_sb/diagrams/Walker%20and%20validating.svg)

## ML Flow

![Alt text](./_sb/diagrams/ML%20Flow.svg)
