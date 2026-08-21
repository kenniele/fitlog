"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { useFieldArray, useForm } from "react-hook-form";
import { Plus, Trash2 } from "lucide-react";
import { z } from "zod";
import { Dialog, DialogActions } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Checkbox, Field, Input, Select, Textarea } from "@/components/ui/field";
import { InlineError } from "@/components/ui/states";
import { apiFetch, fetchAllList, listItems } from "@/lib/api";
import { dateInTimeZone, dateTimeLocalInTimeZone, daysBetweenISO, getDashboardTimezone, shiftISODate, toDateTimeLocal } from "@/lib/format";
import type { Exercise, WorkoutPlan, WorkoutSession } from "@/lib/types";

type SetValues = {
  id?: string;
  type: "warmup" | "working" | "drop";
  weight_kg?: number;
  reps?: number;
  rir?: number;
  planned_weight_kg?: number;
  planned_min_reps?: number;
  planned_max_reps?: number;
  planned_rir?: number;
  rest_seconds?: number;
  completed: boolean;
  completed_at?: string;
  comment?: string;
  source?: string;
  external_id?: string;
};

export type ExerciseValues = {
  id?: string;
  exercise_id?: string;
  name?: string;
  note?: string;
  completed?: boolean;
  rest_after_exercise_seconds?: number;
  source?: string;
  external_id?: string;
  sets: SetValues[];
};

type WorkoutStatus = "scheduled" | "active" | "finished" | "cancelled" | "excused";

export type WorkoutValues = {
  date: string;
  status: WorkoutStatus;
  scheduled_at?: string;
  started_at?: string;
  finished_at?: string;
  plan_id?: string;
  template_id?: string;
  program_name?: string;
  strain?: number;
  notes?: string;
  source?: string;
  external_id?: string;
  exercises?: ExerciseValues[];
};

const optionalNumber = z.number().finite().nonnegative().optional();
const setSchema = z.object({
  id: z.string().optional(),
  type: z.enum(["warmup", "working", "drop"]),
  weight_kg: optionalNumber,
  reps: z.number().int().positive("Повторы должны быть больше нуля").optional(),
  rir: optionalNumber,
  rest_seconds: optionalNumber,
  completed: z.boolean(),
  completed_at: z.string().optional(),
  comment: z.string().max(500).optional(),
  source: z.string().optional(),
  external_id: z.string().optional(),
  planned_weight_kg: optionalNumber,
  planned_min_reps: z.number().int().positive().optional(),
  planned_max_reps: z.number().int().positive().optional(),
  planned_rir: optionalNumber,
}).refine((value) => !value.completed || value.reps != null, { message: "Для выполненного подхода укажите повторы", path: ["reps"] });
const exerciseSchema = z.object({
  id: z.string().optional(),
  exercise_id: z.string().optional(),
  name: z.string().optional(),
  note: z.string().max(1000).optional(),
  completed: z.boolean().optional(),
  rest_after_exercise_seconds: optionalNumber,
  source: z.string().optional(),
  external_id: z.string().optional(),
  sets: z.array(setSchema).min(1, "Добавьте хотя бы один подход"),
}).refine((value) => Boolean(value.exercise_id || value.name?.trim()), {
  message: "Выберите или назовите упражнение",
  path: ["name"],
});
export const workoutSchema = z.object({
  date: z.string().min(1, "Укажите дату"),
  status: z.enum(["scheduled", "active", "finished", "cancelled", "excused"]),
  scheduled_at: z.string().optional(),
  started_at: z.string().optional(),
  finished_at: z.string().optional(),
  plan_id: z.string().optional(),
  template_id: z.string().optional(),
  program_name: z.string().max(200).optional(),
  strain: optionalNumber,
  notes: z.string().max(2000).optional(),
  source: z.string().optional(),
  external_id: z.string().optional(),
  exercises: z.array(exerciseSchema).min(1, "Добавьте хотя бы одно упражнение").optional(),
})
  .refine((value) => !["scheduled", "cancelled", "excused"].includes(value.status) || Boolean(value.scheduled_at), { message: "Укажите запланированное время", path: ["scheduled_at"] })
  .refine((value) => !["active", "finished"].includes(value.status) || Boolean(value.started_at), { message: "Укажите время начала", path: ["started_at"] })
  .refine((value) => value.status !== "finished" || Boolean(value.finished_at), { message: "Укажите время завершения", path: ["finished_at"] })
  .refine((value) => !value.finished_at || !value.started_at || value.finished_at >= value.started_at, { message: "Завершение не может быть раньше начала", path: ["finished_at"] });

const number = { setValueAs: (value: string) => value === "" ? undefined : Number(value) };

function makeSet(completed: boolean, source = "manual", type: SetValues["type"] = "working"): SetValues {
  return {
    id: "",
    type,
    weight_kg: undefined,
    reps: 8,
    rir: undefined,
    planned_weight_kg: undefined,
    planned_min_reps: undefined,
    planned_max_reps: undefined,
    planned_rir: undefined,
    rest_seconds: undefined,
    completed,
    completed_at: "",
    comment: "",
    source,
    external_id: "",
  };
}

function makeExercise(completed: boolean, source = "manual"): ExerciseValues {
  return {
    id: "",
    exercise_id: "",
    name: "",
    note: "",
    completed,
    rest_after_exercise_seconds: undefined,
    source,
    external_id: "",
    sets: [makeSet(completed, source)],
  };
}

function defaults(timeZone: string): WorkoutValues {
  const finished = new Date();
  const started = new Date(finished.getTime() - 60 * 60_000);
  return {
    date: dateInTimeZone(finished, timeZone),
    status: "finished",
    scheduled_at: "",
    started_at: dateTimeLocalInTimeZone(started, timeZone),
    finished_at: dateTimeLocalInTimeZone(finished, timeZone),
    plan_id: "",
    template_id: "",
    program_name: "",
    strain: undefined,
    notes: "",
    source: "manual",
    external_id: "",
    exercises: [makeExercise(true)],
  };
}

function sessionStatus(value: string | null | undefined): WorkoutStatus {
  return value === "scheduled" || value === "active" || value === "cancelled" || value === "excused" ? value : "finished";
}

export function workoutValuesFromSession(session: WorkoutSession | null | undefined, timeZone: string): WorkoutValues {
  if (!session) return defaults(timeZone);
  const source = session.source ?? "manual";
  const status = sessionStatus(session.status);
  const mappedExercises = session.exercises?.map((exercise) => {
    const sets: SetValues[] = exercise.sets?.map((set) => ({
      id: set.id == null ? undefined : String(set.id),
      type: set.type === "warmup" || set.type === "drop" ? set.type : "working",
      weight_kg: set.actual_weight_kg ?? set.weight_kg ?? undefined,
      reps: set.actual_reps ?? set.reps ?? undefined,
      rir: set.actual_rir ?? set.rir ?? undefined,
      planned_weight_kg: set.planned_weight_kg ?? exercise.planned_weight_kg ?? undefined,
      planned_min_reps: set.planned_min_reps ?? exercise.planned_min_reps ?? exercise.min_reps ?? undefined,
      planned_max_reps: set.planned_max_reps ?? exercise.planned_max_reps ?? exercise.max_reps ?? undefined,
      planned_rir: set.planned_rir ?? exercise.planned_target_rir ?? exercise.target_rir ?? undefined,
      rest_seconds: set.rest_seconds ?? exercise.planned_rest_seconds ?? undefined,
      completed: set.completed ?? Boolean(set.completed_at),
      completed_at: set.completed_at ?? undefined,
      comment: set.comment ?? set.notes ?? "",
      source: set.source ?? exercise.source ?? source,
      external_id: set.external_id ?? undefined,
    })) ?? [makeSet(status === "finished", exercise.source ?? source)];
    return {
      id: exercise.id == null ? undefined : String(exercise.id),
      exercise_id: exercise.exercise_id == null ? "" : String(exercise.exercise_id),
      name: exercise.exercise_name ?? exercise.name ?? "",
      note: exercise.note ?? exercise.notes ?? "",
      completed: exercise.completed ?? (sets.length > 0 && sets.every((set) => set.completed)),
      rest_after_exercise_seconds: exercise.rest_after_exercise_seconds ?? undefined,
      source: exercise.source ?? source,
      external_id: exercise.external_id ?? undefined,
      sets,
    };
  });
  return {
    date: session.date || toDateTimeLocal(session.started_at ?? session.scheduled_at, timeZone).slice(0, 10) || defaults(timeZone).date,
    status,
    scheduled_at: toDateTimeLocal(session.scheduled_at, timeZone),
    started_at: toDateTimeLocal(session.started_at, timeZone),
    finished_at: toDateTimeLocal(session.finished_at, timeZone),
    plan_id: session.plan_id == null ? "" : String(session.plan_id),
    template_id: session.template_id == null ? "" : String(session.template_id),
    program_name: session.program_name ?? "",
    strain: session.strain ?? undefined,
    notes: session.notes ?? "",
    source,
    external_id: session.external_id ?? undefined,
    exercises: mappedExercises?.length ? mappedExercises : undefined,
  };
}

function formValues(session: WorkoutSession | null | undefined, initialPlan: WorkoutPlan | null | undefined, timeZone: string): WorkoutValues {
  if (session || !initialPlan) return workoutValuesFromSession(session, timeZone);
  const date = dateInTimeZone(new Date(), timeZone);
  return {
    date,
    status: "scheduled",
    scheduled_at: `${date}T09:00`,
    plan_id: String(initialPlan.id),
    template_id: initialPlan.templates?.[0]?.id == null ? "" : String(initialPlan.templates[0].id),
    program_name: initialPlan.name,
    source: "manual",
    exercises: undefined,
  };
}

export function buildWorkoutSessionPayload(values: WorkoutValues, includeExercises: boolean): Record<string, unknown> {
  const { exercises: inputExercises, ...metadata } = values;
  const body: Record<string, unknown> = {
    ...metadata,
    scheduled_at: values.scheduled_at || null,
    started_at: ["active", "finished"].includes(values.status) ? values.started_at || null : null,
    finished_at: values.status === "finished" ? values.finished_at || null : null,
    plan_id: values.plan_id ? Number(values.plan_id) : null,
    template_id: values.template_id ? Number(values.template_id) : null,
  };
  if (includeExercises && inputExercises !== undefined) {
    body.exercises = inputExercises.map((exercise, position) => ({
      ...exercise,
      id: exercise.id ? Number(exercise.id) : null,
      completed: exercise.completed ?? (exercise.sets.length > 0 && exercise.sets.every((set) => set.completed)),
      exercise_id: exercise.exercise_id ? Number(exercise.exercise_id) : null,
      position: position + 1,
      sets: exercise.sets.map((set, setPosition) => ({
        id: set.id ? Number(set.id) : null,
        position: setPosition + 1,
        type: set.type,
        weight_kg: set.weight_kg,
        reps: set.reps,
        rir: set.rir,
        rest_seconds: set.rest_seconds,
        completed: set.completed,
        completed_at: set.completed_at,
        comment: set.comment,
        source: set.source,
        external_id: set.external_id,
      })),
    }));
  }
  return body;
}

function dateTimeOnDate(value: string | undefined, previousAnchor: string, nextAnchor: string, fallbackTime: string) {
  const valueDate = value?.slice(0, 10) || previousAnchor;
  const date = shiftISODate(nextAnchor, daysBetweenISO(previousAnchor, valueDate));
  return `${date}T${value?.slice(11, 16) || fallbackTime}`;
}

export function WorkoutForm({ open, onOpenChange, session, initialPlan }: { open: boolean; onOpenChange: (open: boolean) => void; session?: WorkoutSession | null; initialPlan?: WorkoutPlan | null }) {
  const client = useQueryClient();
  const timeZone = getDashboardTimezone();
  const snapshotProtected = session?.has_progression_snapshot === true;
  const form = useForm<WorkoutValues>({ resolver: zodResolver(workoutSchema), defaultValues: formValues(session, initialPlan, timeZone) });
  const exercises = useQuery({ queryKey: ["exercise-options"], queryFn: () => fetchAllList<Exercise>("/exercises"), enabled: open && !snapshotProtected });
  const plans = useQuery({ queryKey: ["plan-options"], queryFn: () => fetchAllList<WorkoutPlan>("/workout-plans"), enabled: open });
  const fields = useFieldArray({ control: form.control, name: "exercises", keyName: "formKey" });
  const status = form.watch("status");
  const selectedPlanID = form.watch("plan_id");
  const selectedTemplateID = form.watch("template_id");
  const materializedSchedule = !session && ["scheduled", "cancelled", "excused"].includes(status) && Boolean(selectedPlanID || selectedTemplateID);
  const fetchedPlanOptions = listItems(plans.data);
  const planOptions = initialPlan && !fetchedPlanOptions.some((plan) => String(plan.id) === String(initialPlan.id)) ? [initialPlan, ...fetchedPlanOptions] : fetchedPlanOptions;
  const templateOptions = planOptions.flatMap((plan) => (plan.templates ?? []).map((template) => ({ ...template, planID: String(plan.id), planName: plan.name })));

  const clearExercisesForMaterialization = () => {
    fields.replace([]);
    form.setValue("exercises", undefined, { shouldDirty: true, shouldValidate: true });
  };

  const ensureManualExercise = (completed: boolean) => {
    if (form.getValues("exercises")?.length) return;
    fields.replace([makeExercise(completed, form.getValues("source") ?? "manual")]);
  };

  useEffect(() => { if (open) form.reset(formValues(session, initialPlan, timeZone)); }, [form, initialPlan, open, session, timeZone]);

  const save = useMutation({
    mutationFn: (values: WorkoutValues) => {
      const shouldMaterialize = !session && ["scheduled", "cancelled", "excused"].includes(values.status) && Boolean(values.plan_id || values.template_id);
      const body = buildWorkoutSessionPayload(values, !shouldMaterialize);
      if (snapshotProtected && session) {
        body.plan_id = session.plan_id == null ? null : Number(session.plan_id);
        body.template_id = session.template_id == null ? null : Number(session.template_id);
      }
      return apiFetch(`/workout-sessions${session ? `/${session.id}` : ""}`, { method: session ? "PUT" : "POST", body });
    },
    onSuccess: async () => { await client.invalidateQueries(); onOpenChange(false); },
  });

  const handleDateChange = (nextDate: string) => {
    const previousDate = form.getValues("date") || nextDate;
    form.setValue("date", nextDate, { shouldDirty: true, shouldValidate: true });
    if (["scheduled", "cancelled", "excused"].includes(status)) {
      form.setValue("scheduled_at", dateTimeOnDate(form.getValues("scheduled_at"), previousDate, nextDate, "09:00"), { shouldDirty: true, shouldValidate: true });
      return;
    }
    form.setValue("started_at", dateTimeOnDate(form.getValues("started_at"), previousDate, nextDate, "18:00"), { shouldDirty: true, shouldValidate: true });
    if (status === "finished") {
      form.setValue("finished_at", dateTimeOnDate(form.getValues("finished_at"), previousDate, nextDate, "19:00"), { shouldDirty: true, shouldValidate: true });
    }
  };

  const handleStatusChange = (nextStatus: WorkoutStatus) => {
    form.setValue("status", nextStatus, { shouldDirty: true, shouldValidate: true });
    const date = form.getValues("date") || dateInTimeZone(new Date(), timeZone);
    if (["scheduled", "cancelled", "excused"].includes(nextStatus)) {
      if (!form.getValues("scheduled_at")) form.setValue("scheduled_at", `${date}T09:00`, { shouldDirty: true });
      if (!session && (form.getValues("plan_id") || form.getValues("template_id"))) {
        clearExercisesForMaterialization();
        return;
      }
      const current = form.getValues("exercises");
      if (current) {
        form.setValue("exercises", current.map((exercise) => ({
          ...exercise,
          completed: false,
          sets: exercise.sets.map((set) => ({ ...set, completed: false, completed_at: undefined })),
        })), { shouldDirty: true, shouldValidate: true });
      }
      return;
    }
    if (!form.getValues("started_at")) form.setValue("started_at", `${date}T18:00`, { shouldDirty: true });
    if (nextStatus === "finished" && !form.getValues("finished_at")) form.setValue("finished_at", `${date}T19:00`, { shouldDirty: true });
    if (!session) ensureManualExercise(nextStatus === "finished");
  };

  const handlePlanChange = (planID: string) => {
    form.setValue("plan_id", planID, { shouldDirty: true });
    form.setValue("template_id", "", { shouldDirty: true });
    if (session || !["scheduled", "cancelled", "excused"].includes(form.getValues("status"))) return;
    if (planID) {
      clearExercisesForMaterialization();
    } else if (!form.getValues("exercises")?.length) {
      ensureManualExercise(false);
    }
  };

  const handleTemplateChange = (templateID: string) => {
    form.setValue("template_id", templateID, { shouldDirty: true });
    if (!session && templateID && ["scheduled", "cancelled", "excused"].includes(form.getValues("status"))) {
      clearExercisesForMaterialization();
    }
  };

  const addSet = (index: number, type: SetValues["type"] = "working") => {
    const current = form.getValues(`exercises.${index}.sets`) ?? [];
    const source = form.getValues(`exercises.${index}.source`) ?? form.getValues("source") ?? "manual";
    form.setValue(`exercises.${index}.sets`, [...current, makeSet(status === "finished", source, type)], { shouldDirty: true, shouldValidate: true });
  };
  const removeSet = (exercise: number, set: number) => {
    const current = form.getValues(`exercises.${exercise}.sets`) ?? [];
    form.setValue(`exercises.${exercise}.sets`, current.filter((_, index) => index !== set), { shouldDirty: true, shouldValidate: true });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={session ? "Редактировать тренировку" : initialPlan ? `Запланировать: ${initialPlan.name}` : "Новая тренировка"} description="Завершённая тренировка сразу участвует в аналитике; планы и пропуски сохраняют scheduled_at." className="sm:max-w-4xl">
      <form onSubmit={form.handleSubmit((values) => save.mutate(values))} className="space-y-5">
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Field label="Дата" error={form.formState.errors.date?.message}><Input type="date" {...form.register("date")} onChange={(event) => handleDateChange(event.target.value)} /></Field>
          <Field label="Статус"><Select {...form.register("status")} onChange={(event) => handleStatusChange(event.target.value as WorkoutStatus)}><option value="finished">Завершена</option><option value="active">Активна</option><option value="scheduled">Запланирована</option><option value="cancelled">Отменена</option><option value="excused">Пропущена по причине</option></Select></Field>
          {["scheduled", "cancelled", "excused"].includes(status)
            ? <Field label="Запланировано" error={form.formState.errors.scheduled_at?.message}><Input type="datetime-local" {...form.register("scheduled_at")} /></Field>
            : <Field label="Начало" error={form.formState.errors.started_at?.message}><Input type="datetime-local" {...form.register("started_at")} /></Field>}
          {status === "finished"
            ? <Field label="Завершение" error={form.formState.errors.finished_at?.message}><Input type="datetime-local" {...form.register("finished_at")} /></Field>
            : <Field label="План"><Select disabled={snapshotProtected} {...form.register("plan_id")} onChange={(event) => handlePlanChange(event.target.value)}><option value="">Без плана</option>{planOptions.map((plan) => <option key={plan.id} value={plan.id}>{plan.name}</option>)}</Select></Field>}
        </div>
        {status === "finished" && <Field label="План"><Select disabled={snapshotProtected} {...form.register("plan_id")} onChange={(event) => handlePlanChange(event.target.value)}><option value="">Без плана</option>{planOptions.map((plan) => <option key={plan.id} value={plan.id}>{plan.name}</option>)}</Select></Field>}
        <div className="grid gap-4 sm:grid-cols-3">
          <Field label="Шаблон"><Select disabled={snapshotProtected} {...form.register("template_id")} onChange={(event) => handleTemplateChange(event.target.value)}><option value="">Первый шаблон плана / без шаблона</option>{templateOptions.filter((template) => !selectedPlanID || template.planID === selectedPlanID).map((template) => <option key={`${template.planID}-${template.id}`} value={template.id}>{template.planName} · {template.name}</option>)}</Select></Field>
          <Field label="Название программы"><Input {...form.register("program_name")} placeholder="Свободная тренировка" /></Field>
          <Field label="Strain"><Input type="number" min="0" max="21" step="0.1" {...form.register("strain", number)} /></Field>
        </div>
        <Field label="Заметка"><Textarea {...form.register("notes")} /></Field>

        <div className="space-y-4">
            {snapshotProtected && <div role="note" className="rounded-card border border-warning/25 bg-warning/[.045] p-4"><p className="text-sm font-semibold text-warning">Prescription защищён, фактические результаты доступны</p><p className="mt-1 text-sm leading-6 text-muted">Порядок, типы и плановые значения не заменяются. Вес, повторы, RIR, отдых, комментарии и отметки выполнения обновляются по стабильным ID подходов.</p></div>}
            {materializedSchedule && <div role="note" className="rounded-card border border-accent/20 bg-accent/[.04] p-4"><p className="text-sm font-semibold text-accent">Prescription будет создан из выбранного шаблона</p><p className="mt-1 text-sm leading-6 text-muted">После сохранения FitLog атомарно скопирует порядок, разминку, рабочие подходы, RIR и отдых в независимый снимок сессии.</p></div>}
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold">Упражнения</h3>
              {!snapshotProtected && !materializedSchedule && <Button type="button" size="sm" onClick={() => { const completed = status === "finished"; fields.append(makeExercise(completed, form.getValues("source") ?? "manual")); }}><Plus className="size-4" />Упражнение</Button>}
            </div>
            {form.formState.errors.exercises?.root?.message && <p className="text-xs text-critical">{form.formState.errors.exercises.root.message}</p>}
            {fields.fields.map((field, exerciseIndex) => {
              const sets = form.watch(`exercises.${exerciseIndex}.sets`) ?? [];
              return (
                <div key={field.formKey} className="rounded-card border border-line bg-canvas/35 p-4">
                  <div className="grid gap-3 sm:grid-cols-[1fr_1fr_auto]">
                    <Field label={`Упражнение ${exerciseIndex + 1}`}><Select disabled={snapshotProtected} {...form.register(`exercises.${exerciseIndex}.exercise_id`)}><option value="">Выберите из справочника</option>{listItems(exercises.data).map((exercise) => <option key={exercise.id} value={exercise.id}>{exercise.name}</option>)}</Select></Field>
                    <Field label="Название" error={form.formState.errors.exercises?.[exerciseIndex]?.name?.message}><Input disabled={snapshotProtected} {...form.register(`exercises.${exerciseIndex}.name`)} placeholder="Название упражнения" /></Field>
                    {!snapshotProtected && <Button type="button" variant="ghost" size="icon" className="self-end" aria-label="Удалить упражнение" disabled={fields.fields.length === 1} onClick={() => fields.remove(exerciseIndex)}><Trash2 className="size-4" /></Button>}
                  </div>
                  <div className="mt-3 grid gap-3 sm:grid-cols-[1fr_180px_auto]">
                    <Field label="Заметка к упражнению"><Textarea rows={2} {...form.register(`exercises.${exerciseIndex}.note`)} /></Field>
                    <Field label="Отдых после, сек" error={form.formState.errors.exercises?.[exerciseIndex]?.rest_after_exercise_seconds?.message}><Input disabled={snapshotProtected} type="number" min="0" {...form.register(`exercises.${exerciseIndex}.rest_after_exercise_seconds`, number)} /></Field>
                    <div className="flex items-end pb-2"><Checkbox label="Упражнение завершено" {...form.register(`exercises.${exerciseIndex}.completed`)} /></div>
                  </div>
                  <div className="mt-4 space-y-2">
                    {sets.map((_, setIndex) => (
                      <div key={setIndex} className="grid gap-2 rounded-control border border-line p-3 sm:grid-cols-[120px_repeat(4,minmax(80px,1fr))_auto]">
                        <Field label="Тип"><Select disabled={snapshotProtected && Boolean(sets[setIndex]?.id)} {...form.register(`exercises.${exerciseIndex}.sets.${setIndex}.type`)}><option value="warmup">Разминка</option><option value="working">Рабочий</option><option value="drop">Drop</option></Select></Field>
                        <Field label="Вес, кг"><Input type="number" min="0" step="0.1" placeholder={sets[setIndex]?.planned_weight_kg == null ? undefined : `план ${sets[setIndex]?.planned_weight_kg}`} {...form.register(`exercises.${exerciseIndex}.sets.${setIndex}.weight_kg`, number)} /></Field>
                        <Field label="Повторы" error={form.formState.errors.exercises?.[exerciseIndex]?.sets?.[setIndex]?.reps?.message}><Input type="number" min="1" placeholder={sets[setIndex]?.planned_min_reps == null ? undefined : sets[setIndex]?.planned_max_reps != null && sets[setIndex]?.planned_max_reps !== sets[setIndex]?.planned_min_reps ? `план ${sets[setIndex]?.planned_min_reps}–${sets[setIndex]?.planned_max_reps}` : `план ${sets[setIndex]?.planned_min_reps}`} {...form.register(`exercises.${exerciseIndex}.sets.${setIndex}.reps`, number)} /></Field>
                        <Field label="RIR"><Input type="number" min="0" step="0.5" placeholder={sets[setIndex]?.planned_rir == null ? undefined : `план ${sets[setIndex]?.planned_rir}`} {...form.register(`exercises.${exerciseIndex}.sets.${setIndex}.rir`, number)} /></Field>
                        <Field label="Отдых, сек"><Input type="number" min="0" {...form.register(`exercises.${exerciseIndex}.sets.${setIndex}.rest_seconds`, number)} /></Field>
                        <div className="flex items-end gap-1"><Checkbox label="Готов" {...form.register(`exercises.${exerciseIndex}.sets.${setIndex}.completed`)} />{(!snapshotProtected || !sets[setIndex]?.id) && <Button type="button" variant="ghost" size="icon" aria-label="Удалить подход" disabled={sets.length === 1} onClick={() => removeSet(exerciseIndex, setIndex)}><Trash2 className="size-4" /></Button>}</div>
                        <Field label="Комментарий к подходу" className="sm:col-span-full"><Input {...form.register(`exercises.${exerciseIndex}.sets.${setIndex}.comment`)} /></Field>
                      </div>
                    ))}
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">{!snapshotProtected && <Button type="button" variant="ghost" size="sm" onClick={() => addSet(exerciseIndex, "warmup")}>+ Разминка</Button>}<Button type="button" variant="ghost" size="sm" onClick={() => addSet(exerciseIndex, "working")}>+ Рабочий</Button><Button type="button" variant="ghost" size="sm" onClick={() => addSet(exerciseIndex, "drop")}>+ Drop</Button></div>
                </div>
              );
            })}
          </div>

        <InlineError error={save.error} />
        <DialogActions><Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>Отмена</Button><Button type="submit" variant="primary" loading={save.isPending}>{session ? "Сохранить изменения" : "Создать тренировку"}</Button></DialogActions>
      </form>
    </Dialog>
  );
}
