import { describe, expect, it } from "vitest";
import { dailyPointMetrics } from "@/lib/metrics";

describe("dailyPointMetrics", () => {
  it("keeps missing values empty and compares today with the previous day", () => {
    const metrics = dailyPointMetrics(
      { date: "2026-08-21", recovery_score: 81, protein_g: null },
      { recovery: { recovery: { current: 81, samples: 7 } } },
      [
        { key: "recovery_score", label: "Recovery", aliases: ["recovery"] },
        { key: "protein_g", label: "Белок" },
      ],
      [
        { date: "2026-08-20", recovery_score: 76 },
        { date: "2026-08-21", recovery_score: 81 },
      ],
      { date: "2026-08-20", recovery_score: 74, protein_g: 150 },
    );

    expect(metrics.recovery_score?.value).toBe(81);
    expect(metrics.recovery_score?.delta).toBe(7);
    expect(metrics.protein_g?.value).toBeNull();
    expect(metrics.protein_g?.delta).toBeUndefined();
    expect(metrics.recovery_score?.series).toEqual([
      { date: "2026-08-20", value: 76 },
      { date: "2026-08-21", value: 81 },
    ]);
  });
});
