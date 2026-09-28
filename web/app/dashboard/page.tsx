"use client";

import { Suspense, useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowRight, Plus } from "lucide-react";
import Link from "next/link";
import { apiFetch, listItems, type ListResponse } from "@/lib/api";
import type {
  AnalyticsResponse,
  Overview,
  Settings,
  WorkoutSession,
} from "@/lib/types";
import { useRangeSearch } from "@/lib/hooks";
import {
  PageSkeleton,
  ErrorState,
  EmptyState,
  Skeleton,
} from "@/components/ui/states";
import { MetricGrid } from "@/components/charts/metric-card";
import {
  formatDate,
  dateInTimeZone,
  formatDuration,
  formatNumber,
  getDashboardTimezone,
  shiftISODate,
} from "@/lib/format";
import { comparisonSummaryMetrics } from "@/lib/metrics";
import { MetricSwitcherChart } from "@/components/charts/metric-switcher-chart";
import { TrendChart } from "@/components/charts/trend-chart";
import { DateRangeControls } from "@/components/layout/date-range-controls";
import { YearHistory } from "@/components/dashboard/year-history";
import { MonthReview } from "@/components/dashboard/month-review";
import { InBodyExplorer } from "@/components/body/inbody-explorer";
import { MetricValue } from "@/components/dashboard/metric-value";
import { metricText, numeric, rangeParams } from "@/lib/dashboard";

const statusLabels: Record<string, string> = {
  finished: "Завершена",
  active: "Идёт сейчас",
  scheduled: "По плану",
  cancelled: "Отменена",
  excused: "Пропущена",
};
function SessionLink({ session }: { session: WorkoutSession }) {
  return (
    <Link
      href={`/dashboard/training/sessions/${session.id}`}
      className="group flex min-w-0 items-center justify-between gap-4 border-b border-line py-5 transition-colors last:border-0 hover:text-accent"
    >
      <div className="min-w-0">
        <p className="mb-2 text-xs text-muted">
          {formatDate(
            session.calendar_date ??
              session.scheduled_date ??
              session.actual_date ??
              session.date,
          )}{" "}
          · {statusLabels[session.status ?? ""] ?? session.status}
        </p>
        <h3 className="break-words text-lg font-medium tracking-tight">
          {session.template_name ??
            session.plan_name ??
            session.program_name ??
            "Тренировка"}
        </h3>
        <p className="mt-3 flex flex-wrap gap-x-5 gap-y-2 text-xs tabular-nums text-muted">
          <span>{formatDuration(session.duration_seconds)}</span>
          <span>{formatNumber(session.working_sets)} подх.</span>
          <span>{formatNumber(session.volume_kg, {}, " кг")}</span>
          {numeric(session.strain) !== null && (
            <span>Нагрузка {formatNumber(session.strain)}</span>
          )}
        </p>
      </div>
      <ArrowRight className="size-4 shrink-0 text-muted transition-colors group-hover:text-accent" />
    </Link>
  );
}
function OverviewContent() {
  const { query: range } = useRangeSearch();
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(new Date()), 60_000);
    return () => window.clearInterval(timer);
  }, []);
  const today = dateInTimeZone(now);
  const todayRange = rangeParams(shiftISODate(today, -29), today);
  const settings = useQuery({
    queryKey: ["settings"],
    queryFn: () => apiFetch<Settings>("/settings"),
  });
  const recovery = useQuery({
    queryKey: ["today-recovery", todayRange],
    queryFn: () =>
      apiFetch<AnalyticsResponse>(`/analytics/recovery?${todayRange}`),
  });
  const nutrition = useQuery({
    queryKey: ["today-nutrition", todayRange],
    queryFn: () =>
      apiFetch<AnalyticsResponse>(`/analytics/nutrition?${todayRange}`),
  });
  const overview = useQuery({
    queryKey: ["overview", range],
    queryFn: () => apiFetch<Overview>(`/dashboard/overview?${range}`),
  });
  const sessions = useQuery({
    queryKey: ["recent-workouts", today],
    queryFn: () =>
      apiFetch<ListResponse<WorkoutSession>>(
        `/workout-sessions?to=${today}&status=finished&page=1&page_size=3`,
      ),
  });
  const state = recovery.data?.daily?.find((point) => point.date === today);
  const food = nutrition.data?.daily?.find((point) => point.date === today);
  const hour = Number(
    new Intl.DateTimeFormat("en", {
      hour: "numeric",
      hourCycle: "h23",
      timeZone: getDashboardTimezone(),
    }).format(now),
  );
  const greeting =
    hour < 6
      ? "Доброй ночи"
      : hour < 12
        ? "Доброе утро"
        : hour < 18
          ? "Добрый день"
          : "Добрый вечер";
  const todaySessions =
    overview.data?.today_sessions ?? overview.data?.sessions ?? [];
  const calorieTarget = settings.data?.calorie_target_kcal;
  const calories = numeric(food?.calories_kcal);
  const weekStart = overview.data?.weekly_range?.from;
  return (
    <div className="space-y-[var(--space-section)]">
      <section aria-labelledby="today-title">
        <div className="mb-8 flex flex-wrap items-start justify-between gap-4">
          <div>
            <p className="eyebrow mb-3">
              {new Intl.DateTimeFormat("ru-RU", {
                weekday: "long",
                day: "numeric",
                month: "long",
                timeZone: getDashboardTimezone(),
              }).format(now)}
            </p>
            <h1
              id="today-title"
              className="text-3xl font-medium tracking-[-.035em] sm:text-4xl"
            >
              {greeting}
            </h1>
          </div>
          <p className="hidden pt-1 text-xs text-muted sm:block">
            Ваше состояние. В контексте.
          </p>
        </div>
        <div className="grid gap-8 md:grid-cols-[1.2fr_3fr] md:gap-10">
          <Link
            href="/dashboard/recovery"
            className="group block border-b border-line pb-6 md:border-b-0 md:border-r md:pb-0 md:pr-8"
          >
            <p className="mb-4 flex items-center gap-3 text-sm text-muted">
              Восстановление{" "}
              <ArrowRight className="size-3.5 transition-transform group-hover:translate-x-1" />
            </p>
            {recovery.isPending ? (
              <Skeleton className="h-20 w-40" />
            ) : (
              <MetricValue
                value={state?.recovery_score}
                unit="%"
                className="text-[76px] text-accent sm:text-[88px]"
              />
            )}
            <p className="mt-4 text-xs text-muted">WHOOP · сегодня</p>
          </Link>
          <div className="grid grid-cols-2 gap-x-6 gap-y-7 sm:grid-cols-3">
            {[
              ["sleep_seconds", "Сон", ""],
              ["hrv_ms", "HRV", "мс"],
              ["resting_heart_rate_bpm", "Пульс покоя", "уд/мин"],
              ["daily_strain", "Нагрузка", ""],
              ["sleep_performance_percent", "Качество сна", "%"],
            ].map(([key, label, unit]) => (
              <Link
                key={key}
                href="/dashboard/recovery"
                className="min-w-0 transition-opacity hover:opacity-75"
              >
                <p className="mb-3 text-xs text-muted">{label}</p>
                {recovery.isPending ? (
                  <Skeleton className="h-9 w-24" />
                ) : (
                  <MetricValue
                    value={state?.[key]}
                    metric={key}
                    unit={unit}
                    className="text-[clamp(1.5rem,2.8vw,2.5rem)]"
                  />
                )}
              </Link>
            ))}
            <Link
              href="/dashboard/nutrition"
              className="min-w-0 transition-opacity hover:opacity-75"
            >
              <p className="mb-3 text-xs text-muted">Питание</p>
              {nutrition.isPending ? (
                <Skeleton className="h-9 w-24" />
              ) : (
                <MetricValue
                  value={calories}
                  unit="ккал"
                  className="text-[clamp(1.5rem,2.8vw,2.5rem)]"
                />
              )}
              <p className="mt-2 text-xs text-muted">
                Белок · {metricText(food?.protein_g, "", "г")}
              </p>
            </Link>
          </div>
        </div>
        {recovery.isError && (
          <div className="mt-6">
            <ErrorState
              error={recovery.error}
              retry={() => recovery.refetch()}
              title="WHOOP: не удалось загрузить показатели"
            />
          </div>
        )}
        <div className="mt-8 flex flex-wrap items-center justify-between gap-x-8 gap-y-3 border-t border-line pt-5 text-xs text-muted">
          <p>
            Вес сегодня{" "}
            <span className="ml-2 tabular-nums text-ink">
              {metricText(overview.data?.today?.weight_kg, "", "кг")}
            </span>
          </p>
          <p>
            Выполнение плана{" "}
            <span className="ml-2 tabular-nums text-ink">
              {metricText(
                overview.data?.today?.plan_adherence_percent,
                "",
                "%",
              )}
            </span>
          </p>
          <Link
            href="/dashboard/training"
            className="inline-flex min-h-8 items-center gap-2 text-accent"
          >
            {todaySessions.length
              ? `${todaySessions.length} трен. сегодня`
              : "Тренировки сегодня"}
            <ArrowRight className="size-3.5" />
          </Link>
        </div>
      </section>
      <YearHistory today={today} settings={settings.data ?? {}} />
      <section className="editorial-section" aria-labelledby="body-title">
        <div className="mb-8 flex flex-wrap items-end justify-between gap-3">
          <div>
            <p className="eyebrow mb-3">Тело в динамике</p>
            <h2 id="body-title" className="section-title">
              Больше, чем вес
            </h2>
          </div>
          <Link href="/dashboard/body" className="text-link">
            История и показатели <ArrowRight className="size-4" />
          </Link>
        </div>
        <InBodyExplorer />
      </section>
      <section className="editorial-section" aria-labelledby="training-title">
        <div className="mb-5 flex flex-wrap items-end justify-between gap-4">
          <div>
            <p className="eyebrow mb-3">Тренировки</p>
            <h2 id="training-title" className="section-title">
              Ритм вашей недели
            </h2>
          </div>
          <Link href="/dashboard/training?action=new" className="text-link">
            <Plus className="size-4" />
            Добавить тренировку
          </Link>
        </div>
        {overview.isError ? (
          <ErrorState
            error={overview.error}
            retry={() => overview.refetch()}
            title="Не удалось загрузить неделю"
          />
        ) : overview.isPending ? (
          <Skeleton className="h-28" />
        ) : (
          weekStart && (
            <div className="grid grid-cols-2 gap-x-6 gap-y-5 py-4 sm:grid-cols-4 xl:grid-cols-7">
              {Array.from({ length: 7 }, (_, index) =>
                shiftISODate(weekStart, index),
              ).map((day) => {
                const items = (overview.data.weekly_sessions ?? []).filter(
                  (session) =>
                    (session.calendar_date ??
                      session.scheduled_date ??
                      session.actual_date ??
                      session.date) === day,
                );
                return (
                  <div
                    key={day}
                    className={`border-t-2 pt-3 ${day === today ? "border-accent" : "border-line"}`}
                  >
                    <p className="mb-3 text-xs text-muted">
                      {new Intl.DateTimeFormat("ru-RU", {
                        weekday: "short",
                        day: "numeric",
                        timeZone: "UTC",
                      }).format(new Date(`${day}T12:00:00Z`))}
                    </p>
                    {items.length ? (
                      items.map((session) => (
                        <Link
                          key={session.id}
                          href={`/dashboard/training/sessions/${session.id}`}
                          className="mb-2 block break-words text-sm hover:text-accent"
                        >
                          {session.template_name ??
                            session.plan_name ??
                            "Тренировка"}
                          <span className="mt-1 block text-xs text-muted">
                            {statusLabels[session.status ?? ""] ??
                              session.status}
                          </span>
                        </Link>
                      ))
                    ) : (
                      <p className="text-xs text-muted/70">Без тренировки</p>
                    )}
                  </div>
                );
              })}
            </div>
          )
        )}
        <div className="mt-7 grid gap-8 lg:grid-cols-2">
          <div>
            <h3 className="eyebrow mb-2">Сегодня</h3>
            {overview.isPending ? (
              <Skeleton className="h-32" />
            ) : overview.isError ? (
              <p className="py-5 text-sm text-muted">Без данных</p>
            ) : todaySessions.length ? (
              todaySessions.map((session) => (
                <SessionLink key={session.id} session={session} />
              ))
            ) : (
              <p className="py-6 text-sm text-muted">
                На сегодня тренировок нет. Новую сессию можно добавить в любой
                момент.
              </p>
            )}
          </div>
          <div>
            <h3 className="eyebrow mb-2">Последние завершённые</h3>
            {sessions.isPending ? (
              <Skeleton className="h-32" />
            ) : sessions.isError ? (
              <ErrorState
                error={sessions.error}
                retry={() => sessions.refetch()}
              />
            ) : listItems(sessions.data).length ? (
              listItems(sessions.data).map((session) => (
                <SessionLink key={session.id} session={session} />
              ))
            ) : (
              <EmptyState
                title="Первые тренировки ещё впереди"
                description="Завершённые сессии появятся здесь."
              />
            )}
          </div>
        </div>
      </section>
      <section className="editorial-section" aria-labelledby="nutrition-title">
        <div className="mb-8 flex items-end justify-between gap-4">
          <div>
            <p className="eyebrow mb-3">Питание</p>
            <h2 id="nutrition-title" className="section-title">
              Сегодня и в среднем
            </h2>
          </div>
          <Link className="text-link" href="/dashboard/nutrition">
            Дневник <ArrowRight className="size-4" />
          </Link>
        </div>
        {nutrition.isError ? (
          <ErrorState
            error={nutrition.error}
            retry={() => nutrition.refetch()}
            title="Питание временно недоступно"
          />
        ) : (
          <div className="grid gap-8 lg:grid-cols-[1fr_2fr]">
            <div>
              <MetricValue value={calories} unit="ккал" className="text-5xl" />
              <p className="mt-3 text-xs text-muted">
                {calorieTarget
                  ? `Цель ${formatNumber(calorieTarget)} ккал`
                  : "Сегодня · калорийная цель не задана"}
              </p>
              {calorieTarget && calories !== null && (
                <div
                  className="mt-4 h-1 overflow-hidden rounded-full bg-elevated"
                  role="meter"
                  aria-label="Калории относительно цели"
                  aria-valuemin={0}
                  aria-valuemax={calorieTarget}
                  aria-valuenow={Math.min(calories, calorieTarget)}
                  aria-valuetext={`${formatNumber(calories)} из ${formatNumber(calorieTarget)} ккал`}
                >
                  <div
                    className="h-full bg-accent"
                    style={{
                      width: `${Math.min((calories / calorieTarget) * 100, 100)}%`,
                    }}
                  />
                </div>
              )}
              <dl className="mt-6">
                {[
                  ["protein_g", "Белок", settings.data?.protein_target_g],
                  ["fat_g", "Жиры", settings.data?.fat_target_g],
                  [
                    "carbohydrates_g",
                    "Углеводы",
                    settings.data?.carbohydrates_target_g,
                  ],
                ].map(([key, label, target]) => (
                  <div key={String(key)} className="body-detail-row">
                    <dt className="text-sm text-muted">{label}</dt>
                    <dd className="text-sm tabular-nums">
                      {metricText(food?.[String(key)], "", "г")}
                      {typeof target === "number" && (
                        <span className="ml-2 text-xs text-muted">
                          / {formatNumber(target)} г
                        </span>
                      )}
                    </dd>
                  </div>
                ))}
              </dl>
            </div>
            <TrendChart
              title="Последние 30 дней"
              description="Фактические дневные итоги питания"
              data={nutrition.data?.daily}
              series={[
                { key: "calories_kcal", label: "Калории", unit: "ккал" },
              ]}
              height={220}
            />
          </div>
        )}
      </section>
      <MonthReview today={today} />
      <section className="editorial-section" aria-labelledby="trends-title">
        <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
          <div>
            <p className="eyebrow mb-3">История</p>
            <h2 id="trends-title" className="section-title">
              Посмотрите на изменения
            </h2>
          </div>
          <DateRangeControls />
        </div>
        {overview.isPending ? (
          <Skeleton className="h-72" />
        ) : overview.isError ? (
          <ErrorState error={overview.error} retry={() => overview.refetch()} />
        ) : (
          <div className="space-y-8">
            {overview.data.comparison && (
              <div>
                <p className="mb-3 text-sm text-muted">
                  Выбранный период против предыдущего
                </p>
                <MetricGrid
                  metrics={comparisonSummaryMetrics(
                    overview.data.summary,
                    overview.data.comparison,
                  )}
                />
              </div>
            )}
            <div className="grid min-w-0 gap-6 xl:grid-cols-2">
              <MetricSwitcherChart
                title="Восстановление"
                data={overview.data.daily}
                metrics={[
                  { key: "recovery_score", label: "Восстановление", unit: "%" },
                  { key: "hrv_ms", label: "HRV", unit: "мс" },
                  {
                    key: "resting_heart_rate_bpm",
                    label: "Пульс покоя",
                    unit: "уд/мин",
                  },
                  { key: "sleep_seconds", label: "Сон" },
                  { key: "daily_strain", label: "Нагрузка" },
                ]}
              />
              <MetricSwitcherChart
                title="Состав тела"
                data={overview.data.daily}
                metrics={[
                  { key: "weight_kg", label: "Вес", unit: "кг" },
                  {
                    key: "weight_7d_average",
                    label: "Среднее за 7 дней",
                    unit: "кг",
                  },
                  { key: "body_fat_percent", label: "Жир", unit: "%" },
                  { key: "fat_mass_kg", label: "Жировая масса", unit: "кг" },
                  {
                    key: "lean_mass_kg",
                    label: "Безжировая масса",
                    unit: "кг",
                  },
                ]}
              />
            </div>
            <div>
              <h3 className="eyebrow mb-4">Наблюдения за период</h3>
              {overview.data.highlights?.length ? (
                <div className="divide-y divide-line">
                  {overview.data.highlights.map((item, index) => (
                    <article key={item.id ?? index} className="py-4">
                      <div className="flex flex-wrap justify-between gap-2">
                        <h4 className="text-sm font-medium">{item.title}</h4>
                        {item.date && (
                          <span className="text-xs text-muted">
                            {formatDate(item.date)}
                          </span>
                        )}
                      </div>
                      <p className="mt-2 text-sm leading-6 text-muted">
                        {item.description}
                      </p>
                      {item.rule && (
                        <details className="mt-2 text-xs text-muted">
                          <summary className="cursor-pointer py-2">
                            Основание
                          </summary>
                          <p>{item.rule}</p>
                        </details>
                      )}
                    </article>
                  ))}
                </div>
              ) : (
                <p className="text-sm text-muted">
                  За этот период новых наблюдений нет.
                </p>
              )}
            </div>
          </div>
        )}
      </section>
    </div>
  );
}
export default function OverviewPage() {
  return (
    <Suspense fallback={<PageSkeleton />}>
      <OverviewContent />
    </Suspense>
  );
}
