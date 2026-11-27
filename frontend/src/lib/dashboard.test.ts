import { it, expect } from "vitest";
import { safeBudget, budgetPercent } from "./dashboard";
import type { BudgetView } from "./api/types";
it("detects the 80% and exhaustion thresholds using server micros", () => {
  const budget = {
    daily_budget_micros: 1000000,
    spent_micros: 800000,
    remaining_micros: 200000,
  } as BudgetView;
  expect(safeBudget(budget)).toBe(true);
  expect(budgetPercent(budget)).toBe(80);
  expect(budgetPercent({ ...budget, spent_micros: 1000000 })).toBe(100);
  expect(safeBudget({ ...budget, daily_budget_micros: 0 })).toBe(false);
  expect(safeBudget({ ...budget, spent_micros: NaN })).toBe(false);
});
