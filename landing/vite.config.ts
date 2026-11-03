/// <reference types="vitest/config" />
import adapter from '@sveltejs/adapter-static';
import {sveltekit} from '@sveltejs/kit/vite';
import {vitePreprocess} from '@sveltejs/vite-plugin-svelte';
import {defineConfig} from 'vite';
import {execFileSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';

// The landing of perceptrail.eggs.gd, built once per branch (.github/workflows/pages.yml):
// master's at the root (stable: what a release installs), develop's under /develop
// (what is being tested). LANDING_CHANNEL says which this build is; BASE_PATH where it
// is served.
const channel = process.env.LANDING_CHANNEL === 'develop' ? 'develop' : 'stable';

// The monorepo's version (scripts/version.sh), as svebapp shows it; "dev" without history
function version(): string {
    try {
        return execFileSync(fileURLToPath(new URL('../scripts/version.sh', import.meta.url)), {encoding: 'utf8'}).trim();
    } catch {
        return 'dev';
    }
}

export default defineConfig({
    plugins: [sveltekit({
        preprocess: vitePreprocess(),
        // Every page prerendered (+layout.ts); GitHub Pages serves the files as they are
        adapter: adapter({strict: true}),
        paths: {base: process.argv.includes('dev') ? '' : (process.env.BASE_PATH ?? '') as '' | `/${string}`},
    })],
    define: {
        __CHANNEL__: JSON.stringify(channel),
        __VERSION__: JSON.stringify(version()),
    },
    // The analytics consent's unit tests; coverage as lcov for SonarQube (make sonar)
    test: {
        include: ['src/**/*.test.ts'],
        coverage: {provider: 'v8', include: ['src/**/*.ts'], exclude: ['src/**/*.d.ts', 'src/**/*.test.ts'], reporter: [['lcov', {projectRoot: '..'}], 'text-summary']},
    },
});
