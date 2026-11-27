// @vitest-environment jsdom
import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, fireEvent } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, it, expect, vi } from "vitest";
import { ReportFilters } from "./report-filters";
import { reportCampaignId } from "@/mocks/fixtures/reporting";
afterEach(cleanup);
it("disables invalid hourly ranges and permits correction without a request", () => {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const apply = vi.fn();
  render(
    <QueryClientProvider client={client}>
      <ReportFilters
        fixedCampaignId={reportCampaignId}
        now={new Date("2026-11-25T12:00:00Z")}
        initial={{
          preset: "custom",
          from: "2026-10-01T00:00:00Z",
          to: "2026-11-25T00:00:00Z",
          granularity: "hour",
          campaignIds: [reportCampaignId],
        }}
        onApply={apply}
      />
    </QueryClientProvider>,
  );
  expect(screen.getByRole("button", { name: "Apply filters" })).toBeDisabled();
  expect(screen.getByRole("option", { name: "Hour" })).toBeDisabled();
  fireEvent.change(screen.getByRole("combobox", { name: "Granularity" }), {
    target: { value: "day" },
  });
  expect(screen.getByRole("button", { name: "Apply filters" })).toBeEnabled();
  fireEvent.click(screen.getByRole("button", { name: "Apply filters" }));
  expect(apply).toHaveBeenCalledWith(
    expect.objectContaining({
      granularity: "day",
      campaignIds: [reportCampaignId],
    }),
  );
  client.clear();
});
