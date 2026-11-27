"use client";
import { useMemo } from "react";
import Link from "next/link";
import type { ColumnDef } from "@tanstack/react-table";
import type { ReportRow } from "@/lib/api/types";
import { DataTable } from "@/components/data/data-table";
import { useTimeMode } from "@/components/providers";
import { formatTime } from "@/lib/time";
import { campaignName } from "@/lib/reporting";
import { reportValue } from "./report-kpis";
import { reportingCopy as c } from "@/lib/copy/reporting";
type Row = ReportRow & { id: string };
export function ReportTable({
  data,
  names,
}: {
  data: ReportRow[];
  names: ReadonlyMap<string, string>;
}) {
  const { mode } = useTimeMode();
  const rows = useMemo(
    () =>
      data.map((row) => ({
        ...row,
        id: `${row.campaign_id}:${row.bucket ?? "total"}`,
      })),
    [data],
  );
  const columns = useMemo<ColumnDef<Row>[]>(
    () => [
      {
        id: "campaign",
        accessorFn: (row) => campaignName(row.campaign_id, names),
        header: c.campaign,
        cell: ({ row }) => (
          <Link
            href={`/campaigns/${encodeURIComponent(row.original.campaign_id)}`}
            title={row.original.campaign_id}
          >
            {campaignName(row.original.campaign_id, names)}
          </Link>
        ),
      },
      {
        accessorKey: "bucket",
        header: `${c.bucket} (${mode})`,
        cell: ({ row }) =>
          row.original.bucket
            ? formatTime(row.original.bucket, mode)
            : c.intervals.total,
      },
      ...(
        [
          "impressions",
          "clicks",
          "ctr",
          "spend",
          "bids",
          "wins",
          "win_rate",
        ] as const
      ).map((metric) => ({
        accessorKey: metric,
        header: metric === "spend" ? `${c.spend} (${c.usd})` : c[metric],
        cell: ({ row }: { row: { original: Row } }) =>
          reportValue(metric, row.original[metric]),
      })),
    ],
    [names, mode],
  );
  return (
    <DataTable
      footerHelp={c.exportHelp}
      columns={columns}
      data={rows}
      loading={false}
      retry={() => {}}
      emptyTitle={c.empty}
      emptyDescription={c.emptyHelp}
    />
  );
}
