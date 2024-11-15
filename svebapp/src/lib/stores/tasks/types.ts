type WorkerTask<TData, TResult = void> = (
    signal: AbortSignal,
    data: TData
) => Promise<TResult>;

/**
 * List of available worker tasks
 */
enum WorkerTaskType {
    UpdateDb,
    UpdateLayout
}
