import { it, expect } from "vitest";
import {
  chartPoints,
  relativeDelta,
  reportCsv,
  campaignName,
  topCampaigns,
} from "./reporting";
import {
  reportFixture,
  reportRow,
  reportCampaignId,
  emptyReport,
} from "@/mocks/fixtures/reporting";
import { parseReport } from "./schemas/report";
it("combines bucket spend in integer micros and weights rates by counts", () => {
  const report = reportFixture();
  report.data = [
    {
      ...reportRow,
      impressions: 100,
      clicks: 1,
      bids: 200,
      wins: 1,
      spend: 0.100001,
    },
    {
      ...reportRow,
      campaign_id: "second",
      impressions: 1,
      clicks: 1,
      bids: 1,
      wins: 1,
      spend: 0.000001,
    },
  ];
  const points = chartPoints(report);
  expect(points).toHaveLength(24);
  const point = points.find(
    (item) => item.bucket === "2026-11-25T10:00:00.000Z",
  );
  expect(point).toMatchObject({
    impressions: 101,
    clicks: 2,
    bids: 201,
    wins: 2,
    spendMicros: 100002,
    spend: 0.100002,
    ctr: 2 / 101,
    win_rate: 2 / 201,
  });
  expect(points[0]).toMatchObject({ impressions: 0, spend: 0, ctr: 0 });
  expect(report.totals.impressions).toBe(100);
});
it("maps hour/day/total and empty contracts without fabricated samples", () => {
  for (const granularity of ["hour", "day", "total"] as const) {
    const report = parseReport(reportFixture(granularity));
    expect(chartPoints(report)).toHaveLength(
      granularity === "hour" ? 24 : granularity === "day" ? 1 : 0,
    );
  }
  expect(chartPoints(emptyReport)).toEqual([]);
  expect(parseReport(emptyReport).totals.impressions).toBe(0);
  expect(() =>
    parseReport({
      ...emptyReport,
      totals: {
        ...emptyReport.totals,
        impressions: Number.MAX_SAFE_INTEGER + 1,
      },
    }),
  ).toThrow();
});
it("handles zero baselines, increases, decreases and invalid comparison values", () => {
  expect(relativeDelta(0, 0)).toBe(0);
  expect(relativeDelta(1, 0)).toBeNull();
  expect(relativeDelta(150, 100)).toBe(0.5);
  expect(relativeDelta(50, 100)).toBe(-0.5);
  expect(relativeDelta(NaN, 2)).toBeNull();
});
it("exports a stable CSV with exact server decimals and UTC boundaries", () => {
  const report = reportFixture("total");
  report.data = [{ ...report.data[0]!, spend: 0.00101 }];
  expect(
    reportCsv(report, new Map([[reportCampaignId, 'Acme, "Shoes"']])),
  ).toBe(
    "\ufeffcampaign_id,campaign,bucket_utc,impressions,clicks,ctr,spend_usd,currency,bids,wins,win_rate,from_utc,to_utc\r\n" +
      reportCampaignId +
      ',"Acme, ""Shoes""",,100,3,0.03,0.00101,USD,200,100,0.5,2026-11-25T00:00:00Z,2026-11-26T00:00:00Z\r\n',
  );
});
it("neutralizes formula-like names, quotes newlines, and falls back to IDs", () => {
  const report = reportFixture("total");
  expect(reportCsv(report, new Map([[reportCampaignId, "=1+1"]]))).toContain(
    "'=1+1",
  );
  expect(
    reportCsv(report, new Map([[reportCampaignId, "line1\nline2"]])),
  ).toContain('"line1\nline2"');
  expect(campaignName(reportCampaignId, new Map())).toBe("Unknown (019aab00…)");
  expect(
    topCampaigns([
      { ...reportRow, spend: 1 },
      { ...reportRow, campaign_id: "b", spend: 2 },
    ])[0]?.campaign_id,
  ).toBe("b");
});
