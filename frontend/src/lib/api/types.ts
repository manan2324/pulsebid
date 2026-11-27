import type { components as CampaignComponents } from "./generated/campaign-service";
import type { components as ReportingComponents } from "./generated/reporting";
export type Advertiser = CampaignComponents["schemas"]["Advertiser"];
export type Bidder = CampaignComponents["schemas"]["Bidder"];
export type Campaign = CampaignComponents["schemas"]["Campaign"];
export type Creative = CampaignComponents["schemas"]["Creative"];
export type Targeting = CampaignComponents["schemas"]["Targeting"];
export type BudgetView = CampaignComponents["schemas"]["BudgetView"];
export type ReportRow = ReportingComponents["schemas"]["ReportRow"];
export type ReportResponse = ReportingComponents["schemas"]["ReportResponse"];
export interface Page<T> {
  data: T[];
  next_cursor: string | null;
}
