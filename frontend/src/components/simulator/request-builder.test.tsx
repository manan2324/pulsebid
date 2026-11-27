// @vitest-environment jsdom
import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, it, expect, vi } from "vitest";
import { RequestBuilder } from "./request-builder";
afterEach(cleanup);
it("keeps form/JSON in sync, validates both modes and hides test-only controls", async () => {
  const user = userEvent.setup();
  const send = vi.fn();
  render(
    <RequestBuilder
      gatewayUrl="http://localhost:8080"
      testMode={false}
      sending={false}
      running={false}
      onSend={send}
      onBurst={vi.fn()}
      onCancel={vi.fn()}
    />,
  );
  expect(
    screen.queryByLabelText("Test time (RFC 3339)"),
  ).not.toBeInTheDocument();
  fireEvent.change(screen.getByLabelText("Placement ID"), {
    target: { value: "test-placement" },
  });
  await user.click(screen.getByRole("button", { name: "JSON" }));
  const editor = screen.getByLabelText("Bid request JSON");
  expect((editor as HTMLTextAreaElement).value).toContain("test-placement");
  const request = JSON.parse((editor as HTMLTextAreaElement).value);
  request.geo.country = "US";
  fireEvent.change(editor, { target: { value: JSON.stringify(request) } });
  await user.click(screen.getByRole("button", { name: "Form" }));
  expect(screen.getByLabelText("Country (ISO alpha-2)")).toHaveValue("US");
  fireEvent.change(screen.getByLabelText("Bid floor (USD CPM)"), {
    target: { value: "1.0000001" },
  });
  await user.click(screen.getByRole("button", { name: "Send bid" }));
  expect(send).not.toHaveBeenCalled();
  expect(
    screen.getAllByText(/exponent notation is not supported/).length,
  ).toBeGreaterThan(0);
  fireEvent.change(screen.getByLabelText("Bid floor (USD CPM)"), {
    target: { value: "1.000001" },
  });
  await user.click(screen.getByRole("button", { name: "Send bid" }));
  expect(send).toHaveBeenCalledWith(
    expect.objectContaining({
      imp: expect.objectContaining({
        bidfloor: "1.000001",
        placement_id: "test-placement",
      }),
    }),
    undefined,
  );
}, 15000);

it("shows and validates the clock override only in test mode", () => {
  const send = vi.fn();
  render(
    <RequestBuilder
      gatewayUrl="http://localhost:8080"
      testMode
      sending={false}
      running={false}
      onSend={send}
      onBurst={vi.fn()}
      onCancel={vi.fn()}
    />,
  );
  const clock = screen.getByLabelText("Test time (RFC 3339)");
  fireEvent.change(clock, { target: { value: "invalid-time" } });
  fireEvent.click(screen.getByRole("button", { name: "Send bid" }));
  expect(send).not.toHaveBeenCalled();
  fireEvent.change(clock, { target: { value: "2026-11-25T12:00:00Z" } });
  fireEvent.click(screen.getByRole("button", { name: "Send bid" }));
  expect(send).toHaveBeenCalledWith(expect.any(Object), "2026-11-25T12:00:00Z");
});
