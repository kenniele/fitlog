"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { apiFetch } from "@/lib/api";
import type { AnalyticsResponse, Exercise, SeriesPoint } from "@/lib/types";
import {
  dateInTimeZone,
  formatDate,
  formatNumber,
  shiftISODate,
} from "@/lib/format";
import { Card } from "@/components/ui/card";
import { Select } from "@/components/ui/field";
import { Button } from "@/components/ui/button";
import { EmptyState, InlineError, Skeleton } from "@/components/ui/states";

export function progressSummary(points: SeriesPoint[]) {
  const measured = points
    .filter(
      (p) =>
        typeof p.estimated_1rm === "number" &&
        Number.isFinite(p.estimated_1rm) &&
        p.estimated_1rm > 0,
    )
    .sort((a, b) => a.date.localeCompare(b.date));
  const first = measured[0]?.estimated_1rm as number | undefined;
  const last = measured.at(-1)?.estimated_1rm as number | undefined;
  return {
    measured,
    first,
    last,
    change:
      measured.length > 1 && first && last ? (last / first - 1) * 100 : null,
  };
}

export function ExerciseProgress({ exercises }: { exercises: Exercise[] }) {
  const [exercise, setExercise] = useState("");
  const [weeks, setWeeks] = useState(12);
  const today = dateInTimeZone(new Date());
  const params = new URLSearchParams({
    from: shiftISODate(today, -(weeks * 7 - 1)),
    to: today,
    exercise_id: exercise,
    status: "finished",
  });
  const query = useQuery({
    queryKey: ["exercise-progress", params.toString()],
    queryFn: () => apiFetch<AnalyticsResponse>(`/analytics/training?${params}`),
    enabled: exercise !== "",
  });
  const daily = query.data?.daily ?? [];
  const summary = progressSummary(daily);
  const chart = summary.measured.map((point) => ({
    ...point,
    time: new Date(`${point.date}T12:00:00Z`).getTime(),
  }));
  return (
    <Card className="min-w-0 overflow-hidden p-5 sm:p-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p className="eyebrow">Силовой прогресс</p>
          <h2 className="mt-2 text-lg font-medium">
            {exercises.find((item) => String(item.id) === exercise)?.name ??
              "Прогресс упражнения"}
          </h2>
          <p className="mt-2 text-xs text-muted">
            Расчётный 1ПМ по завершённым рабочим подходам на 1–12 повторов.
          </p>
        </div>
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <Select
            className="max-w-full sm:w-64"
            aria-label="Упражнение для прогресса"
            value={exercise}
            onChange={(event) => setExercise(event.target.value)}
          >
            <option value="">Выберите упражнение</option>
            {exercises.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </Select>
          <div className="flex gap-1" aria-label="Период прогресса">
            {[4, 12].map((period) => (
              <Button
                key={period}
                variant={weeks === period ? "secondary" : "ghost"}
                aria-pressed={weeks === period}
                onClick={() => setWeeks(period)}
              >
                {period} нед.
              </Button>
            ))}
          </div>
        </div>
      </div>
      {exercise === "" ? (
        <EmptyState
          title="Выберите упражнение"
          description="Сравнение силы ведётся внутри одного упражнения."
        />
      ) : query.isPending ? (
        <Skeleton className="mt-6 h-52" />
      ) : query.isError ? (
        <InlineError error={query.error} />
      ) : summary.measured.length === 0 ? (
        <EmptyState
          title="Пока нет точек прогресса"
          description="За этот период нет завершённых подходов с внешним весом и 1–12 повторами."
        />
      ) : (
        <>
          <div
            className="mt-6 flex flex-wrap items-baseline gap-x-5 gap-y-2"
            aria-live="polite"
          >
            <span className="text-3xl font-medium tabular-nums">
              {formatNumber(summary.last, { maximumFractionDigits: 1 })}
              <span className="ml-2 text-sm text-muted">кг</span>
            </span>
            {summary.change !== null && (
              <span className="text-sm tabular-nums text-muted">
                {summary.change > 0 ? "+" : ""}
                {formatNumber(summary.change, { maximumFractionDigits: 1 })}% за
                период
              </span>
            )}
            <span className="text-xs text-muted">
              {summary.measured.length} дней с подходами
            </span>
          </div>
          <div
            className="mt-5 h-52 min-w-0"
            role="img"
            aria-label={`Динамика расчётного 1ПМ: ${summary.first} — ${summary.last} кг`}
          >
            <ResponsiveContainer width="100%" height="100%">
              <LineChart
                data={chart}
                margin={{ top: 12, right: 12, bottom: 4, left: 0 }}
              >
                <XAxis
                  dataKey="time"
                  type="number"
                  scale="time"
                  domain={["dataMin", "dataMax"]}
                  tickFormatter={(time: number) =>
                    formatDate(new Date(time).toISOString().slice(0, 10)).slice(
                      0,
                      5,
                    )
                  }
                  tick={{ fill: "var(--text-secondary)", fontSize: 11 }}
                  axisLine={false}
                  tickLine={false}
                  minTickGap={50}
                />
                <YAxis
                  width={64}
                  domain={["auto", "auto"]}
                  allowDecimals={false}
                  tickCount={4}
                  tickFormatter={(value: number) =>
                    formatNumber(value, { maximumFractionDigits: 0 })
                  }
                  tick={{ fill: "var(--text-secondary)", fontSize: 11 }}
                  axisLine={false}
                  tickLine={false}
                  unit=" кг"
                />
                <Tooltip
                  labelFormatter={(label) =>
                    formatDate(
                      new Date(Number(label)).toISOString().slice(0, 10),
                    )
                  }
                  formatter={(value) => [
                    `${formatNumber(Number(value), { maximumFractionDigits: 1 })} кг`,
                    "Расчётный 1ПМ",
                  ]}
                  contentStyle={{
                    background: "var(--surface-1)",
                    border: "1px solid var(--border)",
                    borderRadius: 10,
                    fontSize: 12,
                  }}
                />
                <Line
                  type="linear"
                  dataKey="estimated_1rm"
                  stroke="var(--accent)"
                  strokeWidth={2}
                  dot={{ r: 2, strokeWidth: 0, fill: "var(--accent)" }}
                  activeDot={{ r: 4 }}
                  connectNulls={false}
                  isAnimationActive={false}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
          <div className="mt-2 flex flex-wrap justify-between gap-2 text-xs tabular-nums text-muted">
            <span>
              {formatDate(summary.measured[0].date)} ·{" "}
              {formatNumber(summary.first, { maximumFractionDigits: 1 })} кг
            </span>
            <span>
              {formatDate(summary.measured.at(-1)?.date)} ·{" "}
              {formatNumber(summary.last, { maximumFractionDigits: 1 })} кг
            </span>
          </div>
        </>
      )}
    </Card>
  );
}
