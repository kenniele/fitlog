"use client";

import Link from "next/link";
import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { FileUp, Pencil, Plus, Trash2 } from "lucide-react";
import { apiFetch, listItems, type ListResponse } from "@/lib/api";
import type { AnalyticsResponse, NutritionDay, Settings } from "@/lib/types";
import { useQuickAction, useRangeSearch } from "@/lib/hooks";
import { PageHeader } from "@/components/ui/page";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { DataTable } from "@/components/ui/data-table";
import { ErrorState, InlineError, PageSkeleton } from "@/components/ui/states";
import { MetricGrid } from "@/components/charts/metric-card";
import { TrendChart } from "@/components/charts/trend-chart";
import { MetricSwitcherChart } from "@/components/charts/metric-switcher-chart";
import { CalendarHeatmap } from "@/components/charts/calendar-heatmap";
import { NutritionForm } from "@/components/forms/record-forms";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { formatDate, formatNumber } from "@/lib/format";
import { Badge } from "@/components/ui/badge";
import { comparisonSummaryMetrics, summaryToMetrics } from "@/lib/metrics";
import { PAGE_SIZE, Pagination } from "@/components/ui/pagination";
import { parseFirstDayOfWeek } from "@/lib/week";
import { nutritionWeeklyAverages } from "@/lib/nutrition-week";

function NutritionContent() {
  const { query: range } = useRangeSearch();
  const client = useQueryClient();
  const [page, setPage] = useState(1);
  const [form, setForm] = useState(false);
  const [editing, setEditing] = useState<NutritionDay | null>(null);
  const [deleting, setDeleting] = useState<NutritionDay | null>(null);
  const open = useCallback(() => { setEditing(null); setForm(true); }, []);
  useQuickAction(open);
  useEffect(() => { setPage(1); }, [range]);

  const analytics = useQuery({ queryKey: ["analytics-nutrition", range], queryFn: () => apiFetch<AnalyticsResponse>(`/analytics/nutrition?${range}`) });
  const days = useQuery({ queryKey: ["nutrition-list", range, page], queryFn: () => apiFetch<ListResponse<NutritionDay>>(`/nutrition?${range}&page=${page}&page_size=${PAGE_SIZE}`), placeholderData: keepPreviousData });
  const settings = useQuery({ queryKey: ["settings"], queryFn: () => apiFetch<Settings>("/settings") });
  const remove = useMutation({ mutationFn: (id: string | number) => apiFetch(`/nutrition/${id}`, { method: "DELETE" }), onSuccess: async () => { setDeleting(null); await client.invalidateQueries(); } });

  const columns = useMemo<ColumnDef<NutritionDay>[]>(() => [
    { accessorKey: "date", header: "Дата", cell: ({ getValue }) => formatDate(getValue<string>()) },
    { accessorKey: "calories_kcal", header: "Ккал", cell: ({ getValue }) => formatNumber(getValue<number | null>()) },
    { accessorKey: "protein_g", header: "Белки", cell: ({ getValue }) => formatNumber(getValue<number | null>(), {}, " г") },
    { accessorKey: "fat_g", header: "Жиры", cell: ({ getValue }) => formatNumber(getValue<number | null>(), {}, " г") },
    { accessorKey: "carbohydrates_g", header: "Углеводы", cell: ({ getValue }) => formatNumber(getValue<number | null>(), {}, " г") },
    { accessorKey: "fiber_g", header: "Клетчатка", cell: ({ getValue }) => formatNumber(getValue<number | null>(), {}, " г") },
    { accessorKey: "water_ml", header: "Вода", cell: ({ getValue }) => formatNumber(getValue<number | null>(), {}, " мл") },
    { accessorKey: "source", header: "Источник", cell: ({ getValue }) => <Badge>{String(getValue() ?? "—")}</Badge> },
    { id: "actions", header: "", cell: ({ row }) => <div className="flex justify-end gap-1"><Button variant="ghost" size="icon" aria-label="Редактировать питание" onClick={() => { setEditing(row.original); setForm(true); }}><Pencil className="size-4" /></Button><Button variant="ghost" size="icon" aria-label="Удалить питание" onClick={() => setDeleting(row.original)}><Trash2 className="size-4" /></Button></div> },
  ], []);

  if (analytics.isError || days.isError || settings.isError) return <ErrorState error={analytics.error ?? days.error ?? settings.error} retry={() => { void Promise.all([analytics.refetch(), days.refetch(), settings.refetch()]); }} />;
  if (analytics.isPending || days.isPending || settings.isPending) return <PageSkeleton />;
  const firstDayOfWeek = parseFirstDayOfWeek(settings.data.first_day_of_week);
  if (firstDayOfWeek === null) return <ErrorState error={new Error("В настройках указан некорректный первый день недели")} retry={() => { void settings.refetch(); }} />;

  const target = settings.data.calorie_target_kcal;
  const proteinTarget = settings.data.protein_target_g;
  const fatTarget = settings.data.fat_target_g;
  const carbohydrateTarget = settings.data.carbohydrates_target_g;
  const daily = (analytics.data.daily ?? analytics.data.series ?? []).map((point) => {
    const calories = typeof point.calories_kcal === "number" ? point.calories_kcal : null;
    const adherence = calories !== null && target ? Math.max(0, 100 - Math.abs(calories - target) / target * 100) : null;
    return {
      ...point,
      calorie_target_kcal: target ?? null,
      protein_target_g: proteinTarget ?? null,
      fat_target_g: fatTarget ?? null,
      carbohydrates_target_g: carbohydrateTarget ?? null,
      target_adherence_percent: adherence,
    };
  });
  const weekly = nutritionWeeklyAverages(daily, firstDayOfWeek);
  const macroTargetDescription = [
    proteinTarget ? `белки ${formatNumber(proteinTarget)} г` : null,
    fatTarget ? `жиры ${formatNumber(fatTarget)} г` : null,
    carbohydrateTarget ? `углеводы ${formatNumber(carbohydrateTarget)} г` : null,
  ].filter(Boolean).join(", ");

  return <>
    <PageHeader eyebrow="Nutrition" title="Питание относительно целей" description="Дневные итоги, цели, недельные средние и соблюдение диапазона без фиктивной базы продуктов." actions={<><Link href="/dashboard/imports?action=new" className="inline-flex h-10 items-center gap-2 rounded-control border border-line bg-elevated px-4 text-sm font-medium text-ink transition hover:border-white/15"><FileUp className="size-4" />Импорт</Link><Button variant="primary" onClick={() => { setEditing(null); setForm(true); }}><Plus className="size-4" />Дневной итог</Button></>} />
    <MetricGrid metrics={analytics.data.comparison ? comparisonSummaryMetrics(analytics.data.summary, analytics.data.comparison) : summaryToMetrics(analytics.data.summary)} />
    <div className="grid min-w-0 gap-4 xl:grid-cols-2">
      <TrendChart title="Калории относительно цели" description={target ? `Сохранённая цель: ${formatNumber(target)} ккал; целевой коридор аналитики — 90–110%.` : "Цель не задана: график показывает только фактическое потребление."} data={daily} series={[{ key: "calories_kcal", label: "Калории" }, { key: "calorie_target_kcal", label: "Цель", color: "var(--warning)" }]} />
      <TrendChart title="Макронутриенты относительно целей" description={macroTargetDescription ? `Пунктиром показаны сохранённые цели: ${macroTargetDescription}.` : "Задайте цели по белкам, жирам и углеводам в Settings, чтобы видеть отклонения."} data={daily} series={[{ key: "protein_g", label: "Белки", color: "var(--accent)" }, { key: "protein_target_g", label: "Цель белков", color: "var(--accent)", strokeDasharray: "5 4" }, { key: "fat_g", label: "Жиры", color: "var(--warning)" }, { key: "fat_target_g", label: "Цель жиров", color: "var(--warning)", strokeDasharray: "5 4" }, { key: "carbohydrates_g", label: "Углеводы", color: "var(--accent-blue)" }, { key: "carbohydrates_target_g", label: "Цель углеводов", color: "var(--accent-blue)", strokeDasharray: "5 4" }]} />
      <TrendChart title="Недельные средние" description="Каждая точка — среднее только по имеющимся дневным записям недели с сохранённым первым днём." data={weekly} series={[{ key: "calories_kcal", label: "Калории" }, { key: "protein_g", label: "Белок", color: "var(--accent-blue)" }, { key: "fat_g", label: "Жиры", color: "var(--warning)" }, { key: "carbohydrates_g", label: "Углеводы", color: "var(--critical)" }]} />
      <MetricSwitcherChart title="Дополнительные нутриенты" data={daily} metrics={[
        { key: "sugar_g", label: "Сахар", variant: "bar" },
        { key: "saturated_fat_g", label: "Насыщ. жиры", variant: "bar" },
        { key: "sodium_mg", label: "Натрий" },
        { key: "potassium_mg", label: "Калий" },
        { key: "water_ml", label: "Вода", variant: "area" },
        { key: "fiber_g", label: "Клетчатка", variant: "bar" },
      ]} />
      <CalendarHeatmap title="Соблюдение калорийной цели" description={target ? "100% означает точное попадание; насыщенность снижается с отклонением от цели." : "Задайте калорийную цель в Settings, чтобы рассчитать соблюдение."} data={target ? daily : []} dataKey="target_adherence_percent" unit="%" />
    </div>
    <Card><DataTable data={listItems(days.data)} columns={columns} emptyTitle="Нет дневных итогов" /><Pagination data={days.data} page={page} onPageChange={setPage} disabled={days.isFetching} /></Card>
    <NutritionForm open={form} onOpenChange={(value) => { setForm(value); if (!value) setEditing(null); }} entry={editing} />
    <InlineError error={remove.error} />
    <ConfirmDialog open={Boolean(deleting)} onOpenChange={(value) => { if (!value) setDeleting(null); }} title="Удалить итог дня?" description="Эта точка исчезнет из всех графиков питания." onConfirm={() => deleting && remove.mutate(deleting.id)} busy={remove.isPending} error={remove.error} />
  </>;
}

export default function NutritionPage() { return <Suspense fallback={<PageSkeleton />}><NutritionContent /></Suspense>; }
