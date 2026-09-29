class Logger {
    private readonly context: string;

    constructor(context:string) {
        this.context = context; //getCallerModule();
    }

    private log(level: 'info' | 'warn' | 'error' | 'debug', color: string, ...messages: any[]) {
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
    return new Logger(callerContext(new Error().stack));
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
    const frames = (stack ?? '').split('\n').filter((line) => /:\d+:\d+\)?\s*$/.test(line));
    const match = frames[1]?.match(/([^\s(@]+):\d+:\d+\)?\s*$/);
    if (!match) return 'unknown';

    const filePath = match[1];
    const category = filePath.replace(/.*\/(lib|routes|components)\/([^\/]+).*\/(\w+\.\w+).*/, (m, g1, g2, g3) => {
        return `${g1[0]}/${g2[0]}/${g3}`
    });
    // Files not nested under lib/routes/components: just the file name
    return category !== filePath ? category : filePath.split('/').pop()!.split('?')[0] || 'unknown';
}
