"use client";

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { Moon, Pencil, Plus, Trash2 } from "lucide-react";
import { apiFetch, listItems, type ListResponse } from "@/lib/api";
import type { AnalyticsResponse, RecoveryEntry, SeriesPoint, Settings, SleepEntry } from "@/lib/types";
import { useQuickAction, useRangeSearch } from "@/lib/hooks";
import { PageHeader, SectionHeader } from "@/components/ui/page";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { DataTable } from "@/components/ui/data-table";
import { ErrorState, InlineError, PageSkeleton } from "@/components/ui/states";
import { MetricGrid } from "@/components/charts/metric-card";
import { TrendChart } from "@/components/charts/trend-chart";
import { MetricSwitcherChart } from "@/components/charts/metric-switcher-chart";
import { CalendarHeatmap } from "@/components/charts/calendar-heatmap";
import { RecoveryForm, SleepForm } from "@/components/forms/record-forms";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { formatDate, formatDuration, formatNumber, formatPercent } from "@/lib/format";
import { Badge } from "@/components/ui/badge";
import { comparisonSummaryMetrics, summaryToMetrics } from "@/lib/metrics";
import { PAGE_SIZE, Pagination } from "@/components/ui/pagination";

function numericAverage(points: SeriesPoint[], key: string) {
  const values = points.map((point) => point[key]).filter((value): value is number => typeof value === "number" && Number.isFinite(value));
  return values.length ? values.reduce((sum, value) => sum + value, 0) / values.length : null;
}

function recoveryThresholds(settings: Settings) {
  const ranges = settings.recovery_ranges ?? {};
  const low = typeof ranges.low === "number" && Number.isFinite(ranges.low) ? ranges.low : 34;
  const high = typeof ranges.high === "number" && Number.isFinite(ranges.high) ? ranges.high : 67;
  return low > 0 && low < high && high <= 100 ? { low, high } : { low: 34, high: 67 };
}

function RecoveryContent() {
  const { query: range } = useRangeSearch();
  const client = useQueryClient();
  const [recoveryPage, setRecoveryPage] = useState(1);
  const [sleepPage, setSleepPage] = useState(1);
  const [recoveryOpen, setRecoveryOpen] = useState(false);
  const [sleepOpen, setSleepOpen] = useState(false);
  const [editingRecovery, setEditingRecovery] = useState<RecoveryEntry | null>(null);
  const [editingSleep, setEditingSleep] = useState<SleepEntry | null>(null);
  const [deleting, setDeleting] = useState<{ kind: "recovery" | "sleep"; id: string | number } | null>(null);
  const open = useCallback(() => { setEditingRecovery(null); setRecoveryOpen(true); }, []);
  useQuickAction(open);
  useEffect(() => { setRecoveryPage(1); setSleepPage(1); }, [range]);

  const analytics = useQuery({ queryKey: ["analytics-recovery", range], queryFn: () => apiFetch<AnalyticsResponse>(`/analytics/recovery?${range}`) });
  const settings = useQuery({ queryKey: ["settings"], queryFn: () => apiFetch<Settings>("/settings") });
  const recovery = useQuery({ queryKey: ["recovery-list", range, recoveryPage], queryFn: () => apiFetch<ListResponse<RecoveryEntry>>(`/recovery?${range}&page=${recoveryPage}&page_size=${PAGE_SIZE}`), placeholderData: keepPreviousData });
  const sleep = useQuery({ queryKey: ["sleep-list", range, sleepPage], queryFn: () => apiFetch<ListResponse<SleepEntry>>(`/sleep?${range}&page=${sleepPage}&page_size=${PAGE_SIZE}`), placeholderData: keepPreviousData });
  const remove = useMutation({ mutationFn: ({ kind, id }: NonNullable<typeof deleting>) => apiFetch(`/${kind}/${id}`, { method: "DELETE" }), onSuccess: async () => { setDeleting(null); await client.invalidateQueries(); } });

  const recoveryColumns = useMemo<ColumnDef<RecoveryEntry>[]>(() => [
    { accessorKey: "date", header: "Дата", cell: ({ getValue }) => formatDate(getValue<string>()) },
    { accessorKey: "recovery_score", header: "Recovery", cell: ({ getValue }) => formatPercent(getValue<number | null>()) },
    { accessorKey: "hrv_ms", header: "HRV", cell: ({ getValue }) => formatNumber(getValue<number | null>(), {}, " мс") },
    { accessorKey: "resting_heart_rate_bpm", header: "RHR", cell: ({ getValue }) => formatNumber(getValue<number | null>(), {}, " bpm") },
    { accessorKey: "daily_strain", header: "Strain", cell: ({ getValue }) => formatNumber(getValue<number | null>()) },
    { accessorKey: "source", header: "Источник", cell: ({ getValue }) => <Badge>{String(getValue() ?? "—")}</Badge> },
    { id: "actions", header: "", cell: ({ row }) => <div className="flex justify-end gap-1"><Button variant="ghost" size="icon" aria-label="Редактировать восстановление" onClick={() => { setEditingRecovery(row.original); setRecoveryOpen(true); }}><Pencil className="size-4" /></Button><Button variant="ghost" size="icon" aria-label="Удалить восстановление" onClick={() => setDeleting({ kind: "recovery", id: row.original.id })}><Trash2 className="size-4" /></Button></div> },
  ], []);
  const sleepColumns = useMemo<ColumnDef<SleepEntry>[]>(() => [
    { accessorKey: "date", header: "Дата", cell: ({ row }) => formatDate(row.original.date ?? row.original.sleep_start) },
    { accessorKey: "actual_sleep_seconds", header: "Сон", cell: ({ getValue }) => formatDuration(getValue<number | null>()) },
    { accessorKey: "sleep_performance_percent", header: "Performance", cell: ({ getValue }) => formatPercent(getValue<number | null>()) },
    { accessorKey: "efficiency_percent", header: "Efficiency", cell: ({ getValue }) => formatPercent(getValue<number | null>()) },
    { accessorKey: "deep_seconds", header: "Deep", cell: ({ getValue }) => formatDuration(getValue<number | null>()) },
    { accessorKey: "rem_seconds", header: "REM", cell: ({ getValue }) => formatDuration(getValue<number | null>()) },
    { id: "actions", header: "", cell: ({ row }) => <div className="flex justify-end gap-1"><Button variant="ghost" size="icon" aria-label="Редактировать сон" onClick={() => { setEditingSleep(row.original); setSleepOpen(true); }}><Pencil className="size-4" /></Button><Button variant="ghost" size="icon" aria-label="Удалить сон" onClick={() => setDeleting({ kind: "sleep", id: row.original.id })}><Trash2 className="size-4" /></Button></div> },
  ], []);

  if (analytics.isError || recovery.isError || sleep.isError || settings.isError) return <ErrorState error={analytics.error ?? recovery.error ?? sleep.error ?? settings.error} retry={() => { void Promise.all([analytics.refetch(), recovery.refetch(), sleep.refetch(), settings.refetch()]); }} />;
  if (analytics.isPending || recovery.isPending || sleep.isPending || settings.isPending) return <PageSkeleton />;

  const daily = analytics.data.daily ?? analytics.data.series ?? [];
  const thresholds = recoveryThresholds(settings.data);
  const lowLabel = `0–${formatNumber(thresholds.low, { maximumFractionDigits: 1 })}`;
  const middleLabel = `${formatNumber(thresholds.low, { maximumFractionDigits: 1 })}–${formatNumber(thresholds.high, { maximumFractionDigits: 1 })}`;
  const highLabel = `${formatNumber(thresholds.high, { maximumFractionDigits: 1 })}–100`;
  const recoveryDistribution: SeriesPoint[] = [
    { date: lowLabel, days: daily.filter((point) => typeof point.recovery_score === "number" && point.recovery_score < thresholds.low).length },
    { date: middleLabel, days: daily.filter((point) => typeof point.recovery_score === "number" && point.recovery_score >= thresholds.low && point.recovery_score < thresholds.high).length },
    { date: highLabel, days: daily.filter((point) => typeof point.recovery_score === "number" && point.recovery_score >= thresholds.high).length },
  ];
  const trainingDays = daily.filter((point) => Number(point.workout_count) > 0);
  const restDays = daily.filter((point) => Number(point.workout_count) === 0);

  return <>
    <PageHeader eyebrow="Recovery" title="Восстановление и сон" description="Дневные показатели, структура сна и 7/28-дневные baseline без подмены missing значений нулями." actions={<><Button onClick={() => { setEditingSleep(null); setSleepOpen(true); }}><Moon className="size-4" />Сон</Button><Button variant="primary" onClick={() => { setEditingRecovery(null); setRecoveryOpen(true); }}><Plus className="size-4" />Recovery</Button></>} />
    <MetricGrid metrics={analytics.data.comparison ? comparisonSummaryMetrics(analytics.data.summary, analytics.data.comparison) : summaryToMetrics(analytics.data.summary)} />

    <MetricSwitcherChart title="Recovery и физиология" description="Показывается одна шкала за раз." data={daily} metrics={[
      { key: "recovery_score", label: "Recovery", variant: "area" },
      { key: "hrv_ms", label: "HRV" },
      { key: "resting_heart_rate_bpm", label: "RHR" },
      { key: "respiratory_rate", label: "Respiratory" },
      { key: "spo2_percent", label: "SpO₂" },
      { key: "skin_temperature_celsius", label: "Skin temp" },
      { key: "daily_strain", label: "Strain", variant: "bar" },
    ]} />

    <div className="grid min-w-0 gap-4 xl:grid-cols-2">
      <TrendChart title="HRV baseline" description="Фактическое HRV и полные rolling windows 7/28 дней." data={daily} series={[{ key: "hrv_ms", label: "HRV" }, { key: "hrv_7d_average", label: "7 дней", color: "var(--accent-blue)" }, { key: "hrv_28d_average", label: "28 дней", color: "var(--warning)" }]} />
      <TrendChart title="RHR baseline" description="Resting HR относительно 7/28-дневной базы." data={daily} series={[{ key: "resting_heart_rate_bpm", label: "RHR" }, { key: "rhr_7d_average", label: "7 дней", color: "var(--accent-blue)" }, { key: "rhr_28d_average", label: "28 дней", color: "var(--warning)" }]} />
      <TrendChart title="Структура сна" description="REM, Deep, Light и Awake в секундах; отсутствующие стадии остаются пустыми." data={daily} series={[{ key: "rem_seconds", label: "REM" }, { key: "deep_seconds", label: "Deep", color: "var(--accent-blue)" }, { key: "light_seconds", label: "Light", color: "var(--warning)" }, { key: "awake_seconds", label: "Awake", color: "var(--critical)" }]} variant="bar" />
      <MetricSwitcherChart title="Качество и долг сна" data={daily} metrics={[
        { key: "sleep_seconds", label: "Сон", variant: "area" },
        { key: "time_in_bed_seconds", label: "В постели", variant: "area" },
        { key: "sleep_performance_percent", label: "Performance" },
        { key: "sleep_efficiency_percent", label: "Efficiency" },
        { key: "consistency_percent", label: "Consistency" },
        { key: "sleep_debt_seconds", label: "Sleep debt", variant: "bar" },
      ]} />
      <TrendChart title="Распределение Recovery" description={`Количество дней в сохранённых диапазонах: ниже ${formatNumber(thresholds.low)}, ${formatNumber(thresholds.low)}–${formatNumber(thresholds.high)} и от ${formatNumber(thresholds.high)}.`} data={recoveryDistribution} series={[{ key: "days", label: "Дни" }]} variant="bar" />
      <CalendarHeatmap title="Календарь восстановления" description="Насыщенность клетки соответствует Recovery Score; пустая клетка означает missing." data={daily} dataKey="recovery_score" unit="%" />
    </div>

    <section>
      <SectionHeader title="Тренировочные и нетренировочные дни" description="Описательное сравнение средних; причинный вывод не делается." className="mb-3" />
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <Card className="p-4"><p className="text-xs text-muted">Recovery · тренировки</p><p className="mt-2 text-2xl font-semibold">{formatPercent(numericAverage(trainingDays, "recovery_score"))}</p><p className="mt-1 text-xs text-muted">n = {trainingDays.filter((point) => typeof point.recovery_score === "number").length}</p></Card>
        <Card className="p-4"><p className="text-xs text-muted">Recovery · отдых</p><p className="mt-2 text-2xl font-semibold">{formatPercent(numericAverage(restDays, "recovery_score"))}</p><p className="mt-1 text-xs text-muted">n = {restDays.filter((point) => typeof point.recovery_score === "number").length}</p></Card>
        <Card className="p-4"><p className="text-xs text-muted">Сон · тренировки</p><p className="mt-2 text-2xl font-semibold">{formatDuration(numericAverage(trainingDays, "sleep_seconds"))}</p><p className="mt-1 text-xs text-muted">Только дни с записью сна</p></Card>
        <Card className="p-4"><p className="text-xs text-muted">Сон · отдых</p><p className="mt-2 text-2xl font-semibold">{formatDuration(numericAverage(restDays, "sleep_seconds"))}</p><p className="mt-1 text-xs text-muted">Только дни с записью сна</p></Card>
      </div>
    </section>

    <Card><div className="border-b border-line p-4 text-sm font-semibold">Ежедневное восстановление</div><DataTable data={listItems(recovery.data)} columns={recoveryColumns} /><Pagination data={recovery.data} page={recoveryPage} onPageChange={setRecoveryPage} disabled={recovery.isFetching} /></Card>
    <Card><div className="border-b border-line p-4 text-sm font-semibold">Записи сна</div><DataTable data={listItems(sleep.data)} columns={sleepColumns} /><Pagination data={sleep.data} page={sleepPage} onPageChange={setSleepPage} disabled={sleep.isFetching} /></Card>
    <RecoveryForm open={recoveryOpen} onOpenChange={(value) => { setRecoveryOpen(value); if (!value) setEditingRecovery(null); }} entry={editingRecovery} />
    <SleepForm open={sleepOpen} onOpenChange={(value) => { setSleepOpen(value); if (!value) setEditingSleep(null); }} entry={editingSleep} />
    <InlineError error={remove.error} />
    <ConfirmDialog open={Boolean(deleting)} onOpenChange={(value) => { if (!value) setDeleting(null); }} title="Удалить запись?" description="Запись исчезнет из аналитики и дневной таблицы." onConfirm={() => deleting && remove.mutate(deleting)} busy={remove.isPending} error={remove.error} />
  </>;
}

export default function RecoveryPage() { return <Suspense fallback={<PageSkeleton />}><RecoveryContent /></Suspense>; }
