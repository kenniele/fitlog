import { ArrowDownRight, ArrowUpRight, Minus } from "lucide-react";
import { MiniTrend } from "@/components/charts/trend-chart";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { formatMissing, formatNumber } from "@/lib/format";
import type { Metric } from "@/lib/types";

export function MetricCard({ metric, label }: { metric: Metric | string | number | null | undefined; label: string }) {
  const normalized: Metric = typeof metric === "object" && metric !== null ? metric : { value: metric };
  const delta = normalized.delta;
  const trend = delta === null || delta === undefined ? "neutral" : delta > 0 ? "up" : delta < 0 ? "down" : "neutral";
  const tone = normalized.status === "good" ? "good" : normalized.status === "warning" ? "warning" : normalized.status === "critical" ? "critical" : "neutral";
  const value = typeof normalized.value === "number" ? formatNumber(normalized.value) : formatMissing(normalized.value);
  return <Card className="group min-w-0 p-4 transition hover:border-white/[.14]"><div className="flex items-start justify-between gap-3"><div className="min-w-0"><p className="truncate text-xs font-medium text-muted">{normalized.label ?? label}</p><p className="mt-2 truncate text-2xl font-semibold tracking-[-.03em] text-ink">{value}{normalized.unit && value !== "—" ? <span className="ml-1 text-sm font-medium text-muted">{normalized.unit}</span> : null}</p></div><MiniTrend data={normalized.series} /></div><div className="mt-3 flex min-h-6 items-center justify-between gap-2"><span className="line-clamp-1 text-xs text-muted">{normalized.context ?? "Нет дополнительного контекста"}</span>{delta !== null && delta !== undefined ? <Badge tone={tone}><span className="flex items-center gap-1">{trend === "up" ? <ArrowUpRight className="size-3" /> : trend === "down" ? <ArrowDownRight className="size-3" /> : <Minus className="size-3" />}{delta > 0 ? "+" : ""}{formatNumber(delta)}</span></Badge> : null}</div></Card>;
}

export function MetricGrid({ metrics, order }: { metrics?: Record<string, Metric | string | number | null> | null; order?: Array<{ key: string; label: string }> }) {
  const entries = order ?? Object.keys(metrics ?? {}).map((key) => ({ key, label: key.replaceAll("_", " ") }));
  return <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">{entries.map((item) => <MetricCard key={item.key} label={item.label} metric={metrics?.[item.key]} />)}</div>;
}
