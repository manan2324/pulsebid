"use client";
import { createContext, useContext, useEffect, useState } from "react";
import {
  MutationCache,
  QueryCache,
  QueryClient,
  QueryClientProvider,
} from "@tanstack/react-query";
import { NuqsAdapter } from "nuqs/adapters/next/app";
import { ThemeProvider } from "next-themes";
import { toast } from "sonner";
import { Toaster } from "@/components/ui/sonner";
import { ApiClientError } from "@/lib/api/client";
import type { TimeMode } from "@/lib/time";
import { copy } from "@/lib/copy/app";
const TimeContext = createContext<{ mode: TimeMode; toggle: () => void }>({
  mode: "UTC",
  toggle: () => {},
});
export const useTimeMode = () => useContext(TimeContext);
function report(error: Error) {
  toast.error(error.message, {
    description:
      error instanceof ApiClientError
        ? `${copy.requestId}: ${error.requestId}`
        : undefined,
    action:
      error instanceof ApiClientError
        ? {
            label: copy.copy,
            onClick: () => {
              void navigator.clipboard.writeText(error.requestId);
            },
          }
        : undefined,
  });
}
export function Providers({
  children,
  nonce,
}: {
  children: React.ReactNode;
  nonce?: string;
}) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        queryCache: new QueryCache({
          onError: (error, query) => {
            if (!query.meta?.quiet) report(error);
          },
        }),
        mutationCache: new MutationCache({ onError: report }),
        defaultOptions: {
          queries: {
            staleTime: 15000,
            refetchOnWindowFocus: true,
            refetchIntervalInBackground: false,
            retry: (count, error) =>
              !(
                error instanceof ApiClientError &&
                error.status >= 400 &&
                error.status < 500
              ) && count < 2,
            retryDelay: (count) => Math.min(1000 * 2 ** count, 10000),
          },
        },
      }),
  );
  const [mode, setMode] = useState<TimeMode>("UTC");
  const [ready, setReady] = useState(
    process.env.NEXT_PUBLIC_API_MOCKING !== "enabled",
  );
  useEffect(() => {
    if (process.env.NEXT_PUBLIC_API_MOCKING === "enabled")
      void import("@/mocks/browser")
        .then(({ worker }) => worker.start({ onUnhandledRequest: "bypass" }))
        .then(() => setReady(true));
  }, []);
  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="system"
      enableSystem
      nonce={nonce}
    >
      <QueryClientProvider client={queryClient}>
        <NuqsAdapter>
          <TimeContext.Provider
            value={{
              mode,
              toggle: () =>
                setMode((value) => (value === "UTC" ? "Local" : "UTC")),
            }}
          >
            {ready ? children : <p role="status">{copy.loading}</p>}
            <Toaster richColors closeButton expand />
          </TimeContext.Provider>
        </NuqsAdapter>
      </QueryClientProvider>
    </ThemeProvider>
  );
}
