// @vitest-environment jsdom
import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import type { ColumnDef } from "@tanstack/react-table";
import { DataTable } from "./data-table";
import { ApiClientError } from "@/lib/api/client";
type RecordRow = { id: string; name: string; status: string };
const columns: ColumnDef<RecordRow>[] = [
  { accessorKey: "name", header: "Name", enableHiding: false },
  { accessorKey: "status", header: "Status" },
];
const props = {
  columns,
  data: [
    { id: "b", name: "Beta", status: "Active" },
    { id: "a", name: "Alpha", status: "Archived" },
  ],
  loading: false,
  retry: vi.fn(),
  emptyTitle: "No advertisers yet",
  emptyDescription: "Create an advertiser",
};
afterEach(cleanup);
it("sorts loaded records and hides optional columns", async () => {
  const user = userEvent.setup();
  render(<DataTable {...props} />);
  await user.click(screen.getByRole("button", { name: "Name" }));
  expect(
    within(screen.getAllByRole("row")[1]!).getByText("Alpha"),
  ).toBeVisible();
  expect(screen.getByRole("columnheader", { name: "Name" })).toHaveAttribute(
    "aria-sort",
    "ascending",
  );
  await user.click(screen.getByText("Columns"));
  await user.click(screen.getByRole("checkbox", { name: "Status" }));
  expect(
    screen.queryByRole("columnheader", { name: "Status" }),
  ).not.toBeInTheDocument();
});
it("shows skeletons, an empty state and a retryable error", () => {
  const { rerender } = render(<DataTable {...props} loading />);
  expect(screen.getByRole("status")).toBeVisible();
  rerender(<DataTable {...props} data={[]} />);
  expect(
    screen.getByRole("heading", { name: "No advertisers yet" }),
  ).toBeVisible();
  rerender(
    <DataTable
      {...props}
      data={[]}
      error={
        new ApiClientError("unavailable", "Unavailable", [], "table-error", 503)
      }
    />,
  );
  expect(screen.getByRole("alert")).toHaveTextContent("table-error");
  expect(screen.getByRole("button", { name: "Try again" })).toBeEnabled();
});
it("renders at most 20 rows per visible page", async () => {
  const user = userEvent.setup();
  render(
    <DataTable
      {...props}
      data={Array.from({ length: 125 }, (_, index) => ({
        id: String(index),
        name: `Record ${index}`,
        status: "Active",
      }))}
    />,
  );
  expect(screen.getAllByRole("row")).toHaveLength(21);
  await user.click(screen.getByRole("button", { name: "Next" }));
  expect(screen.getAllByRole("row")).toHaveLength(21);
  expect(screen.getByText("Record 20")).toBeVisible();
  expect(screen.queryByText("Record 0")).not.toBeInTheDocument();
});
