import { it, expect } from "vitest";
import { promScalar, promSeries, type PromResponse } from "./prometheus";
it("distinguishes real zero from missing, NaN and infinite Prometheus samples", () => {
  const fixture = (value: string): PromResponse => ({
    status: "success",
    data: { resultType: "vector", result: [{ metric: {}, value: [1, value] }] },
  });
  expect(promScalar(fixture("0"))).toBe(0);
  for (const value of ["NaN", "+Inf", "-Inf", "", "-1"])
    expect(promScalar(fixture(value))).toBeNull();
  expect(promScalar(undefined)).toBeNull();
});
it("maps seconds to ms for finite range samples only", () => {
  expect(
    promSeries({
      status: "success",
      data: {
        resultType: "matrix",
        result: [
          {
            metric: {},
            values: [
              [1, "0.03"],
              [2, "NaN"],
              [3, "0"],
            ],
          },
        ],
      },
    }),
  ).toEqual([
    { time: 1000, ms: 30 },
    { time: 3000, ms: 0 },
  ]);
});
