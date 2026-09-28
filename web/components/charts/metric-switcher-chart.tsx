"use client";

import { useEffect, useId, useRef, useState } from "react";
import { TrendChart, type ChartSeries } from "@/components/charts/trend-chart";
import type { SeriesPoint } from "@/lib/types";

export type SwitcherMetric = ChartSeries & {
  description?: string;
  variant?: "line" | "area" | "bar";
};
export function MetricSwitcherChart({
  title,
  description,
  data,
  metrics,
  height = 260,
}: {
  title: string;
  description?: string;
  data?: SeriesPoint[] | null;
  metrics: SwitcherMetric[];
  height?: number;
}) {
  const id = useId();
  const tabs = useRef(new Map<string, HTMLButtonElement>());
  const [active, setActive] = useState(metrics[0]?.key ?? "");
  useEffect(() => {
    if (!metrics.some((metric) => metric.key === active))
      setActive(metrics[0]?.key ?? "");
  }, [active, metrics]);
  const selected =
    metrics.find((metric) => metric.key === active) ?? metrics[0];
  if (!selected) return null;
  const controls = (
    <div role="tablist" aria-label={`Метрика: ${title}`} className="chart-tabs">
      {metrics.map((metric, index) => (
        <button
          ref={(node) => {
            if (node) tabs.current.set(metric.key, node);
          }}
          key={metric.key}
          id={`${id}-${metric.key}`}
          type="button"
          role="tab"
          aria-controls={`${id}-panel`}
          aria-selected={metric.key === selected.key}
          tabIndex={metric.key === selected.key ? 0 : -1}
          onClick={() => setActive(metric.key)}
          className="chart-tab"
          onKeyDown={(event) => {
            const next =
              event.key === "ArrowRight"
                ? (index + 1) % metrics.length
                : event.key === "ArrowLeft"
                  ? (index + metrics.length - 1) % metrics.length
                  : event.key === "Home"
                    ? 0
                    : event.key === "End"
                      ? metrics.length - 1
                      : null;
            if (next !== null) {
              event.preventDefault();
              setActive(metrics[next].key);
              tabs.current.get(metrics[next].key)?.focus();
            }
          }}
        >
          {metric.label}
        </button>
      ))}
    </div>
  );
  return (
    <div
      id={`${id}-panel`}
      role="tabpanel"
      aria-labelledby={`${id}-${selected.key}`}
      className="min-w-0"
    >
      <TrendChart
        title={title}
        description={selected.description ?? description}
        data={data}
        series={[selected]}
        height={height}
        variant={selected.variant ?? "line"}
        controls={controls}
      />
    </div>
  );
}
