# syntax=docker/dockerfile:1

# The server: gontroller and its perceptor plugins, built in one stage with one
# toolchain (a plugin loads only into a host built the same way), on Debian with
# jellyfin-ffmpeg (every hardware backend, HDR tone mapping), libvips (with its HEIC
# decoder) and exiftool. Context: the repository root.

ARG GO_VERSION=1.27.1

FROM golang:${GO_VERSION}-trixie AS build
WORKDIR /src
COPY perceplib perceplib
COPY perceptors perceptors
COPY gontroller gontroller
ARG VERSION=dev
RUN cd gontroller && go build -ldflags "-X perceptrail/gontroller/internal/app.Version=${VERSION}" -o /out/gontroller .
RUN for plugin in exif_geo ml_color; do \
      (cd perceptors/$plugin && go build -buildmode=plugin -o /out/plugins/$plugin.so .) || exit 1; \
    done

FROM debian:trixie-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl gnupg \
 && curl -fsSL https://repo.jellyfin.org/jellyfin_team.gpg.key | gpg --dearmor -o /usr/share/keyrings/jellyfin.gpg \
 && echo "deb [signed-by=/usr/share/keyrings/jellyfin.gpg] https://repo.jellyfin.org/debian trixie main" \
      > /etc/apt/sources.list.d/jellyfin.list \
 && apt-get update \
 && apt-get install -y --no-install-recommends jellyfin-ffmpeg8 libvips-tools libheif-plugin-libde265 libimage-exiftool-perl \
 && apt-get purge -y curl gnupg && apt-get autoremove -y && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/gontroller /usr/bin/gontroller
COPY --from=build /out/plugins /usr/lib/perceptrail/plugins
COPY docker/config.yml /etc/perceptrail/config.yml
VOLUME /data
EXPOSE 1323
ENTRYPOINT ["gontroller", "--config", "/etc/perceptrail/config.yml"]
