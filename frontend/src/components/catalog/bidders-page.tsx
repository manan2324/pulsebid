"use client";
import { useMemo, useState } from "react";
import dynamic from "next/dynamic";
import Link from "next/link";
import { Plus, RefreshCw, Pencil, Trash2, ArrowUpRight } from "lucide-react";
import type { ColumnDef } from "@tanstack/react-table";
import { useBidders, useBidderMutation } from "@/hooks/use-catalog";
import { DataTable } from "@/components/data/data-table";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
const BidderForm = dynamic(() =>
  import("./catalog-forms").then((module) => module.BidderForm),
);
const ConfirmAction = dynamic(() =>
  import("./confirm-action").then((module) => module.ConfirmAction),
);
import { BidderHealth } from "./bidder-health";
import { getBidderIndex } from "@/lib/bidder-index";
import type { Bidder } from "@/lib/api/types";
import type { BidderCreate } from "@/lib/api/bidders";
import { copy } from "@/lib/copy/app";
import { catalogCopy as c } from "@/lib/copy/catalog";
export function BiddersPage({
  basePort,
  count,
}: {
  basePort: number;
  count: number;
}) {
  const query = useBidders();
  const mutation = useBidderMutation();
  const [editor, setEditor] = useState<Bidder | "new" | null>(null);
  const [deleting, setDeleting] = useState<Bidder | null>(null);
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("all");
  const records = useMemo(
    () => query.data?.pages.flatMap((page) => page.data) ?? [],
    [query.data],
  );
  const data = useMemo(
    () =>
      records.filter(
        (record) =>
          (status === "all" ||
            (status === "enabled" ? record.enabled : !record.enabled)) &&
          `${record.name} ${record.id} ${record.endpoint_url}`
            .toLowerCase()
            .includes(search.toLowerCase()),
      ),
    [records, search, status],
  );
  const columns = useMemo<ColumnDef<Bidder>[]>(
    () => [
      {
        accessorKey: "name",
        header: c.name,
        enableHiding: false,
        cell: ({ row }) => (
          <div className="record-name">
            <button onClick={() => setEditor(row.original)}>
              {row.original.name}
            </button>
            <code>{row.original.id}</code>
          </div>
        ),
      },
      {
        accessorKey: "endpoint_url",
        header: c.endpoint,
        cell: ({ row }) => (
          <code className="endpoint-cell" title={row.original.endpoint_url}>
            {row.original.endpoint_url}
          </code>
        ),
      },
      {
        accessorKey: "timeout_ms",
        header: c.timeout,
        cell: ({ row }) =>
          row.original.timeout_ms === null ? (
            <span className="muted-text">{c.defaultTimeout}</span>
          ) : (
            <code>
              {row.original.timeout_ms} {c.ms}
            </code>
          ),
      },
      {
        accessorKey: "enabled",
        header: c.status,
        cell: ({ row }) => (
          <span
            className={`catalog-badge ${row.original.enabled ? "active" : "archived"}`}
          >
            {row.original.enabled ? c.enabled : c.disabled}
          </span>
        ),
      },
      {
        id: "health",
        header: c.health,
        enableSorting: false,
        cell: ({ row }) => (
          <BidderHealth
            index={getBidderIndex(row.original.endpoint_url, {
              basePort,
              count,
            })}
          />
        ),
      },
      {
        id: "actions",
        header: c.actions,
        enableHiding: false,
        enableSorting: false,
        cell: ({ row }) => {
          const index = getBidderIndex(row.original.endpoint_url, {
            basePort,
            count,
          });
          return (
            <div className="row-actions">
              <Button
                variant="ghost"
                size="sm"
                aria-label={`${c.edit} ${row.original.name}`}
                onClick={() => setEditor(row.original)}
              >
                <Pencil size={13} />
                {c.edit}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                aria-label={`${row.original.enabled ? c.disable : c.enable} ${row.original.name}`}
                disabled={mutation.isPending}
                onClick={() =>
                  mutation.mutate({
                    type: "update",
                    id: row.original.id,
                    body: { enabled: !row.original.enabled },
                  })
                }
              >
                {row.original.enabled ? c.disable : c.enable}
              </Button>
              {index !== null && (
                <Link className="quick-tune" href={`/lab?bidder=${index}`}>
                  {c.quickTune}
                  <ArrowUpRight size={12} />
                </Link>
              )}
              <Button
                variant="ghost"
                size="icon"
                className="danger-action"
                aria-label={`${c.delete} ${row.original.name}`}
                onClick={() => setDeleting(row.original)}
              >
                <Trash2 size={14} />
              </Button>
            </div>
          );
        },
      },
    ],
    [basePort, count, mutation],
  );
  const filtered = Boolean(search) || status !== "all";
  return (
    <>
      <div className="catalog-heading">
        <PageHeader
          title={copy.nav[3].label}
          description={c.biddersDescription}
        />
        <div className="catalog-heading-actions">
          <Button variant="outline" asChild>
            <Link href="/lab">
              {c.openLab}
              <ArrowUpRight size={14} />
            </Link>
          </Button>
          <Button onClick={() => setEditor("new")}>
            <Plus size={15} />
            {c.newBidder}
          </Button>
        </div>
      </div>
      <DataTable
        columns={columns}
        data={data}
        loading={query.isPending}
        error={query.error}
        retry={() => {
          void query.refetch();
        }}
        emptyTitle={filtered ? c.noMatches : c.emptyBidders}
        emptyDescription={filtered ? c.noMatchesHelp : c.emptyBiddersHelp}
        emptyAction={
          filtered ? (
            <Button
              variant="outline"
              onClick={() => {
                setSearch("");
                setStatus("all");
              }}
            >
              {c.clearFilters}
            </Button>
          ) : (
            <Button onClick={() => setEditor("new")}>{c.createBidder}</Button>
          )
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
              placeholder={c.searchPlaceholder}
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
            <select
              className="catalog-select"
              aria-label={c.statusFilter}
              value={status}
              onChange={(event) => setStatus(event.target.value)}
            >
              <option value="all">{c.allStatuses}</option>
              <option value="enabled">{c.enabled}</option>
              <option value="disabled">{c.disabled}</option>
            </select>
            <Button
              variant="ghost"
              size="icon"
              aria-label={c.refresh}
              disabled={query.isFetching}
              onClick={() => {
                void query.refetch();
              }}
            >
              <RefreshCw size={15} />
            </Button>
          </>
        }
      />
      {editor && (
        <BidderForm
          key={editor === "new" ? "new" : editor.id}
          bidder={editor === "new" ? undefined : editor}
          onClose={() => setEditor(null)}
          onSave={(body) =>
            mutation.mutateAsync(
              editor === "new"
                ? { type: "create", body: body as BidderCreate }
                : { type: "update", id: editor.id, body },
            )
          }
        />
      )}
      {deleting && (
        <ConfirmAction
          kind="delete"
          name={deleting.name}
          onClose={() => setDeleting(null)}
          onConfirm={() =>
            mutation.mutateAsync({ type: "delete", id: deleting.id })
          }
        />
      )}
    </>
  );
}
