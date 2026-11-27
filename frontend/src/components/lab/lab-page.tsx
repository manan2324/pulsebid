"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import dynamic from "next/dynamic";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { ErrorState } from "@/components/data/shared";
import { ApiClientError } from "@/lib/api/client";
import { labApi } from "@/lib/api/lab";
import { runBurst } from "@/lib/burst";
import { parseBidJson, presetRequest } from "@/lib/schemas/bid";
import {
  defaultBidderConfig,
  presetConfig,
  presets,
  restoreBeacon,
  type BidderConfig,
  type LabPreset,
} from "@/lib/lab";
import { labCopy as c } from "@/lib/copy/lab";
import { BidderCard } from "./bidder-card";
import type { Comparison } from "./scenario-results";
const Results = dynamic(
  () => import("./scenario-results").then((m) => m.ScenarioResults),
  { ssr: false },
);
const Metrics = dynamic(
  () => import("./lab-metrics").then((m) => m.LabMetrics),
  { ssr: false },
);
export function LabPage({
  count,
  basePort,
}: {
  count: number;
  basePort: number;
}) {
  const client = useQueryClient();
  const [selected, setSelected] = useState<number[]>(() =>
    Array.from({ length: count }, (_, i) => i),
  );
  const [preset, setPreset] = useState<LabPreset>("slow"),
    [busy, setBusy] = useState(false),
    [stage, setStage] = useState<string>(c.idle);
  const [request, setRequest] = useState(() =>
    JSON.stringify(presetRequest("mobile"), null, 2),
  );
  const [size, setSize] = useState(60),
    [concurrency, setConcurrency] = useState(6),
    [resetAfter, setResetAfter] = useState(true);
  const [dirty, setDirty] = useState(false),
    [error, setError] = useState<Error | null>(null),
    [result, setResult] = useState<Comparison>();
  const [confirm, setConfirm] = useState<"apply" | "run" | null>(null);
  const original = useRef(new Map<number, BidderConfig>()),
    controller = useRef<AbortController | null>(null),
    mounted = useRef(true),
    resetRequested = useRef(false);
  const remember = useCallback((index: number, config: BidderConfig) => {
    if (!original.current.has(index)) original.current.set(index, config);
    setDirty(true);
  }, []);
  const restore = useCallback(
    async (defaults = false) => {
      const entries = defaults
        ? Array.from(
            { length: count },
            (_, index) => [index, defaultBidderConfig] as const,
          )
        : [...original.current];
      const outcomes = await Promise.allSettled(
        entries.map(async ([index, config]) => {
          const value = await labApi.update(index, config);
          client.setQueryData(["lab-config", index], value);
          original.current.delete(index);
        }),
      );
      if (mounted.current) setDirty(original.current.size > 0);
      const failure = outcomes.find((item) => item.status === "rejected");
      if (failure?.status === "rejected") {
        const error = failure.reason;
        if (error instanceof ApiClientError)
          throw new ApiClientError(
            error.code,
            c.resetFailed,
            error.details,
            error.requestId,
            error.status,
          );
        throw new Error(c.resetFailed);
      }
    },
    [client, count],
  );
  useEffect(() => {
    mounted.current = true;
    const beacon = () => {
      restoreBeacon(
        [...original.current].map(([index, config]) => ({ index, config })),
      );
    };
    const before = (event: BeforeUnloadEvent) => {
      if (!original.current.size) return;
      beacon();
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", before);
    window.addEventListener("pagehide", beacon);
    return () => {
      mounted.current = false;
      controller.current?.abort();
      beacon();
      window.removeEventListener("beforeunload", before);
      window.removeEventListener("pagehide", beacon);
    };
  }, []);
  const apply = async (choice: LabPreset) => {
    if (!selected.length) throw new Error(c.selectionError);
    const configs = await Promise.all(
      selected.map(
        async (index) => [index, await labApi.config(index)] as const,
      ),
    );
    for (const [index, config] of configs) remember(index, config);
    const outcomes = await Promise.allSettled(
      selected.map(async (index, position) => {
        const value = await labApi.update(
          index,
          presetConfig(choice, position),
        );
        client.setQueryData(["lab-config", index], value);
      }),
    );
    const failure = outcomes.find((item) => item.status === "rejected");
    if (failure?.status === "rejected") throw failure.reason;
  };
  const execute = async (action: "apply" | "run") => {
    setError(null);
    setBusy(true);
    resetRequested.current = false;
    const abort = new AbortController();
    controller.current = abort;
    let successful = false;
    try {
      if (action === "apply") {
        await apply(preset);
        toast.success(c.saved);
        successful = true;
        return;
      }
      const bid = parseBidJson(request);
      if (
        !Number.isInteger(size) ||
        size < 1 ||
        size > 200 ||
        !Number.isInteger(concurrency) ||
        concurrency < 1 ||
        concurrency > 10
      )
        throw new Error(c.requestError);
      setStage(c.baseline);
      await apply("healthy");
      if (abort.signal.aborted) return;
      const options = {
        signal: abort.signal,
        onProgress: (done: number) => {
          if (mounted.current) setStage(`${c.progress}: ${done}/${size}`);
        },
      };
      const baseline = await runBurst(
        bid,
        { count: size, concurrency, mode: "fixed" },
        options,
      );
      if (abort.signal.aborted) return;
      setStage(c.scenario);
      await apply(preset);
      const scenario = await runBurst(
        bid,
        { count: size, concurrency, mode: "fixed" },
        options,
      );
      if (mounted.current)
        setResult({
          at: new Date().toISOString(),
          preset,
          bidders: [...selected],
          baseline,
          scenario,
        });
      successful = !abort.signal.aborted;
    } catch (failure) {
      if (mounted.current)
        setError(failure instanceof Error ? failure : new Error(c.error));
    } finally {
      if (
        (action === "run" && resetAfter) ||
        !successful ||
        abort.signal.aborted
      ) {
        if (mounted.current) setStage(c.restoring);
        try {
          await restore(resetRequested.current);
        } catch (failure) {
          if (mounted.current)
            setError(
              failure instanceof Error ? failure : new Error(c.resetFailed),
            );
        }
      }
      if (mounted.current) {
        setBusy(false);
        setStage(abort.signal.aborted ? c.cancelled : c.idle);
      }
      controller.current = null;
    }
  };
  const begin = (action: "apply" | "run") => {
    if (preset === "allError" || preset === "allHang") setConfirm(action);
    else void execute(action);
  };
  const reset = async (defaults: boolean) => {
    if (busy) {
      resetRequested.current = defaults;
      controller.current?.abort();
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await restore(defaults);
      toast.success(c.restored);
    } catch (failure) {
      setError(failure instanceof Error ? failure : new Error(c.resetFailed));
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="lab-workspace">
      <PageHeader title={c.title} description={c.description} />
      <p className="notice">{c.safety}</p>
      <div className="lab-safety">
        <Button variant="destructive" onClick={() => void reset(true)}>
          {c.reset}
        </Button>
        <small>{c.resetHelp}</small>
        {dirty && (
          <>
            <p role="status">{c.changed}</p>
            <Button
              variant="outline"
              disabled={busy}
              onClick={() => void reset(false)}
            >
              {c.restore}
            </Button>
          </>
        )}
      </div>
      {error && (
        <ErrorState
          message={error.message}
          requestId={
            error instanceof ApiClientError ? error.requestId : undefined
          }
        />
      )}
      <section className="panel lab-card">
        <h2>{c.runner}</h2>
        <p className="field-help">{c.requestHelp}</p>
        <fieldset disabled={busy}>
          <div className="report-filter-grid">
            <label>
              {c.preset}
              <select
                className="form-input"
                value={preset}
                onChange={(event) => setPreset(event.target.value as LabPreset)}
              >
                {presets.map((value) => (
                  <option value={value} key={value}>
                    {c[value]}
                  </option>
                ))}
              </select>
            </label>
            <label>
              {c.count}
              <input
                className="form-input"
                type="number"
                min={1}
                max={200}
                value={size}
                onChange={(event) => setSize(Number(event.target.value))}
              />
            </label>
            <label>
              {c.concurrency}
              <input
                className="form-input"
                type="number"
                min={1}
                max={10}
                value={concurrency}
                onChange={(event) => setConcurrency(Number(event.target.value))}
              />
            </label>
          </div>
          <label className="lab-request">
            {c.request}
            <textarea
              className="form-input font-mono"
              rows={10}
              value={request}
              onChange={(event) => setRequest(event.target.value)}
            />
          </label>
          <label>
            <input
              type="checkbox"
              checked={resetAfter}
              onChange={(event) => setResetAfter(event.target.checked)}
            />{" "}
            {c.resetAfter}
          </label>
        </fieldset>
        <p>
          {c.selected}: {selected.map((i) => i + 1).join(", ") || c.noSelection}
        </p>
        <div className="report-actions">
          <Button
            disabled={busy || !selected.length}
            variant="outline"
            onClick={() => begin("apply")}
          >
            {c.apply}
          </Button>
          <Button
            disabled={busy || !selected.length}
            onClick={() => begin("run")}
          >
            {c.run}
          </Button>
          {busy && (
            <Button
              variant="outline"
              onClick={() => controller.current?.abort()}
            >
              {c.stop}
            </Button>
          )}
        </div>
        <p role="status">
          {c.phase}: {stage}
        </p>
      </section>
      {result && <Results result={result} />}
      <div className="lab-grid">
        {Array.from({ length: count }, (_, index) => (
          <BidderCard
            key={index}
            index={index}
            port={basePort + index}
            selected={selected.includes(index)}
            locked={busy}
            remember={remember}
            onSelect={() =>
              setSelected((values) =>
                values.includes(index)
                  ? values.filter((value) => value !== index)
                  : [...values, index].sort((a, b) => a - b),
              )
            }
          />
        ))}
      </div>
      <Metrics />
      <Dialog
        open={confirm !== null}
        onOpenChange={(open) => {
          if (!open) setConfirm(null);
        }}
      >
        <DialogContent>
          <DialogTitle>{c.confirm}</DialogTitle>
          <DialogDescription>{c.confirmHelp}</DialogDescription>
          <Button variant="outline" onClick={() => setConfirm(null)}>
            {c.cancel}
          </Button>
          <Button
            variant="destructive"
            onClick={() => {
              const action = confirm!;
              setConfirm(null);
              void execute(action);
            }}
          >
            {c.confirmApply}
          </Button>
        </DialogContent>
      </Dialog>
    </div>
  );
}
