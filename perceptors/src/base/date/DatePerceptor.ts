import {IPerceptor} from "../../types";

export class DatePerceptor implements IPerceptor {
    provider = {readers:[]};
    focusFunction = () => {};
    filterFunction = () => {};
}
