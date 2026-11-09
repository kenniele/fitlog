"use client";

import { Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { CalendarDays, Eye, List, Pencil, Plus, Trash2 } from "lucide-react";
import { apiFetch, fetchAllList, listItems, type ListResponse } from "@/lib/api";
import type { AnalyticsResponse, Exercise, Settings, WorkoutPlan, WorkoutSession } from "@/lib/types";
import { parseFirstDayOfWeek } from "@/lib/week";
import { useQuickAction, useRangeSearch } from "@/lib/hooks";
import { JournalSections, QuerySection } from "@/components/ui/journal-sections";
import { PageHeader } from "@/components/ui/page";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input, Select } from "@/components/ui/field";
import { DataTable } from "@/components/ui/data-table";
import { ErrorState, InlineError, PageSkeleton, Skeleton } from "@/components/ui/states";
import { MetricGrid } from "@/components/charts/metric-card";
import { TrendChart } from "@/components/charts/trend-chart";
import { ExerciseProgress } from "@/components/charts/exercise-progress";
import { ActivityHeatmap, DistributionBars, TrainingStreakCards } from "@/components/charts/training-insights";
import { WorkoutForm } from "@/components/forms/workout-form";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { formatDate, formatDuration, formatNumber } from "@/lib/format";
import { Badge } from "@/components/ui/badge";
import { comparisonSummaryMetrics, summaryToMetrics } from "@/lib/metrics";
import { PAGE_SIZE, Pagination } from "@/components/ui/pagination";
import { SessionCalendar, calendarSessionQuery } from "@/components/training/session-calendar";
import { TrainingExport } from "@/components/training/export-dialog";

function positivePage(value: string | null) {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : 1;
}

function TrainingContent() {
  const { range: selectedRange, query: range, search: searchParams } = useRangeSearch();
  const router = useRouter();
  const pathname = usePathname();
  const client = useQueryClient();
  const search = searchParams.get("search") ?? "";
  const status = searchParams.get("status") ?? "";
  const exercise = searchParams.get("exercise_id") ?? "";
  const plan = searchParams.get("plan_id") ?? "";
  const view = searchParams.get("view") === "calendar" ? "calendar" : "list";
  const page = positivePage(searchParams.get("page"));
  const [searchDraft, setSearchDraft] = useState(search);
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<WorkoutSession | null>(null);
  const [deleting, setDeleting] = useState<WorkoutSession | null>(null);
  const searchTimer = useRef<number | undefined>(undefined);
  const previousRange = useRef(range);

  const updateParams = useCallback((changes: Record<string, string | null>, resetPage = true) => {
    const params = new URLSearchParams(window.location.search);
    Object.entries(changes).forEach(([key, value]) => {
      if (value) params.set(key, value);
      else params.delete(key);
    });
    if (resetPage) params.delete("page");
    const query = params.toString();
    router.replace(query ? `${pathname}?${query}` : pathname, { scroll: false });
  }, [pathname, router]);

  useEffect(() => {
    if (searchTimer.current !== undefined) window.clearTimeout(searchTimer.current);
    setSearchDraft(search);
  }, [search]);
  useEffect(() => () => {
    if (searchTimer.current !== undefined) window.clearTimeout(searchTimer.current);
  }, []);
  useEffect(() => {
    if (previousRange.current === range) return;
    previousRange.current = range;
    if (page > 1) updateParams({ page: null }, false);
  }, [page, range, updateParams]);

  const setSearch = (value: string) => {
    setSearchDraft(value);
    if (searchTimer.current !== undefined) window.clearTimeout(searchTimer.current);
    searchTimer.current = window.setTimeout(() => {
      updateParams({ search: value });
      searchTimer.current = undefined;
    }, 300);
  };

  const openNew = useCallback(() => { setEditing(null); setFormOpen(true); }, []);
  useQuickAction(openNew);

  const baseParams = new URLSearchParams(range);
  if (search) baseParams.set("search", search);
  if (status) baseParams.set("status", status);
  if (exercise) baseParams.set("exercise_id", exercise);
  if (plan) baseParams.set("plan_id", plan);
  const baseFilters = baseParams.toString();
  const filters = `${baseFilters}&page=${page}&page_size=${PAGE_SIZE}`;
  const exportParams = new URLSearchParams(baseFilters);
  if (view === "calendar") exportParams.set("date_basis", "calendar");
  const exportFilters = exportParams.toString();
  const sessions = useQuery({
    queryKey: ["workout-sessions", filters],
    queryFn: () => apiFetch<ListResponse<WorkoutSession>>(`/workout-sessions?${filters}`),
    placeholderData: keepPreviousData,
  });
  const calendarFilters = calendarSessionQuery(baseFilters);
  const calendarSessions = useQuery({
    queryKey: ["workout-sessions-calendar", calendarFilters],
    queryFn: () => fetchAllList<WorkoutSession>(`/workout-sessions?${calendarFilters}`),
    enabled: view === "calendar",
  });

  const analyticsParams = new URLSearchParams(range);
  if (exercise) analyticsParams.set("exercise_id", exercise);
  if (status) analyticsParams.set("status", status);
  if (plan) analyticsParams.set("plan_id", plan);
  const analyticsFilters = analyticsParams.toString();
  const analytics = useQuery({ queryKey: ["analytics-training", analyticsFilters], queryFn: () => apiFetch<AnalyticsResponse>(`/analytics/training?${analyticsFilters}`) });
  const exercises = useQuery({ queryKey: ["training-exercises"], queryFn: () => fetchAllList<Exercise>("/exercises") });
  const plans = useQuery({ queryKey: ["training-plans"], queryFn: () => fetchAllList<WorkoutPlan>("/workout-plans") });
  const settings = useQuery({ queryKey: ["settings"], queryFn: () => apiFetch<Settings>("/settings") });
  const remove = useMutation({
    mutationFn: (id: string | number) => apiFetch(`/workout-sessions/${id}`, { method: "DELETE" }),
    onSuccess: async () => {
      setDeleting(null);
      await client.invalidateQueries({ queryKey: ["workout-sessions"] });
      await client.invalidateQueries({ queryKey: ["analytics-training"] });
    },
  });

  const columns = useMemo<ColumnDef<WorkoutSession>[]>(() => [
    { accessorKey: "date", header: "Дата", cell: ({ row }) => <span>{formatDate(row.original.date ?? row.original.started_at)}</span> },
    { accessorKey: "template_name", header: "Шаблон", cell: ({ row }) => <span className="font-medium">{row.original.template_name ?? row.original.plan_name ?? "Без плана"}</span> },
    { accessorKey: "status", header: "Статус", cell: ({ getValue }) => <Badge tone={getValue() === "finished" ? "good" : "neutral"}>{({ finished: "Завершена", active: "Активна", scheduled: "Запланирована", cancelled: "Отменена", excused: "Пропущена по причине" } as Record<string, string>)[String(getValue())] ?? "Без данных"}</Badge> },
    { accessorKey: "working_sets", header: "Раб. подходы", cell: ({ getValue }) => formatNumber(getValue<number | null>()) },
    { accessorKey: "volume_kg", header: "Объём", cell: ({ getValue }) => formatNumber(getValue<number | null>(), {}, " кг") },
    { accessorKey: "duration_seconds", header: "Длительность", cell: ({ getValue }) => formatDuration(getValue<number | null>()) },
    { id: "actions", header: "", enableSorting: false, cell: ({ row }) => <div className="flex justify-end gap-1"><Button variant="ghost" size="icon" aria-label="Открыть" onClick={() => location.assign(`/dashboard/training/sessions/${row.original.id}`)}><Eye className="size-4" /></Button><Button variant="ghost" size="icon" aria-label="Редактировать" onClick={() => { setEditing(row.original); setFormOpen(true); }}><Pencil className="size-4" /></Button><Button variant="ghost" size="icon" aria-label="Удалить" onClick={() => setDeleting(row.original)}><Trash2 className="size-4" /></Button></div> },
  ], []);

  const firstDayOfWeek = parseFirstDayOfWeek(settings.data?.first_day_of_week);

  return (
    <>
      <PageHeader eyebrow="Тренировки" title="Тренировки и прогрессия" description="Сессии, рабочий объём и сила внутри конкретных упражнений." actions={<><TrainingExport filters={exportFilters} /><Button variant="primary" onClick={openNew}><Plus className="size-4" />Тренировка</Button></>} />
      <JournalSections journal={<>      {exercises.isError && <ErrorState title="Недоступен фильтр упражнений" error={exercises.error} retry={() => { void exercises.refetch(); }} />}
      {plans.isError && <ErrorState title="Недоступен фильтр программ" error={plans.error} retry={() => { void plans.refetch(); }} />}
      <Card>
        <div className="flex flex-col gap-3 border-b border-line p-4 lg:flex-row lg:items-center">
          <Input value={searchDraft} onChange={(event) => setSearch(event.target.value)} placeholder="Поиск по тренировкам" aria-label="Поиск по тренировкам" className="lg:max-w-xs" />
          <Select disabled={!plans.data} aria-label="Фильтр по плану" value={plan} onChange={(event) => updateParams({ plan_id: event.target.value })} className="lg:max-w-[240px]"><option value="">Все планы</option>{listItems(plans.data).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</Select>
          <Select disabled={!exercises.data} aria-label="Фильтр по упражнению" value={exercise} onChange={(event) => updateParams({ exercise_id: event.target.value })} className="lg:max-w-[240px]"><option value="">Все упражнения</option>{listItems(exercises.data).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</Select>
          <Select aria-label="Фильтр по статусу" value={status} onChange={(event) => updateParams({ status: event.target.value })} className="lg:max-w-[210px]"><option value="">Все статусы</option><option value="scheduled">Запланировано</option><option value="active">Активно</option><option value="finished">Завершено</option><option value="cancelled">Отменено</option><option value="excused">Пропущено по причине</option></Select>
          {(search || status || exercise || plan) && <Button variant="ghost" onClick={() => { if (searchTimer.current !== undefined) window.clearTimeout(searchTimer.current); setSearchDraft(""); updateParams({ search: null, status: null, exercise_id: null, plan_id: null }); }}>Сбросить фильтры</Button>}
          <div className="flex rounded-control border border-line bg-canvas/45 p-1 lg:ml-auto" aria-label="Представление тренировок" role="group"><Button type="button" size="sm" variant={view === "list" ? "secondary" : "ghost"} aria-pressed={view === "list"} onClick={() => updateParams({ view: null })}><List aria-hidden className="size-4" />Список</Button><Button type="button" size="sm" variant={view === "calendar" ? "secondary" : "ghost"} aria-pressed={view === "calendar"} onClick={() => updateParams({ view: "calendar" })}><CalendarDays aria-hidden className="size-4" />Календарь</Button></div>
        </div>
        {view === "list" ? <QuerySection query={sessions} title="Не удалось загрузить тренировки"><DataTable data={listItems(sessions.data)} columns={columns} emptyTitle="Тренировок не найдено" rowKey={(row) => String(row.id)} /><Pagination data={sessions.data} page={page} onPageChange={(nextPage) => updateParams({ page: nextPage === 1 ? null : String(nextPage) }, false)} disabled={sessions.isFetching} /></QuerySection> : <div className="px-4 py-3 text-xs text-muted">Все тренировки выбранного периода. Запланированные показаны на назначенную дату.</div>}
      </Card>
      {view === "calendar" && (calendarSessions.isError
        ? <ErrorState error={calendarSessions.error} retry={() => { void calendarSessions.refetch(); }} title="Не удалось загрузить календарь" />
        : calendarSessions.isPending
          ? <Card aria-busy="true" aria-label="Загрузка календаря" className="p-4"><Skeleton className="h-96" /></Card>
          : firstDayOfWeek === null ? <ErrorState error={new Error("Выберите первый день недели в настройках")} /> : <SessionCalendar from={selectedRange.from} to={selectedRange.to} sessions={calendarSessions.data} firstDayOfWeek={firstDayOfWeek} />)}
</>} analytics={<QuerySection query={analytics} title="Не удалось загрузить динамику тренировок">      <MetricGrid metrics={analytics.data?.comparison ? comparisonSummaryMetrics(analytics.data?.summary, analytics.data?.comparison) : summaryToMetrics(analytics.data?.summary)} />
      <ExerciseProgress exercises={listItems(exercises.data)} />
      <TrainingStreakCards streak={analytics.data?.streak} />
      <div className="grid min-w-0 gap-4 xl:grid-cols-2">
        <TrendChart title="Недельный объём" description="Завершённые рабочие и дроп-подходы по неделям." data={analytics.data?.weekly} series={[{ key: "volume_kg", label: "Объём, кг" }]} variant="bar" />
        <TrendChart title="Длительность по дням" description="Суммарное фактическое время завершённых сессий, сгруппированное по локальной дате старта." data={analytics.data?.daily_duration} series={[{ key: "duration_minutes", label: "Минуты" }]} variant="area" />
        <TrendChart title="Рабочие подходы" data={analytics.data?.daily} series={[{ key: "working_sets", label: "Рабочие подходы" }]} />
        <TrendChart title="План и выполнение" description="Запланированные и выполненные тренировки. Перенос фактического старта не меняет исходную дату плана." data={analytics.data?.adherence} series={[{ key: "planned", label: "Запланировано" }, { key: "completed", label: "Выполнено", color: "var(--accent-blue)" }]} variant="bar" />
      </div>
      <ActivityHeatmap data={analytics.data?.heatmap} />
      <div className="grid min-w-0 gap-4 xl:grid-cols-2">
        <DistributionBars title="Рабочие подходы по мышечным группам" description="Каждый завершённый рабочий или дроп-подход учитывается по основной мышечной группе упражнения." data={analytics.data?.muscle_groups?.map((point) => ({ label: point.muscle_group || "Без группы", value: point.working_sets, detail: `${formatNumber(point.volume_kg, {}, " кг")}` }))} />
        <DistributionBars title="Распределение RIR" description="Только завершённые рабочий или дроп-подходы с указанным фактическим RIR; дробные значения округлены до ближайшего целого." data={analytics.data?.rir_distribution?.map((point) => ({ label: `RIR ${point.rir}`, value: point.sets }))} accent="blue" />
      </div>
</QuerySection>} />
      <WorkoutForm open={formOpen} onOpenChange={(value) => { setFormOpen(value); if (!value) setEditing(null); }} session={editing} />
      <InlineError error={remove.error} />
      <ConfirmDialog open={Boolean(deleting)} onOpenChange={(open) => { if (!open) setDeleting(null); }} title="Удалить тренировку?" description="Сессия и её подходы будут удалены. Это действие нельзя отменить." onConfirm={() => deleting && remove.mutate(deleting.id)} busy={remove.isPending} error={remove.error} />
    </>
  );
}

export default function TrainingPage() {
  return <Suspense fallback={<PageSkeleton />}><TrainingContent /></Suspense>;
}
