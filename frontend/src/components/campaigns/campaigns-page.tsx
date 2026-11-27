"use client";
import { useMemo, useState } from "react";
import Link from "next/link";
import dynamic from "next/dynamic";
import { useQueryStates, parseAsArrayOf, parseAsString } from "nuqs";
import { MoreHorizontal, Plus } from "lucide-react";
import type { ColumnDef } from "@tanstack/react-table";
import { useCampaigns, useCatalogOptions } from "@/hooks/use-campaigns";
import { DataTable } from "@/components/data/data-table";
import { MoneyText, ErrorState } from "@/components/data/shared";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { CampaignStatus } from "./campaign-status";
import { BudgetGauge } from "./budget-gauge";
import { useTimeMode } from "@/components/providers";
import { formatTime } from "@/lib/time";
import type { Campaign } from "@/lib/api/types";
import { campaignCopy as c } from "@/lib/copy/campaign";
const CampaignRowActions = dynamic(() =>
  import("./campaign-row-actions").then((module) => module.CampaignRowActions),
);
function RowActions({ campaign }: { campaign: Campaign }) {
  const [loaded, setLoaded] = useState(false);
  return loaded ? (
    <CampaignRowActions campaign={campaign} />
  ) : (
    <Button
      variant="ghost"
      size="icon"
      aria-label={`${c.actions} ${campaign.name}`}
      onClick={() => setLoaded(true)}
    >
      <MoreHorizontal size={16} />
    </Button>
  );
}
export function CampaignsPage() {
  const [filters, setFilters] = useQueryStates({
    q: parseAsString.withDefault(""),
    advertiser: parseAsString.withDefault(""),
    status: parseAsArrayOf(parseAsString).withDefault([]),
  });
  const query = useCampaigns(filters.advertiser);
  const options = useCatalogOptions();
  const { mode } = useTimeMode();
  const names = useMemo(
    () =>
      new Map(
        options.advertisers.data?.map((item) => [item.id, item.name]) ?? [],
      ),
    [options.advertisers.data],
  );
  const bidders = useMemo(
    () =>
      new Map(options.bidders.data?.map((item) => [item.id, item.name]) ?? []),
    [options.bidders.data],
  );
  const data = useMemo(
    () =>
      query.data?.pages
        .flatMap((page) => page.data)
        .filter(
          (item) =>
            (!filters.status.length || filters.status.includes(item.status)) &&
            item.name.toLowerCase().includes(filters.q.toLowerCase()),
        ) ?? [],
    [query.data, filters.status, filters.q],
  );
  const columns = useMemo<ColumnDef<Campaign>[]>(
    () => [
      {
        accessorKey: "name",
        header: c.name,
        enableHiding: false,
        cell: ({ row }) => (
          <Link
            className="campaign-name"
            href={`/campaigns/${row.original.id}`}
          >
            {row.original.name}
          </Link>
        ),
      },
      {
        accessorKey: "advertiser_id",
        header: c.advertiser,
        cell: ({ row }) =>
          names.get(row.original.advertiser_id) ?? c.unknownAdvertiser,
      },
      {
        accessorKey: "bidder_id",
        header: c.bidder,
        cell: ({ row }) =>
          bidders.get(row.original.bidder_id) ?? row.original.bidder_id,
      },
      {
        accessorKey: "status",
        header: c.status,
        cell: ({ row }) => <CampaignStatus status={row.original.status} />,
      },
      {
        id: "budget",
        header: c.budgetLabel,
        cell: ({ row }) => <BudgetGauge id={row.original.id} compact />,
      },
      {
        accessorKey: "max_bid_cpm",
        header: c.maxBid,
        cell: ({ row }) => (
          <MoneyText
            value={row.original.max_bid_cpm}
            maxFraction={6}
            unit="CPM"
          />
        ),
      },
      {
        accessorKey: "pacing_mode",
        header: c.pacing,
        cell: ({ row }) =>
          row.original.pacing_mode === "even" ? c.even : c.asap,
      },
      {
        accessorKey: "updated_at",
        header: `${c.updated} (${mode})`,
        cell: ({ row }) => (
          <span className="date-cell">
            {formatTime(row.original.updated_at, mode)}
          </span>
        ),
      },
      {
        id: "actions",
        header: c.actions,
        enableHiding: false,
        cell: ({ row }) => <RowActions campaign={row.original} />,
      },
    ],
    [names, bidders, mode],
  );
  return (
    <>
      <div className="catalog-heading">
        <PageHeader title={c.title} description={c.description} />
        <Button asChild>
          <Link href="/campaigns/new">
            <Plus size={15} />
            {c.newCampaign}
          </Link>
        </Button>
      </div>
      {(options.advertisers.error || options.bidders.error) && (
        <ErrorState
          message={c.optionError}
          retry={() => {
            void options.advertisers.refetch();
            void options.bidders.refetch();
          }}
        />
      )}
      <DataTable
        columns={columns}
        data={data}
        loading={query.isPending}
        error={query.error}
        retry={() => {
          void query.refetch();
        }}
        emptyTitle={
          filters.q || filters.status.length || filters.advertiser
            ? c.noMatches
            : c.empty
        }
        emptyDescription={
          filters.q || filters.status.length ? c.noMatchesHelp : c.emptyHelp
        }
        emptyAction={
          <Button asChild>
            <Link href="/campaigns/new">{c.newCampaign}</Link>
          </Button>
        }
        hasMore={query.hasNextPage}
        loadingMore={query.isFetchingNextPage}
        loadMore={() => {
          void query.fetchNextPage();
        }}
        toolbar={
          <>
            <Input
              aria-label={c.search}
              placeholder={c.searchHint}
              value={filters.q}
              onChange={(event) => {
                void setFilters({ q: event.target.value });
              }}
            />
            <select
              className="catalog-select"
              aria-label={c.advertiser}
              value={filters.advertiser}
              onChange={(event) => {
                void setFilters({ advertiser: event.target.value });
              }}
            >
              <option value="">{c.allAdvertisers}</option>
              {options.advertisers.data?.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name}
                </option>
              ))}
            </select>
            <details className="status-filter">
              <summary>
                {c.filterStatus} ({filters.status.length})
              </summary>
              <div>
                {(["draft", "active", "paused", "archived"] as const).map(
                  (status) => (
                    <label key={status}>
                      <input
                        type="checkbox"
                        checked={filters.status.includes(status)}
                        onChange={() => {
                          void setFilters({
                            status: filters.status.includes(status)
                              ? filters.status.filter((item) => item !== status)
                              : [...filters.status, status],
                          });
                        }}
                      />
                      {c[status]}
                    </label>
                  ),
                )}
              </div>
            </details>
          </>
        }
      />
    </>
  );
}
