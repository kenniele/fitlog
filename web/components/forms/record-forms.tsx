"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Dialog, DialogActions } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Checkbox, Field, Input, Select, Textarea } from "@/components/ui/field";
import { InlineError } from "@/components/ui/states";
import { apiFetch } from "@/lib/api";
import { dateInTimeZone, dateTimeLocalInTimeZone, getDashboardTimezone, toDateTimeLocal } from "@/lib/format";
import type { BodyMeasurement, NutritionDay, RecoveryEntry, SleepEntry } from "@/lib/types";

const optionalNumber = z.number().finite().nonnegative().optional();
const positiveNumber = z.number().finite().positive().optional();
const numeric = { setValueAs: (value: string) => value === "" ? undefined : Number(value) };

type RecoveryValues = {
  date: string; recovery_score?: number; hrv_ms?: number; resting_heart_rate_bpm?: number; respiratory_rate?: number; spo2_percent?: number; skin_temperature_celsius?: number; daily_strain?: number; source: string; notes?: string;
};
const recoverySchema = z.object({ date: z.string().min(1, "Укажите дату"), recovery_score: optionalNumber.refine((value) => value === undefined || value <= 100, "Максимум 100"), hrv_ms: optionalNumber, resting_heart_rate_bpm: positiveNumber, respiratory_rate: positiveNumber, spo2_percent: optionalNumber.refine((value) => value === undefined || value <= 100, "Максимум 100"), skin_temperature_celsius: z.number().finite().optional(), daily_strain: optionalNumber.refine((value) => value === undefined || value <= 21, "Максимум 21"), source: z.string().min(1), notes: z.string().max(1000).optional() });

type SleepValues = { date: string; sleep_start?: string; sleep_end?: string; is_nap: boolean; actual_sleep_seconds?: number; time_in_bed_seconds?: number; rem_seconds?: number; deep_seconds?: number; light_seconds?: number; awake_seconds?: number; sleep_performance_percent?: number; efficiency_percent?: number; consistency_percent?: number; sleep_debt_seconds?: number; disturbances?: number; source: string; notes?: string };
const percentage = optionalNumber.refine((value) => value === undefined || value <= 100, "Максимум 100");
const sleepSchema = z.object({ date: z.string().min(1, "Укажите дату"), sleep_start: z.string().optional(), sleep_end: z.string().optional(), is_nap: z.boolean(), actual_sleep_seconds: optionalNumber, time_in_bed_seconds: optionalNumber, rem_seconds: optionalNumber, deep_seconds: optionalNumber, light_seconds: optionalNumber, awake_seconds: optionalNumber, sleep_performance_percent: percentage, efficiency_percent: percentage, consistency_percent: percentage, sleep_debt_seconds: optionalNumber, disturbances: z.number().int().nonnegative().optional(), source: z.string().min(1), notes: z.string().max(1000).optional() });

type NutritionValues = { date: string; calories_kcal?: number; protein_g?: number; fat_g?: number; carbohydrates_g?: number; fiber_g?: number; sugar_g?: number; saturated_fat_g?: number; sodium_mg?: number; potassium_mg?: number; water_ml?: number; source: string; notes?: string };
const nutritionSchema = z.object({ date: z.string().min(1, "Укажите дату"), calories_kcal: optionalNumber, protein_g: optionalNumber, fat_g: optionalNumber, carbohydrates_g: optionalNumber, fiber_g: optionalNumber, sugar_g: optionalNumber, saturated_fat_g: optionalNumber, sodium_mg: optionalNumber, potassium_mg: optionalNumber, water_ml: optionalNumber, source: z.string().min(1), notes: z.string().max(1000).optional() });

type BodySegmentValues = { segment: "left_arm" | "right_arm" | "trunk" | "left_leg" | "right_leg"; lean_mass_kg?: number; lean_percent?: number; fat_mass_kg?: number; fat_percent?: number };
type BodyValues = {
  measured_at: string; weight_kg?: number; body_fat_percent?: number; fat_mass_kg?: number; lean_mass_kg?: number; skeletal_muscle_mass_kg?: number;
  waist_cm?: number; chest_cm?: number; biceps_cm?: number; thigh_cm?: number; total_body_water_l?: number; intracellular_water_l?: number;
  extracellular_water_l?: number; ecw_tbw_ratio?: number; protein_mass_kg?: number; mineral_mass_kg?: number; bmi?: number; visceral_fat_level?: number;
  visceral_fat_area_cm2?: number; basal_metabolic_rate_kcal?: number; inbody_score?: number; phase_angle_degrees?: number;
  segments: BodySegmentValues[]; source: string; notes?: string;
};
const bodySegments = [
  { segment: "left_arm", label: "Левая рука" }, { segment: "right_arm", label: "Правая рука" }, { segment: "trunk", label: "Корпус" },
  { segment: "left_leg", label: "Левая нога" }, { segment: "right_leg", label: "Правая нога" },
] as const;
const emptyBodySegments = (): BodySegmentValues[] => bodySegments.map(({ segment }) => ({ segment }));
const bodySegmentSchema = z.object({
  segment: z.enum(["left_arm", "right_arm", "trunk", "left_leg", "right_leg"]), lean_mass_kg: optionalNumber,
  lean_percent: optionalNumber, fat_mass_kg: optionalNumber, fat_percent: optionalNumber,
});
const bodyScalarFields: Array<keyof BodyValues> = [
  "weight_kg", "body_fat_percent", "fat_mass_kg", "lean_mass_kg", "skeletal_muscle_mass_kg", "waist_cm", "chest_cm", "biceps_cm", "thigh_cm",
  "total_body_water_l", "intracellular_water_l", "extracellular_water_l", "ecw_tbw_ratio", "protein_mass_kg", "mineral_mass_kg", "bmi",
  "visceral_fat_level", "visceral_fat_area_cm2", "basal_metabolic_rate_kcal", "inbody_score", "phase_angle_degrees",
];
const bodySchema = z.object({
  measured_at: z.string().min(1, "Укажите дату и время"), weight_kg: positiveNumber,
  body_fat_percent: optionalNumber.refine((value) => value === undefined || value <= 100, "Максимум 100"),
  fat_mass_kg: optionalNumber, lean_mass_kg: optionalNumber, skeletal_muscle_mass_kg: optionalNumber,
  waist_cm: positiveNumber, chest_cm: positiveNumber, biceps_cm: positiveNumber, thigh_cm: positiveNumber,
  total_body_water_l: optionalNumber, intracellular_water_l: optionalNumber, extracellular_water_l: optionalNumber,
  ecw_tbw_ratio: optionalNumber.refine((value) => value === undefined || value <= 1, "Максимум 1"),
  protein_mass_kg: optionalNumber, mineral_mass_kg: optionalNumber, bmi: positiveNumber, visceral_fat_level: optionalNumber,
  visceral_fat_area_cm2: optionalNumber, basal_metabolic_rate_kcal: optionalNumber,
  inbody_score: optionalNumber.refine((value) => value === undefined || value <= 200, "Максимум 200"), phase_angle_degrees: optionalNumber,
  segments: z.array(bodySegmentSchema), source: z.string().min(1), notes: z.string().max(1000).optional(),
}).superRefine((value, context) => {
  const hasScalar = bodyScalarFields.some((key) => typeof value[key] === "number");
  if (!hasScalar) context.addIssue({ code: "custom", message: "Добавьте хотя бы один общий показатель InBody", path: ["weight_kg"] });
});

const sources = [<option key="manual" value="manual">Вручную</option>, <option key="whoop" value="whoop">WHOOP</option>, <option key="fatsecret" value="fatsecret">FatSecret</option>];

function localDateTime(timeZone: string) { return dateTimeLocalInTimeZone(new Date(), timeZone); }

function useSave<T extends object>(endpoint: string, onClose: () => void, id?: string | number, preserved: Record<string, unknown> = {}) {
  const client = useQueryClient();
  return useMutation({ mutationFn: (body: T) => apiFetch(`${endpoint}${id == null ? "" : `/${id}`}`, { method: id == null ? "POST" : "PUT", body: { ...preserved, ...body } }), onSuccess: async () => { await client.invalidateQueries(); onClose(); } });
}

export function RecoveryForm({ open, onOpenChange, entry }: { open: boolean; onOpenChange: (open: boolean) => void; entry?: RecoveryEntry | null }) {
  const timeZone = getDashboardTimezone();
  const form = useForm<RecoveryValues>({ resolver: zodResolver(recoverySchema), defaultValues: { date: dateInTimeZone(new Date(), timeZone), source: "manual" } });
  useEffect(() => { if (open) form.reset(entry ? { date: entry.date, recovery_score: entry.recovery_score ?? undefined, hrv_ms: entry.hrv_ms ?? undefined, resting_heart_rate_bpm: entry.resting_heart_rate_bpm ?? undefined, respiratory_rate: entry.respiratory_rate ?? undefined, spo2_percent: entry.spo2_percent ?? undefined, skin_temperature_celsius: entry.skin_temperature_celsius ?? undefined, daily_strain: entry.daily_strain ?? undefined, source: entry.source ?? "manual", notes: entry.notes ?? "" } : { date: dateInTimeZone(new Date(), timeZone), source: "manual", notes: "" }); }, [entry, form, open, timeZone]);
  const save = useSave<RecoveryValues>("/recovery", () => { form.reset(); onOpenChange(false); }, entry?.id, { external_id: entry?.external_id ?? null });
  return <Dialog open={open} onOpenChange={onOpenChange} title={entry ? "Редактировать восстановление" : "Запись восстановления"} description="Пустые показатели сохраняются как missing, а не как нули."><form onSubmit={form.handleSubmit((values) => save.mutate(values))} className="space-y-4"><div className="grid gap-4 sm:grid-cols-2"><Field label="Дата" error={form.formState.errors.date?.message}><Input type="date" {...form.register("date")} /></Field><Field label="Источник"><Select {...form.register("source")}>{sources}</Select></Field><Field label="Recovery Score" error={form.formState.errors.recovery_score?.message}><Input type="number" min="0" max="100" step="0.1" {...form.register("recovery_score", numeric)} /></Field><Field label="HRV, мс" error={form.formState.errors.hrv_ms?.message}><Input type="number" min="0" step="0.1" {...form.register("hrv_ms", numeric)} /></Field><Field label="Resting HR, bpm" error={form.formState.errors.resting_heart_rate_bpm?.message}><Input type="number" min="0.1" step="0.1" {...form.register("resting_heart_rate_bpm", numeric)} /></Field><Field label="Respiratory rate" error={form.formState.errors.respiratory_rate?.message}><Input type="number" min="0.1" step="0.1" {...form.register("respiratory_rate", numeric)} /></Field><Field label="SpO₂, %" error={form.formState.errors.spo2_percent?.message}><Input type="number" min="0" max="100" step="0.1" {...form.register("spo2_percent", numeric)} /></Field><Field label="Skin temperature, °C" error={form.formState.errors.skin_temperature_celsius?.message}><Input type="number" step="0.01" {...form.register("skin_temperature_celsius", numeric)} /></Field><Field label="Daily Strain" error={form.formState.errors.daily_strain?.message}><Input type="number" min="0" max="21" step="0.1" {...form.register("daily_strain", numeric)} /></Field></div><Field label="Заметка"><Textarea {...form.register("notes")} /></Field><InlineError error={save.error} /><DialogActions><Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>Отмена</Button><Button type="submit" variant="primary" loading={save.isPending}>Сохранить</Button></DialogActions></form></Dialog>;
}

export function SleepForm({ open, onOpenChange, entry }: { open: boolean; onOpenChange: (open: boolean) => void; entry?: SleepEntry | null }) {
  const timeZone = getDashboardTimezone();
  const form = useForm<SleepValues>({ resolver: zodResolver(sleepSchema), defaultValues: { date: dateInTimeZone(new Date(), timeZone), source: "manual", is_nap: false, notes: "" } });
  useEffect(() => { if (open) form.reset(entry ? { date: entry.date ?? toDateTimeLocal(entry.sleep_start, timeZone).slice(0, 10), sleep_start: toDateTimeLocal(entry.sleep_start, timeZone), sleep_end: toDateTimeLocal(entry.sleep_end, timeZone), is_nap: entry.is_nap === true, actual_sleep_seconds: entry.actual_sleep_seconds ?? undefined, time_in_bed_seconds: entry.time_in_bed_seconds ?? undefined, rem_seconds: entry.rem_seconds ?? undefined, deep_seconds: entry.deep_seconds ?? undefined, light_seconds: entry.light_seconds ?? undefined, awake_seconds: entry.awake_seconds ?? undefined, sleep_performance_percent: entry.sleep_performance_percent ?? undefined, efficiency_percent: entry.efficiency_percent ?? undefined, consistency_percent: entry.consistency_percent ?? undefined, sleep_debt_seconds: entry.sleep_debt_seconds ?? undefined, disturbances: entry.disturbances ?? undefined, source: entry.source ?? "manual", notes: entry.notes ?? "" } : { date: dateInTimeZone(new Date(), timeZone), source: "manual", is_nap: false, notes: "" }); }, [entry, form, open, timeZone]);
  const save = useSave<SleepValues>("/sleep", () => { form.reset(); onOpenChange(false); }, entry?.id, { external_id: entry?.external_id ?? null });
  const duration = (name: keyof SleepValues, label: string) => <Field label={label} error={form.formState.errors[name]?.message}><Input type="number" min="0" step="60" placeholder="секунды" {...form.register(name, numeric)} /></Field>;
  return <Dialog open={open} onOpenChange={onOpenChange} title={entry ? "Редактировать сон" : "Запись сна"} description="Продолжительности передаются в секундах согласно API."><form onSubmit={form.handleSubmit((values) => save.mutate(values))} className="space-y-4"><div className="grid gap-4 sm:grid-cols-2"><Field label="Дата" error={form.formState.errors.date?.message}><Input type="date" {...form.register("date")} /></Field><Field label="Источник"><Select {...form.register("source")}>{sources}</Select></Field><Field label="Начало"><Input type="datetime-local" {...form.register("sleep_start")} /></Field><Field label="Конец"><Input type="datetime-local" {...form.register("sleep_end")} /></Field>{duration("actual_sleep_seconds", "Фактический сон")}{duration("time_in_bed_seconds", "В постели")}{duration("rem_seconds", "REM")}{duration("deep_seconds", "Deep")}{duration("light_seconds", "Light")}{duration("awake_seconds", "Awake")}{duration("sleep_debt_seconds", "Sleep debt")}<Field label="Sleep Performance, %" error={form.formState.errors.sleep_performance_percent?.message}><Input type="number" min="0" max="100" {...form.register("sleep_performance_percent", numeric)} /></Field><Field label="Efficiency, %" error={form.formState.errors.efficiency_percent?.message}><Input type="number" min="0" max="100" {...form.register("efficiency_percent", numeric)} /></Field><Field label="Consistency, %" error={form.formState.errors.consistency_percent?.message}><Input type="number" min="0" max="100" {...form.register("consistency_percent", numeric)} /></Field><Field label="Пробуждения" error={form.formState.errors.disturbances?.message}><Input type="number" min="0" {...form.register("disturbances", numeric)} /></Field></div><Checkbox label="Дневной сон" {...form.register("is_nap")} /><Field label="Заметка"><Textarea {...form.register("notes")} /></Field><InlineError error={save.error} /><DialogActions><Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>Отмена</Button><Button type="submit" variant="primary" loading={save.isPending}>Сохранить</Button></DialogActions></form></Dialog>;
}

export function NutritionForm({ open, onOpenChange, entry }: { open: boolean; onOpenChange: (open: boolean) => void; entry?: NutritionDay | null }) {
  const timeZone = getDashboardTimezone();
  const form = useForm<NutritionValues>({ resolver: zodResolver(nutritionSchema), defaultValues: { date: dateInTimeZone(new Date(), timeZone), source: "manual" } });
  useEffect(() => { if (open) form.reset(entry ? { date: entry.date, calories_kcal: entry.calories_kcal ?? undefined, protein_g: entry.protein_g ?? undefined, fat_g: entry.fat_g ?? undefined, carbohydrates_g: entry.carbohydrates_g ?? undefined, fiber_g: entry.fiber_g ?? undefined, sugar_g: entry.sugar_g ?? undefined, saturated_fat_g: entry.saturated_fat_g ?? undefined, sodium_mg: entry.sodium_mg ?? undefined, potassium_mg: entry.potassium_mg ?? undefined, water_ml: entry.water_ml ?? undefined, source: entry.source ?? "manual", notes: entry.notes ?? "" } : { date: dateInTimeZone(new Date(), timeZone), source: "manual" }); }, [entry, form, open, timeZone]);
  const save = useSave<NutritionValues>("/nutrition", () => { form.reset(); onOpenChange(false); }, entry?.id, { external_id: entry?.external_id ?? null });
  const number = (name: keyof NutritionValues, label: string, suffix: string) => <Field label={`${label}, ${suffix}`} error={form.formState.errors[name]?.message}><Input type="number" min="0" step="0.1" {...form.register(name, numeric)} /></Field>;
  return <Dialog open={open} onOpenChange={onOpenChange} title={entry ? "Редактировать питание" : "Дневные итоги питания"} description="Записываются агрегаты дня, не выдуманная база продуктов."><form onSubmit={form.handleSubmit((values) => save.mutate(values))} className="space-y-4"><div className="grid gap-4 sm:grid-cols-2"><Field label="Дата" error={form.formState.errors.date?.message}><Input type="date" {...form.register("date")} /></Field><Field label="Источник"><Select {...form.register("source")}>{sources}</Select></Field>{number("calories_kcal", "Калории", "ккал")}{number("protein_g", "Белки", "г")}{number("fat_g", "Жиры", "г")}{number("carbohydrates_g", "Углеводы", "г")}{number("fiber_g", "Клетчатка", "г")}{number("sugar_g", "Сахар", "г")}{number("saturated_fat_g", "Насыщенные жиры", "г")}{number("sodium_mg", "Натрий", "мг")}{number("potassium_mg", "Калий", "мг")}{number("water_ml", "Вода", "мл")}</div><Field label="Заметка"><Textarea {...form.register("notes")} /></Field><InlineError error={save.error} /><DialogActions><Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>Отмена</Button><Button type="submit" variant="primary" loading={save.isPending}>Сохранить</Button></DialogActions></form></Dialog>;
}

export function BodyForm({ open, onOpenChange, entry }: { open: boolean; onOpenChange: (open: boolean) => void; entry?: BodyMeasurement | null }) {
  const timeZone = getDashboardTimezone();
  const form = useForm<BodyValues>({ resolver: zodResolver(bodySchema), defaultValues: { measured_at: localDateTime(timeZone), source: "inbody", notes: "", segments: emptyBodySegments() } });
  useEffect(() => {
    if (!open) return;
    form.reset(entry ? {
      measured_at: toDateTimeLocal(entry.measured_at, timeZone), weight_kg: entry.weight_kg ?? undefined,
      body_fat_percent: entry.body_fat_percent ?? undefined, fat_mass_kg: entry.fat_mass_kg ?? undefined,
      lean_mass_kg: entry.lean_mass_kg ?? undefined, skeletal_muscle_mass_kg: entry.skeletal_muscle_mass_kg ?? undefined,
      waist_cm: entry.waist_cm ?? undefined, chest_cm: entry.chest_cm ?? undefined, biceps_cm: entry.biceps_cm ?? undefined,
      thigh_cm: entry.thigh_cm ?? undefined, total_body_water_l: entry.total_body_water_l ?? undefined,
      intracellular_water_l: entry.intracellular_water_l ?? undefined, extracellular_water_l: entry.extracellular_water_l ?? undefined,
      ecw_tbw_ratio: entry.ecw_tbw_ratio ?? undefined, protein_mass_kg: entry.protein_mass_kg ?? undefined,
      mineral_mass_kg: entry.mineral_mass_kg ?? undefined, bmi: entry.bmi ?? undefined,
      visceral_fat_level: entry.visceral_fat_level ?? undefined, visceral_fat_area_cm2: entry.visceral_fat_area_cm2 ?? undefined,
      basal_metabolic_rate_kcal: entry.basal_metabolic_rate_kcal ?? undefined, inbody_score: entry.inbody_score ?? undefined,
      phase_angle_degrees: entry.phase_angle_degrees ?? undefined,
      segments: bodySegments.map(({ segment }) => {
        const saved = entry.segments?.find((candidate) => candidate.segment === segment);
        return { segment, lean_mass_kg: saved?.lean_mass_kg ?? undefined, lean_percent: saved?.lean_percent ?? undefined, fat_mass_kg: saved?.fat_mass_kg ?? undefined, fat_percent: saved?.fat_percent ?? undefined };
      }),
      source: entry.source ?? "inbody", notes: entry.notes ?? "",
    } : { measured_at: localDateTime(timeZone), source: "inbody", notes: "", segments: emptyBodySegments() });
  }, [entry, form, open, timeZone]);
  const save = useSave<BodyValues>("/body-measurements", () => { form.reset(); onOpenChange(false); }, entry?.id, { external_id: entry?.external_id ?? null });
  type NumericField = Exclude<keyof BodyValues, "measured_at" | "segments" | "source" | "notes">;
  const number = (name: NumericField, label: string, step = "0.1", max?: string) => <Field label={label} error={form.formState.errors[name]?.message}><Input type="number" min="0" max={max} step={step} {...form.register(name, numeric)} /></Field>;
  const submit = (values: BodyValues) => save.mutate({
    ...values,
    segments: values.segments.filter((segment) => [segment.lean_mass_kg, segment.lean_percent, segment.fat_mass_kg, segment.fat_percent].some((value) => typeof value === "number")),
  });
  return <Dialog open={open} onOpenChange={onOpenChange} title={entry ? "Редактировать InBody" : "Измерение InBody"} description="Перенесите значения с листа Spirit Fitness / InBody. FitLog анализирует динамику, но не ставит диагнозы." className="sm:max-w-4xl">
    <form onSubmit={form.handleSubmit(submit)} className="space-y-5">
      <section className="space-y-3"><p className="text-xs font-semibold uppercase tracking-[.14em] text-muted">Основной состав</p><div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <Field label="Дата и время" error={form.formState.errors.measured_at?.message}><Input type="datetime-local" {...form.register("measured_at")} /></Field>
        <Field label="Источник"><Select {...form.register("source")}><option value="inbody">InBody</option><option value="manual">Вручную</option><option value="csv">CSV</option><option value="json">JSON</option></Select></Field>
        {number("weight_kg", "Вес, кг")}{number("body_fat_percent", "Жир, %", "0.1", "100")}{number("fat_mass_kg", "Жировая масса, кг")}
        {number("lean_mass_kg", "Безжировая масса, кг")}{number("skeletal_muscle_mass_kg", "Скелетные мышцы, кг")}{number("bmi", "BMI")}
        {number("protein_mass_kg", "Белок, кг")}{number("mineral_mass_kg", "Минералы, кг")}{number("basal_metabolic_rate_kcal", "Базовый обмен, ккал", "1")}
        {number("inbody_score", "InBody Score", "1", "200")}{number("visceral_fat_level", "Уровень висцерального жира", "1")}{number("visceral_fat_area_cm2", "Площадь висцерального жира, см²")}
        {number("phase_angle_degrees", "Фазовый угол, °", "0.01")}
      </div></section>
      <section className="space-y-3 rounded-control border border-line bg-canvas/25 p-4"><div><p className="text-sm font-semibold text-ink">Водный баланс</p><p className="mt-1 text-xs text-muted">TBW, ICW, ECW и ECW/TBW с листа результатов. Пустые поля остаются missing.</p></div><div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {number("total_body_water_l", "Общая вода, л")}{number("intracellular_water_l", "Внутриклеточная, л")}{number("extracellular_water_l", "Внеклеточная, л")}{number("ecw_tbw_ratio", "ECW/TBW", "0.001", "1")}
      </div></section>
      <section className="space-y-3"><div><p className="text-sm font-semibold text-ink">Сегментарный анализ</p><p className="mt-1 text-xs text-muted">Масса и процент от референса для рук, ног и корпуса — нужны для оценки лево-правой асимметрии.</p></div><div className="grid gap-3 md:grid-cols-2">
        {bodySegments.map(({ segment, label }, index) => <div key={segment} className="rounded-control border border-line bg-canvas/25 p-3"><input type="hidden" value={segment} {...form.register(`segments.${index}.segment`)} /><p className="mb-3 text-xs font-semibold text-ink">{label}</p><div className="grid grid-cols-2 gap-3">
          <Field label="Lean, кг" error={form.formState.errors.segments?.[index]?.lean_mass_kg?.message}><Input type="number" min="0" step="0.01" {...form.register(`segments.${index}.lean_mass_kg`, numeric)} /></Field>
          <Field label="Lean, %" error={form.formState.errors.segments?.[index]?.lean_percent?.message}><Input type="number" min="0" step="0.1" {...form.register(`segments.${index}.lean_percent`, numeric)} /></Field>
          <Field label="Fat, кг" error={form.formState.errors.segments?.[index]?.fat_mass_kg?.message}><Input type="number" min="0" step="0.01" {...form.register(`segments.${index}.fat_mass_kg`, numeric)} /></Field>
          <Field label="Fat, %" error={form.formState.errors.segments?.[index]?.fat_percent?.message}><Input type="number" min="0" step="0.1" {...form.register(`segments.${index}.fat_percent`, numeric)} /></Field>
        </div></div>)}
      </div></section>
      <section className="space-y-3"><p className="text-xs font-semibold uppercase tracking-[.14em] text-muted">Окружности и комментарий</p><div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">{number("waist_cm", "Талия, см")}{number("chest_cm", "Грудь, см")}{number("biceps_cm", "Бицепс, см")}{number("thigh_cm", "Бедро, см")}</div><Field label="Комментарий"><Textarea {...form.register("notes")} /></Field></section>
      <InlineError error={save.error} /><DialogActions><Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>Отмена</Button><Button type="submit" variant="primary" loading={save.isPending}>Сохранить</Button></DialogActions>
    </form>
  </Dialog>;
}
