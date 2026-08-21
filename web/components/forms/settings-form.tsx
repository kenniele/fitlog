"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Field, Input, Select, Textarea } from "@/components/ui/field";
import { InlineError } from "@/components/ui/states";
import { apiFetch } from "@/lib/api";
import type { Settings } from "@/lib/types";

type Values = { timezone: string; theme: "dark" | "light" | "system"; units: "metric"; first_day_of_week: number; calorie_target_kcal?: number; protein_target_g?: number; fat_target_g?: number; carbohydrates_target_g?: number; sleep_target_min_seconds?: number; sleep_target_max_seconds?: number; recovery_ranges_json: string };
const optional = z.number().finite().positive().optional();
function validRecoveryRanges(raw: string) {
  try {
    const value: unknown = JSON.parse(raw);
    if (typeof value !== "object" || value === null || Array.isArray(value)) return false;
    const ranges = value as Record<string, unknown>;
    const low = ranges.low == null ? 34 : ranges.low;
    const high = ranges.high == null ? 67 : ranges.high;
    return typeof low === "number" && Number.isFinite(low) && typeof high === "number" && Number.isFinite(high) && low > 0 && low < high && high <= 100;
  } catch { return false; }
}
const schema = z.object({ timezone: z.string().min(1), theme: z.enum(["dark", "light", "system"]), units: z.literal("metric"), first_day_of_week: z.number().int().min(1).max(7), calorie_target_kcal: optional, protein_target_g: optional, fat_target_g: optional, carbohydrates_target_g: optional, sleep_target_min_seconds: optional, sleep_target_max_seconds: optional, recovery_ranges_json: z.string().refine(validRecoveryRanges, "Используйте JSON {\"low\": 34, \"high\": 67}, где 0 < low < high ≤ 100") }).refine((value) => !value.sleep_target_min_seconds || !value.sleep_target_max_seconds || value.sleep_target_min_seconds <= value.sleep_target_max_seconds, { message: "Минимум не может превышать максимум", path: ["sleep_target_max_seconds"] });
const numeric = { setValueAs: (value: string) => value === "" ? undefined : Number(value) };

export function SettingsForm({ settings }: { settings: Settings }) {
  const client = useQueryClient();
  const browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { timezone: browserTimezone, theme: "dark", units: "metric", first_day_of_week: 1, recovery_ranges_json: "{}" } });
  useEffect(() => { form.reset({ timezone: settings.timezone ?? browserTimezone, theme: settings.theme === "light" || settings.theme === "system" ? settings.theme : "dark", units: "metric", first_day_of_week: settings.first_day_of_week ?? 1, calorie_target_kcal: settings.calorie_target_kcal ?? undefined, protein_target_g: settings.protein_target_g ?? undefined, fat_target_g: settings.fat_target_g ?? undefined, carbohydrates_target_g: settings.carbohydrates_target_g ?? undefined, sleep_target_min_seconds: settings.sleep_target_min_seconds ?? undefined, sleep_target_max_seconds: settings.sleep_target_max_seconds ?? undefined, recovery_ranges_json: JSON.stringify(settings.recovery_ranges ?? {}, null, 2) }); }, [browserTimezone, form, settings]);
  const save = useMutation({ mutationFn: ({ recovery_ranges_json, ...values }: Values) => apiFetch<Settings>("/settings", { method: "PUT", body: { ...values, recovery_ranges: JSON.parse(recovery_ranges_json) as Record<string, unknown> } }), onSuccess: (data) => client.setQueryData(["settings"], data) });
  return <form onSubmit={form.handleSubmit((values) => save.mutate(values))} className="space-y-6"><input type="hidden" {...form.register("units")} /><div className="grid gap-4 sm:grid-cols-2"><Field label="Timezone" error={form.formState.errors.timezone?.message}><Input {...form.register("timezone")} placeholder={browserTimezone} /></Field><Field label="Тема"><Select {...form.register("theme")}><option value="dark">Тёмная</option><option value="light">Светлая</option><option value="system">Системная</option></Select></Field><Field label="Первый день недели"><Select {...form.register("first_day_of_week", { valueAsNumber: true })}><option value="1">Понедельник</option><option value="2">Вторник</option><option value="3">Среда</option><option value="4">Четверг</option><option value="5">Пятница</option><option value="6">Суббота</option><option value="7">Воскресенье</option></Select></Field><Field label="Единицы"><Select value="metric" disabled aria-readonly><option value="metric">Метрические</option></Select></Field></div><div><h3 className="mb-3 text-sm font-semibold">Цели питания</h3><div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4"><Field label="Калории, ккал" error={form.formState.errors.calorie_target_kcal?.message}><Input type="number" min="1" {...form.register("calorie_target_kcal", numeric)} /></Field><Field label="Белки, г" error={form.formState.errors.protein_target_g?.message}><Input type="number" min="1" {...form.register("protein_target_g", numeric)} /></Field><Field label="Жиры, г" error={form.formState.errors.fat_target_g?.message}><Input type="number" min="1" {...form.register("fat_target_g", numeric)} /></Field><Field label="Углеводы, г" error={form.formState.errors.carbohydrates_target_g?.message}><Input type="number" min="1" {...form.register("carbohydrates_target_g", numeric)} /></Field></div></div><div><h3 className="mb-3 text-sm font-semibold">Целевой сон</h3><div className="grid gap-4 sm:grid-cols-2"><Field label="Минимум, секунд" error={form.formState.errors.sleep_target_min_seconds?.message}><Input type="number" min="60" step="60" {...form.register("sleep_target_min_seconds", numeric)} /></Field><Field label="Максимум, секунд" error={form.formState.errors.sleep_target_max_seconds?.message}><Input type="number" min="60" step="60" {...form.register("sleep_target_max_seconds", numeric)} /></Field></div></div><Field label="Recovery ranges, JSON" hint={'Пороги зон: например {"low": 34, "high": 67}.'} error={form.formState.errors.recovery_ranges_json?.message}><Textarea className="font-mono text-xs" spellCheck={false} {...form.register("recovery_ranges_json")} /></Field><InlineError error={save.error} />{save.isSuccess && <p role="status" className="text-sm text-accent">Настройки сохранены.</p>}<Button type="submit" variant="primary" loading={save.isPending}>Сохранить настройки</Button></form>;
}
