"use client";
import { Button } from "@/components/ui/button";
import { MoneyText } from "@/components/data/shared";
import type { AuctionHistoryEntry } from "@/lib/auction-history";
import { useTimeMode } from "@/components/providers";
import { formatTime } from "@/lib/time";
import { formatNumber } from "@/lib/format";
import { reasonText } from "@/lib/simulator";
import { simulatorCopy as c } from "@/lib/copy/simulator";
export function AuctionHistory({
  entries,
  busy,
  onReplay,
  onClear,
}: {
  entries: AuctionHistoryEntry[];
  busy: boolean;
  onReplay: (entry: AuctionHistoryEntry) => void;
  onClear: () => void;
}) {
  const { mode } = useTimeMode();
  return (
    <section className="panel simulator-history">
      <div className="section-heading">
        <h2>{c.history}</h2>
        <Button
          variant="outline"
          disabled={!entries.length || busy}
          onClick={() => {
            if (window.confirm(c.clearConfirm)) onClear();
          }}
        >
          {c.clearHistory}
        </Button>
      </div>
      <p className="field-help">{c.historyHelp}</p>
      {!entries.length ? (
        <p>{c.historyEmpty}</p>
      ) : (
        <div className="table-scroll">
          <table className="simulator-simple-table">
            <thead>
              <tr>
                {[
                  `${c.time} (${mode})`,
                  c.outcome,
                  c.price,
                  c.gateway,
                  c.reason,
                  c.replay,
                ].map((label) => (
                  <th key={label} scope="col">
                    {label}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {entries.map((entry) => (
                <tr key={entry.id}>
                  <td>{formatTime(entry.at, mode)}</td>
                  <td>
                    {entry.status || "—"}{" "}
                    {entry.outcome === "filled"
                      ? c.filled
                      : entry.outcome === "no-fill"
                        ? c.noFill
                        : c.error}
                  </td>
                  <td>
                    {entry.price === null ? (
                      "—"
                    ) : (
                      <MoneyText
                        value={entry.price}
                        maxFraction={6}
                        unit="CPM"
                      />
                    )}
                  </td>
                  <td>
                    {entry.gatewayMs === null
                      ? c.unavailable
                      : `${formatNumber(entry.gatewayMs, 1)} ms`}
                  </td>
                  <td>
                    {entry.outcome === "no-fill"
                      ? reasonText(entry.reason)
                      : "—"}
                  </td>
                  <td>
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={busy}
                      aria-label={`${c.replay} ${entry.request.id}`}
                      onClick={() => onReplay(entry)}
                    >
                      {c.replay}
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
