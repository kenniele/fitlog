"use client";

import { CalendarCheck2, Flame, Trophy } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/states";
import { formatDate, formatNumber } from "@/lib/format";
import type { TrainingHeatmapPoint, TrainingStreakSummary } from "@/lib/types";

const intensityClasses = [
  "border-line bg-white/[.025]",
  "border-accent/10 bg-accent/15",
  "border-accent/20 bg-accent/30",
  "border-accent/30 bg-accent/55",
  "border-accent/40 bg-accent/85",
];

export function activityIntensity(value: number, maximum: number) {
  if (!Number.isFinite(value) || value <= 0 || !Number.isFinite(maximum) || maximum <= 0) return 0;
  return Math.min(4, Math.max(1, Math.ceil((value / maximum) * 4)));
}

function mondayOffset(date: string) {
  const parsed = new Date(`${date}T00:00:00Z`);
  if (Number.isNaN(parsed.getTime())) return 0;
  return (parsed.getUTCDay() + 6) % 7;
}

export function ActivityHeatmap({ data }: { data?: TrainingHeatmapPoint[] | null }) {
  const points = Array.isArray(data) ? data.filter((point) => /^\d{4}-\d{2}-\d{2}$/.test(point.date)) : [];
  const maximum = Math.max(0, ...points.map((point) => Math.max(point.working_sets ?? 0, point.sessions ?? 0)));
  const padding = points.length ? mondayOffset(points[0].date) : 0;
  const cells: Array<TrainingHeatmapPoint | null> = [...Array.from({ length: padding }, () => null), ...points];
  const columns = Math.max(1, Math.ceil(cells.length / 7));

  return <Card className="min-w-0">
    <CardHeader className="flex-col sm:flex-row">
      <div><CardTitle>Календарь активности</CardTitle><CardDescription>Каждая ячейка — локальная дата; интенсивность отражает рабочие подходы, а при их отсутствии — сессии.</CardDescription></div>
      <div className="flex shrink-0 items-center gap-1 text-[10px] text-muted"><span>меньше</span>{intensityClasses.map((className) => <span key={className} aria-hidden className={`size-2.5 rounded-[3px] border ${className}`} />)}<span>больше</span></div>
    </CardHeader>
    <CardContent>
      {!points.length ? <EmptyState description="В выбранном периоде нет календарных точек." /> : <div className="scrollbar-thin overflow-x-auto pb-2">
        <div className="flex min-w-max items-start gap-2">
          <div aria-hidden className="grid grid-rows-7 gap-1 pt-px text-[9px] leading-3 text-muted"><span>Пн</span><span /><span>Ср</span><span /><span>Пт</span><span /><span>Вс</span></div>
          <div role="img" aria-label="Календарь тренировочной активности" className="grid grid-flow-col grid-rows-7 gap-1" style={{ minWidth: columns * 16 }}>
            {cells.map((point, index) => point ? <span
              key={point.date}
              className={`size-3 rounded-[3px] border transition hover:scale-125 ${intensityClasses[activityIntensity(Math.max(point.working_sets ?? 0, point.sessions ?? 0), maximum)]}`}
              title={`${formatDate(point.date)}: ${formatNumber(point.sessions)} трен., ${formatNumber(point.working_sets)} раб. подходов, ${formatNumber(point.volume_kg, {}, " кг")}`}
            /> : <span key={`empty-${index}`} aria-hidden className="size-3" />)}
          </div>
        </div>
      </div>}
    </CardContent>
  </Card>;
}

export type DistributionDatum = { label: string; value: number; detail?: string };

export function DistributionBars({ title, description, data, valueLabel = "подходов", accent = "accent" }: { title: string; description: string; data?: DistributionDatum[] | null; valueLabel?: string; accent?: "accent" | "blue" }) {
  const points = Array.isArray(data) ? data.filter((point) => Number.isFinite(point.value) && point.value >= 0) : [];
  const maximum = Math.max(0, ...points.map((point) => point.value));
  return <Card className="min-w-0"><CardHeader><div><CardTitle>{title}</CardTitle><CardDescription>{description}</CardDescription></div></CardHeader><CardContent>
    {!points.length ? <EmptyState description="Для этого распределения пока недостаточно данных." /> : <div role="list" className="space-y-3">
      {points.map((point) => <div role="listitem" key={point.label} className="grid min-w-0 grid-cols-[minmax(88px,0.8fr)_minmax(100px,2fr)_auto] items-center gap-3">
        <span className="truncate text-xs font-medium text-ink" title={point.label}>{point.label}</span>
        <div className="h-2 overflow-hidden rounded-full bg-white/[.045]"><div className={`h-full rounded-full ${accent === "blue" ? "bg-blue" : "bg-accent"}`} style={{ width: maximum > 0 ? `${Math.max(3, point.value / maximum * 100)}%` : "0%" }} /></div>
        <span className="min-w-16 text-right tabular-nums"><span className="block text-xs text-muted">{formatNumber(point.value)} {valueLabel}</span>{point.detail ? <span className="mt-0.5 block text-[10px] text-muted opacity-80">{point.detail}</span> : null}</span>
      </div>)}
    </div>}
  </CardContent></Card>;
}

export function TrainingStreakCards({ streak }: { streak?: TrainingStreakSummary | null }) {
  const cards = [
    { key: "current", label: "Текущая серия", value: streak?.current_days, context: "дней подряд до конца периода", Icon: Flame, tone: "text-accent" },
    { key: "longest", label: "Лучшая за 30 дней", value: streak?.longest_last_30_days, context: "последовательных активных дней", Icon: Trophy, tone: "text-warning" },
    { key: "active", label: "Активность за 30 дней", value: streak?.active_days_last_30, context: "дней хотя бы с одной тренировкой", Icon: CalendarCheck2, tone: "text-blue" },
  ];
  return <div className="grid gap-3 sm:grid-cols-3">{cards.map(({ key, label, value, context, Icon, tone }) => <Card key={key} className="min-w-0 p-4"><div className="flex items-start justify-between gap-3"><div className="min-w-0"><p className="text-xs font-medium text-muted">{label}</p><p className="mt-2 text-2xl font-semibold tracking-[-.03em] text-ink">{formatNumber(value)}<span className="ml-1 text-sm font-medium text-muted">дн.</span></p></div><span className="rounded-control border border-line bg-white/[.035] p-2"><Icon aria-hidden className={`size-4 ${tone}`} /></span></div><p className="mt-3 text-xs leading-5 text-muted">{context}</p></Card>)}</div>;
}
