# Photos lying on their side

## The symptom

Old photos (the 2000s, compact cameras) show turned a quarter — in Perceptrail,
and as well in a browser, Immich, macOS Preview. They looked right back then, in
Windows XP.

## The cause

A camera's sensor always writes landscape pixels; a portrait photo is landscape
pixels plus the EXIF tag `Orientation` saying "turn me" (6 or 8). Some cameras'
own rotate and some old tools turned the **pixels** upright but left the tag. A
viewer that reads the tag (every one today) turns the photo again; XP's did not
read it, so the fault stayed unseen.

A sign it is this: the photo is already portrait in its pixels and still carries
`Orientation` 6 or 8. A correct portrait shot from such a camera is landscape in
its pixels.

In the one archive measured: 415 of 2 359 photos before 2010, nearly all from one
Panasonic DMC-LS70 — the camera's "Ver.1.0" still in `Software`, so nothing else
had touched them.

## How to check

ExifTool lists them (it is in the server's Docker image; on a Mac `brew install
exiftool`). In the server container's console (`docker compose exec server sh`,
or Portainer), with your library's path:

```sh
exiftool -r -n -q -q -ext jpg -ext jpeg \
  -if '($Orientation == 6 or $Orientation == 8) and $ImageHeight > $ImageWidth' \
  -p '$Directory/$FileName' /library > /tmp/sideways.txt
wc -l /tmp/sideways.txt
```

Open a few of the listed files in any viewer: they should be on their side. A
landscape photo with `Orientation` 6 is a correct portrait shot and is not listed.

## What to do

Set the tag to "as is" (`Orientation` 1) on exactly those files. Perceptrail
never writes to a library, so this is yours to run — on your machine, not in the
container (its mounts are read only), **on a backed-up copy first**:

```sh
exiftool -r -n -ext jpg -ext jpeg \
  -if '($Orientation == 6 or $Orientation == 8) and $ImageHeight > $ImageWidth' \
  -Orientation=1 /path/to/photos
```

ExifTool keeps each original beside it as `<name>_original`; once you have looked,
`exiftool -r -delete_original /path/to/photos` removes them. Only the tag changes —
the pixels are not re-encoded. Perceptrail picks the files up on its next walk.

Perceptrail is to see through this by itself (the tag ignored on portrait pixels):
[roadmap](../../_sb/docs/roadmap.md), "Product bugs".
