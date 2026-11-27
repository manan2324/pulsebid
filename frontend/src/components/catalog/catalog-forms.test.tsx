// @vitest-environment jsdom
import "@testing-library/jest-dom/vitest";
import {
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { AdvertiserForm, BidderForm } from "./catalog-forms";
import { ConfirmAction } from "./confirm-action";
import { ApiClientError } from "@/lib/api/client";
import { advertiserFixture, bidderFixture } from "@/mocks/fixtures/catalog";
afterEach(cleanup);
it("requires an advertiser name and validates optional email", async () => {
  const user = userEvent.setup();
  const save = vi.fn();
  render(<AdvertiserForm onSave={save} onClose={vi.fn()} />);
  await user.click(screen.getByRole("button", { name: "Create advertiser" }));
  expect(
    await screen.findByText("Enter a name containing 1–200 characters."),
  ).toBeVisible();
  expect(save).not.toHaveBeenCalled();
  await user.type(screen.getByLabelText("Name"), "Acme");
  await user.type(screen.getByLabelText("Contact email"), "invalid");
  await user.click(screen.getByRole("button", { name: "Create advertiser" }));
  expect(
    await screen.findByText("Enter a valid email address or leave it blank."),
  ).toBeVisible();
});
it("submits blank email as null", async () => {
  const user = userEvent.setup();
  const save = vi.fn().mockResolvedValue(advertiserFixture);
  const close = vi.fn();
  render(<AdvertiserForm onSave={save} onClose={close} />);
  await user.type(screen.getByLabelText("Name"), "Acme");
  await user.click(screen.getByRole("button", { name: "Create advertiser" }));
  await waitFor(() =>
    expect(save).toHaveBeenCalledWith({ name: "Acme", contact_email: null }),
  );
  expect(close).toHaveBeenCalled();
});
it("maps server field errors and keeps request ID visible", async () => {
  const user = userEvent.setup();
  const save = vi.fn().mockRejectedValue(
    new ApiClientError(
      "invalid_request",
      "Validation failed",
      [
        { field: "name", issue: "Name rejected by server" },
        { field: "body", issue: "Unknown field" },
      ],
      "req-field-1",
      400,
    ),
  );
  render(
    <AdvertiserForm
      advertiser={advertiserFixture}
      onSave={save}
      onClose={vi.fn()}
    />,
  );
  await user.type(screen.getByLabelText("Name"), " Updated");
  await user.click(screen.getByRole("button", { name: "Save changes" }));
  expect(await screen.findByText("Name rejected by server")).toBeVisible();
  expect(screen.getByLabelText("Name")).toHaveAttribute("aria-invalid", "true");
  expect(screen.getByText(/req-field-1/)).toBeVisible();
});
it("surfaces an advertiser name conflict", async () => {
  const user = userEvent.setup();
  render(
    <AdvertiserForm
      onSave={vi
        .fn()
        .mockRejectedValue(
          new ApiClientError(
            "conflict",
            "An active advertiser already uses this name",
            [],
            "conflict-1",
            409,
          ),
        )}
      onClose={vi.fn()}
    />,
  );
  await user.type(screen.getByLabelText("Name"), "Acme");
  await user.click(screen.getByRole("button", { name: "Create advertiser" }));
  expect(
    await screen.findByText(/An active advertiser already uses this name/),
  ).toBeVisible();
});
it("validates bidder ID, endpoint and timeout", async () => {
  const user = userEvent.setup();
  const save = vi.fn();
  render(<BidderForm onSave={save} onClose={vi.fn()} />);
  await user.type(screen.getByLabelText("Bidder ID"), "Bad_ID");
  await user.type(screen.getByLabelText("Name"), "DSP");
  await user.type(screen.getByLabelText("Endpoint URL"), "ftp://bad.test");
  await user.type(screen.getByLabelText("Timeout (ms)"), "9");
  await user.click(screen.getByRole("button", { name: "Create bidder" }));
  expect(
    await screen.findByText("Use a lowercase bidder slug of 2–63 characters."),
  ).toBeVisible();
  expect(
    screen.getByText("Enter an absolute HTTP(S) URL without credentials."),
  ).toBeVisible();
  expect(
    screen.getByText(
      "Enter a whole number from 10 to 1000, or leave it blank.",
    ),
  ).toBeVisible();
  expect(save).not.toHaveBeenCalled();
});
it("sends only changed bidder fields and cannot change ID", async () => {
  const user = userEvent.setup();
  const save = vi.fn().mockResolvedValue(bidderFixture);
  render(<BidderForm bidder={bidderFixture} onSave={save} onClose={vi.fn()} />);
  expect(screen.getByLabelText("Bidder ID")).toHaveAttribute("readonly");
  await user.type(screen.getByLabelText("Timeout (ms)"), "100");
  await user.click(screen.getByRole("button", { name: "Save changes" }));
  await waitFor(() => expect(save).toHaveBeenCalledWith({ timeout_ms: 100 }));
});
it("asks before discarding edited fields", async () => {
  const user = userEvent.setup();
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
  const close = vi.fn();
  render(<AdvertiserForm onSave={vi.fn()} onClose={close} />);
  await user.type(screen.getByLabelText("Name"), "Unsaved");
  await user.click(screen.getByRole("button", { name: "Cancel" }));
  expect(confirm).toHaveBeenCalled();
  expect(close).not.toHaveBeenCalled();
});
it("keeps a failed bidder deletion open with the 409 message", async () => {
  const user = userEvent.setup();
  const close = vi.fn();
  render(
    <ConfirmAction
      kind="delete"
      name="DSP in use"
      onClose={close}
      onConfirm={vi
        .fn()
        .mockRejectedValue(
          new ApiClientError(
            "conflict",
            "Bidder is in use by 3 campaigns",
            [],
            "delete-409",
            409,
          ),
        )}
    />,
  );
  await user.click(
    within(screen.getByRole("dialog")).getByRole("button", {
      name: "Delete",
    }),
  );
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "Bidder is in use by 3 campaigns",
  );
  expect(screen.getByRole("alert")).toHaveTextContent("delete-409");
  expect(close).not.toHaveBeenCalled();
});
