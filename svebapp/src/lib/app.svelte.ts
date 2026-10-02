// What the server says about itself (GET /app): its version and the mode. Debug: the
// gallery shows its debug marks and logs everything; release: not — one client build
// serves both.
import {PUBLIC_API_PATH} from '$env/static/public';
import {getLogger, setLogLevel} from '$lib/logger';

const logger = getLogger();

export const app = $state({version: '', mode: 'release'});

/** Loads the server's info; resolves with the mode (workers get it too) */
export async function loadApp(): Promise<string> {
    try {
        const response = await fetch(`${PUBLIC_API_PATH}/app`);
        Object.assign(app, await response.json());
    } catch (error) {
        logger.error('app info', error);
    }
    setLogLevel(app.mode);
    return app.mode;
}

export const debug = () => app.mode === 'debug';
