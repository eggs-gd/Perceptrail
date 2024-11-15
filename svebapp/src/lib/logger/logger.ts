export class Logger {
    private readonly context: string;

    constructor() {
        this.context = this.getCallerModule();
    }

    private getCallerModule(): string {
        const stack = new Error().stack;
        if (!stack) {
            return 'unknown';
        }

        const stackLine = stack.split('\n')[2]; // 3-d line is caller
        const match = stackLine.match(/\((.*):\d+:\d+\)/);

        if (match) {
            const filePath = match[1];
            const category = filePath.replace(/.*\/(lib|views|components)\/([^\/]+).*/, '$1/$2');
            return category || 'unknown';
        }

        return 'unknown';
    }

    private log(level: 'info' | 'warn' | 'error' | 'debug', color: string, ...messages: any[]) {
        const timestamp = new Date().toISOString();
        console.log(
            `%c[${level.toUpperCase()}] [${this.context}] [${timestamp}]`,
            `color: ${color}; font-weight: bold;`,
            ...messages
        );
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
