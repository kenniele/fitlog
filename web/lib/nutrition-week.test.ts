import { describe, expect, it } from "vitest";
import { nutritionWeeklyAverages } from "@/lib/nutrition-week";

const boundaryPoints = [
  { date: "2026-08-16", calories_kcal: 100, protein_g: null },
  { date: "2026-08-17", calories_kcal: 300, protein_g: 120 },
];

describe("nutrition weekly averages", () => {
  it("starts separate buckets at Monday when configured", () => {
    const weeks = nutritionWeeklyAverages(boundaryPoints, 1);
    expect(weeks.map((week) => [week.date, week.calories_kcal])).toEqual([
      ["2026-08-10", 100],
      ["2026-08-17", 300],
    ]);
  });

  it("keeps Sunday and Monday in one Sunday-first bucket", () => {
    const weeks = nutritionWeeklyAverages(boundaryPoints, 7);
    expect(weeks).toHaveLength(1);
    expect(weeks[0]).toMatchObject({ date: "2026-08-16", calories_kcal: 200, protein_g: 120 });
  });
});
