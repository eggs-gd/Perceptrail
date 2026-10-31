import Dexie, {type EntityTable} from "dexie";

export interface SpikeItem {
    id: number;
    order: number;
    /** Date.now() in the worker right before the write */
    writtenAt: number;
}

export const db = new Dexie('spike-livequery') as Dexie & {
    items: EntityTable<SpikeItem, 'id'>;
};
db.version(1).stores({items: '&id, order'});
