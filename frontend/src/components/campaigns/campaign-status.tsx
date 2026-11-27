import type { Campaign } from "@/lib/api/types";
import { campaignCopy as c } from "@/lib/copy/campaign";
export function CampaignStatus({ status }: { status: Campaign["status"] }) {
  return <span className={`campaign-status ${status}`}>{c[status]}</span>;
}
