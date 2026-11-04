#!/usr/bin/env bash
# The landing's link preview (Open Graph, 1200×630): the name, the line under it, the
# footer, on the page's dark blue. Needs libvips.
#
#   landing/og.sh
# Writes landing/static/og.png.
#
# Not obvious:
# - a line break is a real newline in the Pango markup: "\n" in single quotes is two
#   characters, and the preview showed them
set -euo pipefail
out="$(cd "$(dirname "$0")" && pwd)/static/og.png"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cd "$work"

# The background: #1b1f3b, opaque
vips black bg.v 1200 630 --bands 3
vips linear bg.v bg2.v "1 1 1" "27 31 59"
vips cast bg2.v bg3.v uchar
vips bandjoin_const bg3.v bg4.v 255
vips copy bg4.v bg.v --interpretation srgb

# The three texts
vips text name.v '<span foreground="#eceef7" font_desc="sans bold 96">Perceptrail</span>' --rgba --dpi 72
vips text line.v '<span foreground="#a9adc6" font_desc="sans 40">One endless sheet of your photos.
Choose the point of view.</span>' --rgba --dpi 72
vips text foot.v '<span foreground="#ffb347" font_desc="sans 30">Self-hosted · by date, place, size · perceptrail.eggs.gd</span>' --rgba --dpi 72

# Placed on it
vips composite2 bg.v name.v c1.v over --x 80 --y 150
vips composite2 c1.v line.v c2.v over --x 84 --y 290
vips composite2 c2.v foot.v c3.v over --x 84 --y 500
vips flatten c3.v "$out"
