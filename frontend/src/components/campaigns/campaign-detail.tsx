"use client";
import { useState } from "react";
import dynamic from "next/dynamic";
import Link from "next/link";
import { useQueryState, parseAsStringEnum } from "nuqs";
import {
  useCampaign,
  useCreatives,
  useCampaignMutation,
  useCatalogOptions,
} from "@/hooks/use-campaigns";
import { CampaignStatus } from "./campaign-status";
import { BudgetGauge } from "./budget-gauge";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { ErrorState } from "@/components/data/shared";
import { ApiClientError } from "@/lib/api/client";
import { campaignCopy as c } from "@/lib/copy/campaign";
const loading = () => <Skeleton className="h-64 w-full" />;
const Targeting = dynamic(
  () =>
    import("./campaign-targeting").then((module) => module.CampaignTargeting),
  { loading },
);
const Creatives = dynamic(
  () =>
    import("./campaign-creatives").then((module) => module.CampaignCreatives),
  { loading },
);
const Settings = dynamic(
  () => import("./campaign-settings").then((module) => module.CampaignSettings),
  { loading },
);
const Report = dynamic(
  () => import("./campaign-report").then((module) => module.CampaignReport),
  { loading },
);
const Archive = dynamic(() =>
  import("./archive-campaign").then((module) => module.ArchiveCampaign),
);
const tabs = [
  "overview",
  "targeting",
  "creatives",
  "reports",
  "settings",
] as const;
export function CampaignDetail({
  id,
  testMode,
}: {
  id: string;
  testMode: boolean;
}) {
  const query = useCampaign(id);
  const creatives = useCreatives(id);
  const options = useCatalogOptions();
  const mutation = useCampaignMutation();
  const [tab, setTab] = useQueryState(
    "tab",
    parseAsStringEnum([...tabs]).withDefault("overview"),
  );
  const [dirty, setDirty] = useState(false);
  const [archive, setArchive] = useState(false);
  if (query.isPending) return <Skeleton className="h-96 w-full" />;
  if (query.isError)
    return (
      <>
        <Link href="/campaigns">← {c.title}</Link>
        <ErrorState
          message={query.error.message}
          requestId={
            query.error instanceof ApiClientError
              ? query.error.requestId
              : undefined
          }
          retry={() => {
            void query.refetch();
          }}
        />
      </>
    );
  const campaign = query.data;
  const canActivate =
    campaign.status !== "archived" &&
    Boolean(creatives.data?.some((item) => item.status === "active"));
  const disabled = campaign.status === "archived";
  return (
    <>
      <Link className="back-link" href="/campaigns">
        ← {c.title}
      </Link>
      <div className="campaign-detail-heading">
        <div>
          <h1>{campaign.name}</h1>
          <div className="campaign-subtitle">
            <CampaignStatus status={campaign.status} />
            <span>
              {options.advertisers.data?.find(
                (item) => item.id === campaign.advertiser_id,
              )?.name ?? campaign.advertiser_id}
            </span>
            <span>
              {options.bidders.data?.find(
                (item) => item.id === campaign.bidder_id,
              )?.name ?? campaign.bidder_id}
            </span>
          </div>
        </div>
        <div className="row-actions">
          {campaign.status === "active" ? (
            <Button
              disabled={mutation.isPending}
              onClick={() => mutation.mutate({ type: "pause", id })}
            >
              {c.pause}
            </Button>
          ) : (
            <span
              title={
                canActivate
                  ? undefined
                  : disabled
                    ? c.archiveTerminal
                    : c.cannotActivate
              }
            >
              <Button
                disabled={!canActivate || mutation.isPending}
                onClick={() => mutation.mutate({ type: "activate", id })}
              >
                {c.activate}
              </Button>
            </span>
          )}
          <Button
            variant="outline"
            disabled={disabled || mutation.isPending}
            onClick={() => setArchive(true)}
          >
            {c.archive}
          </Button>
        </div>
      </div>
      <p className="field-help">{c.snapshotHint}</p>
      {disabled && <p className="notice">{c.readOnly}</p>}
      <Tabs
        value={tab}
        onValueChange={(value) => {
          if (!dirty || window.confirm(c.discard)) {
            setDirty(false);
            void setTab(value as (typeof tabs)[number]);
          }
        }}
      >
        <TabsList className="campaign-tabs">
          {tabs.map((value) => (
            <TabsTrigger key={value} value={value}>
              {c[value]}
            </TabsTrigger>
          ))}
        </TabsList>
        <TabsContent value="overview">
          <section className="panel detail-panel">
            <h2>{c.budgetLabel}</h2>
            <BudgetGauge id={id} />
            {testMode && (
              <Button
                variant="outline"
                disabled={mutation.isPending}
                onClick={() => {
                  if (window.confirm(c.resetHelp))
                    mutation.mutate({ type: "reset", id });
                }}
              >
                {c.testReset}
              </Button>
            )}
          </section>
          <section className="panel detail-panel">
            <Report id={id} />
            <Report id={id} range="7d" compact />
          </section>
        </TabsContent>
        <TabsContent value="targeting">
          <Targeting id={id} disabled={disabled} onDirty={setDirty} />
        </TabsContent>
        <TabsContent value="creatives">
          <Creatives id={id} disabled={disabled} />
        </TabsContent>
        <TabsContent value="reports">
          <section className="panel detail-panel">
            <h2>{c.reportTitle}</h2>
            <p className="field-help">{c.reportDescription}</p>
            <Report id={id} range="7d" filters />
          </section>
        </TabsContent>
        <TabsContent value="settings">
          <Settings key={id} campaign={campaign} onDirty={setDirty} />
        </TabsContent>
      </Tabs>
      {archive && (
        <Archive campaign={campaign} onClose={() => setArchive(false)} />
      )}
    </>
  );
}
