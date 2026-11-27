import { Suspense } from "react";
import { CampaignWizard } from "@/components/campaigns/campaign-wizard";
import { Skeleton } from "@/components/ui/skeleton";
import { getEnv } from "@/lib/server/env";
export default function Page() {
  const env = getEnv();
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full" />}>
      <CampaignWizard
        basePort={env.BIDDER_BASE_PORT}
        count={env.BIDDER_COUNT}
      />
    </Suspense>
  );
}
