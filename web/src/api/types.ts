import { z } from 'zod';

export const ItemTypeSchema = z.enum([
  'book',
  'learning-plan',
  'article',
  'video',
  'audiobook',
  'live-event-series',
  'scenario',
  'certs-practice-exam',
]);
export type ItemType = z.infer<typeof ItemTypeSchema>;

export const PublisherSchema = z.object({
  uuid: z.string(),
  name: z.string(),
  item_count: z.number().optional(),
});
export type Publisher = z.infer<typeof PublisherSchema>;

// publisher_uuid is omitempty in the API but indexed in Dexie for future filtering
export const ItemSchema = z.object({
  ourn: z.string(),
  name: z.string(),
  type: ItemTypeSchema,
  publication_date: z.string(),
  popularity: z.number(),
  publisher_uuid: z.string().optional(),
});
export type Item = z.infer<typeof ItemSchema>;

export const PublisherIndexSchema = z.object({
  publishers: z.array(PublisherSchema),
});

export const ItemListSchema = z.object({
  items: z.array(ItemSchema),
});
