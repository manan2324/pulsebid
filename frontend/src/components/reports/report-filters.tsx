"use client";
import { useState } from "react";
import { useReportCatalog } from "@/hooks/use-reports";
import {
  utcPreset,
  utcInput,
  fromUtcInput,
  validateReportRange,
  type ReportPreset,
} from "@/lib/reportRange";
import { campaignName } from "@/lib/reporting";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { reportingCopy as c } from "@/lib/copy/reporting";
export type ReportSelection = {
  preset: string;
  from: string;
  to: string;
  granularity: string;
  campaignIds: string[];
};
export function ReportFilters({
  initial,
  now,
  fixedCampaignId,
  onApply,
}: {
  initial: ReportSelection;
  now: Date;
  fixedCampaignId?: string;
  onApply: (value: ReportSelection) => void;
}) {
  const [preset, setPreset] = useState(initial.preset);
  const [from, setFrom] = useState(initial.from);
  const [to, setTo] = useState(initial.to);
  const [granularity, setGranularity] = useState(initial.granularity);
  const [ids, setIds] = useState(initial.campaignIds);
  const [search, setSearch] = useState("");
  const catalog = useReportCatalog(!fixedCampaignId);
  const names = new Map(catalog.data?.map((row) => [row.id, row.name]) ?? []);
  const error = validateReportRange({
    from,
    to,
    granularity,
    campaignIds: fixedCampaignId ? [fixedCampaignId] : ids,
  });
  const options = (catalog.data ?? []).filter((row) =>
    `${row.name} ${row.id}`.toLowerCase().includes(search.toLowerCase()),
  );
  return (
    <form
      className="panel report-filters"
      onSubmit={(event) => {
        event.preventDefault();
        if (!error)
          onApply({
            preset,
            from,
            to,
            granularity,
            campaignIds: fixedCampaignId ? [fixedCampaignId] : ids,
          });
      }}
      noValidate
    >
      <div className="report-filter-grid">
        <label>
          {c.range}
          <select
            aria-label={c.range}
            value={preset}
            onChange={(event) => {
              const key = event.target.value as ReportPreset;
              setPreset(key);
              if (key !== "custom") {
                const bounds = utcPreset(key, now);
                setFrom(bounds.from);
                setTo(bounds.to);
              }
            }}
          >
            {Object.entries(c.presets).map(([key, label]) => (
              <option key={key} value={key}>
                {label}
              </option>
            ))}
          </select>
        </label>
        <label>
          {c.from}
          <Input
            type="datetime-local"
            step="0.001"
            value={utcInput(from)}
            onChange={(event) => {
              setFrom(fromUtcInput(event.target.value));
              setPreset("custom");
            }}
          />
        </label>
        <label>
          {c.to}
          <Input
            type="datetime-local"
            step="0.001"
            value={utcInput(to)}
            onChange={(event) => {
              setTo(fromUtcInput(event.target.value));
              setPreset("custom");
            }}
          />
        </label>
        <label>
          {c.granularity}
          <select
            aria-label={c.granularity}
            value={granularity}
            onChange={(event) => setGranularity(event.target.value)}
          >
            {Object.entries(c.intervals).map(([key, label]) => (
              <option
                key={key}
                value={key}
                disabled={Boolean(
                  validateReportRange({ from, to, granularity: key }),
                )}
              >
                {label}
              </option>
            ))}
          </select>
        </label>
      </div>
      {!fixedCampaignId && (
        <details className="report-campaign-picker">
          <summary>
            {c.campaigns}:{" "}
            {ids.length ? `${ids.length} ${c.selected}` : c.allCampaigns}
          </summary>
          <Input
            aria-label={c.searchCampaigns}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
          <p className="field-help">{c.selectionLimit}</p>
          <Button
            type="button"
            variant="ghost"
            disabled={!ids.length}
            onClick={() => setIds([])}
          >
            {c.clear}
          </Button>
          {catalog.isPending ? (
            <Skeleton className="h-24 w-full" />
          ) : catalog.isError ? (
            <p role="status">
              {c.namesUnavailable}
              <Button
                type="button"
                variant="ghost"
                onClick={() => {
                  void catalog.refetch();
                }}
              >
                {c.retry}
              </Button>
            </p>
          ) : (
            <div className="report-campaign-options">
              {options.slice(0, 200).map((row) => (
                <label key={row.id}>
                  <input
                    type="checkbox"
                    checked={ids.includes(row.id)}
                    disabled={!ids.includes(row.id) && ids.length >= 100}
                    onChange={() =>
                      setIds((previous) =>
                        previous.includes(row.id)
                          ? previous.filter((id) => id !== row.id)
                          : [...previous, row.id],
                      )
                    }
                  />
                  {row.name}
                </label>
              ))}
            </div>
          )}
          {options.length > 200 && (
            <p className="field-help">{c.refineSearch}</p>
          )}
          {ids.length > 0 && (
            <p className="field-help">
              {ids.map((id) => campaignName(id, names)).join(", ")}
            </p>
          )}
        </details>
      )}
      <p className="field-help">{c.rangeHelp}</p>
      {error && (
        <p role="alert" className="field-error">
          {error}
        </p>
      )}
      <Button disabled={Boolean(error)}>{c.apply}</Button>
    </form>
  );
}
