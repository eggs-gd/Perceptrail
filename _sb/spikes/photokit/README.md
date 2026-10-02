# PhotoKit spike

Question (roadmap, step 0 "Apple Photos first"): when Photos is asked through
PhotoKit for an image of an asset that is only in iCloud, does the rendition become
local **in the library** (a file in `resources/derivatives`, `ZLOCALAVAILABILITY` in
the DB), or are we only handed the pixels? How long does it take — can the viewer
wait for it on open? And the Photos permission of a bare binary (no `.app` bundle).

Go + cgo (Objective-C, `photokit.m`), as the product would do it; the Info.plist with
`NSPhotoLibraryUsageDescription` is put into the binary by the linker
(`-sectcreate __TEXT __info_plist`). Not part of any build — a module of its own.

It only reads the library; Photos itself may download renditions into it.

```bash
go build -o photokit-spike .
./photokit-spike -status                 # the permission, without asking
./photokit-spike <asset UUID>...         # asks once, then per asset:
```

Per asset: PhotoKit's resources, the DB rows (`ZINTERNALRESOURCE`) and the files on
disk; a request without network, with network (timed), without network again; after
`-wait` the resources, the DB and the files again. Run it from the terminal that runs
gontroller (it can read the library — the agent's shell cannot).

Results: `_sb/docs/findings.md`, "PhotoKit spike".
