import { describe, expect, it } from "vitest";
import {
  calendarDates,
  heatScale,
  metricText,
  monthRange,
  observedStats,
} from "./dashboard";

describe("calendar and observed data", () => {
  it("includes leap day without shifting local dates", () => {
    const days = calendarDates("2024-01-01", "2024-12-31");
    expect(days).toHaveLength(366);
    expect(days[59]).toBe("2024-02-29");
    expect(days.at(-1)).toBe("2024-12-31");
    expect(calendarDates("2025-01-01", "2025-12-31")).toHaveLength(365);
  });
  it("caps the current month at today and retains the real month boundary", () => {
    expect(monthRange(2026, 8, "2026-09-28")).toEqual({
      from: "2026-09-01",
      to: "2026-09-28",
      fullTo: "2026-09-30",
    });
    expect(monthRange(2024, 1, "2026-09-28").to).toBe("2024-02-29");
    expect(monthRange(2025, 11, "2026-09-28").to).toBe("2025-12-31");
  });
  it("averages recorded days only and retains a real zero", () => {
    expect(
      observedStats(
        [
          { date: "2026-09-01", value: 0 },
          { date: "2026-09-02", value: null },
          { date: "2026-09-03", value: 100 },
        ],
        "value",
      ),
    ).toEqual({
      count: 2,
      sum: 100,
      average: 50,
      first: 0,
      last: 100,
      change: 100,
    });
    expect(
      observedStats([{ date: "2026-09-01", value: null }], "value").sum,
    ).toBeNull();
    expect(
      observedStats([{ date: "2026-09-01", value: 80 }], "value").change,
    ).toBeNull();
  });
  it("uses saved sleep and recovery thresholds", () => {
    expect(
      heatScale(
        "sleep_seconds",
        {
          sleep_target_min_seconds: 8 * 3600,
          sleep_target_max_seconds: 10 * 3600,
        },
        [],
      ).bucket(7.5 * 3600),
    ).toBe(1);
    expect(
      heatScale(
        "recovery_score",
        { recovery_ranges: { low: 40, high: 75 } },
        [],
      ).bucket(70),
    ).toBe(1);
    expect(
      heatScale("calories_kcal", { calorie_target_kcal: 2000 }, []).bucket(
        2000,
      ),
    ).toBe(2);
  });
  it("does not confuse absent, invalid and zero values", () => {
    expect(metricText(null, "sleep_seconds")).toBe("Без данных");
    expect(metricText(Number.NaN, "weight_kg")).toBe("Без данных");
    expect(metricText(0, "calories_kcal", "ккал")).toBe("0 ккал");
    expect(metricText(0.001, "ecw_tbw_ratio")).toBe("0,001");
  });
});
