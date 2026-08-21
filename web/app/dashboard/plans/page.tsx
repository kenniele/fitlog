"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { CalendarPlus, Pencil, Plus, Trash2 } from "lucide-react";
import { apiFetch, listItems, type ListResponse } from "@/lib/api";
import type { Exercise, WorkoutPlan } from "@/lib/types";
import { useQuickAction } from "@/lib/hooks";
import { PageHeader, SectionHeader } from "@/components/ui/page";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { DataTable } from "@/components/ui/data-table";
import { Badge } from "@/components/ui/badge";
import { ErrorState, InlineError, PageSkeleton } from "@/components/ui/states";
import { ExerciseForm, PlanForm } from "@/components/forms/plan-forms";
import { WeeklyScheduleDialog } from "@/components/forms/weekly-schedule-dialog";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { formatNumber } from "@/lib/format";
import { PAGE_SIZE, Pagination } from "@/components/ui/pagination";

function PlansContent() {
  const client = useQueryClient();
  const [planPage, setPlanPage] = useState(1);
  const [exercisePage, setExercisePage] = useState(1);
  const [planForm, setPlanForm] = useState(false);
  const [exerciseForm, setExerciseForm] = useState(false);
  const [editingPlan, setEditingPlan] = useState<WorkoutPlan | null>(null);
  const [editingExercise, setEditingExercise] = useState<Exercise | null>(null);
  const [schedulePlan, setSchedulePlan] = useState<WorkoutPlan | null>(null);
  const [deleting, setDeleting] = useState<{ kind: "workout-plans" | "exercises"; id: string | number; name: string } | null>(null);
  const open = useCallback(() => { setEditingPlan(null); setPlanForm(true); }, []);
  useQuickAction(open);

  const plans = useQuery({ queryKey: ["plans", planPage], queryFn: () => apiFetch<ListResponse<WorkoutPlan>>(`/workout-plans?page=${planPage}&page_size=${PAGE_SIZE}`), placeholderData: keepPreviousData });
  const exercises = useQuery({ queryKey: ["exercises", exercisePage], queryFn: () => apiFetch<ListResponse<Exercise>>(`/exercises?page=${exercisePage}&page_size=${PAGE_SIZE}`), placeholderData: keepPreviousData });
  const remove = useMutation({ mutationFn: (target: NonNullable<typeof deleting>) => apiFetch(`/${target.kind}/${target.id}`, { method: "DELETE" }), onSuccess: async () => { setDeleting(null); await client.invalidateQueries(); } });
  const columns = useMemo<ColumnDef<Exercise>[]>(() => [
    { accessorKey: "name", header: "Упражнение", cell: ({ getValue }) => <span className="font-medium">{String(getValue())}</span> },
    { accessorKey: "muscle_groups", header: "Мышечные группы", cell: ({ getValue }) => <div className="flex flex-wrap gap-1">{Array.isArray(getValue()) && (getValue() as string[]).length ? (getValue() as string[]).map((group) => <Badge key={group}>{group}</Badge>) : "—"}</div> },
    { id: "actions", header: "", cell: ({ row }) => <div className="flex justify-end gap-1"><Button variant="ghost" size="icon" aria-label="Редактировать" onClick={() => { setEditingExercise(row.original); setExerciseForm(true); }}><Pencil className="size-4" /></Button><Button variant="ghost" size="icon" aria-label="Удалить" onClick={() => setDeleting({ kind: "exercises", id: row.original.id, name: row.original.name })}><Trash2 className="size-4" /></Button></div> },
  ], []);

  if (plans.isError || exercises.isError) return <ErrorState error={plans.error ?? exercises.error} retry={() => { void Promise.all([plans.refetch(), exercises.refetch()]); }} />;
  if (plans.isPending || exercises.isPending) return <PageSkeleton />;

  return (
    <>
      <PageHeader eyebrow="Plans" title="Планы, шаблоны и упражнения" description="Версионируемые планы, prescription и расписание на общей тренировочной модели." actions={<><Button onClick={() => { setEditingExercise(null); setExerciseForm(true); }}><Plus className="size-4" />Упражнение</Button><Button variant="primary" onClick={open}><Plus className="size-4" />План</Button></>} />
      <section>
        <SectionHeader title="Тренировочные планы" description="Порядок, разминка, шаг веса и отдых сохраняются в active revision; расписание создаёт реальную scheduled session." className="mb-3" />
        <div className="grid gap-3 lg:grid-cols-2">
          {listItems(plans.data).map((plan) => (
            <Card key={plan.id} className="p-5">
              <div className="flex items-start justify-between gap-3"><div><h3 className="font-semibold">{plan.name}</h3><p className="mt-1 text-sm text-muted">{plan.description || "Без описания"}</p></div><Badge tone={plan.active ? "good" : "neutral"}>{plan.active ? "активен" : "не активен"}</Badge></div>
              <div className="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-line pt-4">
                <span className="text-xs text-muted">{formatNumber(plan.days_per_week)} дн./нед. · {formatNumber(plan.templates?.length)} шаблона</span>
                <div className="flex flex-wrap gap-1"><Button size="sm" disabled={!plan.active || !(plan.templates ?? []).some((template) => template.id != null)} title={!plan.active ? "У плана нет активной ревизии" : undefined} onClick={() => setSchedulePlan(plan)}><CalendarPlus className="size-4" />Неделя в расписание</Button><Button variant="ghost" size="icon" aria-label="Редактировать" onClick={() => { setEditingPlan(plan); setPlanForm(true); }}><Pencil className="size-4" /></Button><Button variant="ghost" size="icon" aria-label="Удалить" onClick={() => setDeleting({ kind: "workout-plans", id: plan.id, name: plan.name })}><Trash2 className="size-4" /></Button></div>
              </div>
            </Card>
          ))}
        </div>
        <Pagination data={plans.data} page={planPage} onPageChange={setPlanPage} disabled={plans.isFetching} />
      </section>
      <Card>
        <div className="border-b border-line p-4"><SectionHeader title="Справочник упражнений" description="Мышечные группы и metadata сохраняются через API." /></div>
        <DataTable data={listItems(exercises.data)} columns={columns} emptyTitle="Справочник пуст" />
        <Pagination data={exercises.data} page={exercisePage} onPageChange={setExercisePage} disabled={exercises.isFetching} />
      </Card>
      <InlineError error={remove.error} />
      <PlanForm open={planForm} onOpenChange={(value) => { setPlanForm(value); if (!value) setEditingPlan(null); }} plan={editingPlan} />
      <ExerciseForm open={exerciseForm} onOpenChange={(value) => { setExerciseForm(value); if (!value) setEditingExercise(null); }} exercise={editingExercise} />
      <WeeklyScheduleDialog open={Boolean(schedulePlan)} onOpenChange={(value) => { if (!value) setSchedulePlan(null); }} plan={schedulePlan} />
      <ConfirmDialog open={Boolean(deleting)} onOpenChange={(value) => { if (!value) setDeleting(null); }} title={`Удалить «${deleting?.name ?? ""}»?`} description="Сервер проверит связи и не позволит разрушить историю без явной поддержки этого действия." onConfirm={() => deleting && remove.mutate(deleting)} busy={remove.isPending} error={remove.error} />
    </>
  );
}

export default function PlansPage() {
  return <Suspense fallback={<PageSkeleton />}><PlansContent /></Suspense>;
}
