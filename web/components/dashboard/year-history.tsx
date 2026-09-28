"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { ArrowRight, ChevronLeft, ChevronRight } from "lucide-react";
import { apiFetch } from "@/lib/api";
import type { AnalyticsResponse, Settings } from "@/lib/types";
import {
  calendarDates,
  heatMetrics,
  heatScale,
  metricText,
  monthNames,
  numeric,
  rangeParams,
  type HeatMetricKey,
} from "@/lib/dashboard";
import { CalendarGrid } from "@/components/charts/calendar-grid";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/field";
import { ErrorState, Skeleton } from "@/components/ui/states";

export function YearHistory({
  today,
  settings,
}: {
  today: string;
  settings: Settings;
}) {
  const currentYear = Number(today.slice(0, 4));
  const [year, setYear] = useState(currentYear);
  const [month, setMonth] = useState("all");
  const [metric, setMetric] = useState<HeatMetricKey>("sleep_seconds");
  const definition = heatMetrics.find((item) => item.key === metric)!;
  const range = rangeParams(
    `${year}-01-01`,
    year === currentYear ? today : `${year}-12-31`,
  );
  const query = useQuery({
    queryKey: ["year-history", definition.source, range],
    queryFn: () =>
      apiFetch<AnalyticsResponse>(`/analytics/${definition.source}?${range}`),
  });
  const points = useMemo(
    () =>
      new Map((query.data?.daily ?? []).map((point) => [point.date, point])),
    [query.data],
  );
  const dates = useMemo(
    () =>
      calendarDates(`${year}-01-01`, `${year}-12-31`).filter(
        (date) => month === "all" || Number(date.slice(5, 7)) === Number(month),
      ),
    [year, month],
  );
  const values = dates
    .filter((date) => date <= today)
    .map((date) => numeric(points.get(date)?.[metric]))
    .filter((value): value is number => value !== null);
  const scale = heatScale(metric, settings, values);
  const text = (date: string) =>
    `${definition.label}: ${metricText(points.get(date)?.[metric], metric, definition.unit)}`;
  const dayDetails = (date: string) => {
    const point = points.get(date);
    const items = [
      ...heatMetrics,
      { key: "hrv_ms", label: "HRV", unit: "мс" },
      { key: "resting_heart_rate_bpm", label: "Пульс покоя", unit: "уд/мин" },
      { key: "fat_g", label: "Жиры", unit: "г" },
      { key: "carbohydrates_g", label: "Углеводы", unit: "г" },
    ];
    const recorded = items.filter(
      (item) => numeric(point?.[item.key]) !== null,
    );
    return (
      <div>
        {recorded.length ? (
          <dl>
            {recorded.map((item) => (
              <div key={item.key} className="body-detail-row">
                <dt className="text-sm text-muted">{item.label}</dt>
                <dd className="font-medium tabular-nums">
                  {metricText(point?.[item.key], item.key, item.unit)}
                </dd>
              </div>
            ))}
          </dl>
        ) : (
          <p className="py-6 text-muted">
            Без данных. За этот день нет сохранённых показателей.
          </p>
        )}
        {numeric(point?.workout_count) !== null && (
          <p className="mt-5 text-sm">
            Завершённых тренировок: {point?.workout_count}
          </p>
        )}
        <Link
          className="text-link mt-4"
          href={`/dashboard/${definition.source === "body" ? "body" : definition.source}?${rangeParams(date, date)}`}
        >
          Открыть день в разделе <ArrowRight className="size-4" />
        </Link>
      </div>
    );
  };
  return (
    <section className="editorial-section" aria-labelledby="year-title">
      <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="eyebrow mb-3">Год в движении</p>
          <div className="flex items-center gap-2">
            <h2
              id="year-title"
              className="metric-number mr-2 text-5xl font-medium sm:text-6xl"
            >
              {year}
            </h2>
            <Button
              size="icon"
              variant="ghost"
              aria-label="Предыдущий год"
              disabled={year <= 1900}
              onClick={() => setYear(year - 1)}
            >
              <ChevronLeft className="size-4" />
            </Button>
            <Button
              size="icon"
              variant="ghost"
              aria-label="Следующий год"
              disabled={year >= currentYear}
              onClick={() => setYear(year + 1)}
            >
              <ChevronRight className="size-4" />
            </Button>
          </div>
        </div>
        <div className="flex w-full items-center justify-between gap-5 sm:w-auto">
          <p className="shrink-0 text-xs text-muted sm:text-right">
            <span className="mb-1 block text-2xl tabular-nums text-ink">
              {query.isPending || query.isError ? "Без данных" : values.length}
            </span>
            дней с записью
          </p>
          <Select
            aria-label="Месяц календаря"
            className="max-w-40"
            value={month}
            onChange={(event) => setMonth(event.target.value)}
          >
            <option value="all">Весь год</option>
            {monthNames.map((name, index) => (
              <option key={name} value={index + 1}>
                {name}
              </option>
            ))}
          </Select>
        </div>
      </div>
      <div
        className="mb-7 flex flex-wrap gap-1"
        role="group"
        aria-label="Показатель календаря"
      >
        {heatMetrics.map((item) => (
          <button
            key={item.key}
            className={`quiet-control ${metric === item.key ? "bg-elevated text-ink" : ""}`}
            aria-pressed={metric === item.key}
            onClick={() => setMetric(item.key)}
          >
            {item.label}
          </button>
        ))}
      </div>
      {query.isPending ? (
        <div aria-busy="true" aria-label="Загрузка календаря">
          <Skeleton className="h-48 w-full" />
        </div>
      ) : query.isError ? (
        <ErrorState
          error={query.error}
          retry={() => query.refetch()}
          title="Календарь временно недоступен"
        />
      ) : (
        <>
          <CalendarGrid
            key={`${year}-${month}-${metric}`}
            dates={dates}
            today={today}
            label={`Календарь: ${definition.label}, ${year}`}
            valueLabel={text}
            color={(date) => {
              const value = numeric(points.get(date)?.[metric]);
              return value === null
                ? undefined
                : scale.colors[scale.bucket(value)];
            }}
            details={dayDetails}
            previewDetails={(date) => (
              <dl className="mt-3 space-y-2">
                {heatMetrics
                  .filter(
                    (item) => numeric(points.get(date)?.[item.key]) !== null,
                  )
                  .slice(0, 5)
                  .map((item) => (
                    <div key={item.key} className="flex justify-between gap-3">
                      <dt className="text-muted">{item.label}</dt>
                      <dd className="tabular-nums">
                        {metricText(
                          points.get(date)?.[item.key],
                          item.key,
                          item.unit,
                        )}
                      </dd>
                    </div>
                  ))}
                {numeric(points.get(date)?.[metric]) === null && (
                  <p className="text-muted">{definition.label}: Без данных</p>
                )}
              </dl>
            )}
            focusDate={today}
          />
          <div className="mt-5 flex flex-wrap items-center justify-between gap-x-8 gap-y-4 text-xs text-muted">
            <p>
              {!values.length
                ? "Без данных за выбранный период"
                : month === "all"
                  ? "Один день — одна точка вашей истории"
                  : `${monthNames[Number(month) - 1]} · ${year}`}
            </p>
            <div className="flex flex-wrap gap-x-4 gap-y-2">
              <span className="flex items-center gap-1.5">
                <i className="size-3 rounded-sm bg-[var(--heat-empty)]" />
                Без данных
              </span>
              {scale.labels.map((label, index) => (
                <span
                  key={`${index}-${label}`}
                  className="flex items-center gap-1.5"
                >
                  <i
                    className="size-3 rounded-sm"
                    style={{ background: scale.colors[index] }}
                  />
                  {label}
                </span>
              ))}
            </div>
          </div>
          {(metric === "weight_kg" || metric === "body_fat_percent") && (
            <p className="mt-3 text-xs text-muted">
              Цвет отражает величину измерения, а не оценку результата. Пропуски
              не заполняются.
            </p>
          )}
        </>
      )}
    </section>
  );
}
