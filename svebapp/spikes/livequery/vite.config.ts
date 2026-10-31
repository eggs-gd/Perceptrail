import {defineConfig} from "vite";

// Standalone spike page (not part of the SvelteKit app): npx vite spikes/livequery
export default defineConfig({
    root: __dirname,
    server: {port: 5175, strictPort: true},
});
