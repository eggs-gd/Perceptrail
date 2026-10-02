import {sveltekit} from '@sveltejs/kit/vite';
import {defineConfig} from "vite";
import {execFileSync} from "node:child_process";
import {fileURLToPath} from "node:url";

// Monorepo version derived from git history (scripts/version.sh); "dev" if unavailable
function appVersion(): string {
    try {
        const script = fileURLToPath(new URL('../scripts/version.sh', import.meta.url));
        return execFileSync(script, {encoding: 'utf8'}).trim();
    } catch {
        return 'dev';
    }
}

export default defineConfig({
    plugins: [sveltekit()],
    define: {
        __APP_VERSION__: JSON.stringify(appVersion()),
    },
});
