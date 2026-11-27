import { Suspense } from "react";
import { CampaignsPage } from "@/components/campaigns/campaigns-page";
import { Skeleton } from "@/components/ui/skeleton";

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full" />}>
      <CampaignsPage />
    </Suspense>
  );
}
