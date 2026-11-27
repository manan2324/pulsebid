"use client";
import dynamic from "next/dynamic";
import type { BurstResult } from "@/lib/burst";
import { scenarioVerdict, type LabPreset } from "@/lib/lab";
import { labCopy as c } from "@/lib/copy/lab";
import { formatNumber, formatPercent } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
const Histogram = dynamic(
  () =>
    import("@/components/charts/latency-histogram").then(
      (m) => m.LatencyHistogram,
    ),
  { ssr: false },
);
export type Comparison = {
  at: string;
  preset: LabPreset;
  bidders: number[];
  baseline: BurstResult;
  scenario: BurstResult;
};
export function ScenarioResults({ result }: { result: Comparison }) {
  const verdict = scenarioVerdict(result.baseline, result.scenario);
  const ms = (value: number | null) =>
    value === null ? c.unavailable : `${formatNumber(value, 1)} ms`;
  const download = (markdown: boolean) => {
    try {
      const text = markdown
        ? `# PulseBid resilience comparison\n\n${result.at}\n\nPreset: ${c[result.preset]}\n\n${verdict}\n\n| Burst | Fill rate | p50 | p95 | p99 |\n|---|---:|---:|---:|---:|\n${[result.baseline, result.scenario].map((r, i) => `| ${i ? c.scenario : c.baseline} | ${formatPercent(r.fillRate)} | ${ms(r.p50)} | ${ms(r.p95)} | ${ms(r.p99)} |`).join("\n")}\n\nA bounded demo, not a throughput benchmark.\n`
        : JSON.stringify({ ...result, verdict }, null, 2);
      const url = URL.createObjectURL(
        new Blob([text], {
          type: markdown ? "text/markdown" : "application/json",
        }),
      );
      const link = document.createElement("a");
      link.href = url;
      link.download = `pulsebid-resilience.${markdown ? "md" : "json"}`;
      link.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch {
      toast.error(c.exportFailed);
    }
  };
  return (
    <section aria-label={c.complete}>
      <h2>{c.complete}</h2>
      <p role="status" data-testid="scenario-verdict">
        {verdict}
      </p>
      <div className="report-actions">
        <Button variant="outline" onClick={() => download(false)}>
          {c.exportJson}
        </Button>
        <Button variant="outline" onClick={() => download(true)}>
          {c.exportMarkdown}
        </Button>
      </div>
      <div className="lab-comparison">
        {[result.baseline, result.scenario].map((r, index) => (
          <section className="panel lab-card" key={index}>
            <h3>{index ? c.scenario : c.baseline}</h3>
            <p>{r.cancelled ? c.cancelled : `${r.completed}/${r.requested}`}</p>
            <dl className="debug-counters">
              <div>
                <dt>{c.fill}</dt>
                <dd>{formatPercent(r.fillRate)}</dd>
              </div>
              {(["p50", "p95", "p99"] as const).map((key) => (
                <div key={key}>
                  <dt>{key}</dt>
                  <dd data-testid={`${index ? "scenario" : "baseline"}-${key}`}>
                    {ms(r[key])}
                  </dd>
                </div>
              ))}
              <div>
                <dt>{c.timing}</dt>
                <dd>
                  {r.sampleCount}/{r.completed}
                </dd>
              </div>
            </dl>
            <p>
              {c.outcomes}: {JSON.stringify(r.counts)}
            </p>
            <Histogram data={r.histogram} />
            <details>
              <summary>{c.histogram}</summary>
              <table className="simulator-simple-table">
                <thead>
                  <tr>
                    <th>{c.bucket}</th>
                    <th>{c.samples}</th>
                  </tr>
                </thead>
                <tbody>
                  {r.histogram.map((row) => (
                    <tr key={row.label}>
                      <th>{row.label}</th>
                      <td>{row.count}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </details>
          </section>
        ))}
      </div>
    </section>
  );
}
