type Level = 'debug' | 'info' | 'warn' | 'error';
const ORDER: Record<Level, number> = {debug: 0, info: 1, warn: 2, error: 3};

// The server's mode sets it (GET /app; the workers get it as a message): debug —
// everything, release — warnings and errors. Before that a dev build shows all.
let minLevel: Level = import.meta.env.DEV ? 'debug' : 'warn';

export function setLogLevel(mode: string) {
    minLevel = mode === 'debug' ? 'debug' : 'warn';
}

class Logger {
    private readonly context: string;

    constructor(context:string) {
        this.context = context; //getCallerModule();
    }

    private log(level: Level, color: string, ...messages: any[]) {
        if (ORDER[level] < ORDER[minLevel]) return;
        const timestamp = new Date().toLocaleString();

        console[level](
            `%c[${level.toUpperCase()}][${timestamp}][${this.context}]`,
            `color: ${color}; font-weight: bold;`,
            ...messages
        );

        // console.info()
        // console.warn()
        // console.error()
        // console.debug()
    }

    info(...messages: any[]) {
        this.log('info', 'blue', ...messages);
    }

    warn(...messages: any[]) {
        this.log('warn', 'orange', ...messages);
    }

    error(...messages: any[]) {
        this.log('error', 'red', ...messages);
    }

    debug(...messages: any[]) {
        this.log('debug', 'green', ...messages);
    }
}

export function getLogger(){
    return new Logger(callerContext(new Error('caller').stack));
}

/**
 * Short name of the calling module from a stack trace. Stack formats differ:
 *   Chrome:          "Error\n    at getLogger (http://…/logger.ts:42:19)\n    at http://…/proxy.ts:9:16"
 *   Safari, Firefox: "getLogger@http://…/logger.ts:42:19\nmodule code@http://…/proxy.ts:9:26"
 * Must never throw: it runs at module initialisation (a throw there broke the whole
 * app in Safari, which has no "Error" header line).
 */
export function callerContext(stack: string | undefined): string {
    // Only frames that carry a location; the first one is getLogger itself
    const files = (stack ?? '').split('\n').map(frameFile).filter((f) => f !== undefined);
    const filePath = files[1];
    if (!filePath) return 'unknown';
    // Files not nested under lib/routes/components: just the file name
    return category(filePath) ?? (filePath.split('/').pop()!.split('?')[0] || 'unknown');
}

/**
 * frameFile: the file a stack frame points at — "at f (…/x.ts:1:2)", "at …/x.ts:1:2",
 * "f@…/x.ts:1:2"; undefined: no location. Read from the end, without a pattern that
 * backtracks: it runs on every module's start.
 */
function frameFile(frame: string): string | undefined {
    let end = frame.trimEnd();
    if (end.endsWith(')')) end = end.slice(0, -1);
    const column = end.lastIndexOf(':');
    const line = end.lastIndexOf(':', column - 1);
    if (line < 0 || !isNumber(end.slice(line + 1, column)) || !isNumber(end.slice(column + 1))) return undefined;
    const location = end.slice(0, line);
    return location.slice(Math.max(location.lastIndexOf(' '), location.lastIndexOf('('), location.lastIndexOf('@')) + 1);
}

/** category: "l/s/file.ts" — a file under lib/, routes/ or components/ by their first letters and the next directory's */
function category(filePath: string): string | undefined {
    const parts = filePath.split('/');
    const name = /^\w+\.\w+/.exec(parts.at(-1) ?? '')?.[0];
    if (!name) return undefined;
    for (let i = parts.length - 3; i >= 0; i--) {
        if (CATEGORIES.has(parts[i]) && parts[i + 1]) return `${parts[i][0]}/${parts[i + 1][0]}/${name}`;
    }
    return undefined;
}

const CATEGORIES = new Set(['lib', 'routes', 'components']);

const isNumber = (s: string) => s.length > 0 && [...s].every((c) => c >= '0' && c <= '9');
