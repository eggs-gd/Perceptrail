# syntax=docker/dockerfile:1

# The web gallery: svebapp built as a single-page app, served by Caddy, which sends
# /api to the server — one address for the browser, whatever host it runs on.
# Context: the repository root.

FROM node:24-alpine AS build
WORKDIR /src
COPY svebapp/package.json svebapp/package-lock.json ./
RUN npm ci
COPY svebapp ./
ARG VERSION=dev
ENV PUBLIC_API_PATH=/api APP_VERSION=${VERSION}
RUN npm run build

FROM caddy:2-alpine
LABEL org.opencontainers.image.source=https://github.com/eggs-gd/Perceptrail \
      org.opencontainers.image.description="Perceptrail: the web gallery (Caddy; /api to the server)"
COPY --from=build /src/build /srv
COPY docker/Caddyfile /etc/caddy/Caddyfile
EXPOSE 80
