"use client";
import { useState } from "react";
import dynamic from "next/dynamic";
import Link from "next/link";
import { MoreHorizontal } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useCampaignMutation, useCreatives } from "@/hooks/use-campaigns";
import type { Campaign } from "@/lib/api/types";
import { campaignCopy as c } from "@/lib/copy/campaign";
const Archive = dynamic(() =>
  import("./archive-campaign").then((module) => module.ArchiveCampaign),
);
export function CampaignRowActions({ campaign }: { campaign: Campaign }) {
  const [open, setOpen] = useState(true);
  const [archive, setArchive] = useState(false);
  const mutation = useCampaignMutation();
  const creatives = useCreatives(
    campaign.id,
    open && campaign.status !== "active" && campaign.status !== "archived",
  );
  const canActivate = Boolean(
    creatives.data?.some((item) => item.status === "active"),
  );
  return (
    <>
      <DropdownMenu open={open} onOpenChange={setOpen}>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            aria-label={`${c.actions} ${campaign.name}`}
          >
            <MoreHorizontal size={16} />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem asChild>
            <Link href={`/campaigns/${campaign.id}`}>{c.open}</Link>
          </DropdownMenuItem>
          {campaign.status !== "archived" && (
            <>
              <DropdownMenuItem
                disabled={
                  mutation.isPending ||
                  (campaign.status !== "active" && !canActivate)
                }
                title={!canActivate ? c.cannotActivate : undefined}
                onSelect={() =>
                  mutation.mutate({
                    type: campaign.status === "active" ? "pause" : "activate",
                    id: campaign.id,
                  })
                }
              >
                {campaign.status === "active"
                  ? c.pause
                  : creatives.isPending
                    ? c.loading
                    : c.activate}
              </DropdownMenuItem>
              {creatives.isError && (
                <DropdownMenuItem
                  onSelect={(event) => {
                    event.preventDefault();
                    void creatives.refetch();
                  }}
                >
                  {c.retry}
                </DropdownMenuItem>
              )}
              <DropdownMenuItem onSelect={() => setArchive(true)}>
                {c.archive}
              </DropdownMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      {archive && (
        <Archive campaign={campaign} onClose={() => setArchive(false)} />
      )}
    </>
  );
}
