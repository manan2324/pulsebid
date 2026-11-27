"use client";
import { useEffect, useRef, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { fireEvent, sendBid, type AuctionResult } from "@/lib/api/gateway";
import { runBurst, type BurstConfig, type BurstResult } from "@/lib/burst";
import {
  auctionHistorySchema,
  historyKey,
  historyEntry,
  appendHistory,
  type AuctionHistoryEntry,
} from "@/lib/auction-history";
import type { BidRequest } from "@/lib/schemas/bid";
import { simulatorCopy as c } from "@/lib/copy/simulator";
export function useSimulator() {
  const [history, setHistory] = useState<AuctionHistoryEntry[]>([]);
  const historyRef = useRef<AuctionHistoryEntry[]>([]);
  const [storageError, setStorageError] = useState(false);
  const [result, setResult] = useState<AuctionResult | null>(null);
  const [running, setRunning] = useState(false);
  const [progress, setProgress] = useState({ completed: 0, started: 0 });
  const [burst, setBurst] = useState<BurstResult | null>(null);
  const [burstError, setBurstError] = useState("");
  const controller = useRef<AbortController | null>(null);
  const busy = useRef(false);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    try {
      const raw = localStorage.getItem(historyKey);
      if (raw) {
        const saved = auctionHistorySchema.parse(JSON.parse(raw));
        historyRef.current = saved;
        setHistory(saved);
      }
    } catch {
      setStorageError(true);
    }
    return () => {
      mounted.current = false;
      controller.current?.abort();
    };
  }, []);
  const save = (entries: AuctionHistoryEntry[]) => {
    if (!mounted.current) return;
    const next = appendHistory(historyRef.current, entries);
    historyRef.current = next;
    setHistory(next);
    try {
      localStorage.setItem(historyKey, JSON.stringify(next));
    } catch {
      setStorageError(true);
    }
  };
  const single = useMutation({
    retry: false,
    mutationFn: async ({
      request,
      testTime,
    }: {
      request: BidRequest;
      testTime?: string;
    }) => {
      if (busy.current) return;
      busy.current = true;
      controller.current = new AbortController();
      try {
        const current = await sendBid(request, {
          signal: controller.current.signal,
          testTime,
        });
        if (mounted.current) {
          setResult(current);
          save([historyEntry(current)]);
        }
        return current;
      } catch (error) {
        if (!controller.current.signal.aborted) throw error;
      } finally {
        busy.current = false;
        controller.current = null;
      }
    },
  });
  const startBurst = async (
    request: BidRequest,
    config: BurstConfig,
    testTime?: string,
  ) => {
    if (busy.current) return;
    busy.current = true;
    const active = new AbortController();
    controller.current = active;
    setRunning(true);
    setBurst(null);
    setBurstError("");
    setProgress({ completed: 0, started: 0 });
    const entries: AuctionHistoryEntry[] = [];
    try {
      const summary = await runBurst(request, config, {
        signal: active.signal,
        testTime,
        onResult: (value) => entries.push(historyEntry(value)),
        onProgress: (completed, started) => {
          if (mounted.current) setProgress({ completed, started });
        },
      });
      if (mounted.current) {
        setBurst(summary);
        toast.success(summary.cancelled ? c.burstCancelled : c.burstFinished);
      }
    } catch (error) {
      if (mounted.current)
        setBurstError(error instanceof Error ? error.message : c.networkError);
    } finally {
      save(entries);
      busy.current = false;
      controller.current = null;
      if (mounted.current) setRunning(false);
    }
  };
  return {
    history,
    storageError,
    result,
    sending: single.isPending,
    running,
    progress,
    burst,
    burstError,
    send: (request: BidRequest, testTime?: string) =>
      single.mutate({ request, testTime }),
    startBurst,
    cancel: () => controller.current?.abort(),
    clear: () => {
      historyRef.current = [];
      setHistory([]);
      try {
        localStorage.removeItem(historyKey);
      } catch {
        setStorageError(true);
      }
    },
  };
}
export function useGatewayEvent(
  kind: "impression" | "click",
  url: string,
  testTime?: string,
) {
  return useMutation({
    retry: false,
    mutationFn: () => fireEvent(kind, url, { testTime }),
    onSuccess: () => toast.success(c.eventSaved),
  });
}
