import { z } from "zod";
export const journalSchema = z.object({
  campaignId: z.string().nullable(),
  targetingSaved: z.boolean(),
  creativeIds: z.record(z.string(), z.string()),
  activated: z.boolean(),
  pending: z
    .object({
      kind: z.enum(["campaign", "creative"]),
      index: z.number(),
      priorIds: z.array(z.string()),
    })
    .nullable(),
});
export type CreationJournal = z.infer<typeof journalSchema>;
export const emptyJournal: CreationJournal = {
  campaignId: null,
  targetingSaved: false,
  creativeIds: {},
  activated: false,
  pending: null,
};
