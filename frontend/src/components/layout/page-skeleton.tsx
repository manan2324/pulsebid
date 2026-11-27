import { Skeleton } from "@/components/ui/skeleton";
import { polishCopy as c } from "@/lib/copy/polish";
export function PageSkeleton() {
  return (
    <div role="status" aria-label={c.loading} className="lab-workspace">
      <Skeleton className="h-20 w-full" />
      <div className="metric-grid">
        {[0, 1, 2].map((key) => (
          <Skeleton key={key} className="h-32 w-full" />
        ))}
      </div>
      <Skeleton className="h-96 w-full" />
      <span className="sr-only">{c.loading}</span>
    </div>
  );
}
