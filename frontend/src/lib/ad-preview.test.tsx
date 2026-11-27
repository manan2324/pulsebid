// @vitest-environment jsdom
import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";
import { AdPreview } from "@/components/campaigns/ad-preview";
import { neutralizeMarkup, previewDocument } from "./ad-preview";
afterEach(cleanup);
it("neutralizes placeholders, relative and absolute tracking URLs", () => {
  const markup =
    '<a href="{click_url}">Ad</a><img src="{impression_url}"><img src="http://localhost:8080/v1/events/impression?t=secret"><img src="/api/gateway/impression?t=secret">';
  const clean = neutralizeMarkup(markup);
  expect(clean).not.toContain("secret");
  expect(clean).not.toContain("{click_url}");
  expect(clean).not.toContain("{impression_url}");
  expect(clean).toContain('href="#"');
});
it("removes active content and blocks resources before embedding", () => {
  const clean = previewDocument(
    '<script>alert(1)</script><meta http-equiv="refresh" content="0;url=/api/gateway/click?t=x"><iframe src="/api/gateway/impression?t=x"></iframe><img src="https://external.test/pixel" onerror="alert(2)">',
  );
  expect(clean).not.toContain("<script");
  expect(clean).not.toContain("<iframe");
  expect(clean).not.toContain('http-equiv="refresh"');
  expect(clean).not.toContain("onerror");
  expect(clean).toContain("default-src 'none'");
  expect(clean).not.toContain("external.test");
});
it("renders markup only in an empty-sandbox iframe", () => {
  render(<AdPreview markup="<strong>Private ad text</strong>" />);
  const iframe = screen.getByTitle("Sandboxed ad preview");
  expect(iframe).toHaveAttribute("sandbox", "");
  expect(iframe).toHaveAttribute("referrerpolicy", "no-referrer");
  expect(iframe).toHaveAttribute(
    "srcdoc",
    expect.stringContaining("Private ad text"),
  );
  expect(screen.queryByText("Private ad text")).not.toBeInTheDocument();
});
