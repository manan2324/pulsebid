"use client";
import { useMemo } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import { DataTable } from "@/components/data/data-table";
import type { ReportMetric, ReportPoint } from "@/lib/reporting";
import { reportValue } from "./report-kpis";
import { useTimeMode } from "@/components/providers";
import { formatTime } from "@/lib/time";
import { reportingCopy as c } from "@/lib/copy/reporting";
export function ChartDataTable({
  data,
  metric,
}: {
  data: ReportPoint[];
  metric: ReportMetric;
}) {
  const { mode } = useTimeMode();
  const rows = useMemo(
    () => data.map((row) => ({ ...row, id: row.bucket })),
    [data],
  );
  const columns = useMemo<ColumnDef<ReportPoint & { id: string }>[]>(
    () => [
      {
        accessorKey: "bucket",
        header: `${c.bucket} (${mode})`,
        cell: ({ row }) => formatTime(row.original.bucket, mode),
      },
      {
        accessorKey: metric,
        header: c[metric],
        cell: ({ row }) => reportValue(metric, row.original[metric]),
      },
    ],
    [metric, mode],
  );
  return (
    <DataTable
      footerHelp={c.chartHelp}
      columns={columns}
      data={rows}
      loading={false}
      retry={() => {}}
      emptyTitle={c.chartEmpty}
      emptyDescription={c.emptyHelp}
    />
  );
}
