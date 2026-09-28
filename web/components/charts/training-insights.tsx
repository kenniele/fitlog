"use client";

import { CalendarCheck2, Flame, Trophy } from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { EmptyState } from "@/components/ui/states";
import { dateInTimeZone, formatDate, formatNumber } from "@/lib/format";
import { calendarDates, numeric } from "@/lib/dashboard";
import { CalendarGrid } from "./calendar-grid";
import type { TrainingHeatmapPoint, TrainingStreakSummary } from "@/lib/types";

export function activityIntensity(value: number, maximum: number) {
  if (
    !Number.isFinite(value) ||
    value <= 0 ||
    !Number.isFinite(maximum) ||
    maximum <= 0
  )
    return 0;
  return Math.min(4, Math.max(1, Math.ceil((value / maximum) * 4)));
}

export function ActivityHeatmap({
  data,
}: {
  data?: TrainingHeatmapPoint[] | null;
}) {
  const points = (data ?? [])
    .filter((point) => /^\d{4}-\d{2}-\d{2}$/.test(point.date))
    .sort((a, b) => a.date.localeCompare(b.date));
  const maximum = Math.max(
    0,
    ...points.map((point) =>
      Math.max(point.working_sets ?? 0, point.sessions ?? 0),
    ),
  );
  const byDate = new Map(points.map((point) => [point.date, point]));
  const dates = points.length
    ? calendarDates(points[0].date, points.at(-1)!.date)
    : [];
  return (
    <Card>
      <CardHeader>
        <div>
          <CardTitle>Календарь активности</CardTitle>
          <CardDescription>
            Рабочие подходы за день; если подходов нет — число сессий.
          </CardDescription>
        </div>
      </CardHeader>
      <CardContent>
        {!dates.length ? (
          <EmptyState description="В выбранном периоде нет календарных точек." />
        ) : (
          <CalendarGrid
            dates={dates}
            today={dateInTimeZone()}
            label="Календарь тренировочной активности"
            color={(date) => {
              const point = byDate.get(date);
              const intensity = activityIntensity(
                Math.max(
                  numeric(point?.working_sets) ?? 0,
                  numeric(point?.sessions) ?? 0,
                ),
                maximum,
              );
              return intensity ? `var(--heat-${intensity})` : undefined;
            }}
            valueLabel={(date) => {
              const point = byDate.get(date);
              return point
                ? `${formatDate(date)}: ${formatNumber(point.sessions)} трен., ${formatNumber(point.working_sets)} раб. подходов, ${formatNumber(point.volume_kg, {}, " кг")}`
                : "Без данных";
            }}
          />
        )}
      </CardContent>
    </Card>
  );
}

export type DistributionDatum = {
  label: string;
  value: number;
  detail?: string;
};

export function DistributionBars({
  title,
  description,
  data,
  valueLabel = "подходов",
  accent = "accent",
}: {
  title: string;
  description: string;
  data?: DistributionDatum[] | null;
  valueLabel?: string;
  accent?: "accent" | "blue";
}) {
  const points = Array.isArray(data)
    ? data.filter((point) => Number.isFinite(point.value) && point.value >= 0)
    : [];
  const maximum = Math.max(0, ...points.map((point) => point.value));
  return (
    <Card className="min-w-0">
      <CardHeader>
        <div>
          <CardTitle>{title}</CardTitle>
          <CardDescription>{description}</CardDescription>
        </div>
      </CardHeader>
      <CardContent>
        {!points.length ? (
          <EmptyState description="Для этого распределения пока недостаточно данных." />
        ) : (
          <div role="list" className="space-y-3">
            {points.map((point) => (
              <div
                role="listitem"
                key={point.label}
                className="grid min-w-0 grid-cols-[minmax(65px,0.8fr)_minmax(36px,2fr)_auto] items-center gap-3"
              >
                <span
                  className="truncate text-xs font-medium text-ink"
                  title={point.label}
                >
                  {point.label}
                </span>
                <div className="h-2 overflow-hidden rounded-full bg-ink/[.045]">
                  <div
                    className={`h-full rounded-full ${accent === "blue" ? "bg-blue" : "bg-accent"}`}
                    style={{
                      width:
                        maximum > 0
                          ? `${Math.max(3, (point.value / maximum) * 100)}%`
                          : "0%",
                    }}
                  />
                </div>
                <span className="min-w-16 text-right tabular-nums">
                  <span className="block text-xs text-muted">
                    {formatNumber(point.value)} {valueLabel}
                  </span>
                  {point.detail ? (
                    <span className="mt-0.5 block text-[10px] text-muted opacity-80">
                      {point.detail}
                    </span>
                  ) : null}
                </span>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

export function TrainingStreakCards({
  streak,
}: {
  streak?: TrainingStreakSummary | null;
}) {
  const cards = [
    {
      key: "current",
      label: "Текущая серия",
      value: streak?.current_days,
      context: "дней подряд до конца периода",
      Icon: Flame,
      tone: "text-accent",
    },
    {
      key: "longest",
      label: "Лучшая за 30 дней",
      value: streak?.longest_last_30_days,
      context: "последовательных активных дней",
      Icon: Trophy,
      tone: "text-warning",
    },
    {
      key: "active",
      label: "Активность за 30 дней",
      value: streak?.active_days_last_30,
      context: "дней хотя бы с одной тренировкой",
      Icon: CalendarCheck2,
      tone: "text-blue",
    },
  ];
  return (
    <div className="grid gap-6 border-b border-line pb-6 sm:grid-cols-3">
      {cards.map(({ key, label, value, context, Icon, tone }) => (
        <Card key={key} className="min-w-0 bg-transparent">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="text-xs font-medium text-muted">{label}</p>
              <p className="mt-2 text-2xl font-semibold tracking-[-.03em] text-ink">
                {formatNumber(value)}
                <span className="ml-1 text-sm font-medium text-muted">дн.</span>
              </p>
            </div>
            <span className="p-2">
              <Icon aria-hidden className={`size-4 ${tone}`} />
            </span>
          </div>
          <p className="mt-3 text-xs leading-5 text-muted">{context}</p>
        </Card>
      ))}
    </div>
  );
}
