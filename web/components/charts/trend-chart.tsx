"use client";

import { useId } from "react";
import type { ReactNode } from "react";
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { EmptyState } from "@/components/ui/states";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { formatDate, formatNumber } from "@/lib/format";
import { metricText, numeric } from "@/lib/dashboard";
import type { SeriesPoint } from "@/lib/types";

export type ChartSeries = {
  key: string;
  label: string;
  color?: string;
  unit?: string;
  type?: "line" | "area" | "bar";
  strokeDasharray?: string;
};
function unitFor(key: string) {
  if (key.endsWith("_kg") || key === "estimated_1rm") return "кг";
  if (key.endsWith("_g")) return "г";
  if (key.endsWith("_mg")) return "мг";
  if (key.endsWith("_ml")) return "мл";
  if (key.endsWith("_l")) return "л";
  if (key.endsWith("_cm")) return "см";
  if (key.endsWith("_cm2")) return "см²";
  if (key.endsWith("_kcal")) return "ккал";
  if (key.endsWith("_percent") || key === "recovery_score") return "%";
  if (key.endsWith("_ms")) return "мс";
  if (key.endsWith("_bpm")) return "уд/мин";
  if (key.endsWith("minutes")) return "мин";
  return "";
}
export function TrendChart({
  title,
  description,
  data,
  series,
  height = 260,
  variant = "line",
  controls,
}: {
  title: string;
  description?: string;
  data?: SeriesPoint[] | null;
  series: ChartSeries[];
  height?: number;
  variant?: "line" | "area" | "bar";
  controls?: ReactNode;
}) {
  const id = useId();
  const points = Array.isArray(data) ? data : [];
  const hasValues = points.some((point) =>
    series.some((item) => numeric(point[item.key]) !== null),
  );
  const axes = (
    <>
      <CartesianGrid
        vertical={false}
        stroke="var(--border)"
        strokeDasharray="2 5"
      />
      <XAxis
        dataKey="date"
        tickFormatter={(date: string) =>
          /^\d{4}-\d{2}-\d{2}/.test(date) ? formatDate(date).slice(0, 5) : date
        }
        tick={{ fill: "var(--text-secondary)", fontSize: 11 }}
        axisLine={false}
        tickLine={false}
        minTickGap={36}
        tickMargin={12}
      />
      <YAxis
        tick={{ fill: "var(--text-secondary)", fontSize: 11 }}
        axisLine={false}
        tickLine={false}
        tickFormatter={(value: number) =>
          series.every((item) => item.key.endsWith("seconds"))
            ? `${formatNumber(value / 3600)} ч`
            : formatNumber(value, { notation: "compact" })
        }
      />
      <Tooltip
        cursor={{ stroke: "var(--border)", fill: "var(--border)" }}
        labelFormatter={(label) => formatDate(String(label))}
        formatter={(value, name, item) => [
          metricText(
            value,
            String(item.dataKey),
            series.find((s) => s.key === item.dataKey)?.unit ??
              unitFor(String(item.dataKey)),
          ),
          name,
        ]}
        contentStyle={{
          background: "var(--surface-2)",
          color: "var(--text-primary)",
          border: "1px solid var(--border)",
          borderRadius: 8,
          fontSize: 12,
          boxShadow: "var(--shadow-popover)",
        }}
        labelStyle={{ color: "var(--text-secondary)", marginBottom: 8 }}
      />
    </>
  );
  const color = (item: ChartSeries, index: number) =>
    item.color ?? (index ? "var(--accent-blue)" : "var(--accent)");
  const margin = { top: 12, right: 8, left: -12, bottom: 8 };
  return (
    <Card className="min-w-0">
      <CardHeader className="flex-wrap">
        <div>
          <CardTitle>{title}</CardTitle>
          {description && <CardDescription>{description}</CardDescription>}
        </div>
        {!controls && (
          <div className="flex flex-wrap gap-x-4 gap-y-2">
            {series.map((item, index) => (
              <span
                key={item.key}
                className="flex items-center gap-1.5 text-[11px] text-muted"
              >
                <span
                  className="h-px w-3"
                  style={{ background: color(item, index) }}
                />
                {item.label}
                {item.unit ? `, ${item.unit}` : ""}
              </span>
            ))}
          </div>
        )}
      </CardHeader>
      {controls && <div className="px-5">{controls}</div>}
      <CardContent className="pt-4">
        {!hasValues ? (
          <EmptyState
            title="Без данных"
            description="Запишите показатели или выберите период, в котором есть измерения."
          />
        ) : (
          <div
            style={{ height }}
            className="w-full min-w-0"
            aria-label={title}
            role="group"
          >
            <ResponsiveContainer width="100%" height="100%">
              {variant === "bar" ? (
                <BarChart accessibilityLayer data={points} margin={margin}>
                  {axes}
                  {series.map((item, index) => (
                    <Bar
                      isAnimationActive={false}
                      key={item.key}
                      dataKey={item.key}
                      name={item.label}
                      fill={color(item, index)}
                      radius={[2, 2, 0, 0]}
                      maxBarSize={24}
                    />
                  ))}
                </BarChart>
              ) : variant === "area" ? (
                <AreaChart accessibilityLayer data={points} margin={margin}>
                  <defs>
                    <linearGradient id={id} x1="0" y1="0" x2="0" y2="1">
                      <stop
                        offset="0%"
                        stopColor={color(series[0], 0)}
                        stopOpacity={0.12}
                      />
                      <stop
                        offset="100%"
                        stopColor={color(series[0], 0)}
                        stopOpacity={0}
                      />
                    </linearGradient>
                  </defs>
                  {axes}
                  {series.map((item, index) => (
                    <Area
                      isAnimationActive={false}
                      key={item.key}
                      type="linear"
                      dataKey={item.key}
                      name={item.label}
                      stroke={color(item, index)}
                      fill={index ? "transparent" : `url(#${id})`}
                      strokeWidth={1.8}
                      dot={{ r: 2, strokeWidth: 0 }}
                      connectNulls={false}
                    />
                  ))}
                </AreaChart>
              ) : (
                <LineChart accessibilityLayer data={points} margin={margin}>
                  {axes}
                  {series.map((item, index) => (
                    <Line
                      isAnimationActive={false}
                      key={item.key}
                      type="linear"
                      dataKey={item.key}
                      name={item.label}
                      stroke={color(item, index)}
                      strokeDasharray={item.strokeDasharray}
                      strokeWidth={1.8}
                      dot={{ r: points.length < 40 ? 2 : 1.5, strokeWidth: 0 }}
                      activeDot={{ r: 4 }}
                      connectNulls={false}
                    />
                  ))}
                </LineChart>
              )}
            </ResponsiveContainer>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
export function MiniTrend({
  data,
  dataKey = "value",
  color = "var(--accent)",
}: {
  data?: SeriesPoint[] | null;
  dataKey?: string;
  color?: string;
}) {
  const points = Array.isArray(data) ? data : [];
  if (!points.some((point) => numeric(point[dataKey]) !== null)) return null;
  return (
    <div className="hidden h-8 w-20 shrink-0 sm:block" aria-hidden>
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={points}>
          <Line
            isAnimationActive={false}
            type="linear"
            dataKey={dataKey}
            dot={false}
            stroke={color}
            strokeWidth={1.5}
            connectNulls={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
