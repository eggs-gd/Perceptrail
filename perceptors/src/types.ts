export interface IWriter {
}

export interface IReader {
}

export interface IPerceptionProvider {
    readers: IReader[];
    writer?: IWriter;
}

export interface IPerceptor {
    provider: IPerceptionProvider
    focusFunction: Function
    filterFunction: Function
    editSingleFunction?: Function
    editListFunction?: Function
}
