"use client";

import { Suspense, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertCircle } from "lucide-react";
import { apiFetch, fetchAllList, listItems } from "@/lib/api";
import type { AnalyticsResponse, Exercise, WorkoutPlan } from "@/lib/types";
import { useRangeSearch } from "@/lib/hooks";
import { PageHeader } from "@/components/ui/page";
import { Card } from "@/components/ui/card";
import { Select } from "@/components/ui/field";
import { Badge } from "@/components/ui/badge";
import { ErrorState, EmptyState, PageSkeleton } from "@/components/ui/states";
import { formatNumber } from "@/lib/format";

function AnalyticsContent() {
  const { query: range } = useRangeSearch(); const [exercise, setExercise] = useState(""); const [template, setTemplate] = useState(""); const [dayType, setDayType] = useState("");
  const filters = `${range}&exercise_id=${encodeURIComponent(exercise)}&template_id=${encodeURIComponent(template)}&day_type=${encodeURIComponent(dayType)}`;
  const query = useQuery({ queryKey: ["correlations", filters], queryFn: () => apiFetch<AnalyticsResponse>(`/analytics/correlations?${filters}`) });
  const exercises = useQuery({ queryKey: ["analytics-exercises"], queryFn: () => fetchAllList<Exercise>("/exercises") });
  const plans = useQuery({ queryKey: ["analytics-plans"], queryFn: () => fetchAllList<WorkoutPlan>("/workout-plans") });
  if (query.isError || exercises.isError || plans.isError) return <ErrorState error={query.error ?? exercises.error ?? plans.error} retry={() => { void Promise.all([query.refetch(), exercises.refetch(), plans.refetch()]); }} />;
  if (query.isPending || exercises.isPending || plans.isPending) return <PageSkeleton />;
  const templateOptions = Array.from(new Map(listItems(plans.data).flatMap((plan) => [...(plan.templates ?? []), ...(plan.historical_templates ?? [])].map((item) => [String(item.id ?? `${plan.id}-${item.name}`), { plan, item }] as const))).values());
  return <><PageHeader eyebrow="Analytics" title="Связи между нагрузкой и восстановлением" description="Корреляция описывает совместное изменение показателей и не доказывает причинность." /><Card className="grid gap-3 p-4 sm:grid-cols-3"><Select aria-label="Упражнение" value={exercise} onChange={(event) => setExercise(event.target.value)}><option value="">Все упражнения</option>{listItems(exercises.data).map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</Select><Select aria-label="Шаблон" value={template} onChange={(event) => setTemplate(event.target.value)}><option value="">Все шаблоны</option>{templateOptions.map(({ plan, item }) => <option key={`${plan.id}-${item.id ?? item.name}`} value={item.id ?? ""}>{plan.name} · {item.name}</option>)}</Select><Select aria-label="Тип дня" value={dayType} onChange={(event) => setDayType(event.target.value)}><option value="">Все дни</option><option value="training">Тренировочные</option><option value="rest">Нетренировочные</option></Select></Card>{query.data.correlations?.length ? <div className="grid gap-4 lg:grid-cols-2">{query.data.correlations.map((item, index) => { const small = item.insufficient_sample === true; const previous = query.data.comparison?.correlations?.find((candidate) => candidate.id === item.id); return <Card key={item.id ?? index} className="p-5"><div className="flex items-start justify-between gap-3"><div><h2 className="text-sm font-semibold">{item.title}</h2><p className="mt-1 text-xs text-muted">{item.period ?? "Период не указан"}</p></div><Badge tone={small ? "warning" : "blue"}>n = {formatNumber(item.sample_size)}</Badge></div><p className="mt-5 text-4xl font-semibold tracking-[-.04em]">r = {formatNumber(item.coefficient, { maximumFractionDigits: 2 })}</p>{query.data.comparison && <p className="mt-2 text-xs text-muted">Предыдущий период: r = {formatNumber(previous?.coefficient, { maximumFractionDigits: 2 })}, n = {formatNumber(previous?.sample_size)}</p>}<p className="mt-3 text-sm leading-6 text-muted">{item.definition ?? `${item.x_label ?? "X"} сопоставляется с ${item.y_label ?? "Y"}.`}</p><div className="mt-4 flex gap-2 rounded-control border border-warning/15 bg-warning/[.05] p-3 text-xs leading-5 text-muted"><AlertCircle className="mt-0.5 size-4 shrink-0 text-warning" /><span>{small ? "Маленькая выборка: результат нельзя считать устойчивым выводом. " : ""}Корреляция не означает, что один показатель вызывает другой.</span></div></Card>; })}</div> : <Card><EmptyState title="Недостаточно данных для корреляций" description="Добавьте больше пересекающихся дней восстановления, тренировок и питания или расширьте период." /></Card>}</>;
}
export default function AnalyticsPage() { return <Suspense fallback={<PageSkeleton />}><AnalyticsContent /></Suspense>; }
