"use client";

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { EmptyState } from "@/components/ui/states";
import { dateInTimeZone } from "@/lib/format";
import { calendarDates, metricText, numeric } from "@/lib/dashboard";
import type { SeriesPoint } from "@/lib/types";
import { CalendarGrid } from "@/components/charts/calendar-grid";

export function CalendarHeatmap({
  title,
  description,
  data,
  dataKey,
  unit = "",
  color = "var(--accent)",
}: {
  title: string;
  description?: string;
  data?: SeriesPoint[] | null;
  dataKey: string;
  unit?: string;
  color?: string;
}) {
  const points = (data ?? [])
    .filter((point) => /^\d{4}-\d{2}-\d{2}$/.test(point.date))
    .sort((a, b) => a.date.localeCompare(b.date));
  const byDate = new Map(points.map((point) => [point.date, point]));
  const maximum = Math.max(
    ...points.map((p) => Math.abs(numeric(p[dataKey]) ?? 0)),
    1,
  );
  const dates = points.length
    ? calendarDates(points[0].date, points.at(-1)!.date)
    : [];
  return (
    <Card>
      <CardHeader>
        <div>
          <CardTitle>{title}</CardTitle>
          {description && <CardDescription>{description}</CardDescription>}
        </div>
      </CardHeader>
      <CardContent>
        {!dates.length ? (
          <EmptyState description="В выбранном периоде нет календарных точек." />
        ) : (
          <CalendarGrid
            dates={dates}
            today={dateInTimeZone()}
            label={title}
            valueLabel={(date) =>
              metricText(byDate.get(date)?.[dataKey], dataKey, unit)
            }
            color={(date) => {
              const value = numeric(byDate.get(date)?.[dataKey]);
              return value === null
                ? undefined
                : `color-mix(in srgb, ${color} ${18 + Math.round(Math.min(Math.abs(value) / maximum, 1) * 78)}%, var(--heat-empty))`;
            }}
          />
        )}
      </CardContent>
    </Card>
  );
}
