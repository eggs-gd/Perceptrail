# landing

The project's page for the public, [perceptrail.eggs.gd](https://perceptrail.eggs.gd):
what Perceptrail is, its perceptors, the libraries it reads, how to install it. A
static SvelteKit site (adapter-static, every page prerendered), deployed to GitHub
Pages by [`pages.yml`](../.github/workflows/pages.yml).

```bash
npm install
npm run dev       # dev server
npm run check     # svelte-check (must be 0 errors)
npm test          # unit tests (vitest); npm run coverage: with coverage (lcov)
npm run build     # the static site in build/
```

## Two channels, one site

The site is built once per branch: `master`'s at the root — **stable**, what a release
installs (images tagged `latest`) — and `develop`'s under `/develop` — what is being
tested (images tagged `develop`; `noindex`, the stable page is the one to find).
`LANDING_CHANNEL` and `BASE_PATH` say which build it is; the install section switches
between both channels' commands and links to the other page. Before a release carries
the landing, the root is `develop`'s too.

The facts the page shows — its address, the channels, the install commands — are in
one place, [`src/lib/site.ts`](src/lib/site.ts).

## The organization's rules

SEO, analytics and consent follow the organization's spec (the reference copy is
`eggs.gd`'s `docs/seo.md`; the rules in
[`eggs-gd/.github`](https://github.com/eggs-gd/.github/blob/main/docs/working-rules.md#analytics-and-consent)):
a canonical URL, Open Graph and Twitter cards, `SoftwareApplication` JSON-LD,
`robots.txt` and `sitemap.xml`; the Google tag in Consent Mode (`denied` until the
visitor accepts) — `src/app.html`, `src/lib/analytics/` are the template's files, kept
identical to it; the footer of a `*.eggs.gd` project (© earwiG Games, eggs.gd, Privacy,
Cookie settings). English only for now.

The tests are the pure code's: the install commands per channel and the consent's
state.
