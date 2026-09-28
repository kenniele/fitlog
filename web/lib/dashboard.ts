import {
  daysBetweenISO,
  formatDuration,
  formatNumber,
  shiftISODate,
} from "@/lib/format";
import type { SeriesPoint, Settings } from "@/lib/types";

export const monthNames = [
  "Январь",
  "Февраль",
  "Март",
  "Апрель",
  "Май",
  "Июнь",
  "Июль",
  "Август",
  "Сентябрь",
  "Октябрь",
  "Ноябрь",
  "Декабрь",
];
export const heatMetrics = [
  { key: "sleep_seconds", label: "Сон", unit: "", source: "recovery" },
  {
    key: "recovery_score",
    label: "Восстановление",
    unit: "%",
    source: "recovery",
  },
  { key: "daily_strain", label: "Нагрузка", unit: "", source: "recovery" },
  { key: "calories_kcal", label: "Калории", unit: "ккал", source: "nutrition" },
  { key: "protein_g", label: "Белок", unit: "г", source: "nutrition" },
  { key: "weight_kg", label: "Вес", unit: "кг", source: "body" },
  { key: "body_fat_percent", label: "Процент жира", unit: "%", source: "body" },
] as const;
export type HeatMetricKey = (typeof heatMetrics)[number]["key"];
export function numeric(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}
export function metricText(value: unknown, key: string, unit = "") {
  const number = numeric(value);
  if (number === null) return "Без данных";
  return key.endsWith("seconds")
    ? formatDuration(number)
    : formatNumber(
        number,
        {
          maximumFractionDigits:
            key === "ecw_tbw_ratio" ? 3 : key.endsWith("_kg") ? 2 : 1,
        },
        unit ? ` ${unit}` : "",
      );
}
export function calendarDates(from: string, to: string) {
  const count = daysBetweenISO(from, to) + 1;
  if (count < 1 || count > 366) return [];
  return Array.from({ length: count }, (_, index) => shiftISODate(from, index));
}
export function monthRange(year: number, month: number, today: string) {
  const from = `${year}-${String(month + 1).padStart(2, "0")}-01`;
  const end = shiftISODate(
    `${month === 11 ? year + 1 : year}-${String(month === 11 ? 1 : month + 2).padStart(2, "0")}-01`,
    -1,
  );
  return { from, to: end < today ? end : today, fullTo: end };
}
export function rangeParams(from: string, to: string) {
  return new URLSearchParams({ range: "custom", from, to }).toString();
}

export type HeatScale = {
  colors: string[];
  labels: string[];
  bucket: (value: number) => number;
};
const sequential = [
  "var(--heat-1)",
  "var(--heat-2)",
  "var(--heat-3)",
  "var(--heat-4)",
];
export function heatScale(
  key: HeatMetricKey,
  settings: Settings,
  values: number[],
): HeatScale {
  if (key === "sleep_seconds") {
    const min = settings.sleep_target_min_seconds ?? 7 * 3600;
    const max = Math.max(min, settings.sleep_target_max_seconds ?? 9 * 3600);
    return {
      colors: [
        "var(--heat-low)",
        sequential[0],
        sequential[2],
        sequential[3],
        "var(--warning)",
      ],
      labels: [
        `< ${formatDuration(Math.max(0, min - 3600))}`,
        `${formatDuration(Math.max(0, min - 3600))}–${formatDuration(min)}`,
        `${formatDuration(min)}–${formatDuration((min + max) / 2)}`,
        `${formatDuration((min + max) / 2)}–${formatDuration(max)}`,
        `≥ ${formatDuration(max)}`,
      ],
      bucket: (v) =>
        v < min - 3600
          ? 0
          : v < min
            ? 1
            : v < (min + max) / 2
              ? 2
              : v < max
                ? 3
                : 4,
    };
  }
  if (key === "recovery_score") {
    const low = numeric(settings.recovery_ranges?.low) ?? 34;
    const high = numeric(settings.recovery_ranges?.high) ?? 67;
    return {
      colors: ["var(--heat-low)", "var(--warning)", sequential[3]],
      labels: [`< ${low}%`, `${low}–${high - 1}%`, `≥ ${high}%`],
      bucket: (v) => (v < low ? 0 : v < high ? 1 : 2),
    };
  }
  const target =
    key === "calories_kcal"
      ? settings.calorie_target_kcal
      : key === "protein_g"
        ? settings.protein_target_g
        : null;
  if (target && target > 0) {
    return {
      colors: [sequential[0], sequential[1], sequential[3], "var(--warning)"],
      labels: ["< 70% цели", "70–90%", "90–110%", "> 110%"],
      bucket: (v) =>
        v < target * 0.7 ? 0 : v < target * 0.9 ? 1 : v <= target * 1.1 ? 2 : 3,
    };
  }
  const min = key === "daily_strain" ? 0 : Math.min(...values, 0);
  const max = key === "daily_strain" ? 21 : Math.max(...values, 1);
  // Weight and body-fat scales describe magnitude, not a health judgement.
  const lower =
    key === "weight_kg" || key === "body_fat_percent"
      ? Math.min(...values, max)
      : min;
  return {
    colors: sequential,
    labels: Array.from({ length: 4 }, (_, i) => {
      const start = lower + ((max - lower) * i) / 4;
      const end = lower + ((max - lower) * (i + 1)) / 4;
      const unit = heatMetrics.find((metric) => metric.key === key)?.unit;
      return `${formatNumber(start)}–${formatNumber(end)}${unit ? ` ${unit}` : ""}`;
    }),
    bucket: (v) =>
      Math.min(
        3,
        Math.max(0, Math.floor(((v - lower) / (max - lower || 1)) * 4)),
      ),
  };
}
export function observedStats(points: SeriesPoint[], key: string) {
  const measured = points
    .filter((p) => numeric(p[key]) !== null)
    .sort((a, b) => a.date.localeCompare(b.date));
  const values = measured.map((p) => p[key] as number);
  const sum = values.length ? values.reduce((a, b) => a + b, 0) : null;
  return {
    count: values.length,
    sum,
    average: sum === null ? null : sum / values.length,
    first: values[0] ?? null,
    last: values.at(-1) ?? null,
    change: values.length >= 2 ? values.at(-1)! - values[0] : null,
  };
}
