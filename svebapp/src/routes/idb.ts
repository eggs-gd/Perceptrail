import Dexie, {type EntityTable} from "dexie";
import type {Item} from "$lib/gallery";

interface SvItem extends Item {
    date: Date;
}

const db = new Dexie('myDatabase') as Dexie & {
    items: EntityTable<SvItem, 'guid'>;
};
db.version(2).stores({
    items: '&guid, width, height, mimeType, date, path'
});


export {type SvItem, db}
