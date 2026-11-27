import type { Advertiser, Bidder } from "@/lib/api/types";
export const advertiserFixture: Advertiser = {
  id: "019aab00-0000-7000-8000-000000000001",
  name: "Acme Shoes",
  contact_email: "ads@acme.test",
  status: "active",
  created_at: "2026-11-24T00:00:00Z",
  updated_at: "2026-11-24T00:00:00Z",
};
export const bidderFixture: Bidder = {
  id: "test-dsp",
  name: "Test DSP",
  endpoint_url: "http://localhost:9101/bid",
  timeout_ms: null,
  enabled: true,
  created_at: "2026-11-24T00:00:00Z",
  updated_at: "2026-11-24T00:00:00Z",
};
