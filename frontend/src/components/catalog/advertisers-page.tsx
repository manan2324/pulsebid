"use client";
import { useMemo, useState } from "react";
import dynamic from "next/dynamic";
import { Plus, RefreshCw, Pencil, Archive } from "lucide-react";
import type { ColumnDef } from "@tanstack/react-table";
import { useAdvertisers, useAdvertiserMutation } from "@/hooks/use-catalog";
import { DataTable } from "@/components/data/data-table";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
const AdvertiserForm = dynamic(() =>
  import("./catalog-forms").then((module) => module.AdvertiserForm),
);
const ConfirmAction = dynamic(() =>
  import("./confirm-action").then((module) => module.ConfirmAction),
);
import { useTimeMode } from "@/components/providers";
import { formatTime } from "@/lib/time";
import type { Advertiser } from "@/lib/api/types";
import type { AdvertiserCreate } from "@/lib/api/advertisers";
import { copy } from "@/lib/copy/app";
import { catalogCopy as c } from "@/lib/copy/catalog";
export function AdvertisersPage() {
  const query = useAdvertisers();
  const mutation = useAdvertiserMutation();
  const { mode } = useTimeMode();
  const [editor, setEditor] = useState<Advertiser | "new" | null>(null);
  const [archiving, setArchiving] = useState<Advertiser | null>(null);
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
          (status === "all" || record.status === status) &&
          `${record.name} ${record.id} ${record.contact_email ?? ""}`
            .toLowerCase()
            .includes(search.toLowerCase()),
      ),
    [records, search, status],
  );
  const columns = useMemo<ColumnDef<Advertiser>[]>(
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
        accessorKey: "contact_email",
        header: c.email,
        cell: ({ row }) =>
          row.original.contact_email ? (
            <a
              className="email-link"
              href={`mailto:${row.original.contact_email}`}
            >
              {row.original.contact_email}
            </a>
          ) : (
            <span className="muted-text">{c.noEmail}</span>
          ),
      },
      {
        accessorKey: "status",
        header: c.status,
        cell: ({ row }) => (
          <span className={`catalog-badge ${row.original.status}`}>
            {row.original.status === "active" ? c.active : c.archived}
          </span>
        ),
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
        enableSorting: false,
        cell: ({ row }) => (
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
              className="danger-action"
              aria-label={`${c.archive} ${row.original.name}`}
              disabled={row.original.status === "archived"}
              onClick={() => setArchiving(row.original)}
            >
              <Archive size={13} />
              {c.archive}
            </Button>
          </div>
        ),
      },
    ],
    [mode],
  );
  const filtered = Boolean(search) || status !== "all";
  return (
    <>
      <div className="catalog-heading">
        <PageHeader
          title={copy.nav[2].label}
          description={c.advertisersDescription}
        />
        <Button onClick={() => setEditor("new")}>
          <Plus size={15} />
          {c.newAdvertiser}
        </Button>
      </div>
      <DataTable
        columns={columns}
        data={data}
        loading={query.isPending}
        error={query.error}
        retry={() => {
          void query.refetch();
        }}
        emptyTitle={filtered ? c.noMatches : c.emptyAdvertisers}
        emptyDescription={filtered ? c.noMatchesHelp : c.emptyAdvertisersHelp}
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
            <Button onClick={() => setEditor("new")}>
              {c.createAdvertiser}
            </Button>
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
              <option value="active">{c.active}</option>
              <option value="archived">{c.archived}</option>
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
        <AdvertiserForm
          key={editor === "new" ? "new" : editor.id}
          advertiser={editor === "new" ? undefined : editor}
          onClose={() => setEditor(null)}
          onSave={(body) =>
            mutation.mutateAsync(
              editor === "new"
                ? { type: "create", body: body as AdvertiserCreate }
                : { type: "update", id: editor.id, body },
            )
          }
        />
      )}
      {archiving && (
        <ConfirmAction
          kind="archive"
          name={archiving.name}
          onClose={() => setArchiving(null)}
          onConfirm={() =>
            mutation.mutateAsync({ type: "archive", id: archiving.id })
          }
        />
      )}
    </>
  );
}
