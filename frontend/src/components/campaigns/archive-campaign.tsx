"use client";
import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/data/shared";
import { useCampaignMutation } from "@/hooks/use-campaigns";
import type { Campaign } from "@/lib/api/types";
import { ApiClientError } from "@/lib/api/client";
import { campaignCopy as c } from "@/lib/copy/campaign";
export function ArchiveCampaign({
  campaign,
  onClose,
}: {
  campaign: Campaign;
  onClose: () => void;
}) {
  const [name, setName] = useState("");
  const mutation = useCampaignMutation();
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !mutation.isPending) onClose();
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{c.archiveTitle}</DialogTitle>
          <DialogDescription>{c.archiveHelp}</DialogDescription>
        </DialogHeader>
        <strong>{campaign.name}</strong>
        <label className="form-field">
          {c.confirmName}
          <Input
            value={name}
            onChange={(event) => setName(event.target.value)}
            disabled={mutation.isPending}
          />
        </label>
        {mutation.error && (
          <ErrorState
            message={mutation.error.message}
            requestId={
              mutation.error instanceof ApiClientError
                ? mutation.error.requestId
                : undefined
            }
          />
        )}
        <div className="form-actions">
          <Button
            variant="outline"
            disabled={mutation.isPending}
            onClick={onClose}
          >
            {c.cancel}
          </Button>
          <Button
            variant="destructive"
            disabled={name !== campaign.name || mutation.isPending}
            onClick={() =>
              mutation.mutate(
                { type: "archive", id: campaign.id },
                { onSuccess: onClose },
              )
            }
          >
            {c.archive}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
