import { reportingCopy as c } from "./copy/reporting";
export type Granularity = "hour" | "day" | "total";
export type ReportPreset = "today" | "yesterday" | "7d" | "30d" | "custom";
export type ReportRange = {
  from: string;
  to: string;
  granularity: Granularity;
  campaignIds: string[];
};
export const dayMs = 86400000;
export function utcPreset(
  preset: Exclude<ReportPreset, "custom">,
  now: Date = new Date(),
) {
  const start = Date.UTC(
    now.getUTCFullYear(),
    now.getUTCMonth(),
    now.getUTCDate(),
  );
  const from =
    start -
    (preset === "yesterday"
      ? 1
      : preset === "7d"
        ? 6
        : preset === "30d"
          ? 29
          : 0) *
      dayMs;
  return {
    from: new Date(from).toISOString(),
    to: new Date(preset === "yesterday" ? start : start + dayMs).toISOString(),
  };
}
export function validTimestamp(value: string) {
  const match =
    /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(Z|[+-](\d{2}):(\d{2}))$/.exec(
      value,
    );
  if (!match || !Number.isFinite(Date.parse(value))) return false;
  const year = Number(match[1]),
    month = Number(match[2]),
    day = Number(match[3]);
  const last = new Date(0);
  last.setUTCFullYear(year, month, 0);
  return (
    month >= 1 &&
    month <= 12 &&
    day >= 1 &&
    day <= last.getUTCDate() &&
    Number(match[4]) <= 23 &&
    Number(match[5]) <= 59 &&
    Number(match[6]) <= 59 &&
    (match[7] === "Z" || (Number(match[8]) <= 23 && Number(match[9]) <= 59))
  );
}
export function validateReportRange(range: {
  from: string;
  to: string;
  granularity: string;
  campaignIds?: readonly string[];
}) {
  if (!["hour", "day", "total"].includes(range.granularity))
    return c.invalidGranularity;
  if (!validTimestamp(range.from) || !validTimestamp(range.to))
    return c.invalidDates;
  const nanos = (value: string) =>
    BigInt(Math.floor(Date.parse(value) / 1000)) * BigInt(1000000000) +
    BigInt(
      (/\.(\d+)(?:Z|[+-]\d{2}:\d{2})$/.exec(value)?.[1] ?? "")
        .padEnd(9, "0")
        .slice(0, 9),
    );
  const duration = nanos(range.to) - nanos(range.from);
  if (duration <= BigInt(0)) return c.invalidOrder;
  if (
    duration >
    BigInt((range.granularity === "hour" ? 31 : 366) * dayMs) * BigInt(1000000)
  )
    return range.granularity === "hour" ? c.hourLimit : c.dayLimit;
  if ((range.campaignIds?.length ?? 0) > 100) return c.selectionLimit;
  if (
    range.campaignIds?.some(
      (id) =>
        !/^([a-f0-9]{8}-){1}[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/i.test(
          id,
        ),
    )
  )
    return c.invalidIds;
  return null;
}
export function resolveReportRange(
  value: {
    preset: string;
    from: string;
    to: string;
    granularity: string;
    campaignIds: string[];
  },
  now: Date,
) {
  if (!["today", "yesterday", "7d", "30d", "custom"].includes(value.preset))
    return { range: null, error: c.invalidPreset };
  const bounds =
    value.preset === "custom"
      ? { from: value.from, to: value.to }
      : utcPreset(value.preset as Exclude<ReportPreset, "custom">, now);
  const candidate = {
    ...bounds,
    granularity: value.granularity,
    campaignIds: value.campaignIds,
  };
  const error = validateReportRange(candidate);
  return error
    ? { range: null, error }
    : {
        range: { ...candidate, granularity: value.granularity as Granularity },
        error: null,
      };
}
export function utcInput(value: string) {
  return validTimestamp(value)
    ? new Date(value).toISOString().slice(0, 23)
    : value;
}
export function fromUtcInput(value: string) {
  return value ? `${value.length === 16 ? value + ":00" : value}Z` : "";
}
export function last24Hours(now: Date) {
  const end = Math.floor(now.getTime() / 3600000) * 3600000 + 3600000;
  return {
    from: new Date(end - dayMs).toISOString(),
    to: new Date(end).toISOString(),
  };
}
