export function formatNumber(value: number, maximumFractionDigits = 0) {
  return Number.isFinite(value)
    ? new Intl.NumberFormat("en-US", { maximumFractionDigits }).format(value)
    : "—";
}

export function formatPercent(value: number, maximumFractionDigits = 2) {
  return Number.isFinite(value)
    ? new Intl.NumberFormat("en-US", {
        style: "percent",
        maximumFractionDigits,
      }).format(value)
    : "—";
}
