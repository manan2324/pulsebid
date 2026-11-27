/** Nearest-rank percentiles: deterministic for small demo samples. */
export function percentile(
  values: readonly number[],
  percent: number,
): number | null {
  if (!Number.isFinite(percent) || percent < 0 || percent > 100)
    throw new RangeError("Percent must be 0..100");
  const sorted = values
    .filter((value) => Number.isFinite(value) && value >= 0)
    .sort((a, b) => a - b);
  if (!sorted.length) return null;
  return sorted[Math.max(0, Math.ceil((percent / 100) * sorted.length) - 1)]!;
}
export function latencyHistogram(values: readonly number[]) {
  const bounds = [20, 40, 60, 80, 100, 120, Infinity];
  return bounds.map((upper, index) => ({
    label: upper === Infinity ? "120+ ms" : `${index * 20}–${upper} ms`,
    count: values.filter(
      (value) => Number.isFinite(value) && value >= index * 20 && value < upper,
    ).length,
  }));
}
