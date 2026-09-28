"use client";

import { Suspense, useCallback, useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { apiFetch, type ListResponse } from "@/lib/api";
import type { AnalyticsResponse, BodyMeasurement } from "@/lib/types";
import { useQuickAction, useRangeSearch } from "@/lib/hooks";
import { PageHeader } from "@/components/ui/page";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ErrorState, InlineError, PageSkeleton } from "@/components/ui/states";
import { MetricGrid } from "@/components/charts/metric-card";
import { TrendChart } from "@/components/charts/trend-chart";
import { MetricSwitcherChart } from "@/components/charts/metric-switcher-chart";
import { CalendarHeatmap } from "@/components/charts/calendar-heatmap";
import { BodyForm } from "@/components/forms/record-forms";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { daysBetweenISO, formatDate, formatNumber } from "@/lib/format";
import { comparisonSummaryMetrics, summaryToMetrics } from "@/lib/metrics";
import { InBodyExplorer } from "@/components/body/inbody-explorer";
import { BodyFatEstimateCard } from "@/components/body/body-fat-estimate";
import { BodyHistory, bodyHistoryPath, type BodyHistoryScope } from "@/components/body/body-history";

function BodyContent() {
  const { query: range } = useRangeSearch();
  const client = useQueryClient();
  const [page, setPage] = useState(1);
  const [historyScope, setHistoryScope] = useState<BodyHistoryScope>("all");
  const [form, setForm] = useState(false);
  const [editing, setEditing] = useState<BodyMeasurement | null>(null);
  const [deleting, setDeleting] = useState<BodyMeasurement | null>(null);
  const open = useCallback(() => { setEditing(null); setForm(true); }, []);
  useQuickAction(open);

  const analytics = useQuery({ queryKey: ["analytics-body", range], queryFn: () => apiFetch<AnalyticsResponse>(`/analytics/body?${range}`) });
  const measurements = useQuery({ queryKey: ["body-history", historyScope, page], queryFn: () => apiFetch<ListResponse<BodyMeasurement>>(bodyHistoryPath(historyScope, page)), placeholderData: keepPreviousData });
  const remove = useMutation({ mutationFn: (id: string | number) => apiFetch(`/body-measurements/${id}`, { method: "DELETE" }), onSuccess: async () => { setDeleting(null); await client.invalidateQueries(); } });

  if (analytics.isError || measurements.isError) return <ErrorState error={analytics.error ?? measurements.error} retry={() => { void Promise.all([analytics.refetch(), measurements.refetch()]); }} />;
  if (analytics.isPending || measurements.isPending) return <PageSkeleton />;

  const daily = analytics.data.daily ?? analytics.data.series ?? [];
  const averagedWeights = daily.filter((point) => typeof point.weight_7d_average === "number");
  const first = averagedWeights[0];
  const last = averagedWeights.at(-1);
  const elapsedDays = first && last ? daysBetweenISO(first.date, last.date) : 0;
  const weeklyRate = first && last && elapsedDays > 0 && typeof first.weight_7d_average === "number" && typeof last.weight_7d_average === "number"
    ? (last.weight_7d_average - first.weight_7d_average) / elapsedDays * 7
    : null;

  return <>
    <PageHeader eyebrow="InBody" title="Состав тела и InBody" description="История измерений состава тела и ориентировочная оценка жира по питанию после последнего InBody." actions={<Button variant="primary" onClick={() => { setEditing(null); setForm(true); }}><Plus className="size-4" />Добавить InBody</Button>} />
    <InBodyExplorer />
    <MetricGrid metrics={analytics.data.comparison ? comparisonSummaryMetrics(analytics.data.summary, analytics.data.comparison) : summaryToMetrics(analytics.data.summary)} order={[{ key: "weight", label: "Вес" }, { key: "body_fat", label: "Жир" }, { key: "skeletal_muscle_mass", label: "Скелетные мышцы" }, { key: "inbody_score", label: "InBody Score" }]} />
    <Card className="p-4"><p className="text-xs text-muted">Средняя скорость по 7-дневному весу</p><p className="mt-2 text-2xl font-semibold">{formatNumber(weeklyRate, { maximumFractionDigits: 2 }, " кг/нед")}</p><p className="mt-1 text-xs text-muted">{weeklyRate === null ? "Нужно минимум две полные 7-дневные точки." : `Расчёт по периоду ${formatDate(first?.date)} — ${formatDate(last?.date)}; это описание истории, не прогноз.`}</p></Card>
    <BodyHistory data={measurements.data} scope={historyScope} page={page} fetching={measurements.isFetching} onScopeChange={(scope) => { setHistoryScope(scope); setPage(1); }} onPageChange={setPage} onEdit={(entry) => { setEditing(entry); setForm(true); }} onDelete={setDeleting} />

    <BodyFatEstimateCard estimate={analytics.data.body_fat_estimate} />
    <div className="grid min-w-0 gap-4 xl:grid-cols-2">
      <TrendChart title="Вес и 7-дневное среднее" description="Среднее рассчитывается только по полному окну наблюдений." data={daily} series={[{ key: "weight_kg", label: "Вес" }, { key: "weight_7d_average", label: "Среднее 7д", color: "var(--accent-blue)" }]} />
      <TrendChart title="Жировая и безжировая масса" data={daily} series={[{ key: "fat_mass_kg", label: "Жировая масса", color: "var(--warning)" }, { key: "lean_mass_kg", label: "Безжировая", color: "var(--accent)" }, { key: "skeletal_muscle_mass_kg", label: "Скелетные мышцы", color: "var(--accent-blue)" }]} />
      <MetricSwitcherChart title="Процент жира и окружности" data={daily} metrics={[
        { key: "body_fat_percent", label: "Жир, %" },
        { key: "waist_cm", label: "Талия" },
        { key: "chest_cm", label: "Грудь" },
        { key: "biceps_cm", label: "Бицепс" },
        { key: "thigh_cm", label: "Бедро" },
      ]} />
      <TrendChart title="Водный баланс InBody" description="TBW = ICW + ECW; пропуски измерений оставляют разрывы." data={daily} series={[{ key: "total_body_water_l", label: "TBW, л", color: "var(--accent-blue)" }, { key: "intracellular_water_l", label: "ICW, л", color: "var(--accent)" }, { key: "extracellular_water_l", label: "ECW, л", color: "var(--warning)" }]} />
      <MetricSwitcherChart title="Расширенная динамика InBody" data={daily} metrics={[
        { key: "ecw_tbw_ratio", label: "ECW/TBW" }, { key: "visceral_fat_area_cm2", label: "Висцеральный жир, см²" },
        { key: "visceral_fat_level", label: "Уровень висцерального жира" }, { key: "basal_metabolic_rate_kcal", label: "BMR, ккал" },
        { key: "inbody_score", label: "InBody Score" }, { key: "phase_angle_degrees", label: "Фазовый угол" },
      ]} />
      <CalendarHeatmap title="История измерений веса" description="Пустые дни не трактуются как нулевой вес." data={daily} dataKey="weight_kg" unit="кг" color="var(--accent-blue)" />
    </div>
    <BodyForm open={form} onOpenChange={(value) => { setForm(value); if (!value) setEditing(null); }} entry={editing} />
    <InlineError error={remove.error} />
    <ConfirmDialog open={Boolean(deleting)} onOpenChange={(value) => { if (!value) setDeleting(null); }} title="Удалить измерение?" description="Эта точка исчезнет из истории состава тела." onConfirm={() => deleting && remove.mutate(deleting.id)} busy={remove.isPending} error={remove.error} />
  </>;
}

export default function BodyPage() { return <Suspense fallback={<PageSkeleton />}><BodyContent /></Suspense>; }
