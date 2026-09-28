"use client";

import { useState } from "react";
import { useQueries } from "@tanstack/react-query";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { apiFetch } from "@/lib/api";
import type { AnalyticsResponse, SeriesPoint } from "@/lib/types";
import { daysBetweenISO, formatDuration, formatNumber } from "@/lib/format";
import {
  metricText,
  monthNames,
  monthRange,
  numeric,
  observedStats,
  rangeParams,
} from "@/lib/dashboard";
import { Button } from "@/components/ui/button";
import { ErrorState, Skeleton } from "@/components/ui/states";
import { useSectionVisible } from "./use-section-visible";
import { MetricValue } from "./metric-value";

const sources = ["recovery", "nutrition", "training", "body"] as const;
function ReviewRow({
  label,
  value,
  note,
}: {
  label: string;
  value: string;
  note?: string;
}) {
  return (
    <div className="body-detail-row">
      <dt className="text-sm text-muted">{label}</dt>
      <dd className="text-right text-sm tabular-nums">
        <span className="block font-medium">{value}</span>
        {note && <span className="mt-1 block text-xs text-muted">{note}</span>}
      </dd>
    </div>
  );
}
function comparison(
  current: SeriesPoint[],
  previous: SeriesPoint[],
  key: string,
  unit: string,
  currentDays: number,
  previousDays: number,
) {
  const a = observedStats(current, key);
  const b = observedStats(previous, key);
  if (
    a.average === null ||
    b.average === null ||
    a.count < 7 ||
    b.count < 7 ||
    a.count / currentDays < 0.7 ||
    b.count / previousDays < 0.7
  )
    return undefined;
  const delta = a.average - b.average;
  const text = key.endsWith("seconds")
    ? `${delta < 0 ? "−" : "+"}${formatDuration(Math.abs(delta))}`
    : `${delta > 0 ? "+" : ""}${metricText(delta, key, unit)}`;
  return `${text} к прошлому месяцу`;
}
export function MonthReview({ today }: { today: string }) {
  const { ref, visible } = useSectionVisible();
  const [selected, setSelected] = useState(today.slice(0, 7));
  const [year, number] = selected.split("-").map(Number);
  const month = number - 1;
  const current = monthRange(year, month, today);
  const previous = monthRange(
    month === 0 ? year - 1 : year,
    month === 0 ? 11 : month - 1,
    today,
  );
  const ranges = [
    rangeParams(current.from, current.to),
    rangeParams(previous.from, previous.to),
  ];
  const queries = useQueries({
    queries: ranges.flatMap((range) =>
      sources.map((source) => ({
        enabled: visible,
        queryKey: ["month-review", source, range],
        queryFn: () =>
          apiFetch<AnalyticsResponse>(`/analytics/${source}?${range}`),
      })),
    ),
  });
  const shiftMonth = (amount: number) => {
    const date = new Date(Date.UTC(year, month + amount, 1));
    setSelected(date.toISOString().slice(0, 7));
  };
  const elapsed = daysBetweenISO(current.from, current.to) + 1;
  const previousDays = daysBetweenISO(previous.from, previous.to) + 1;
  const incomplete = current.to < current.fullTo;
  const panel = (
    index: number,
    render: (
      data: AnalyticsResponse,
      before: AnalyticsResponse | undefined,
    ) => React.ReactNode,
  ) => {
    const query = queries[index];
    if (query.isPending) return <Skeleton className="h-64" />;
    if (query.isError)
      return (
        <ErrorState
          error={query.error}
          retry={() => query.refetch()}
          title="Раздел не загрузился"
        />
      );
    return render(query.data, queries[index + 4].data);
  };
  return (
    <section
      ref={ref}
      className="editorial-section"
      aria-labelledby="month-review-title"
    >
      <div className="mb-8 flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="eyebrow mb-3">Итоги месяца</p>
          <h2
            id="month-review-title"
            className="text-3xl font-medium tracking-tight sm:text-4xl"
          >
            {monthNames[month]} <span className="text-muted">{year}</span>
          </h2>
          <p className="mt-3 text-sm text-muted">
            {incomplete
              ? `Месяц продолжается · данные за ${elapsed} дн.`
              : "Завершённый месяц"}{" "}
            · средние только по дням с записями
          </p>
        </div>
        <div className="flex gap-1">
          <Button
            variant="ghost"
            size="icon"
            aria-label="Предыдущий месяц"
            onClick={() => shiftMonth(-1)}
          >
            <ChevronLeft className="size-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            aria-label="Следующий месяц"
            disabled={selected >= today.slice(0, 7)}
            onClick={() => shiftMonth(1)}
          >
            <ChevronRight className="size-4" />
          </Button>
        </div>
      </div>
      <div className="grid gap-9 md:grid-cols-2 2xl:grid-cols-4">
        <section className="min-w-0">
          <h3 className="eyebrow mb-5">Восстановление</h3>
          {panel(0, (data, before) => {
            const daily = data.daily ?? [];
            const sleep = observedStats(daily, "sleep_seconds");
            return (
              <>
                <MetricValue
                  value={sleep.average}
                  metric="sleep_seconds"
                  className="text-4xl"
                />
                <p className="mb-5 mt-2 text-xs text-muted">
                  средний сон · {sleep.count} из {elapsed} дней
                </p>
                <dl>
                  {[
                    ["recovery_score", "Восстановление", "%", "п.п."],
                    ["hrv_ms", "HRV", "мс", "мс"],
                    [
                      "resting_heart_rate_bpm",
                      "Пульс покоя",
                      "уд/мин",
                      "уд/мин",
                    ],
                    ["daily_strain", "Нагрузка", "", ""],
                    ["sleep_seconds", "Сон", "", ""],
                  ].map(([key, label, unit, deltaUnit]) => (
                    <ReviewRow
                      key={key}
                      label={label}
                      value={metricText(
                        observedStats(daily, key).average,
                        key,
                        unit,
                      )}
                      note={comparison(
                        daily,
                        before?.daily ?? [],
                        key,
                        deltaUnit,
                        elapsed,
                        previousDays,
                      )}
                    />
                  ))}
                </dl>
              </>
            );
          })}
        </section>
        <section className="min-w-0">
          <h3 className="eyebrow mb-5">Тренировки</h3>
          {panel(2, (data, before) => {
            const summary = data.summary ?? {};
            const sessions = numeric(summary.sessions);
            const previousSessions = numeric(before?.summary?.sessions);
            const minutes = observedStats(
              data.daily_duration ?? [],
              "duration_minutes",
            ).sum;
            return (
              <>
                <MetricValue value={sessions} className="text-4xl" />
                <p className="mb-5 mt-2 text-xs text-muted">
                  завершено
                  {!incomplete && sessions !== null && previousSessions !== null
                    ? ` · ${sessions - previousSessions > 0 ? "+" : ""}${sessions - previousSessions} к прошлому месяцу`
                    : " за месяц"}
                </p>
                <dl>
                  <ReviewRow
                    label="Время тренировок"
                    value={metricText(
                      minutes === null ? null : minutes * 60,
                      "duration_seconds",
                    )}
                  />
                  <ReviewRow
                    label="Поднятый вес"
                    value={metricText(
                      numeric(summary.volume_kg) === null
                        ? null
                        : Number(summary.volume_kg) / 1000,
                      "",
                      "т",
                    )}
                  />
                  <ReviewRow
                    label="Рабочие подходы"
                    value={metricText(summary.working_sets, "")}
                  />
                  <ReviewRow
                    label="Личные рекорды"
                    value={metricText(summary.personal_records, "")}
                  />
                </dl>
              </>
            );
          })}
        </section>
        <section className="min-w-0">
          <h3 className="eyebrow mb-5">Питание</h3>
          {panel(1, (data, before) => {
            const daily = data.daily ?? [];
            const calories = observedStats(daily, "calories_kcal");
            return (
              <>
                <MetricValue
                  value={calories.average}
                  unit="ккал"
                  className="text-4xl"
                />
                <p className="mb-5 mt-2 text-xs text-muted">
                  в среднем · {calories.count} из {elapsed} дней
                </p>
                <dl>
                  {[
                    ["calories_kcal", "Калории", "ккал"],
                    ["protein_g", "Белок", "г"],
                    ["fat_g", "Жиры", "г"],
                    ["carbohydrates_g", "Углеводы", "г"],
                  ].map(([key, label, unit]) => {
                    const stats = observedStats(daily, key);
                    return (
                      <ReviewRow
                        key={key}
                        label={label}
                        value={metricText(stats.sum, key, unit)}
                        note={
                          stats.count
                            ? `${metricText(stats.average, key, unit)} / день · ${stats.count} дн.`
                            : undefined
                        }
                      />
                    );
                  })}
                </dl>
                {comparison(
                  daily,
                  before?.daily ?? [],
                  "calories_kcal",
                  "ккал",
                  elapsed,
                  previousDays,
                ) && (
                  <p className="mt-3 text-xs text-muted">
                    Среднее:{" "}
                    {comparison(
                      daily,
                      before?.daily ?? [],
                      "calories_kcal",
                      "ккал",
                      elapsed,
                      previousDays,
                    )}
                  </p>
                )}
              </>
            );
          })}
        </section>
        <section className="min-w-0">
          <h3 className="eyebrow mb-5">Состав тела</h3>
          {panel(3, (data) => {
            const daily = data.daily ?? [];
            const weight = observedStats(daily, "weight_kg");
            return (
              <>
                <p className="metric-number text-4xl font-medium">
                  {weight.change === null ? (
                    <span className="text-base text-muted">Без данных</span>
                  ) : (
                    `${weight.change > 0 ? "+" : ""}${formatNumber(weight.change)}`
                  )}
                  <span className="ml-1 text-base text-muted">
                    {weight.change !== null ? "кг" : ""}
                  </span>
                </p>
                <p className="mb-5 mt-2 text-xs text-muted">
                  изменение веса · {weight.count} дней с измерениями
                </p>
                <dl>
                  {[
                    ["weight_kg", "Вес", "кг", "кг"],
                    ["body_fat_percent", "Жир", "%", "п.п."],
                    ["skeletal_muscle_mass_kg", "Мышцы", "кг", "кг"],
                    ["lean_mass_kg", "Безжировая масса", "кг", "кг"],
                  ].map(([key, label, unit, deltaUnit]) => {
                    const stats = observedStats(daily, key);
                    return (
                      <ReviewRow
                        key={key}
                        label={label}
                        value={
                          stats.count >= 2
                            ? `${metricText(stats.first, key, unit)} → ${metricText(stats.last, key, unit)}`
                            : metricText(stats.last, key, unit)
                        }
                        note={
                          stats.change !== null
                            ? `Δ ${stats.change > 0 ? "+" : ""}${metricText(stats.change, key, deltaUnit)}`
                            : stats.count === 1
                              ? "Одно измерение"
                              : undefined
                        }
                      />
                    );
                  })}
                </dl>
              </>
            );
          })}
        </section>
      </div>
      {queries.slice(4).some((query) => query.isError) && (
        <p className="mt-6 text-xs text-muted">
          Часть данных прошлого месяца не загрузилась. Сравнения показаны только
          для доступных разделов.{" "}
          <button
            className="text-accent underline"
            onClick={() => {
              queries
                .slice(4)
                .filter((query) => query.isError)
                .forEach((query) => {
                  void query.refetch();
                });
            }}
          >
            Повторить
          </button>
        </p>
      )}
    </section>
  );
}
