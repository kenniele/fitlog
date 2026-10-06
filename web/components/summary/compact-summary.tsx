"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { ArrowUpRight, RefreshCw } from "lucide-react";
import { apiFetch, APIError } from "@/lib/api";
import type { CompactSnapshot } from "@/lib/compact";
import {
  formatDate,
  formatDuration,
  formatNumber,
  shiftISODate,
} from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ErrorState, PageSkeleton } from "@/components/ui/states";
import { ScriptableSetup } from "@/components/summary/scriptable-setup";

type Value = number | null | undefined;
const numeric = (value: Value, unit = "", digits = 1) =>
  value == null || !Number.isFinite(value)
    ? "—"
    : formatNumber(value, { maximumFractionDigits: digits }, unit);
const duration = (value: Value) =>
  value == null ? "—" : formatDuration(value);

function MetricGroup({
  title,
  date,
  metrics,
}: {
  title: string;
  date: string;
  metrics: [label: string, value: string][];
}) {
  return (
    <Card className="min-w-0 p-5">
      <div className="mb-4 flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
        <h2 className="text-sm font-medium">{title}</h2>
        <p className="text-xs text-muted">{date}</p>
      </div>
      <dl className="grid grid-cols-2 gap-x-4 gap-y-4">
        {metrics.map(([label, value], index) => (
          <div
            key={label}
            className={
              index === 0 ? "col-span-2 border-b border-line pb-4" : "min-w-0"
            }
          >
            <dt className="text-xs text-muted">{label}</dt>
            <dd
              className={
                index === 0
                  ? "mt-1 text-3xl font-semibold tabular-nums tracking-tight text-accent"
                  : "mt-1 text-lg font-medium tabular-nums tracking-tight"
              }
            >
              {value}
            </dd>
          </div>
        ))}
      </dl>
    </Card>
  );
}

export function SummaryNumbers({ data }: { data: CompactSnapshot }) {
  const day = (date?: string) =>
    date ? formatDate(date) : "Нет записей за 30 дней";
  const { recovery: r, sleep: s, nutrition: n, body: b, training: t } = data;
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      <MetricGroup
        title="Восстановление"
        date={day(r?.date)}
        metrics={[
          ["Восстановление", numeric(r?.recovery_score, "%", 0)],
          ["HRV", numeric(r?.hrv_ms, " мс", 0)],
          ["Пульс в покое", numeric(r?.resting_heart_rate_bpm, " уд/мин", 0)],
          ["Нагрузка", numeric(r?.daily_strain)],
          ["Дыхание", numeric(r?.respiratory_rate, " /мин")],
          ["SpO₂", numeric(r?.spo2_percent, "%")],
          ["Температура кожи", numeric(r?.skin_temperature_c, " °C")],
        ]}
      />
      <MetricGroup
        title="Сон"
        date={day(s?.date)}
        metrics={[
          ["Продолжительность", duration(s?.sleep_seconds)],
          ["Выполнение нормы", numeric(s?.sleep_performance_percent, "%", 0)],
          ["Эффективность", numeric(s?.sleep_efficiency_percent, "%", 0)],
          ["REM", duration(s?.rem_seconds)],
          ["Глубокий сон", duration(s?.deep_seconds)],
        ]}
      />
      <MetricGroup
        title="Тренировки · 7 дней"
        date={`${formatDate(shiftISODate(data.date, -6))} — ${formatDate(data.date)}`}
        metrics={[
          ["Завершённые тренировки", numeric(t.sessions, "", 0)],
          ["Рабочие подходы", numeric(t.working_sets, "", 0)],
          ["Тоннаж", numeric(t.volume_kg, " кг", 0)],
          ["Повторения", numeric(t.repetitions, "", 0)],
          ["Средний RIR", numeric(t.average_rir)],
        ]}
      />
      <MetricGroup
        title="Питание"
        date={day(n?.date)}
        metrics={[
          ["Калории", numeric(n?.calories_kcal, " ккал", 0)],
          ["Белки", numeric(n?.protein_g, " г")],
          ["Жиры", numeric(n?.fat_g, " г")],
          ["Углеводы", numeric(n?.carbohydrates_g, " г")],
        ]}
      />
      <MetricGroup
        title="Состав тела"
        date={
          b
            ? formatDate(b.measured_at, "dd.MM.yyyy", data.timezone)
            : "Нет измерений"
        }
        metrics={[
          ["Вес", numeric(b?.weight_kg, " кг")],
          ["Жир", numeric(b?.body_fat_percent, "%")],
          ["Скелетные мышцы", numeric(b?.skeletal_muscle_mass_kg, " кг")],
        ]}
      />
      <Card className="min-w-0 p-5">
        <h2 className="text-sm font-medium">Ближайшая тренировка</h2>
        {data.next_workout ? (
          <>
            <p className="mt-4 break-words text-xl font-semibold tracking-tight">
              {data.next_workout.title}
            </p>
            <p className="mt-2 text-sm text-muted">
              {formatDate(
                data.next_workout.scheduled_at,
                "dd.MM.yyyy HH:mm",
                data.timezone,
              )}
            </p>
          </>
        ) : (
          <p className="mt-4 text-sm text-muted">Пока не запланирована</p>
        )}
        <Link
          href="/dashboard/training"
          className="mt-5 inline-flex min-h-11 items-center gap-1 text-sm text-accent"
        >
          Открыть тренировки <ArrowUpRight aria-hidden className="size-4" />
        </Link>
      </Card>
    </div>
  );
}

export function CompactSummary() {
  const query = useQuery({
    queryKey: ["compact-summary"],
    queryFn: () => apiFetch<CompactSnapshot>("/dashboard/summary"),
    refetchInterval: 60_000,
    refetchOnWindowFocus: "always",
  });
  const unauthorized =
    query.error instanceof APIError && query.error.status === 401;
  return (
    <main className="mx-auto min-h-screen max-w-3xl px-4 pb-8 pt-6 sm:px-6">
      <header className="mb-6 flex items-start justify-between gap-3">
        <div>
          <p className="text-xs font-medium tracking-[.16em] text-accent">
            FITLOG
          </p>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight">Сводка</h1>
          <p className="mt-1 text-xs text-muted">
            {query.data && !unauthorized
              ? `Обновлено ${formatDate(query.data.generated_at, "dd.MM.yyyy HH:mm", query.data.timezone)}`
              : "Восстановление, сон и тренировки"}
          </p>
        </div>
        <div className="flex items-center gap-1">
          <Link
            href="/dashboard"
            className="inline-flex min-h-11 items-center px-2 text-sm text-muted hover:text-ink"
          >
            Обзор <ArrowUpRight aria-hidden className="ml-1 size-4" />
          </Link>
          <Button
            size="icon"
            variant="ghost"
            aria-label="Обновить сводку"
            title="Обновить"
            loading={query.isFetching}
            onClick={() => void query.refetch()}
          >
            <RefreshCw aria-hidden className="size-4" />
          </Button>
        </div>
      </header>
      {query.isPending ? (
        <PageSkeleton />
      ) : unauthorized ? (
        <Card className="p-5">
          <p>Войдите, чтобы посмотреть сводку.</p>
          <Link
            className="mt-4 inline-flex min-h-11 items-center text-accent"
            href="/dashboard/login?next=%2Fdashboard%2Fsummary"
          >
            Войти в FitLog
          </Link>
        </Card>
      ) : (
        <>
          {query.isError && (
            <div className="mb-4">
              <ErrorState
                error={query.error}
                retry={() => void query.refetch()}
                title={
                  query.data
                    ? "Не удалось обновить. Показаны предыдущие данные."
                    : "Не удалось загрузить сводку"
                }
              />
            </div>
          )}
          {query.data && <SummaryNumbers data={query.data} />}
        </>
      )}
      {!unauthorized && <ScriptableSetup />}
      <details className="mt-5 rounded-control border border-line px-4 py-1 text-sm">
        <summary className="cursor-pointer py-3 font-medium">
          Добавить иконку FitLog
        </summary>
        <div className="space-y-3 pb-4 text-muted">
          <p>
            <span className="font-medium text-ink">iPhone:</span> откройте эту
            страницу в Safari → «Поделиться» → «На экран Домой». Если есть
            переключатель «Открывать как веб-приложение», включите его.
          </p>
          <p>
            <span className="font-medium text-ink">Mac:</span> откройте эту
            страницу в Safari → «Файл» → «Добавить в Dock» (macOS Sonoma или
            новее).
          </p>
          <p>
            Иконка открывает эту сводку в отдельном окне. Для обновления цифр
            нужен интернет. При первом запуске может потребоваться вход.
          </p>
        </div>
      </details>
      <p className="mt-4 text-xs leading-5 text-muted">
        — означает отсутствие данных. Даты указаны в часовом поясе профиля.
      </p>
    </main>
  );
}
