"use client";

import { use, useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Pencil, Trash2 } from "lucide-react";
import { apiFetch } from "@/lib/api";
import type { ExerciseResult, SessionExercise, WorkoutSession } from "@/lib/types";
import { PageHeader } from "@/components/ui/page";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { ErrorState, EmptyState, InlineError, PageSkeleton } from "@/components/ui/states";
import { WorkoutForm } from "@/components/forms/workout-form";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { formatDate, formatDuration, formatNumber, signedDelta } from "@/lib/format";
import { cn } from "@/lib/utils";

function delta(current: number | null | undefined, previous: number | null | undefined, suffix = "") {
  if (current == null || previous == null) return "—";
  return signedDelta(current - previous, suffix);
}

function actualOrPlan(actual: number | null | undefined, planned: number | null | undefined, suffix = "") {
  if (actual != null) return formatNumber(actual, {}, suffix);
  return <span className="text-muted">план {formatNumber(planned, {}, suffix)}</span>;
}

function repsActualOrPlan(actual: number | null | undefined, minimum: number | null | undefined, maximum: number | null | undefined) {
  if (actual != null) return formatNumber(actual);
  const planned = minimum == null ? "—" : maximum != null && maximum !== minimum ? `${minimum}–${maximum}` : String(minimum);
  return <span className="text-muted">план {planned}</span>;
}

function ResultBlock({ title, result, previous }: { title: string; result: ExerciseResult | null | undefined; previous?: ExerciseResult | null }) {
  if (!result) return <div className="rounded-control border border-line p-3 text-xs text-muted">{title}: данных нет</div>;
  return (
    <div className="rounded-control border border-line bg-canvas/35 p-3">
      <div className="flex items-center justify-between gap-2"><p className="text-xs font-semibold">{title}</p><span className="text-[11px] text-muted">{formatDate(result.date)}</span></div>
      <div className="mt-3 grid grid-cols-2 gap-3 text-xs sm:grid-cols-3">
        <span><span className="block text-muted">Рабочие сеты</span><strong className="mt-1 block text-ink">{formatNumber(result.working_sets)}</strong></span>
        <span><span className="block text-muted">Повторы</span><strong className="mt-1 block text-ink">{formatNumber(result.repetitions)}</strong></span>
        <span><span className="block text-muted">Объём</span><strong className="mt-1 block text-ink">{formatNumber(result.volume_kg, {}, " кг")}{previous && <small className="ml-1 text-muted">({delta(result.volume_kg, previous.volume_kg, " кг")})</small>}</strong></span>
        <span><span className="block text-muted">Лучший вес</span><strong className="mt-1 block text-ink">{formatNumber(result.best_weight_kg, {}, " кг")}{previous && <small className="ml-1 text-muted">({delta(result.best_weight_kg, previous.best_weight_kg, " кг")})</small>}</strong></span>
        <span><span className="block text-muted">Estimated 1RM</span><strong className="mt-1 block text-ink">{formatNumber(result.estimated_1rm, {}, " кг")}{previous && <small className="ml-1 text-muted">({delta(result.estimated_1rm, previous.estimated_1rm, " кг")})</small>}</strong></span>
        <span><span className="block text-muted">Средний RIR</span><strong className="mt-1 block text-ink">{formatNumber(result.average_rir)}</strong></span>
      </div>
    </div>
  );
}

function ExerciseCard({ exercise, index }: { exercise: SessionExercise; index: number }) {
  return (
    <Card className="overflow-hidden">
      <div className="flex flex-wrap items-start justify-between gap-3 border-b border-line p-4">
        <div>
          <h2 className="text-sm font-semibold">{exercise.exercise_name ?? exercise.name ?? `Упражнение ${index + 1}`}</h2>
          <p className="mt-1 text-xs text-muted">{exercise.note || "Без заметки"}</p>
          {exercise.rest_after_exercise_seconds != null && <p className="mt-1 text-xs text-muted">Отдых до следующего упражнения: {formatDuration(exercise.rest_after_exercise_seconds)}</p>}
        </div>
        <Badge tone={exercise.completed ? "good" : "neutral"}>{exercise.completed ? "готово" : "не завершено"}</Badge>
      </div>
      {(exercise.current_result || exercise.previous_result) && (
        <div className="grid gap-3 border-b border-line p-4 lg:grid-cols-2">
          <ResultBlock title="Текущая сессия" result={exercise.current_result} previous={exercise.previous_result} />
          <ResultBlock title="Предыдущая сессия" result={exercise.previous_result} />
        </div>
      )}
      {exercise.sets?.length ? (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[860px] text-sm">
            <thead><tr className="border-b border-line text-left text-[11px] uppercase tracking-wider text-muted"><th className="p-3">Подход</th><th className="p-3">Тип</th><th className="p-3">Вес</th><th className="p-3">Повторы</th><th className="p-3">RIR</th><th className="p-3">Отдых</th><th className="p-3">Выполнение</th><th className="p-3">Комментарий</th></tr></thead>
            <tbody>{exercise.sets.map((set, setIndex) => (
              <tr key={set.id ?? setIndex} className={cn("border-b border-line/70 last:border-0", set.type === "warmup" ? "bg-blue/[.035]" : set.type === "drop" ? "bg-warning/[.035]" : "bg-accent/[.02]")}>
                <td className="p-3">{setIndex + 1}</td>
                <td className="p-3"><Badge tone={set.type === "warmup" ? "blue" : set.type === "drop" ? "warning" : "good"}>{set.type === "warmup" ? "разминка" : set.type === "drop" ? "drop" : "рабочий"}</Badge></td>
                <td className="p-3">{actualOrPlan(set.actual_weight_kg ?? set.weight_kg, set.planned_weight_kg, " кг")}</td>
                <td className="p-3">{repsActualOrPlan(set.actual_reps ?? set.reps, set.planned_min_reps, set.planned_max_reps)}</td>
                <td className="p-3">{actualOrPlan(set.actual_rir ?? set.rir, set.planned_rir)}</td>
                <td className="p-3">{formatDuration(set.rest_seconds)}</td>
                <td className="p-3"><Badge tone={set.completed || set.completed_at ? "good" : "neutral"}>{set.completed || set.completed_at ? "готов" : "не готов"}</Badge></td>
                <td className="max-w-[260px] truncate p-3" title={set.comment ?? set.notes ?? undefined}>{set.comment ?? set.notes ?? "—"}</td>
              </tr>
            ))}</tbody>
          </table>
        </div>
      ) : <EmptyState title="Подходов нет" />}
    </Card>
  );
}

export default function SessionPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const client = useQueryClient();
  const [edit, setEdit] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const query = useQuery({ queryKey: ["workout-session", id], queryFn: () => apiFetch<WorkoutSession>(`/workout-sessions/${id}`) });
  const remove = useMutation({ mutationFn: () => apiFetch(`/workout-sessions/${id}`, { method: "DELETE" }), onSuccess: () => { void client.invalidateQueries(); location.assign("/dashboard/training"); } });
  if (query.isPending) return <PageSkeleton />;
  if (query.isError) return <ErrorState error={query.error} retry={() => query.refetch()} />;
  const session = query.data;
  const facts = [
    ["Статус", session.status ?? "—"],
    ["Начало", formatDate(session.started_at ?? session.scheduled_at, "dd.MM.yyyy HH:mm")],
    ["Завершение", formatDate(session.finished_at, "dd.MM.yyyy HH:mm")],
    ["Длительность", formatDuration(session.duration_seconds)],
    ["Strain", formatNumber(session.strain)],
    ["Упражнения", formatNumber(session.exercises?.length)],
    ["Рабочие подходы", formatNumber(session.working_sets)],
    ["Объём", formatNumber(session.volume_kg, {}, " кг")],
  ];

  return (
    <>
      <Link href="/dashboard/training" className="inline-flex items-center gap-2 text-sm text-muted hover:text-ink"><ArrowLeft className="size-4" />К тренировкам</Link>
      <PageHeader eyebrow={formatDate(session.date ?? session.started_at ?? session.scheduled_at)} title={session.template_name ?? session.plan_name ?? "Тренировка"} description={session.notes ?? "Без заметки"} actions={<><Button onClick={() => setEdit(true)}><Pencil className="size-4" />Редактировать</Button><Button variant="danger" onClick={() => setConfirm(true)}><Trash2 className="size-4" />Удалить</Button></>} />
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">{facts.map(([label, value]) => <Card key={label} className="p-4"><p className="text-xs text-muted">{label}</p><p className="mt-2 text-xl font-semibold">{value}</p></Card>)}</div>
      <div className="space-y-4">{session.exercises?.length ? session.exercises.map((exercise, index) => <ExerciseCard key={exercise.id ?? index} exercise={exercise} index={index} />) : <Card><EmptyState title="Упражнений нет" /></Card>}</div>
      <InlineError error={remove.error} />
      <WorkoutForm open={edit} onOpenChange={setEdit} session={session} />
      <ConfirmDialog open={confirm} onOpenChange={setConfirm} title="Удалить тренировку?" description="Все упражнения и подходы этой сессии будут удалены." onConfirm={() => remove.mutate()} busy={remove.isPending} error={remove.error} />
    </>
  );
}
