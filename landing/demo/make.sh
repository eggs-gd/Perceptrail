#!/usr/bin/env bash
# The perceptors' demo, end to end: the demo library from the manifest, a test pair
# over it (gontroller on :1329 with the geo plugin, the gallery on :5174 — never the
# owner's :1323 / :5173), its renditions waited for, the recording, then the files the
# landing and the README show. Needs exiftool, libvips, ffmpeg, Google Chrome.
#
#   landing/demo/make.sh <work dir>
# Writes landing/static/demo/{perceptors.mp4,perceptors.webm,poster.webp} and
# docs/images/perceptors.gif.
set -euo pipefail
work=$(mkdir -p "$1" && cd "$1" && pwd)
repo=$(cd "$(dirname "$0")/../.." && pwd)
lib="$work/library"
pair="$work/pair"
mkdir -p "$pair/plugins"

node "$repo/landing/demo/fetch.mjs" "$repo/landing/demo/manifest.json" "$lib"

# The pair, its data in the work dir; stopped by its config's path on exit
(cd "$repo/perceptors/exif_geo" && go build -buildmode=plugin -o "$pair/plugins/exif_geo.so" .)
(cd "$repo/gontroller" && go build -o "$pair/gontroller" .)
cat > "$pair/config.yml" <<EOF
paths: ["$lib"]
plugins: ["$pair/plugins/exif_geo.so"]
server: {port: 1329}
EOF
"$pair/gontroller" --config "$pair/config.yml" > "$pair/server.log" 2>&1 &
(cd "$repo/svebapp" && SVEBAPP_API=http://localhost:1329 PUBLIC_API_PATH=/api \
    npx vite dev --port 5174 --strictPort > "$pair/vite.log" 2>&1) &
trap 'pkill -f -- "--config $pair/config.yml"; pkill -f -- "vite dev --port 5174"' EXIT

# Every photo with its renditions before the camera rolls — or as many as render: a
# few Commons files may not, so it waits while the count still grows
want=$(node -e 'console.log(JSON.parse(require("fs").readFileSync(process.argv[1])).length)' "$repo/landing/demo/manifest.json")
done=0 still=0
while [ "$done" -lt "$want" ] && [ "$still" -lt 6 ]; do
    sleep 5
    now=$(curl -s localhost:1329/items | grep -c '/r/400' || true)
    if [ "$now" -gt "$done" ]; then still=0; elif [ "$now" -gt 0 ]; then still=$((still + 1)); fi
    done=$now
done
echo "make.sh: $done of $want rendered"

node "$repo/landing/demo/record.mjs" http://localhost:5174 "$work/rec"

# The landing's video (H.264 for every browser, WebM beside it), its poster, the
# README's GIF (smaller: GitHub shows a GIF inline)
out="$repo/landing/static/demo"
mkdir -p "$out" "$repo/docs/images"
ffmpeg -loglevel error -y -i "$work/rec/raw.mp4" -an -c:v libx264 -crf 26 -preset slow -pix_fmt yuv420p -movflags +faststart "$out/perceptors.mp4"
ffmpeg -loglevel error -y -i "$work/rec/raw.mp4" -an -c:v libvpx-vp9 -crf 38 -b:v 0 "$out/perceptors.webm"
vips copy "$work/rec/poster.png" "$out/poster.webp[Q=80]"
ffmpeg -loglevel error -y -i "$work/rec/raw.mp4" -vf "fps=10,scale=720:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=128[p];[b][p]paletteuse=dither=bayer:bayer_scale=4" \
    "$repo/docs/images/perceptors.gif"
ls -l "$out" "$repo/docs/images/perceptors.gif"
