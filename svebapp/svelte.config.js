import adapter from '@sveltejs/adapter-static';
import {vitePreprocess} from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
    preprocess: vitePreprocess(),

    kit: {
        // A single-page app: everything renders in the browser (workers, IndexedDB);
        // the web container serves these files and every other path gets the
        // fallback page, which routes in the browser
        adapter: adapter({fallback: '200.html'}),
    },
};

export default config;
