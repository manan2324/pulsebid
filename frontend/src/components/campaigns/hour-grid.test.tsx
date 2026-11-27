// @vitest-environment jsdom
import "@testing-library/jest-dom/vitest";
import { useState } from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { HourGrid } from "./hour-grid";
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
function Harness() {
  const [hours, setHours] = useState<number[]>([]);
  return <HourGrid value={hours} onChange={setHours} />;
}
it("supports arrow keys and Space", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  screen.getByRole("button", { name: "00:00" }).focus();
  await user.keyboard("[ArrowRight] ");
  expect(screen.getByRole("button", { name: "01:00" })).toHaveFocus();
  expect(screen.getByRole("button", { name: "01:00" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
});
it("selects a contiguous dragged range", () => {
  vi.stubGlobal("PointerEvent", MouseEvent);
  render(<Harness />);
  fireEvent.pointerDown(screen.getByRole("button", { name: "02:00" }), {
    button: 0,
  });
  fireEvent.pointerEnter(screen.getByRole("button", { name: "05:00" }), {
    buttons: 1,
  });
  fireEvent.pointerUp(window);
  for (const hour of [2, 3, 4, 5])
    expect(screen.getByRole("button", { name: `0${hour}:00` })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  expect(screen.getByRole("button", { name: "01:00" })).toHaveAttribute(
    "aria-pressed",
    "false",
  );
});
it("selects business hours and clears to any hour", async () => {
  const user = userEvent.setup();
  render(<Harness />);
  await user.click(screen.getByRole("button", { name: "Business hours" }));
  expect(screen.getByRole("button", { name: "09:00" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  expect(screen.getByRole("button", { name: "16:00" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  expect(screen.getByRole("button", { name: "17:00" })).toHaveAttribute(
    "aria-pressed",
    "false",
  );
  await user.click(screen.getByRole("button", { name: "Clear" }));
  expect(screen.getByText(/Any hour/)).toBeVisible();
});
