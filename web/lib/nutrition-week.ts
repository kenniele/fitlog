import type { SeriesPoint } from "@/lib/types";
import { startOfWeekISO, type FirstDayOfWeek } from "@/lib/week";

const weeklyKeys = ["calories_kcal", "protein_g", "fat_g", "carbohydrates_g", "fiber_g"] as const;

export function nutritionWeeklyAverages(points: SeriesPoint[], firstDayOfWeek: FirstDayOfWeek): SeriesPoint[] {
  const weeks = new Map<string, { sums: Record<string, number>; samples: Record<string, number> }>();
  points.forEach((point) => {
    const key = startOfWeekISO(point.date, firstDayOfWeek);
    const bucket = weeks.get(key) ?? { sums: {}, samples: {} };
    weeklyKeys.forEach((metric) => {
      const value = point[metric];
      if (typeof value !== "number" || !Number.isFinite(value)) return;
      bucket.sums[metric] = (bucket.sums[metric] ?? 0) + value;
      bucket.samples[metric] = (bucket.samples[metric] ?? 0) + 1;
    });
    weeks.set(key, bucket);
  });
  return Array.from(weeks, ([date, bucket]) => {
    const averages: SeriesPoint = { date };
    weeklyKeys.forEach((metric) => {
      averages[metric] = bucket.samples[metric] ? bucket.sums[metric] / bucket.samples[metric] : null;
    });
    return averages;
  });
}
