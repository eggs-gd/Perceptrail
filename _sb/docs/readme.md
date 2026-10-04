# Perceptrail docs

- [roadmap.md](roadmap.md) — what is open, the designs not built yet, core vs.
  perceptors; done work one line per PR.
- [findings.md](findings.md) — the why: decisions, rejected approaches, traps. Read
  the relevant section before touching the gallery, sync, the import, Apple Photos,
  plugins or exiftool. How things work now is in the module READMEs (below).
- [review-pr24.md](review-pr24.md) — the strict review of the import refactor; its
  open items are in the roadmap ("Before the transcodes").
- [../puml](../puml) — design diagrams (PlantUML sources), rendered in
  [../diagrams](../diagrams):

  | Diagram | What |
  |---|---|
  | [Import chain](../puml/Import%20chain.puml) | gontroller's import: walk → group → gate → identify → exif → commit, a pass |
  | [Walker](../puml/Walker.puml) | the gate and the validator in detail: same / moved / duplicate / changed, the gone files |
  | [Perceptor data](../puml/Perceptor%20data.puml) | what the core keeps for a perceptor |
  | [Perceptors](../puml/Perceptors.puml) | perceptors and the sheet's navigation |
  | [Client flow](../puml/Client%20flow.puml) | server ↔ client over MQTT: New / Updated / Processed Item |
  | [ML Flow](../puml/ML%20Flow.puml) | goMLer: ML plugins over processed items |
  | [Workers](../puml/Workers.puml) | svebapp: SyncWorker, LayoutWorker, View subscribed to LayoutDB |
  | [Protocol](../puml/Protocol.puml) | item fields on each layer: SQL → Go → svebapp → Dexie → View |

Modules:

- [gontroller](../../gontroller/readme.md) — Go backend: import, EXIF, HTTP API;
  [importer](../../gontroller/internal/importer/README.md),
  [library](../../gontroller/internal/library/README.md) (providers, Apple Photos).
- [perceplib](../../perceplib/README.md) — shared library for the server and plugins.
- [perceptors](../../perceptors/readme.md) — plugins.
- [svebapp](../../svebapp/README.md) — frontend gallery.
