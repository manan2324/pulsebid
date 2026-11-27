import type { BudgetView } from "./api/types";
export function safeBudget(view: BudgetView | undefined): view is BudgetView {
  return Boolean(
    view &&
    [view.spent_micros, view.daily_budget_micros, view.remaining_micros].every(
      (value) => Number.isSafeInteger(value) && value >= 0,
    ) &&
    view.daily_budget_micros > 0,
  );
}
export function budgetPercent(view: BudgetView) {
  return (view.spent_micros / view.daily_budget_micros) * 100;
}
