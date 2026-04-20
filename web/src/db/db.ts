import Dexie, { type Table } from 'dexie';
import type { Item, Publisher } from '../api/types';

interface Like {
  ourn: string;
  liked_at: number;
}

interface ScrollPos {
  view_key: string;
  updated_at: number;
  scroll_index: number;
  scroll_offset: number;
}

export interface SyncMeta {
  endpoint: string;
  last_modified: string;
}

class OReillyCacheDB extends Dexie {
  items!: Table<Item, string>;
  publishers!: Table<Publisher, string>;
  likes!: Table<Like, string>;
  scrollPos!: Table<ScrollPos, string>;
  syncMeta!: Table<SyncMeta, string>;

  constructor() {
    super('oreilly-cache');
    this.version(1).stores({
      items: '&ourn, type, publication_date, popularity, publisher_uuid',
      publishers: '&uuid, name',
      likes: '&ourn, liked_at',
      scrollPos: '&view_key, updated_at',
      syncMeta: '&endpoint',
    });
  }
}

export const db = new OReillyCacheDB();
