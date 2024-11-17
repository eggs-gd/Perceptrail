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
    let context:string = 'unknown';
    const stack = new Error().stack;

    if (stack) {
        const stackLine = stack.split('\n')[2]; // 3-d line is caller
        const match = stackLine.match(/\(?(.*):\d+:\d+\)?/);

        if (match) {
            const filePath = match[1];
            const category = filePath.replace(/.*\/(lib|routes|components)\/([^\/]+).*\/(\w+\.\w+).*/, (m, g1, g2, g3) => {
                return `${g1[0]}/${g2[0]}/${g3}`
            });
            context = category || 'unknown';
        }
    }
    return new Logger(context)
}
