import adapter from '@sveltejs/adapter-static';
import {vitePreprocess} from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
    preprocess: vitePreprocess(),

    kit: {
        // A single-page app: everything renders in the browser (workers, IndexedDB);
        // the web container serves these files and every other path gets the
        // fallback page, which routes in the browser. index.html: nothing is
        // prerendered, so it conflicts with no page — and / is served as any host
        // serves a directory
        adapter: adapter({fallback: 'index.html'}),
    },
};

export default config;
