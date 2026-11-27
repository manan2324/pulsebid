"use client";
import { useState } from "react";
import dynamic from "next/dynamic";
import { PageHeader } from "@/components/layout/page-header";
import { Skeleton } from "@/components/ui/skeleton";
import { useSimulator } from "@/hooks/use-simulator";
import type { AuctionHistoryEntry } from "@/lib/auction-history";
import { simulatorCopy as c } from "@/lib/copy/simulator";
import { RequestBuilder } from "./request-builder";
import { AuctionHistory } from "./auction-history";

const Result = dynamic(
  () => import("./auction-result").then((module) => module.AuctionResult),
  { ssr: false, loading: () => <Skeleton className="h-96 w-full" /> },
);

const Summary = dynamic(
  () => import("./burst-summary").then((module) => module.BurstSummary),
  { loading: () => <Skeleton className="h-64 w-full" /> },
);

export function SimulatorPage({
  gatewayUrl,
  testMode,
}: {
  gatewayUrl: string;
  testMode: boolean;
}) {
  const simulator = useSimulator();
  const [replay, setReplay] = useState<AuctionHistoryEntry | null>(null);
  const busy = simulator.sending || simulator.running;
  return (
    <>
      <PageHeader title={c.title} description={c.description} />
      {simulator.storageError && (
        <p className="notice" role="status">
          {c.storageError}
        </p>
      )}
      <div className="simulator-workspace">
        <RequestBuilder
          key={replay?.id ?? "initial"}
          initial={replay?.request}
          initialTime={replay?.testTime}
          gatewayUrl={gatewayUrl}
          testMode={testMode}
          sending={simulator.sending}
          running={simulator.running}
          onCancel={simulator.cancel}
          onSend={simulator.send}
          onBurst={(request, config, time) => {
            void simulator.startBurst(request, config, time);
          }}
          serverErrors={simulator.result?.error?.details}
        />
        <Result
          key={simulator.result?.id ?? "empty"}
          result={simulator.result}
          busy={busy}
        />
      </div>
      {simulator.running && (
        <p className="notice" role="status">
          {c.running} {simulator.progress.completed} {c.completed} ·{" "}
          {simulator.progress.started} {c.started}
        </p>
      )}
      {simulator.burstError && (
        <p role="alert" className="field-error">
          {simulator.burstError}
        </p>
      )}
      {simulator.burst && <Summary summary={simulator.burst} />}
      <AuctionHistory
        entries={simulator.history}
        busy={busy}
        onClear={simulator.clear}
        onReplay={(entry) => {
          setReplay(entry);
          simulator.send(entry.request, testMode ? entry.testTime : undefined);
        }}
      />
    </>
  );
}
