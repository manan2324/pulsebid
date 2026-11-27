// @vitest-environment jsdom
import "@testing-library/jest-dom/vitest";
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { HealthPanel } from "./health-panel";
import { useHealth } from "@/hooks/use-health";
import { ApiClientError } from "@/lib/api/client";
vi.mock("@/hooks/use-health", () => ({ useHealth: vi.fn() }));
const state = {
  isPending: false,
  isError: false,
  isFetching: false,
  refetch: vi.fn(),
  data: { checked_at: "2026-11-24T00:00:00Z", services: [] },
  error: null,
};
afterEach(cleanup);
it("reserves loading rows", () => {
  vi.mocked(useHealth).mockReturnValue({
    ...state,
    isPending: true,
  } as unknown as ReturnType<typeof useHealth>);
  render(<HealthPanel />);
  expect(screen.getByRole("status")).toHaveAttribute(
    "aria-label",
    "Loading services…",
  );
});
it("explains an empty health matrix", () => {
  vi.mocked(useHealth).mockReturnValue(
    state as unknown as ReturnType<typeof useHealth>,
  );
  render(<HealthPanel />);
  expect(screen.getByText("No services configured")).toBeVisible();
});
it("shows labels and latency for service readiness", () => {
  vi.mocked(useHealth).mockReturnValue({
    ...state,
    data: {
      checked_at: state.data.checked_at,
      services: [
        {
          name: "Campaign",
          status: "healthy",
          latency_ms: 12,
          status_code: 200,
        },
      ],
    },
  } as unknown as ReturnType<typeof useHealth>);
  render(<HealthPanel />);
  expect(screen.getByText("Healthy")).toBeVisible();
  expect(screen.getByText("12 ms")).toBeVisible();
  expect(screen.getByText(/UTC/)).toBeVisible();
});
it("shows request ID and retry on failure", () => {
  vi.mocked(useHealth).mockReturnValue({
    ...state,
    isError: true,
    error: new ApiClientError(
      "unavailable",
      "Cannot connect",
      [],
      "request-123",
      503,
    ),
  } as unknown as ReturnType<typeof useHealth>);
  render(<HealthPanel />);
  expect(screen.getByRole("alert")).toHaveTextContent("request-123");
  expect(
    within(screen.getByRole("alert")).getByRole("button", {
      name: "Try again",
    }),
  ).toBeEnabled();
});

it("retains healthy rows alongside partial service failures", () => {
  vi.mocked(useHealth).mockReturnValue({
    ...state,
    data: {
      checked_at: state.data.checked_at,
      services: [
        {
          name: "Gateway",
          status: "healthy",
          latency_ms: 10,
          status_code: 200,
        },
        {
          name: "Bidder 1",
          status: "unavailable",
          latency_ms: 2000,
          status_code: null,
        },
      ],
    },
  } as unknown as ReturnType<typeof useHealth>);
  render(<HealthPanel />);
  expect(screen.getByText("Gateway")).toBeVisible();
  expect(screen.getByText("Healthy")).toBeVisible();
  expect(screen.getByText("Bidder 1")).toBeVisible();
  expect(screen.getByText("Unavailable")).toBeVisible();
});
