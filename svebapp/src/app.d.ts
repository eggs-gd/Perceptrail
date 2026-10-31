// See https://kit.svelte.dev/docs/types#app
// for information about these interfaces
declare global {
    /** Monorepo version, injected at build time by vite.config.ts */
    const __APP_VERSION__: string;

    namespace App {
        // interface Error {}
        // interface Locals {}
        // interface PageData {}
        interface PageState {
            /** The viewer was opened from the gallery: closing it goes back */
            fromGallery?: boolean;
        }
        // interface Platform {}
    }
}

export {};
