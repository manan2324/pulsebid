// @vitest-environment jsdom
import "@testing-library/jest-dom/vitest";
import { useState } from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it } from "vitest";
import { TargetingEditor } from "./targeting-editor";
import { emptyTargeting, type TargetingValues } from "@/lib/schemas/targeting";
afterEach(cleanup);
function Harness() {
  const [value, setValue] = useState<TargetingValues>(emptyTargeting);
  return (
    <>
      <TargetingEditor value={value} onChange={setValue} />
      <output data-testid="value">{JSON.stringify(value)}</output>
    </>
  );
}
it("preserves empty arrays as any and trims/deduplicates placement tags", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  expect(screen.getByTestId("value")).toHaveTextContent('"countries":[]');
  expect(screen.getByText("Any country")).toBeVisible();
  fireEvent.change(screen.getByLabelText("Placement IDs"), {
    target: { value: " home-top,home-top, footer " },
  });
  await user.click(screen.getByRole("button", { name: "Add placement" }));
  expect(screen.getByTestId("value")).toHaveTextContent(
    '"placement_ids":["home-top","footer"]',
  );
  fireEvent.change(screen.getByLabelText("Search countries"), {
    target: { value: "India" },
  });
  await user.click(screen.getByRole("checkbox", { name: "India IN" }));
  expect(screen.getByTestId("value")).toHaveTextContent('"countries":["IN"]');
  await user.click(screen.getByRole("checkbox", { name: "India IN" }));
  expect(screen.getByTestId("value")).toHaveTextContent('"countries":[]');
  await user.click(screen.getByRole("checkbox", { name: "mobile" }));
  expect(screen.getByTestId("value")).toHaveTextContent(
    '"device_types":["mobile"]',
  );
}, 15000);
