import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/states";
import { formatDate, formatNumber } from "@/lib/format";
import type { SeriesPoint } from "@/lib/types";

export function CalendarHeatmap({ title, description, data, dataKey, unit = "", color = "var(--accent)" }: {
  title: string;
  description?: string;
  data?: SeriesPoint[] | null;
  dataKey: string;
  unit?: string;
  color?: string;
}) {
  const points = Array.isArray(data) ? data.filter((point) => /^\d{4}-\d{2}-\d{2}$/.test(point.date)) : [];
  const values = points.map((point) => typeof point[dataKey] === "number" ? point[dataKey] as number : null).filter((value): value is number => value !== null && Number.isFinite(value));
  const maximum = Math.max(...values.map((value) => Math.abs(value)), 1);
  const first = points[0] ? new Date(`${points[0].date}T00:00:00Z`) : null;
  const padding = first && !Number.isNaN(first.getTime()) ? (first.getUTCDay() + 6) % 7 : 0;
  const cells: Array<SeriesPoint | null> = [...Array.from({ length: padding }, () => null), ...points];
  const columns = Math.max(1, Math.ceil(cells.length / 7));

  return <Card className="min-w-0">
    <CardHeader><div><CardTitle>{title}</CardTitle>{description && <CardDescription>{description}</CardDescription>}</div></CardHeader>
    <CardContent>
      {!points.length ? <EmptyState description="В выбранном периоде нет календарных точек." /> : <div className="scrollbar-thin overflow-x-auto pb-2"><div className="flex min-w-max items-start gap-2"><div aria-hidden className="grid grid-rows-7 gap-1 pt-px text-[9px] leading-3 text-muted"><span>Пн</span><span /><span>Ср</span><span /><span>Пт</span><span /><span>Вс</span></div><div role="img" aria-label={title} className="grid grid-flow-col grid-rows-7 gap-1" style={{ minWidth: columns * 16 }}>
        {cells.map((point, index) => {
          if (!point) return <span key={`empty-${index}`} aria-hidden className="size-3" />;
          const value = typeof point[dataKey] === "number" ? point[dataKey] as number : null;
          const strength = value === null ? 0 : 12 + Math.round(Math.min(Math.abs(value) / maximum, 1) * 76);
          const label = `${formatDate(point.date)}: ${value === null ? "нет данных" : `${formatNumber(value)}${unit ? ` ${unit}` : ""}`}`;
          return <span key={point.date} title={label} aria-label={label} className="size-3 rounded-[3px] border border-line transition hover:scale-125" style={{ background: value === null ? "transparent" : `color-mix(in srgb, ${color} ${strength}%, transparent)` }} />;
        })}
      </div></div></div>}
    </CardContent>
  </Card>;
}
