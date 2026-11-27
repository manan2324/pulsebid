import { Suspense } from "react";
import { CampaignDetail } from "@/components/campaigns/campaign-detail";
import { Skeleton } from "@/components/ui/skeleton";
import { getEnv } from "@/lib/server/env";
export default async function Page({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full" />}>
      <CampaignDetail id={id} testMode={getEnv().TEST_MODE_UI === "true"} />
    </Suspense>
  );
}
