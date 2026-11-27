// @vitest-environment jsdom
import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, it, expect, vi } from "vitest";
const state = vi.hoisted(() => ({ mode: "error", refetch: vi.fn() }));
vi.mock("@/hooks/use-prometheus", async () => {
  const { ApiClientError } = await import("@/lib/api/client");
  return {
    usePrometheus: (_name: string, range?: string) => ({
      isPending: false,
      isError: state.mode === "error",
      error:
        state.mode === "error"
          ? new ApiClientError(
              "unavailable",
              "Metrics unavailable",
              [],
              "prom-failure-123",
              503,
            )
          : null,
      data:
        state.mode === "error"
          ? undefined
          : {
              status: "success",
              data: {
                resultType: range ? "matrix" : "vector",
                result: range
                  ? []
                  : [
                      {
                        metric: {},
                        value: [1, state.mode === "zero" ? "0" : "NaN"],
                      },
                    ],
              },
            },
      refetch: state.refetch,
    }),
  };
});
import { LiveExchange } from "./live-exchange";
beforeEach(() => {
  state.mode = "error";
  state.refetch.mockReset();
});
afterEach(cleanup);
it("keeps the operator view usable when metrics fail and retains request correlation", () => {
  render(<LiveExchange />);
  expect(
    screen.getByText("Live metrics are unavailable or have no finite samples."),
  ).toBeVisible();
  expect(screen.getByText("prom-failure-123")).toBeVisible();
  expect(screen.queryByText("Bid requests / second")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Refresh" }));
  expect(state.refetch).toHaveBeenCalledTimes(4);
});
it("hides missing data while displaying a legitimate zero", () => {
  state.mode = "missing";
  const { rerender } = render(<LiveExchange />);
  expect(screen.queryByText("Bid requests / second")).not.toBeInTheDocument();
  state.mode = "zero";
  rerender(<LiveExchange />);
  expect(screen.getByText("Bid requests / second")).toBeVisible();
  expect(
    screen.queryByText(
      "Live metrics are unavailable or have no finite samples.",
    ),
  ).not.toBeInTheDocument();
  expect(screen.getByText("0 ms")).toBeVisible();
});
