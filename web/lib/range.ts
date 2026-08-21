import { dateInTimeZone, getDashboardTimezone, shiftISODate } from "@/lib/format";

export type RangePreset = "7" | "30" | "90" | "180" | "custom";
export type DateRangeState = { range: RangePreset; from: string; to: string; compare: boolean };

const presets = new Set<RangePreset>(["7", "30", "90", "180", "custom"]);

export function rangeFromParams(params: Pick<URLSearchParams, "get">, today: Date | string = new Date(), timeZone = getDashboardTimezone()): DateRangeState {
  const raw = params.get("range") as RangePreset | null;
  const range = raw && presets.has(raw) ? raw : "30";
  const to = params.get("to") || dateInTimeZone(today, timeZone);
  const fallbackDays = range === "custom" ? 30 : Number(range);
  const from = params.get("from") || shiftISODate(to, -Math.max(fallbackDays - 1, 0));
  return { range, from, to, compare: params.get("compare") === "1" };
}

export function rangeQuery(range: DateRangeState) {
  const params = new URLSearchParams({ range: range.range, from: range.from, to: range.to });
  if (range.compare) params.set("compare", "1");
  return params;
}
