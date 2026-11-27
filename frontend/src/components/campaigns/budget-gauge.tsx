"use client";
import { useBudget } from "@/hooks/use-campaigns";
import { MoneyText, ErrorState } from "@/components/data/shared";
import { Skeleton } from "@/components/ui/skeleton";
import { ApiClientError } from "@/lib/api/client";
import { campaignCopy as c } from "@/lib/copy/campaign";
export function BudgetGauge({
  id,
  compact = false,
}: {
  id: string;
  compact?: boolean;
}) {
  const query = useBudget(id);
  if (query.isPending)
    return <Skeleton className={compact ? "h-8 w-28" : "h-28 w-full"} />;
  if (query.isError)
    return (
      <ErrorState
        message={c.budgetUnavailable}
        requestId={
          query.error instanceof ApiClientError
            ? query.error.requestId
            : undefined
        }
        retry={() => {
          void query.refetch();
        }}
      />
    );
  const budget = query.data;
  const percent =
    budget.daily_budget_micros > 0
      ? Math.max(
          0,
          Math.min(
            100,
            (budget.spent_micros / budget.daily_budget_micros) * 100,
          ),
        )
      : 0;
  return (
    <div className={`budget-gauge ${compact ? "compact-budget" : ""}`}>
      <div>
        <MoneyText micros={budget.spent_micros} />
        <span>
          {" "}
          / <MoneyText micros={budget.daily_budget_micros} />
        </span>
      </div>
      <div
        className={`budget-track ${percent >= 100 ? "exhausted" : percent >= 80 ? "near-limit" : ""}`}
        role="progressbar"
        aria-label={c.budgetGauge}
        aria-valuenow={Math.round(percent)}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        <span style={{ width: `${percent}%` }} />
      </div>
      {!compact && (
        <div className="budget-details">
          <span>
            {c.remaining}: <MoneyText micros={budget.remaining_micros} />
          </span>
          <code>{budget.date} UTC</code>
        </div>
      )}
    </div>
  );
}
