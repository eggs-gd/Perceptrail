export function match(param: string | number): boolean {
    return ((param != null) &&
        (param !== '') &&
        !isNaN(Number(param.toString())));
}
