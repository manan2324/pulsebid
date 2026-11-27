export function formatUsd(
  value: number,
  { maxFraction = 4 }: { maxFraction?: number } = {},
) {
  if (!Number.isFinite(value)) return "—";
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: Math.min(2, maxFraction),
    maximumFractionDigits: maxFraction,
  }).format(value);
}

export function microsToUsd(micros: number) {
  if (!Number.isSafeInteger(micros))
    throw new RangeError("Micros must be a safe integer");
  return micros / 1_000_000;
}

export function hasMaxDecimals(value: string, max = 6) {
  if (!Number.isInteger(max) || max < 0) return false;
  const match = /^[+-]?(?:\d+)(?:\.(\d+))?$/.exec(value);
  return Boolean(match && (match[1]?.length ?? 0) <= max);
}
