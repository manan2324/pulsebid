// @vitest-environment jsdom
import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, it, expect } from "vitest";
import { ReportKpis } from "./report-kpis";
import { reportFixture, emptyReport } from "@/mocks/fixtures/reporting";
afterEach(cleanup);
it("displays server totals directly with exact small spend and zero-baseline comparisons", () => {
  const totals = { ...reportFixture().totals, spend: 0.00101 };
  render(<ReportKpis totals={totals} previous={emptyReport.totals} compare />);
  expect(screen.getByTestId("report-impressions")).toHaveTextContent("100");
  expect(screen.getByTestId("report-impressions")).toHaveAttribute(
    "data-value",
    "100",
  );
  expect(screen.getByTestId("report-spend")).toHaveTextContent("$0.00101");
  expect(screen.getByTestId("report-ctr")).toHaveTextContent("3%");
  expect(screen.getAllByText(/No prior baseline/).length).toBeGreaterThan(0);
});
it("keeps legitimate zero values visible", () => {
  render(<ReportKpis totals={emptyReport.totals} />);
  expect(screen.getByTestId("report-impressions")).toHaveTextContent("0");
  expect(screen.getByTestId("report-spend")).toHaveTextContent("$0.00");
});
