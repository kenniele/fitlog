"use client";

import { Suspense } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowRight, CalendarCheck2 } from "lucide-react";
import Link from "next/link";
import { apiFetch } from "@/lib/api";
import type { Overview, Settings, WorkoutSession } from "@/lib/types";
import { useRangeSearch } from "@/lib/hooks";
import { PageHeader, SectionHeader } from "@/components/ui/page";
import { PageSkeleton, ErrorState, EmptyState } from "@/components/ui/states";
import { MetricGrid } from "@/components/charts/metric-card";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { formatDate, formatDuration, formatNumber, shiftISODate } from "@/lib/format";
import { comparisonSummaryMetrics, dailyPointMetrics } from "@/lib/metrics";
import { MetricSwitcherChart } from "@/components/charts/metric-switcher-chart";
import { isRecord } from "@/lib/utils";

const todayDefinitions = [
  { key: "recovery_score", label: "Recovery", unit: "%", aliases: ["recovery"] },
  { key: "sleep_seconds", label: "Сон", duration: true, aliases: ["sleep"] },
  { key: "sleep_performance_percent", label: "Sleep Performance", unit: "%" },
  { key: "hrv_ms", label: "HRV", unit: "мс", aliases: ["hrv"] },
  { key: "resting_heart_rate_bpm", label: "Resting HR", unit: "bpm", aliases: ["rhr"] },
  { key: "daily_strain", label: "Daily Strain", aliases: ["strain"] },
  { key: "calories_kcal", label: "Калории", unit: "ккал", aliases: ["calories"] },
  { key: "protein_g", label: "Белок", unit: "г", aliases: ["protein"] },
  { key: "weight_kg", label: "Вес", unit: "кг", aliases: ["weight"] },
  { key: "plan_adherence_percent", label: "Выполнение плана", unit: "%", aliases: ["adherence_percent"] },
];

function statusTone(status: string | null | undefined) {
  if (status === "finished") return "good" as const;
  if (status === "cancelled" || status === "excused") return "warning" as const;
  return "neutral" as const;
}

function SessionLink({ session, compact = false }: { session: WorkoutSession; compact?: boolean }) {
  return (
    <Link href={`/dashboard/training/sessions/${session.id}`} className="block rounded-control border border-line bg-white/[.025] p-3 transition hover:border-white/15">
      <span className="block truncate text-sm font-medium">{session.template_name ?? session.plan_name ?? session.program_name ?? "Тренировка"}</span>
      <span className="mt-2 flex items-center justify-between gap-2">
        <Badge tone={statusTone(session.status)}>{session.status ?? "scheduled"}</Badge>
        {!compact && <ArrowRight className="size-4 text-muted" />}
      </span>
      <span className="mt-2 block text-xs text-muted">
        {formatDuration(session.duration_seconds)} · {formatNumber(session.working_sets)} подх. · {formatNumber(session.volume_kg, {}, " кг")}
      </span>
    </Link>
  );
}

function WeeklyTraining({ data }: { data: Overview }) {
  const first = data.weekly_range?.from;
  if (!first) return <Card><EmptyState title="Диапазон недели недоступен" description="Проверьте первый день недели в настройках." /></Card>;
  const sessions = data.weekly_sessions ?? [];
  const days = Array.from({ length: 7 }, (_, index) => shiftISODate(first, index));
  return (
    <section>
      <SectionHeader title="Weekly Training" description="Завершённые и запланированные сессии текущей недели в timezone профиля." className="mb-3" />
      <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-7">
        {days.map((day) => {
          const daySessions = sessions.filter((session) => (session.calendar_date ?? session.scheduled_date ?? session.actual_date ?? session.date) === day);
          return (
            <Card key={day} className="min-w-0 p-3">
              <p className="text-[11px] font-semibold uppercase tracking-wider text-muted">{formatDate(day)}</p>
              {daySessions.length ? <div className="mt-3 space-y-2">{daySessions.map((session) => <SessionLink key={session.id} session={session} compact />)}</div> : <p className="mt-3 text-xs text-muted">Нет сессии</p>}
            </Card>
          );
        })}
      </div>
    </section>
  );
}

function OverviewContent() {
  const { query: range } = useRangeSearch();
  const query = useQuery({ queryKey: ["overview", range], queryFn: () => apiFetch<Overview>(`/dashboard/overview?${range}`) });
  const settings = useQuery({ queryKey: ["settings"], queryFn: () => apiFetch<Settings>("/settings") });
  if (query.isPending || settings.isPending) return <PageSkeleton />;
  if (query.isError || settings.isError) return <ErrorState error={query.error ?? settings.error} retry={() => { void Promise.all([query.refetch(), settings.refetch()]); }} />;
  const data = query.data;
  const today = dailyPointMetrics(data.today, data.summary, todayDefinitions, data.daily, data.previous_day);
  const todaySessions = data.today_sessions ?? data.sessions ?? [];
  const calorieTarget = settings.data.calorie_target_kcal;
  const overviewDaily = (data.daily ?? []).map((point) => ({ ...point, calorie_target_kcal: calorieTarget ?? null }));
  const nutritionSummary = isRecord(data.summary) && isRecord(data.summary.nutrition) ? data.summary.nutrition : null;
  const caloriesSummary = nutritionSummary && isRecord(nutritionSummary.calories) ? nutritionSummary.calories : null;
  const nutritionContext = `Среднее: ${formatNumber(typeof caloriesSummary?.average === "number" ? caloriesSummary.average : null, {}, " ккал")} · дней в цели: ${formatNumber(typeof nutritionSummary?.days_in_target === "number" ? nutritionSummary.days_in_target : null)}`;

  return (
    <>
      <PageHeader eyebrow="Сегодня" title="Состояние, нагрузка и следующий шаг" description="Живой обзор выбранного периода. Пустые значения остаются пустыми — FitLog не подменяет их нулями." />
      <MetricGrid metrics={today} order={todayDefinitions.map(({ key, label }) => ({ key, label }))} />
      {data.comparison && <section><SectionHeader title="Период против предыдущего" description="Дельта — абсолютная разница сводных значений выбранного и предыдущего периодов." className="mb-3" /><MetricGrid metrics={comparisonSummaryMetrics(data.summary, data.comparison)} /></section>}

      <section className="grid min-w-0 gap-4 xl:grid-cols-[1.5fr_1fr]">
        <MetricSwitcherChart title="Recovery trend" description="Одна шкала за раз: Recovery, HRV, RHR, сон или strain." data={data.daily} metrics={[{ key: "recovery_score", label: "Recovery", variant: "area" }, { key: "hrv_ms", label: "HRV" }, { key: "resting_heart_rate_bpm", label: "RHR" }, { key: "sleep_seconds", label: "Сон", variant: "area" }, { key: "daily_strain", label: "Strain", variant: "bar" }]} />
        <Card className="p-5">
          <SectionHeader title="Today" description="Только сегодняшние план и фактическая нагрузка" />
          {todaySessions.length ? <div className="mt-4 space-y-3">{todaySessions.map((session) => <SessionLink key={session.id} session={session} />)}</div> : <EmptyState title="Сегодня тренировок нет" description="Нет запланированной или завершённой сессии на сегодня." />}
          <Link href="/dashboard/training?action=new" className="mt-4 flex items-center gap-2 text-sm font-medium text-accent hover:underline"><CalendarCheck2 className="size-4" />Добавить тренировку</Link>
        </Card>
      </section>

      <WeeklyTraining data={data} />

      <section className="grid min-w-0 gap-4 xl:grid-cols-2">
        <MetricSwitcherChart title="Nutrition progress" description={nutritionContext} data={overviewDaily} metrics={[{ key: "calories_kcal", label: "Калории", variant: "bar" }, { key: "calorie_target_kcal", label: "Цель" }, { key: "protein_g", label: "Белок", variant: "bar" }, { key: "fat_g", label: "Жиры", variant: "bar" }, { key: "carbohydrates_g", label: "Углеводы", variant: "bar" }, { key: "fiber_g", label: "Клетчатка", variant: "bar" }]} />
        <MetricSwitcherChart title="Body trend" description="Вес, 7-дневное среднее и состав тела; прогноз не строится." data={data.daily} metrics={[{ key: "weight_kg", label: "Вес" }, { key: "weight_7d_average", label: "Среднее 7д" }, { key: "body_fat_percent", label: "Жир, %" }, { key: "fat_mass_kg", label: "Жировая масса" }, { key: "lean_mass_kg", label: "Безжировая масса" }]} />
      </section>

      <section>
        <SectionHeader title="Highlights" description="Прозрачные rule-based наблюдения, без фиктивного AI." className="mb-3" />
        {data.highlights?.length ? <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">{data.highlights.map((item, index) => <Card key={item.id ?? index} className="p-4"><div className="flex items-center justify-between gap-2"><Badge tone={item.type === "positive" ? "good" : item.type === "warning" ? "warning" : item.type === "critical" ? "critical" : "neutral"}>{item.type ?? "наблюдение"}</Badge><span className="text-[11px] text-muted">{formatDate(item.date)}</span></div><h3 className="mt-3 text-sm font-semibold">{item.title}</h3><p className="mt-1 text-sm leading-6 text-muted">{item.description ?? "—"}</p>{item.rule && <p className="mt-3 border-t border-line pt-3 text-[11px] text-muted">Правило: {item.rule}</p>}</Card>)}</div> : <Card><EmptyState title="Нет новых наблюдений" description="Правила не нашли событий, заслуживающих внимания в этом периоде." /></Card>}
      </section>
      <p className="sr-only">Сводное значение: {formatNumber(Object.keys(data.summary ?? {}).length)}</p>
    </>
  );
}

export default function OverviewPage() {
  return <Suspense fallback={<PageSkeleton />}><OverviewContent /></Suspense>;
}
