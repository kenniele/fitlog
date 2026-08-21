"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CalendarCheck2, CalendarDays, Clock3 } from "lucide-react";
import { Dialog, DialogActions } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/field";
import { InlineError } from "@/components/ui/states";
import { Badge } from "@/components/ui/badge";
import { apiFetch } from "@/lib/api";
import { dateInTimeZone, getDashboardTimezone, shiftISODate } from "@/lib/format";
import { nextWeekStartISO, parseFirstDayOfWeek, type FirstDayOfWeek } from "@/lib/week";
import type { Settings, WorkoutPlan, WorkoutSession, WorkoutTemplate } from "@/lib/types";

export type WeeklyScheduleRow = {
  templateID: string | number;
  templateName: string;
  position: number;
  exerciseCount: number;
  date: string;
  time: string;
};

const preferredOffsets = [0, 2, 4, 1, 3, 5, 6];

function activeTemplates(plan: WorkoutPlan | null | undefined): WorkoutTemplate[] {
  return (plan?.templates ?? [])
    .filter((template) => template.id != null)
    .slice()
    .sort((left, right) => (left.position ?? 0) - (right.position ?? 0));
}

export function weeklyScheduleDefaults(plan: WorkoutPlan | null | undefined, today: string, firstDayOfWeek: FirstDayOfWeek): WeeklyScheduleRow[] {
  const weekStart = nextWeekStartISO(today, firstDayOfWeek);
  return activeTemplates(plan).map((template, index) => ({
    templateID: template.id as string | number,
    templateName: template.name,
    position: template.position ?? index + 1,
    exerciseCount: template.exercises?.length ?? 0,
    date: shiftISODate(weekStart, preferredOffsets[index] ?? index),
    time: "18:30",
  }));
}

export function scheduledSessionPayload(plan: WorkoutPlan, row: WeeklyScheduleRow) {
  return {
    date: row.date,
    status: "scheduled",
    scheduled_at: `${row.date}T${row.time}`,
    plan_id: Number(plan.id),
    template_id: Number(row.templateID),
    source: "manual",
    external_id: `web-schedule:${plan.id}:${row.templateID}:${row.date}T${row.time}`,
    notes: "",
  };
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : "Не удалось создать расписание";
}

function plural(count: number, one: string, few: string, many: string) {
  const mod100 = count % 100;
  const mod10 = count % 10;
  if (mod100 >= 11 && mod100 <= 14) return many;
  if (mod10 === 1) return one;
  if (mod10 >= 2 && mod10 <= 4) return few;
  return many;
}

export function WeeklyScheduleDialog({ plan, open, onOpenChange }: { plan?: WorkoutPlan | null; open: boolean; onOpenChange: (open: boolean) => void }) {
  const client = useQueryClient();
  const timeZone = getDashboardTimezone();
  const [rows, setRows] = useState<WeeklyScheduleRow[]>([]);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [previouslyCreated, setPreviouslyCreated] = useState(0);
  const [createdDates, setCreatedDates] = useState<string[]>([]);
  const today = dateInTimeZone(new Date(), timeZone);
  const settings = useQuery({ queryKey: ["settings"], queryFn: () => apiFetch<Settings>("/settings"), enabled: open });
  const firstDayOfWeek = parseFirstDayOfWeek(settings.data?.first_day_of_week);
  const settingsError = settings.isError
    ? settings.error
    : settings.isSuccess && firstDayOfWeek === null
      ? new Error("В настройках указан некорректный первый день недели")
      : null;

  const save = useMutation({
    mutationFn: async (schedule: WeeklyScheduleRow[]) => {
      if (!plan) throw new Error("План не выбран");
      let created = 0;
      try {
        for (const row of schedule) {
          await apiFetch<WorkoutSession>("/workout-sessions", { method: "POST", body: scheduledSessionPayload(plan, row) });
          created++;
        }
      } catch (error) {
        await client.invalidateQueries();
        if (created > 0) {
          setPreviouslyCreated((current) => current + created);
          setCreatedDates((current) => [...current, ...schedule.slice(0, created).map((row) => row.date)]);
          setRows(schedule.slice(created));
          throw new Error(`Создано ${created} из ${schedule.length}. Оставшиеся строки можно отправить повторно. ${errorMessage(error)}`);
        }
        throw error;
      }
      return created;
    },
    onSuccess: async () => { await client.invalidateQueries(); },
  });

  useEffect(() => {
    if (!open) return;
    setRows(firstDayOfWeek === null ? [] : weeklyScheduleDefaults(plan, today, firstDayOfWeek));
    setValidationError(null);
    setPreviouslyCreated(0);
    setCreatedDates([]);
    save.reset();
  }, [firstDayOfWeek, open, plan, today]); // eslint-disable-line react-hooks/exhaustive-deps

  const calendarHref = useMemo(() => {
    if (!rows.length) return "/dashboard/training?view=calendar";
    const dates = [...createdDates, ...rows.map((row) => row.date)].sort();
    const params = new URLSearchParams({ view: "calendar", range: "custom", from: dates[0], to: dates[dates.length - 1] });
    return `/dashboard/training?${params.toString()}`;
  }, [createdDates, rows]);

  const updateRow = (index: number, patch: Partial<WeeklyScheduleRow>) => {
    setRows((current) => current.map((row, rowIndex) => rowIndex === index ? { ...row, ...patch } : row));
    setValidationError(null);
    if (save.isError || save.isSuccess) save.reset();
  };

  const submit = () => {
    if (firstDayOfWeek === null) {
      setValidationError("Сначала загрузите корректную настройку первого дня недели");
      return;
    }
    if (!rows.length) {
      setValidationError("В активной ревизии нет шаблонов для расписания");
      return;
    }
    if (rows.some((row) => !/^\d{4}-\d{2}-\d{2}$/.test(row.date) || !/^\d{2}:\d{2}$/.test(row.time))) {
      setValidationError("Укажите дату и время для каждого шаблона");
      return;
    }
    save.mutate(rows);
  };

  const handleOpenChange = (value: boolean) => {
    if (!value) {
      setValidationError(null);
      save.reset();
    }
    onOpenChange(value);
  };

  return <Dialog open={open} onOpenChange={handleOpenChange} title={plan ? `Расписание: ${plan.name}` : "Недельное расписание"} description="Каждый активный шаблон станет отдельной scheduled session с независимым snapshot prescription." className="sm:max-w-3xl">
    {save.isSuccess ? <div className="rounded-card border border-accent/20 bg-accent/[.055] p-5 text-center"><span className="mx-auto flex size-11 items-center justify-center rounded-full border border-accent/25 bg-accent/10"><CalendarCheck2 aria-hidden className="size-5 text-accent" /></span><h3 className="mt-3 text-base font-semibold text-ink">Неделя добавлена в расписание</h3><p className="mt-1 text-sm text-muted">Создано: {previouslyCreated + save.data} {plural(previouslyCreated + save.data, "сессия", "сессии", "сессий")}. Шаблоны и плановые подходы уже зафиксированы в snapshots.</p><div className="mt-5 flex flex-col-reverse justify-center gap-2 sm:flex-row"><Button type="button" variant="ghost" onClick={() => handleOpenChange(false)}>Закрыть</Button><Link href={calendarHref} onClick={() => handleOpenChange(false)} className="inline-flex h-10 items-center justify-center gap-2 rounded-control border border-accent/30 bg-accent px-4 text-sm font-medium text-[#07120c] transition hover:bg-[#98ffc3]"><CalendarDays aria-hidden className="size-4" />Открыть календарь</Link></div></div> : <>
      <div className="mb-4 flex items-center justify-between gap-3 rounded-control border border-line bg-canvas/40 px-3 py-2"><span className="text-xs text-muted">Часовой пояс: {timeZone}</span><Badge tone="blue">{rows.length} {plural(rows.length, "шаблон", "шаблона", "шаблонов")}</Badge></div>
      <div className="space-y-2">
        {settings.isPending ? <div aria-busy="true" className="rounded-card border border-dashed border-line px-5 py-10 text-center text-sm text-muted">Загружаем настройку начала недели…</div> : null}
        {rows.map((row, index) => <div key={String(row.templateID)} className="grid gap-3 rounded-card border border-line bg-canvas/35 p-3 sm:grid-cols-[minmax(150px,1fr)_180px_130px] sm:items-end">
          <div className="min-w-0 self-center"><div className="flex items-center gap-2"><span className="flex size-7 shrink-0 items-center justify-center rounded-full border border-line bg-white/[.035] text-xs font-semibold text-muted">{row.position}</span><span className="truncate text-sm font-semibold text-ink">{row.templateName}</span></div><p className="ml-9 mt-1 text-xs text-muted">{row.exerciseCount} упражнений</p></div>
          <Field label={`Дата · ${row.templateName}`}><Input type="date" value={row.date} disabled={save.isPending} onChange={(event) => updateRow(index, { date: event.target.value })} /></Field>
          <Field label={`Время · ${row.templateName}`}><div className="relative"><Clock3 aria-hidden className="pointer-events-none absolute left-3 top-3 size-4 text-muted" /><Input type="time" value={row.time} disabled={save.isPending} className="pl-9" onChange={(event) => updateRow(index, { time: event.target.value })} /></div></Field>
        </div>)}
        {!settings.isPending && !settingsError && !rows.length ? <div className="rounded-card border border-dashed border-line px-5 py-10 text-center text-sm text-muted">У плана нет активных шаблонов.</div> : null}
      </div>
      {validationError ? <p role="alert" className="mt-4 rounded-control border border-critical/20 bg-critical/10 px-3 py-2 text-sm text-critical">{validationError}</p> : null}
      <InlineError error={settingsError} />
      <InlineError error={save.error} />
      <DialogActions><Button type="button" variant="ghost" onClick={() => handleOpenChange(false)}>Отмена</Button><Button type="button" variant="primary" loading={save.isPending} disabled={!rows.length || settings.isPending || Boolean(settingsError)} onClick={submit}><CalendarCheck2 aria-hidden className="size-4" />Добавить {rows.length} {plural(rows.length, "тренировку", "тренировки", "тренировок")}</Button></DialogActions>
    </>}
  </Dialog>;
}
