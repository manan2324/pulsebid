// @vitest-environment jsdom
import "@testing-library/jest-dom/vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, expect, it, vi } from "vitest";
import { defaultBidderConfig } from "@/lib/lab";
import { labApi } from "@/lib/api/lab";
import { runBurst } from "@/lib/burst";
import { LabPage } from "./lab-page";
vi.mock("next/dynamic", () => ({ default: () => () => null }));
vi.mock("./bidder-card", () => ({ BidderCard: () => null }));
vi.mock("@/lib/api/lab", () => ({
  labApi: { config: vi.fn(), update: vi.fn() },
}));
vi.mock("@/lib/burst", () => ({ runBurst: vi.fn() }));
afterEach(cleanup);
function setup() {
  vi.mocked(labApi.config).mockResolvedValue({
    ...defaultBidderConfig,
    latency_ms: 37,
  });
  vi.mocked(labApi.update).mockImplementation(async (_index, config) => ({
    ...defaultBidderConfig,
    ...config,
  }));
  render(
    <QueryClientProvider client={new QueryClient()}>
      <LabPage count={2} basePort={9101} />
    </QueryClientProvider>,
  );
}
it("restores every captured bidder if the baseline burst fails", async () => {
  vi.mocked(runBurst).mockRejectedValueOnce(new Error("burst failed"));
  setup();
  fireEvent.click(screen.getByRole("button", { name: "Run comparison" }));
  await waitFor(() =>
    expect(screen.getByRole("alert")).toHaveTextContent("burst failed"),
  );
  await waitFor(() =>
    expect(labApi.update).toHaveBeenCalledWith(1, {
      ...defaultBidderConfig,
      latency_ms: 37,
    }),
  );
  expect(labApi.update).toHaveBeenCalledWith(0, {
    ...defaultBidderConfig,
    latency_ms: 37,
  });
});
it("cancel aborts in-flight work and restores original settings", async () => {
  vi.mocked(runBurst).mockImplementationOnce(
    (_request, _config, options) =>
      new Promise((_resolve, reject) =>
        options.signal.addEventListener("abort", () =>
          reject(new DOMException("Aborted", "AbortError")),
        ),
      ),
  );
  setup();
  fireEvent.click(screen.getByRole("button", { name: "Run comparison" }));
  await waitFor(() => expect(runBurst).toHaveBeenCalled());
  fireEvent.click(screen.getByRole("button", { name: "Cancel run" }));
  await waitFor(() =>
    expect(
      screen.getByRole("button", { name: "Run comparison" }),
    ).toBeEnabled(),
  );
  expect(labApi.update).toHaveBeenCalledWith(0, {
    ...defaultBidderConfig,
    latency_ms: 37,
  });
  expect(labApi.update).toHaveBeenCalledWith(1, {
    ...defaultBidderConfig,
    latency_ms: 37,
  });
});
