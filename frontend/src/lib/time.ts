export type TimeMode = "UTC" | "Local";

export function formatTime(value: string | Date, mode: TimeMode = "UTC") {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  return new Intl.DateTimeFormat("en-GB", {
    year: "numeric",
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    timeZone: mode === "UTC" ? "UTC" : undefined,
    timeZoneName: "short",
    hour12: false,
  }).format(date);
}

export function utcDay(value: Date = new Date()) {
  if (Number.isNaN(value.getTime())) throw new RangeError("Invalid date");
  return value.toISOString().slice(0, 10);
}
