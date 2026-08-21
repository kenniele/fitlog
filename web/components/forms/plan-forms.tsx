"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { ArrowDown, ArrowUp, Plus, Trash2 } from "lucide-react";
import { z } from "zod";
import { Dialog, DialogActions } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Checkbox, Field, Input, Select, Textarea } from "@/components/ui/field";
import { InlineError } from "@/components/ui/states";
import { apiFetch } from "@/lib/api";
import type { Exercise, WorkoutPlan, WorkoutTemplate, WorkoutTemplateExercise, WorkoutTemplateWarmupSet } from "@/lib/types";

type ExerciseValues = {
  name: string;
  slug?: string;
  primary_muscle_group?: string;
  secondary_muscle_groups: string;
  exercise_type?: string;
  equipment?: string;
  unilateral: boolean;
  notes?: string;
  source: string;
  external_id?: string;
};
const exerciseSchema = z.object({
  name: z.string().trim().min(2, "Минимум два символа").max(160),
  slug: z.string().max(160).optional(),
  primary_muscle_group: z.string().max(160).optional(),
  secondary_muscle_groups: z.string().max(500),
  exercise_type: z.string().max(120).optional(),
  equipment: z.string().max(120).optional(),
  unilateral: z.boolean(),
  notes: z.string().max(1000).optional(),
  source: z.string().min(1),
  external_id: z.string().max(255).optional(),
});

export function ExerciseForm({ open, onOpenChange, exercise }: { open: boolean; onOpenChange: (value: boolean) => void; exercise?: Exercise | null }) {
  const client = useQueryClient();
  const form = useForm<ExerciseValues>({ resolver: zodResolver(exerciseSchema), defaultValues: { name: "", slug: "", primary_muscle_group: "", secondary_muscle_groups: "", exercise_type: "", equipment: "", unilateral: false, notes: "", source: "manual", external_id: "" } });
  useEffect(() => {
    if (!open) return;
    const legacyGroups = exercise?.muscle_groups ?? [];
    form.reset({
      name: exercise?.name ?? "",
      slug: exercise?.slug ?? "",
      primary_muscle_group: exercise?.primary_muscle_group ?? legacyGroups[0] ?? "",
      secondary_muscle_groups: (exercise?.secondary_muscle_groups ?? legacyGroups.slice(1)).join(", "),
      exercise_type: exercise?.exercise_type ?? "",
      equipment: exercise?.equipment ?? "",
      unilateral: exercise?.unilateral === true,
      notes: exercise?.notes ?? "",
      source: exercise?.source ?? "manual",
      external_id: exercise?.external_id ?? "",
    });
  }, [exercise, form, open]);
  const save = useMutation({
    mutationFn: (values: ExerciseValues) => apiFetch(`/exercises${exercise ? `/${exercise.id}` : ""}`, {
      method: exercise ? "PUT" : "POST",
      body: {
        name: values.name,
        slug: values.slug?.trim() || null,
        primary_muscle_group: values.primary_muscle_group?.trim() || null,
        secondary_muscle_groups: values.secondary_muscle_groups.split(",").map((value) => value.trim()).filter(Boolean),
        exercise_type: values.exercise_type?.trim() || null,
        equipment: values.equipment?.trim() || null,
        unilateral: values.unilateral,
        notes: values.notes ?? "",
        source: values.source,
        external_id: values.external_id?.trim() || null,
      },
    }),
    onSuccess: async () => { await client.invalidateQueries(); onOpenChange(false); },
  });
  return <Dialog open={open} onOpenChange={onOpenChange} title={exercise ? "Редактировать упражнение" : "Новое упражнение"} className="sm:max-w-2xl"><form onSubmit={form.handleSubmit((values) => save.mutate(values))} className="space-y-4"><div className="grid gap-4 sm:grid-cols-2"><Field label="Название" error={form.formState.errors.name?.message}><Input autoFocus {...form.register("name")} /></Field><Field label="Slug"><Input {...form.register("slug")} /></Field><Field label="Основная мышечная группа"><Input {...form.register("primary_muscle_group")} /></Field><Field label="Дополнительные группы" hint="Через запятую"><Input {...form.register("secondary_muscle_groups")} /></Field><Field label="Тип упражнения"><Input {...form.register("exercise_type")} /></Field><Field label="Оборудование"><Input {...form.register("equipment")} /></Field><Field label="Источник"><Select {...form.register("source")}><option value="manual">Вручную</option><option value="csv">CSV</option><option value="json">JSON</option><option value="whoop">WHOOP</option><option value="fatsecret">FatSecret</option></Select></Field><Field label="External ID"><Input {...form.register("external_id")} /></Field></div><Checkbox label="Одностороннее упражнение" {...form.register("unilateral")} /><Field label="Заметка"><Textarea {...form.register("notes")} /></Field><InlineError error={save.error} /><DialogActions><Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>Отмена</Button><Button variant="primary" type="submit" loading={save.isPending}>Сохранить</Button></DialogActions></form></Dialog>;
}

type PlanValues = { name: string; description?: string; days_per_week?: number };
const planSchema = z.object({
  name: z.string().trim().min(2, "Введите название").max(160),
  description: z.string().max(2000).optional(),
  days_per_week: z.number().int().min(1).max(7).optional(),
});
type DraftTemplate = WorkoutTemplate & { key: string };

const warmupDraftSchema = z.object({
  weight_mode: z.string().nullish(),
  bar: z.boolean().nullish(),
  weight_kg: z.number().positive().nullable().optional(),
  reps: z.number().int().min(1, "повторы разминки должны быть не меньше 1"),
}).superRefine((set, context) => {
  if (set.weight_mode !== "bar" && !set.bar && !(typeof set.weight_kg === "number" && set.weight_kg > 0)) {
    context.addIssue({ code: "custom", path: ["weight_kg"], message: "укажите положительный вес или режим «Гриф»" });
  }
});

const planExerciseDraftSchema = z.object({
  exercise_id: z.union([z.number(), z.string()]).nullish(),
  name: z.string().nullish(),
  working_sets: z.number().int().min(1, "нужен хотя бы один рабочий подход").nullish(),
  min_reps: z.number().int().min(1, "минимум повторов должен быть не меньше 1").nullish(),
  max_reps: z.number().int().min(1, "максимум повторов должен быть не меньше 1").nullish(),
  target_rir: z.number().min(0, "RIR не может быть отрицательным").nullish(),
  weight_step_kg: z.number().positive("шаг веса должен быть положительным").nullish(),
  starting_weight_kg: z.number().positive("стартовый вес должен быть положительным").nullish(),
  progression_type: z.string().nullish(),
  rest_seconds: z.number().int().min(0, "отдых не может быть отрицательным").nullish(),
  rest_after_exercise_seconds: z.number().int().min(0, "отдых не может быть отрицательным").nullish(),
  after_seconds: z.number().int().min(0).nullish(),
  warmup_sets: z.array(warmupDraftSchema).nullish(),
}).passthrough().superRefine((exercise, context) => {
  if (exercise.exercise_id == null && !exercise.name?.trim()) context.addIssue({ code: "custom", path: ["name"], message: "выберите или введите упражнение" });
  if (exercise.min_reps != null && exercise.max_reps != null && exercise.min_reps > exercise.max_reps) context.addIssue({ code: "custom", path: ["max_reps"], message: "максимум повторов меньше минимума" });
  const structured = exercise.working_sets != null || exercise.min_reps != null || exercise.max_reps != null ||
    exercise.target_rir != null || exercise.weight_step_kg != null || exercise.starting_weight_kg != null ||
    Boolean(exercise.progression_type) || Boolean(exercise.warmup_sets?.length);
  if (structured) {
    if (exercise.working_sets == null) context.addIssue({ code: "custom", path: ["working_sets"], message: "для structured progression укажите рабочие подходы" });
    if (exercise.min_reps == null) context.addIssue({ code: "custom", path: ["min_reps"], message: "для structured progression укажите минимум повторов" });
    if (exercise.max_reps == null) context.addIssue({ code: "custom", path: ["max_reps"], message: "для structured progression укажите максимум повторов" });
    if (exercise.weight_step_kg == null) context.addIssue({ code: "custom", path: ["weight_step_kg"], message: "для structured progression укажите шаг веса" });
  }
});

const templatesDraftSchema = z.array(z.object({
  name: z.string().trim().min(1, "введите название шаблона"),
  exercises: z.array(planExerciseDraftSchema).min(1, "добавьте хотя бы одно упражнение"),
}).passthrough()).min(1, "добавьте хотя бы один шаблон");

function makeTemplates(plan?: WorkoutPlan | null): DraftTemplate[] {
  const input = plan?.templates?.length ? plan.templates : [{ name: "Full Body A", position: 1, exercises: [] }];
  return input.map((template, index) => ({
    ...template,
    position: index + 1,
    key: String(template.id ?? `${Date.now()}-${index}`),
    exercises: template.exercises?.map((exercise) => ({ ...exercise, warmup_sets: exercise.warmup_sets ?? [] })) ?? [],
  }));
}

function cleanWarmup(set: WorkoutTemplateWarmupSet, position: number) {
  const mode = set.weight_mode === "bar" || set.bar ? "bar" : "kg";
  return {
    position: position + 1,
    weight_mode: mode,
    bar: mode === "bar",
    weight_kg: mode === "bar" ? null : set.weight_kg ?? null,
    reps: set.reps,
  };
}

export function PlanForm({ open, onOpenChange, plan }: { open: boolean; onOpenChange: (value: boolean) => void; plan?: WorkoutPlan | null }) {
  const client = useQueryClient();
  const form = useForm<PlanValues>({ resolver: zodResolver(planSchema), defaultValues: { name: "", description: "", days_per_week: 3 } });
  const [templates, setTemplates] = useState<DraftTemplate[]>(makeTemplates(plan));
  const [templateErrors, setTemplateErrors] = useState<string[]>([]);

  useEffect(() => {
    if (!open) return;
    form.reset({ name: plan?.name ?? "", description: plan?.description ?? "", days_per_week: plan?.days_per_week ?? 3 });
    setTemplates(makeTemplates(plan));
    setTemplateErrors([]);
  }, [form, open, plan]);

  const save = useMutation({
    mutationFn: (values: PlanValues) => apiFetch(`/workout-plans${plan ? `/${plan.id}` : ""}`, {
      method: plan ? "PUT" : "POST",
      body: {
        ...values,
        source: plan?.source ?? "manual",
        external_id: plan?.external_id ?? null,
        templates: templates.map((template, position) => ({
          name: template.name,
          description: template.description ?? "",
          external_id: template.external_id ?? "",
          position: position + 1,
          exercises: (template.exercises ?? []).map((exercise, exercisePosition) => ({
            exercise_id: exercise.exercise_id ?? null,
            name: exercise.name ?? "",
            position: exercisePosition + 1,
            working_sets: exercise.working_sets ?? null,
            min_reps: exercise.min_reps ?? null,
            max_reps: exercise.max_reps ?? null,
            target_rir: exercise.target_rir ?? null,
            weight_step_kg: exercise.weight_step_kg ?? null,
            starting_weight_kg: exercise.starting_weight_kg ?? null,
            progression_type: exercise.progression_type || null,
            warmup_sets: (exercise.warmup_sets ?? []).map(cleanWarmup),
            rest_seconds: exercise.rest_seconds ?? null,
            rest_after_exercise_seconds: exercise.rest_after_exercise_seconds ?? exercise.after_seconds ?? null,
            notes: exercise.notes ?? "",
          })),
        })),
      },
    }),
    onSuccess: async () => { await client.invalidateQueries(); onOpenChange(false); },
  });

  const patchTemplate = (index: number, changes: Partial<DraftTemplate>) => setTemplates((current) => current.map((template, position) => position === index ? { ...template, ...changes } : template));
  const move = (index: number, by: number) => setTemplates((current) => {
    const target = index + by;
    if (target < 0 || target >= current.length) return current;
    const copy = [...current];
    [copy[index], copy[target]] = [copy[target]!, copy[index]!];
    return copy;
  });
  const patchExercise = (templateIndex: number, exerciseIndex: number, changes: Partial<WorkoutTemplateExercise>) => patchTemplate(templateIndex, {
    exercises: (templates[templateIndex]?.exercises ?? []).map((exercise, index) => index === exerciseIndex ? { ...exercise, ...changes } : exercise),
  });
  const addExercise = (index: number) => patchTemplate(index, {
    exercises: [...(templates[index]?.exercises ?? []), {
      name: "",
      position: (templates[index]?.exercises?.length ?? 0) + 1,
      working_sets: 3,
      min_reps: 8,
      max_reps: 12,
      target_rir: 2,
      weight_step_kg: 2.5,
      progression_type: "double",
      rest_seconds: 120,
      rest_after_exercise_seconds: 180,
      warmup_sets: [],
    }],
  });
  const moveExercise = (templateIndex: number, exerciseIndex: number, by: number) => setTemplates((current) => current.map((template, index) => {
    if (index !== templateIndex) return template;
    const exercises = [...(template.exercises ?? [])];
    const target = exerciseIndex + by;
    if (target < 0 || target >= exercises.length) return template;
    [exercises[exerciseIndex], exercises[target]] = [exercises[target]!, exercises[exerciseIndex]!];
    return { ...template, exercises };
  }));
  const patchWarmup = (templateIndex: number, exerciseIndex: number, warmupIndex: number, changes: Partial<WorkoutTemplateWarmupSet>) => {
    const exercise = templates[templateIndex]?.exercises?.[exerciseIndex];
    if (!exercise) return;
    patchExercise(templateIndex, exerciseIndex, {
      warmup_sets: (exercise.warmup_sets ?? []).map((set, index) => index === warmupIndex ? { ...set, ...changes } : set),
    });
  };
  const optionalInputNumber = (value: string) => value === "" ? null : Number(value);
  const submitPlan = (values: PlanValues) => {
    const parsed = templatesDraftSchema.safeParse(templates);
    if (!parsed.success) {
      setTemplateErrors(parsed.error.issues.map((issue) => {
        const templateIndex = typeof issue.path[0] === "number" ? issue.path[0] + 1 : null;
        const exerciseIndex = issue.path[1] === "exercises" && typeof issue.path[2] === "number" ? issue.path[2] + 1 : null;
        const location = [templateIndex && `шаблон ${templateIndex}`, exerciseIndex && `упражнение ${exerciseIndex}`].filter(Boolean).join(", ");
        return `${location ? `${location}: ` : ""}${issue.message}`;
      }));
      return;
    }
    setTemplateErrors([]);
    save.mutate(values);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={plan ? "Редактировать план" : "Новый тренировочный план"} description="Prescription, разминка и порядок сохраняются в общей модели Telegram и Control Center." className="sm:max-w-6xl">
      <form onSubmit={form.handleSubmit(submitPlan)} className="space-y-5">
        <div className="grid gap-4 sm:grid-cols-[1fr_160px]">
          <Field label="Название" error={form.formState.errors.name?.message}><Input {...form.register("name")} /></Field>
          <Field label="Дней в неделю"><Input type="number" min="1" max="7" {...form.register("days_per_week", { setValueAs: (value) => value === "" ? undefined : Number(value) })} /></Field>
        </div>
        <Field label="Описание"><Textarea {...form.register("description")} /></Field>
        <div className="space-y-4">
          <div className="flex items-center justify-between"><h3 className="text-sm font-semibold">Шаблоны</h3><Button type="button" size="sm" onClick={() => setTemplates((current) => [...current, { key: String(Date.now()), name: `Шаблон ${current.length + 1}`, position: current.length + 1, exercises: [] }])}><Plus className="size-4" />Шаблон</Button></div>
          {templates.map((template, templateIndex) => (
            <section key={template.key} className="rounded-card border border-line bg-canvas/35 p-4">
              <div className="flex items-end gap-2">
                <Field label={`Шаблон ${templateIndex + 1}`} className="flex-1"><Input value={template.name} onChange={(event) => patchTemplate(templateIndex, { name: event.target.value })} required /></Field>
                <Button type="button" variant="ghost" size="icon" aria-label="Шаблон выше" disabled={templateIndex === 0} onClick={() => move(templateIndex, -1)}><ArrowUp className="size-4" /></Button>
                <Button type="button" variant="ghost" size="icon" aria-label="Шаблон ниже" disabled={templateIndex === templates.length - 1} onClick={() => move(templateIndex, 1)}><ArrowDown className="size-4" /></Button>
                <Button type="button" variant="ghost" size="icon" aria-label="Удалить шаблон" disabled={templates.length === 1} onClick={() => setTemplates((current) => current.filter((_, index) => index !== templateIndex))}><Trash2 className="size-4" /></Button>
              </div>
              <div className="mt-4 space-y-3">
                {(template.exercises ?? []).map((exercise, exerciseIndex) => (
                  <div key={String(exercise.id ?? exerciseIndex)} className="rounded-control border border-line p-3">
                    <div className="grid gap-2 md:grid-cols-[minmax(160px,1fr)_70px_78px_78px_64px_90px_90px_auto]">
                      <Field label="Упражнение"><Input value={exercise.name ?? ""} onChange={(event) => patchExercise(templateIndex, exerciseIndex, { name: event.target.value, exercise_id: event.target.value === exercise.name ? exercise.exercise_id : undefined })} required /></Field>
                      <Field label="Сеты"><Input type="number" min="1" value={exercise.working_sets ?? ""} onChange={(event) => patchExercise(templateIndex, exerciseIndex, { working_sets: optionalInputNumber(event.target.value) })} /></Field>
                      <Field label="Мин. повт."><Input type="number" min="1" value={exercise.min_reps ?? ""} onChange={(event) => patchExercise(templateIndex, exerciseIndex, { min_reps: optionalInputNumber(event.target.value) })} /></Field>
                      <Field label="Макс. повт."><Input type="number" min="1" value={exercise.max_reps ?? ""} onChange={(event) => patchExercise(templateIndex, exerciseIndex, { max_reps: optionalInputNumber(event.target.value) })} /></Field>
                      <Field label="RIR"><Input type="number" min="0" step="0.5" value={exercise.target_rir ?? ""} onChange={(event) => patchExercise(templateIndex, exerciseIndex, { target_rir: optionalInputNumber(event.target.value) })} /></Field>
                      <Field label="Между сетами"><Input type="number" min="0" value={exercise.rest_seconds ?? ""} onChange={(event) => patchExercise(templateIndex, exerciseIndex, { rest_seconds: optionalInputNumber(event.target.value) })} /></Field>
                      <Field label="После упражнения"><Input type="number" min="0" value={exercise.rest_after_exercise_seconds ?? exercise.after_seconds ?? ""} onChange={(event) => patchExercise(templateIndex, exerciseIndex, { rest_after_exercise_seconds: optionalInputNumber(event.target.value) })} /></Field>
                      <div className="flex items-end gap-1">
                        <Button type="button" variant="ghost" size="icon" aria-label="Упражнение выше" disabled={exerciseIndex === 0} onClick={() => moveExercise(templateIndex, exerciseIndex, -1)}><ArrowUp className="size-4" /></Button>
                        <Button type="button" variant="ghost" size="icon" aria-label="Упражнение ниже" disabled={exerciseIndex === (template.exercises?.length ?? 0) - 1} onClick={() => moveExercise(templateIndex, exerciseIndex, 1)}><ArrowDown className="size-4" /></Button>
                        <Button type="button" variant="ghost" size="icon" aria-label="Удалить упражнение" onClick={() => patchTemplate(templateIndex, { exercises: (template.exercises ?? []).filter((_, index) => index !== exerciseIndex) })}><Trash2 className="size-4" /></Button>
                      </div>
                    </div>
                    <div className="mt-3 grid gap-2 md:grid-cols-4">
                      <Field label="Шаг веса, кг" hint="Обязателен для structured progression"><Input type="number" min="0.01" step="0.01" value={exercise.weight_step_kg ?? ""} onChange={(event) => patchExercise(templateIndex, exerciseIndex, { weight_step_kg: optionalInputNumber(event.target.value) })} /></Field>
                      <Field label="Стартовый вес, кг"><Input type="number" min="0.01" step="0.01" value={exercise.starting_weight_kg ?? ""} onChange={(event) => patchExercise(templateIndex, exerciseIndex, { starting_weight_kg: optionalInputNumber(event.target.value) })} /></Field>
                      <Field label="Прогрессия"><Select value={exercise.progression_type ?? ""} onChange={(event) => patchExercise(templateIndex, exerciseIndex, { progression_type: event.target.value || null })}><option value="">Без прогрессии</option><option value="double">Double progression</option></Select></Field>
                      <Field label="Заметка"><Input value={exercise.notes ?? ""} onChange={(event) => patchExercise(templateIndex, exerciseIndex, { notes: event.target.value })} /></Field>
                    </div>
                    <div className="mt-3 rounded-control border border-line/70 bg-canvas/30 p-3">
                      <div className="flex items-center justify-between"><p className="text-xs font-semibold text-muted">Разминка</p><Button type="button" variant="ghost" size="sm" onClick={() => patchExercise(templateIndex, exerciseIndex, { warmup_sets: [...(exercise.warmup_sets ?? []), { weight_mode: "kg", weight_kg: 20, reps: 10 }] })}><Plus className="size-3.5" />Подход</Button></div>
                      {(exercise.warmup_sets ?? []).length ? <div className="mt-2 space-y-2">{(exercise.warmup_sets ?? []).map((set, warmupIndex) => {
                        const mode = set.weight_mode === "bar" || set.bar ? "bar" : "kg";
                        return <div key={warmupIndex} className="grid gap-2 sm:grid-cols-[140px_1fr_1fr_auto]"><Select aria-label={`Режим разминки ${warmupIndex + 1}`} value={mode} onChange={(event) => patchWarmup(templateIndex, exerciseIndex, warmupIndex, { weight_mode: event.target.value, bar: event.target.value === "bar", weight_kg: event.target.value === "bar" ? null : set.weight_kg ?? 20 })}><option value="kg">Вес, кг</option><option value="bar">Гриф</option></Select><Input aria-label={`Вес разминки ${warmupIndex + 1}`} type="number" min="0.01" step="0.01" disabled={mode === "bar"} value={mode === "bar" ? "" : set.weight_kg ?? ""} onChange={(event) => patchWarmup(templateIndex, exerciseIndex, warmupIndex, { weight_kg: optionalInputNumber(event.target.value) })} /><Input aria-label={`Повторы разминки ${warmupIndex + 1}`} type="number" min="1" value={set.reps} onChange={(event) => patchWarmup(templateIndex, exerciseIndex, warmupIndex, { reps: Number(event.target.value) })} /><Button type="button" variant="ghost" size="icon" aria-label={`Удалить разминку ${warmupIndex + 1}`} onClick={() => patchExercise(templateIndex, exerciseIndex, { warmup_sets: (exercise.warmup_sets ?? []).filter((_, index) => index !== warmupIndex) })}><Trash2 className="size-4" /></Button></div>;
                      })}</div> : <p className="mt-2 text-xs text-muted">Разминочных подходов нет.</p>}
                    </div>
                  </div>
                ))}
              </div>
              <Button type="button" variant="ghost" size="sm" className="mt-3" onClick={() => addExercise(templateIndex)}><Plus className="size-4" />Упражнение</Button>
            </section>
          ))}
        </div>
        {templateErrors.length > 0 && <div role="alert" className="rounded-control border border-critical/20 bg-critical/10 px-3 py-2 text-sm text-critical"><p className="font-medium">Проверьте структуру плана</p><ul className="mt-1 list-disc space-y-0.5 pl-5 text-xs">{templateErrors.map((error) => <li key={error}>{error}</li>)}</ul></div>}
        <InlineError error={save.error} />
        <DialogActions><Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>Отмена</Button><Button type="submit" variant="primary" loading={save.isPending}>Сохранить план</Button></DialogActions>
      </form>
    </Dialog>
  );
}
