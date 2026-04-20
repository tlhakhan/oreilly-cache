import Dexie, { type Table } from 'dexie';
import type { Item, Publisher } from '../api/types';

export interface SyncMeta {
  endpoint: string;
  last_modified: string;
}

class OReillyCacheDB extends Dexie {
  items!: Table<Item, string>;
  publishers!: Table<Publisher, string>;
  syncMeta!: Table<SyncMeta, string>;

  constructor() {
    super('oreilly-cache');
    this.version(3).stores({
      items: '&ourn, type, publication_date, popularity, publisher_uuid',
      publishers: '&uuid, name',
      syncMeta: '&endpoint',
    });
  }
}

export const db = new OReillyCacheDB();
