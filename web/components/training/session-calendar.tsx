import Link from "next/link";
import { CalendarDays, Clock3, Dumbbell } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { formatDate, formatNumber, toDateTimeLocal } from "@/lib/format";
import { buildWeekCalendarDates, weekDayLabels, type FirstDayOfWeek } from "@/lib/week";
import type { WorkoutSession } from "@/lib/types";

const statusPresentation: Record<string, { label: string; tone: "neutral" | "good" | "warning" | "critical" | "blue" }> = {
  scheduled: { label: "запланирована", tone: "blue" },
  active: { label: "активна", tone: "warning" },
  finished: { label: "завершена", tone: "good" },
  cancelled: { label: "отменена", tone: "critical" },
  excused: { label: "уважительный пропуск", tone: "neutral" },
};

export function calendarSessionQuery(filters: string) {
  const params = new URLSearchParams(filters);
  params.set("date_basis", "calendar");
  return params.toString();
}

function calendarDate(session: WorkoutSession) {
  return session.calendar_date ?? session.scheduled_date ?? session.actual_date ?? session.date ?? "";
}

function sessionTitle(session: WorkoutSession) {
  return session.template_name ?? session.plan_name ?? session.program_name ?? "Свободная тренировка";
}

function sessionTime(session: WorkoutSession) {
  const value = session.scheduled_at ?? session.started_at;
  return value ? toDateTimeLocal(value).slice(11, 16) || "—" : "—";
}

function sessionCountLabel(count: number) {
  const mod100 = count % 100;
  const mod10 = count % 10;
  if (mod100 >= 11 && mod100 <= 14) return "сессий";
  if (mod10 === 1) return "сессия";
  if (mod10 >= 2 && mod10 <= 4) return "сессии";
  return "сессий";
}

export function SessionCalendar({ from, to, sessions, firstDayOfWeek }: { from: string; to: string; sessions?: WorkoutSession[] | null; firstDayOfWeek: FirstDayOfWeek }) {
  const dates = buildWeekCalendarDates(from, to, firstDayOfWeek);
  const grouped = new Map<string, WorkoutSession[]>();
  for (const session of Array.isArray(sessions) ? sessions : []) {
    const date = calendarDate(session);
    if (!date) continue;
    grouped.set(date, [...(grouped.get(date) ?? []), session]);
  }

  return <Card className="min-w-0 overflow-hidden">
    <div className="flex flex-col gap-2 border-b border-line p-4 sm:flex-row sm:items-center sm:justify-between">
      <div><h2 className="text-sm font-semibold text-ink">Календарь сессий</h2><p className="mt-1 text-xs text-muted">Плановые сессии стоят на scheduled date, остальные — на фактической дате старта.</p></div>
      <Badge tone="blue">{formatNumber(Array.isArray(sessions) ? sessions.length : 0)} {sessionCountLabel(Array.isArray(sessions) ? sessions.length : 0)}</Badge>
    </div>
    <div className="scrollbar-thin overflow-x-auto">
      <div className="min-w-[980px]">
        <div className="grid grid-cols-7 border-b border-line bg-white/[.018]">{weekDayLabels(firstDayOfWeek).map((day) => <div key={day} className="border-r border-line px-3 py-2 text-center text-[11px] font-medium uppercase tracking-[.12em] text-muted last:border-r-0">{day}</div>)}</div>
        <div className="grid grid-cols-7">
          {dates.map((date, index) => {
            if (!date) return <div key={`padding-${index}`} aria-hidden className="min-h-36 border-b border-r border-line bg-canvas/20" />;
            const daySessions = grouped.get(date) ?? [];
            return <div key={date} className="min-h-36 border-b border-r border-line bg-surface p-2.5 transition hover:bg-white/[.018]">
              <div className="mb-2 flex items-center justify-between gap-2"><span className="text-xs font-semibold text-ink">{formatDate(date)}</span>{daySessions.length > 0 ? <span className="text-[10px] tabular-nums text-muted">{daySessions.length}</span> : null}</div>
              <div className="space-y-1.5">
                {daySessions.map((session) => {
                  const presentation = statusPresentation[session.status ?? ""] ?? { label: session.status ?? "без статуса", tone: "neutral" as const };
                  return <Link key={session.id} href={`/dashboard/training/sessions/${session.id}`} className="block rounded-control border border-line bg-canvas/55 p-2.5 transition hover:border-accent/30 hover:bg-accent/[.035] focus-visible:ring-2 focus-visible:ring-accent/70 focus-visible:ring-offset-2 focus-visible:ring-offset-surface">
                    <span className="block truncate text-xs font-semibold text-ink" title={sessionTitle(session)}>{sessionTitle(session)}</span>
                    <span className="mt-1.5 flex items-center justify-between gap-2"><Badge tone={presentation.tone}>{presentation.label}</Badge><span className="flex items-center gap-1 text-[10px] text-muted"><Clock3 aria-hidden className="size-3" />{sessionTime(session)}</span></span>
                    {(session.working_sets != null || session.volume_kg != null) && <span className="mt-1.5 flex items-center gap-1 text-[10px] text-muted"><Dumbbell aria-hidden className="size-3" />{formatNumber(session.working_sets)} подх. · {formatNumber(session.volume_kg, {}, " кг")}</span>}
                  </Link>;
                })}
                {!daySessions.length ? <span className="flex items-center gap-1.5 py-2 text-[11px] text-muted/60"><CalendarDays aria-hidden className="size-3" />Нет сессий</span> : null}
              </div>
            </div>;
          })}
        </div>
      </div>
    </div>
  </Card>;
}
