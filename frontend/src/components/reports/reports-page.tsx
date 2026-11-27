"use client";
import { PageHeader } from "@/components/layout/page-header";
import { ReportWorkspace } from "./report-workspace";
import { reportingCopy as c } from "@/lib/copy/reporting";
export function ReportsPage() {
  return (
    <>
      <PageHeader title={c.reports} description={c.reportsHelp} />
      <ReportWorkspace />
    </>
  );
}
