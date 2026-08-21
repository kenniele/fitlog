import { formatDuration, formatNumber } from "@/lib/format";
import type { Metric, SeriesPoint } from "@/lib/types";
import { isRecord } from "@/lib/utils";

type MetricSummary = { current?: number | null; average?: number | null; minimum?: number | null; maximum?: number | null; change?: number | null; samples?: number | null };

function isMetricSummary(value: unknown): value is MetricSummary {
  return isRecord(value) && ["current", "average", "minimum", "maximum", "change", "samples"].some((key) => key in value);
}

function humanize(key: string) {
  return key.split(".").at(-1)?.replaceAll("_", " ").replace(/\b\w/g, (char) => char.toUpperCase()) ?? key;
}

function unitFor(key: string) {
  if (key.includes("inbody_score")) return undefined;
  if (key.includes("percent") || key.includes("score") || key.includes("adherence")) return "%";
  if (key.includes("hrv")) return "мс";
  if (key.includes("heart_rate") || key.includes("rhr")) return "bpm";
  if (key.includes("calorie")) return "ккал";
  if (key.includes("weight") || key.includes("mass") || key.includes("volume")) return "кг";
  if (key.endsWith("_g")) return "г";
  return undefined;
}

function collect(summary: unknown, prefix = "", result: Record<string, Metric> = {}) {
  if (!isRecord(summary)) return result;
  Object.entries(summary).forEach(([key, value]) => {
    const path = prefix ? `${prefix}.${key}` : key;
    if (isMetricSummary(value)) {
      result[path] = { label: humanize(path), value: value.current ?? value.average ?? null, unit: unitFor(path), delta: value.change, context: value.samples == null ? null : `${formatNumber(value.samples)} наблюдений` };
      if (!result[key]) result[key] = result[path];
    } else if (typeof value === "number" || value === null) {
      result[path] = { label: humanize(path), value, unit: unitFor(path) };
      if (!result[key]) result[key] = result[path];
    } else if (isRecord(value)) collect(value, path, result);
  });
  return result;
}

export function summaryToMetrics(summary: unknown): Record<string, Metric> {
  const all = collect(summary);
  const entries = Object.entries(all);
  const nested = entries.filter(([key]) => key.includes("."));
  return Object.fromEntries(nested.length ? nested : entries);
}

export function comparisonSummaryMetrics(summary: unknown, comparison: unknown): Record<string, Metric> {
  const previousSummary = isRecord(comparison) && isRecord(comparison.summary) ? comparison.summary : comparison;
  const current = summaryToMetrics(summary);
  const previous = summaryToMetrics(previousSummary);
  return Object.fromEntries(Object.entries(current).map(([key, metric]) => {
    const before = previous[key];
    const currentValue = typeof metric.value === "number" ? metric.value : null;
    const previousValue = typeof before?.value === "number" ? before.value : null;
    return [key, {
      ...metric,
      delta: currentValue == null || previousValue == null ? null : currentValue - previousValue,
      context: previousValue == null ? "Нет данных прошлого периода" : `Прошлый период: ${formatNumber(previousValue)}${metric.unit ? ` ${metric.unit}` : ""}`,
    }];
  }));
}

function summaryFor(summary: unknown, aliases: string[]) {
  const all = collect(summary);
  for (const alias of aliases) {
    if (all[alias]) return all[alias];
    const matched = Object.entries(all).find(([key]) => key.endsWith(`.${alias}`));
    if (matched) return matched[1];
  }
  return undefined;
}

export type DailyMetricDefinition = { key: string; label: string; unit?: string; aliases?: string[]; duration?: boolean };

export function dailyPointMetrics(
  today: Record<string, unknown> | null | undefined,
  summary: unknown,
  definitions: DailyMetricDefinition[],
  daily?: SeriesPoint[] | null,
  previousDay?: Record<string, unknown> | null,
) {
  const source = today ?? {};
  const result: Record<string, Metric> = {};
  definitions.forEach((definition) => {
    const raw = source[definition.key];
    const value = typeof raw === "number" ? raw : raw == null ? null : String(raw);
    const aliases = [definition.key, ...(definition.aliases ?? [])];
    const summaryMetric = summaryFor(summary, aliases);
    const currentValue = typeof raw === "number" ? raw : null;
    const previousValue = typeof previousDay?.[definition.key] === "number" ? previousDay[definition.key] as number : null;
    const delta = currentValue == null || previousValue == null ? undefined : currentValue - previousValue;
    result[definition.key] = {
      ...summaryMetric,
      label: definition.label,
      value: definition.duration && typeof value === "number" ? formatDuration(value) : value,
      unit: definition.duration ? undefined : definition.unit ?? summaryMetric?.unit ?? unitFor(definition.key),
      delta,
      series: daily?.map((point) => ({ date: point.date, value: typeof point[definition.key] === "number" ? point[definition.key] as number : null })),
      context: value == null ? "Данных за сегодня нет" : delta == null ? (summaryMetric?.context ?? "Сегодня") : "Относительно предыдущего дня",
    };
  });
  return result;
}

export function mergeWeightMovingAverage(daily: SeriesPoint[] | null | undefined, summary: unknown): SeriesPoint[] {
  const points = Array.isArray(daily) ? daily.map((point) => ({ ...point })) : [];
  if (!isRecord(summary) || !isRecord(summary.body) || !Array.isArray(summary.body.weight_moving_average_7d)) return points;
  const moving = summary.body.weight_moving_average_7d;
  if (moving.every((entry) => entry == null || typeof entry === "number")) {
    return points.map((point, index) => ({ ...point, weight_7d_average: point.weight_7d_average ?? moving[index] ?? null }));
  }
  const averages = new Map<string, number | null>();
  moving.forEach((entry) => {
    if (!isRecord(entry) || typeof entry.date !== "string") return;
    const value = typeof entry.value === "number" ? entry.value : typeof entry.weight_kg === "number" ? entry.weight_kg : typeof entry.average === "number" ? entry.average : null;
    averages.set(entry.date, value);
  });
  return points.map((point) => ({ ...point, weight_7d_average: point.weight_7d_average ?? (averages.has(point.date) ? averages.get(point.date) : null) }));
}
