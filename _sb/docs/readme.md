# Perceptrail docs

- [roadmap.md](roadmap.md) — what is done, what is next, core vs. perceptors.
- [findings.md](findings.md) — findings and decisions: what broke, why, what was
  decided. [review-pr24.md](review-pr24.md) — the strict review of the import
  refactor, and what is left for the next PRs. Read the relevant section before touching layout, streaming, plugins or
  exiftool.
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

- [gontroller](../../gontroller/readme.md) — Go backend: import, EXIF, HTTP API.
- [perceplib](../../perceplib/README.md) — shared library for the server and plugins.
- [perceptors](../../perceptors/readme.md) — plugins.
- [svebapp](../../svebapp/README.md) — frontend gallery.
