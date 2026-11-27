import { Suspense } from "react";
import { ReportsPage } from "@/components/reports/reports-page";
import { Skeleton } from "@/components/ui/skeleton";
export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full" />}>
      <ReportsPage />
    </Suspense>
  );
}
