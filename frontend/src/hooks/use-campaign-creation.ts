"use client";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import type { CampaignDraft, CreationJournal } from "@/lib/campaign-wizard";
import { campaignCopy as c } from "@/lib/copy/campaign";
import { catalogCopy } from "@/lib/copy/catalog";
export function useCampaignCreation() {
  const query = useQueryClient();
  return useMutation({
    mutationFn: async ({
      draft,
      journal,
      onCheckpoint,
    }: {
      draft: CampaignDraft;
      journal: CreationJournal;
      onCheckpoint: (state: CreationJournal) => void;
    }) =>
      (await import("@/lib/campaign-wizard")).runCampaignCreation(
        draft,
        journal,
        onCheckpoint,
      ),
    onSuccess: async () => {
      toast.success(c.created, { description: catalogCopy.gatewayHint });
      await Promise.all(
        ["campaigns", "snapshot", "campaign-options"].map((key) =>
          query.invalidateQueries({ queryKey: [key] }),
        ),
      );
    },
  });
}
