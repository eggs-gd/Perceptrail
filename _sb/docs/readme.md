# Perceptrail docs

- [roadmap.md](roadmap.md) — what is done, what is next, core vs. perceptors.
- [findings.md](findings.md) — findings and decisions: what broke, why, what was
  decided. Read the relevant section before touching layout, streaming, plugins or
  exiftool.
- [../puml](../puml) — design diagrams (PlantUML sources), rendered in
  [../diagrams](../diagrams):

  | Diagram | What |
  |---|---|
  | [Item flow](../puml/Item%20flow.puml) | gontroller chain: fswalker → metaprocessor → transcoder, events to client and ML |
  | [Walker](../puml/Walker.puml) | grouping files and the validator: same / moved / duplicate / changed by hash |
  | [Client flow](../puml/Client%20flow.puml) | server ↔ client over MQTT: New / Updated / Processed Item |
  | [ML Flow](../puml/ML%20Flow.puml) | goMLer: ML plugins over processed items |
  | [Workers](../puml/Workers.puml) | svebapp: SyncWorker, LayoutWorker, View subscribed to LayoutDB |
  | [Protocol](../puml/Protocol.puml) | item fields on each layer: SQL → Go → svebapp → Dexie → View |

Modules:

- [gontroller](../../gontroller/readme.md) — Go backend: import, EXIF, HTTP API.
- [perceplib](../../perceplib/README.md) — shared library for the server and plugins.
- [perceptors](../../perceptors/readme.md) — plugins.
- [svebapp](../../svebapp/README.md) — frontend gallery.
