/// <reference types="vitest/config" />
import {sveltekit} from '@sveltejs/kit/vite';
import {defineConfig, loadEnv} from "vite";
import {execFileSync} from "node:child_process";
import {fileURLToPath} from "node:url";

// Monorepo version: APP_VERSION when given (a Docker build has no git history), else
// derived from git history (scripts/version.sh); "dev" if unavailable
function appVersion(): string {
    if (process.env.APP_VERSION) return process.env.APP_VERSION;
    try {
        const script = fileURLToPath(new URL('../scripts/version.sh', import.meta.url));
        return execFileSync(script, {encoding: 'utf8'}).trim();
    } catch {
        return 'dev';
    }
}

export default defineConfig(({mode}) => {
    // /api goes to gontroller in dev as the web container sends it in Docker:
    // PUBLIC_API_PATH=/api works the same in both (an absolute address works too)
    const api = loadEnv(mode, process.cwd(), 'SVEBAPP_').SVEBAPP_API ?? 'http://localhost:1323';
    // The unit tests' API path: $env/static/public reads it as the plugin loads
    if (mode === 'test') process.env.PUBLIC_API_PATH ??= '/api';
    return {
        plugins: [sveltekit()],
        define: {
            __APP_VERSION__: JSON.stringify(appVersion()),
        },
        server: {
            proxy: {'/api': {target: api, rewrite: (path) => path.replace(/^\/api/, '')}},
        },
        // Unit tests of the pure code (no DOM, no IndexedDB); coverage as lcov for
        // SonarQube (make sonar)
        test: {
            include: ['src/**/*.test.ts'],
            coverage: {provider: 'v8', include: ['src/**/*.ts'], exclude: ['src/**/*.d.ts', 'src/**/*.test.ts'], reporter: [['lcov', {projectRoot: '..'}], 'text-summary']},
        },
    };
});
