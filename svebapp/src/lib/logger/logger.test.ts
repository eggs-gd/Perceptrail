import {describe, expect, it} from 'vitest';
import {callerContext} from './logger';

// The calling module's short name from a stack as each browser writes it: the
// second frame with a location (the first is getLogger itself)
const getLogger = 'http://h:5173/src/lib/logger/logger.ts:53:20';
const stacks: Record<string, (file: string) => string> = {
    chrome: (file) => `Error: caller\n    at getLogger (${getLogger})\n    at ${file}:9:16`,
    'chrome, a named frame': (file) => `Error: caller\n    at getLogger (${getLogger})\n    at Module.x (${file}:9:16)`,
    'safari, firefox': (file) => `getLogger@${getLogger}\nmodule code@${file}:9:26`,
    'firefox, anonymous': (file) => `getLogger@${getLogger}\n@${file}:9:26\n`,
};
const files: [string, string][] = [
    ['/src/lib/stores/internal/layoutDb.ts', 'l/s/layoutDb.ts'],
    ['/src/lib/logger/logger.ts?t=1730000', 'l/l/logger.ts'],
    ['/src/lib/gallery/Gallery.svelte', 'l/g/Gallery.svelte'],
    ['/src/lib/workers/tasks/wlayout.ts?worker_file&type=module', 'l/w/wlayout.ts'],
    ['/src/routes/+layout.svelte', '+layout.svelte'],
    ['/src/routes/v/[view]/[guid]/+page.svelte?t=5', '+page.svelte'],
    ['/src/lib/app.svelte.ts', 'app.svelte.ts'],
    ['/node_modules/.vite/deps/chunk-ABC.js?v=12', 'chunk-ABC.js'],
    ['/_app/immutable/chunks/Ck3x.js', 'Ck3x.js'],
];

describe('callerContext', () => {
    for (const [browser, stack] of Object.entries(stacks)) {
        it.each(files)(`${browser}: %s`, (file, want) => {
            expect(callerContext(stack(`http://h:5173${file}`))).toBe(want);
        });
    }

    it.each([undefined, '', 'Error', 'garbage:1', 'a\nb', `getLogger@${getLogger}`])('no caller in %j: unknown', (stack) => {
        expect(callerContext(stack)).toBe('unknown');
    });

    it('a long line without a location stays linear (no backtracking)', () => {
        const started = performance.now();
        expect(callerContext(`a\n${'x:1'.repeat(20000)}:y`)).toBe('unknown');
        expect(performance.now() - started).toBeLessThan(200);
    });
});
