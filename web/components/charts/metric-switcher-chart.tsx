"use client";

import { useEffect, useState } from "react";
import { TrendChart, type ChartSeries } from "@/components/charts/trend-chart";
import type { SeriesPoint } from "@/lib/types";
import { cn } from "@/lib/utils";

export type SwitcherMetric = ChartSeries & { description?: string; variant?: "line" | "area" | "bar" };

export function MetricSwitcherChart({ title, description, data, metrics, height = 280 }: {
  title: string;
  description?: string;
  data?: SeriesPoint[] | null;
  metrics: SwitcherMetric[];
  height?: number;
}) {
  const [active, setActive] = useState(metrics[0]?.key ?? "");
  useEffect(() => {
    if (!metrics.some((metric) => metric.key === active)) setActive(metrics[0]?.key ?? "");
  }, [active, metrics]);
  const selected = metrics.find((metric) => metric.key === active) ?? metrics[0];
  if (!selected) return null;

  return <div className="min-w-0 space-y-2">
    <div role="tablist" aria-label={`Метрика: ${title}`} className="flex flex-wrap gap-1 rounded-control border border-line bg-surface p-1">
      {metrics.map((metric) => <button key={metric.key} type="button" role="tab" aria-selected={metric.key === selected.key} onClick={() => setActive(metric.key)} className={cn("rounded-lg px-2.5 py-1.5 text-xs font-medium transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent", metric.key === selected.key ? "bg-white/[.09] text-ink" : "text-muted hover:text-ink")}>{metric.label}</button>)}
    </div>
    <TrendChart title={title} description={selected.description ?? description} data={data} series={[selected]} height={height} variant={selected.variant ?? "line"} />
  </div>;
}
