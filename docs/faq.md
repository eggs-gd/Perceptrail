# FAQ

**Does Perceptrail change my photos?** No. Libraries are read only (in Docker,
mounted `:ro`); an Apple Photos library is only read, an Immich only through a
read-only key. What Perceptrail makes — the database, renditions, previews it
extracts — lives in its data directory (`./data` in Docker).

**Then how do I fix something in my files?** With your own tools, on your own
copy; the [playbooks](README.md) say how for the faults we know. Perceptrail sees
the change on its next walk.

**Where is my data?** `data_dir` (`./data` in Docker): the SQLite database and
the cache — renditions and extracted previews, a tree by each item's ID.

**How do I move it to another server?** Stop it, copy the data directory and the
config, start it there with the libraries mounted at the same paths (the
container's paths in Docker): an item is found by its path, so a different path
is a new item, rendered again.

**Why is the first start slow?** The first walk reads every file's metadata
(ExifTool), then render makes the renditions in the background — at nice 10,
yielding to everything else. The gallery shows items as they are ready.

**Why does a library not need render?** Apple Photos and Immich keep their own
previews and video versions; Perceptrail serves those.

**Can it run without Docker?** Yes, from source: [install](install.md).
