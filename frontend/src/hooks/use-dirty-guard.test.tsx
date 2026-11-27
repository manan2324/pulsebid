// @vitest-environment jsdom
import { cleanup, fireEvent, renderHook } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { useDirtyGuard } from "./use-dirty-guard";
afterEach(() => {
  cleanup();
  document.body.replaceChildren();
});
it("blocks a sidebar navigation when unsaved changes are declined", () => {
  vi.spyOn(window, "confirm").mockReturnValue(false);
  renderHook(() => useDirtyGuard(true));
  const link = document.createElement("a");
  link.href = "/campaigns";
  document.body.append(link);
  expect(fireEvent.click(link)).toBe(false);
  expect(window.confirm).toHaveBeenCalledOnce();
});
it("successful completion can bypass the unload guard", () => {
  const { result } = renderHook(() => useDirtyGuard(true));
  const before = new Event("beforeunload", { cancelable: true });
  window.dispatchEvent(before);
  expect(before.defaultPrevented).toBe(true);
  result.current();
  const after = new Event("beforeunload", { cancelable: true });
  window.dispatchEvent(after);
  expect(after.defaultPrevented).toBe(false);
});
