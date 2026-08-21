"use client";

import { Activity, ArrowRight, Droplets, Flame, ScanLine, Scale, ShieldAlert } from "lucide-react";
import type { BodyMeasurement, BodySegment } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { formatDate, formatNumber } from "@/lib/format";

const segmentLabels: Record<string, string> = {
  left_arm: "Левая рука", right_arm: "Правая рука", trunk: "Корпус", left_leg: "Левая нога", right_leg: "Правая нога",
};

export function relativeDifference(left?: number | null, right?: number | null) {
  if (typeof left !== "number" || typeof right !== "number") return null;
  const average = (Math.abs(left) + Math.abs(right)) / 2;
  return average === 0 ? null : Math.abs(left - right) / average * 100;
}

function segment(snapshot: BodyMeasurement, name: string) {
  return snapshot.segments?.find((item) => item.segment === name);
}

function delta(current?: number | null, previous?: number | null) {
  return typeof current === "number" && typeof previous === "number" ? current - previous : null;
}

function Delta({ value, unit, better }: { value: number | null; unit: string; better: "higher" | "lower" }) {
  if (value === null) return <span className="text-xs text-muted">Нет предыдущего InBody</span>;
  const improved = better === "higher" ? value > 0 : value < 0;
  return <Badge tone={value === 0 ? "neutral" : improved ? "good" : "warning"}>{value > 0 ? "+" : ""}{formatNumber(value, { maximumFractionDigits: 2 })} {unit}</Badge>;
}

function Metric({ icon: Icon, label, value, unit, context, tone = "neutral", maximumFractionDigits = 1 }: {
  icon: typeof Activity; label: string; value?: number | null; unit?: string; context: string; tone?: "neutral" | "good" | "warning"; maximumFractionDigits?: number;
}) {
  return <Card className="p-4"><div className="flex items-start justify-between gap-3"><div><p className="text-xs text-muted">{label}</p><p className="mt-2 text-2xl font-semibold text-ink">{formatNumber(value, { maximumFractionDigits })}{typeof value === "number" && unit ? <span className="ml-1 text-sm text-muted">{unit}</span> : null}</p></div><span className="rounded-xl border border-line bg-white/[.04] p-2 text-accent"><Icon className="size-4" /></span></div><div className="mt-3"><Badge tone={tone}>{context}</Badge></div></Card>;
}

function segmentValue(value: number | null | undefined, unit: string) {
  return typeof value === "number" ? `${formatNumber(value, { maximumFractionDigits: 2 })} ${unit}` : "—";
}

function signedValue(value: number | null, unit: string, maximumFractionDigits = 2, minimumFractionDigits?: number) {
  if (value === null) return "—";
  return `${value > 0 ? "+" : ""}${formatNumber(value, { maximumFractionDigits, minimumFractionDigits })}${unit ? ` ${unit}` : ""}`;
}

function ComparisonItem({ label, current, previous, unit = "", deltaUnit = unit, maximumFractionDigits = 1, minimumFractionDigits }: {
  label: string; current?: number | null; previous?: number | null; unit?: string; deltaUnit?: string; maximumFractionDigits?: number; minimumFractionDigits?: number;
}) {
  const change = delta(current, previous);
  const formatted = (value?: number | null) => typeof value === "number" ? `${formatNumber(value, { maximumFractionDigits, minimumFractionDigits })}${unit ? ` ${unit}` : ""}` : "—";
  return <div aria-label={`Сравнение ${label}`} className="rounded-control border border-line bg-white/[.025] p-3"><p className="text-xs font-medium text-muted">{label}</p><div className="mt-2 flex items-center gap-2 text-sm"><span className="text-muted">{formatted(previous)}</span><ArrowRight className="size-3.5 shrink-0 text-muted" aria-hidden /><span className="font-semibold text-ink">{formatted(current)}</span></div><p className="mt-2 text-xs text-muted">Δ {signedValue(change, deltaUnit, maximumFractionDigits, minimumFractionDigits)}</p></div>;
}

function SegmentRow({ item, previous }: { item: BodySegment; previous?: BodySegment }) {
  return <div aria-label={`Сегмент ${segmentLabels[item.segment] ?? item.segment}`} className="grid grid-cols-[minmax(7rem,1.3fr)_repeat(6,minmax(4rem,1fr))] gap-2 border-t border-line px-3 py-2.5 text-xs first:border-t-0"><span className="font-medium text-ink">{segmentLabels[item.segment] ?? item.segment}</span><span>{segmentValue(item.lean_mass_kg, "кг")}</span><span className="text-muted">{signedValue(delta(item.lean_mass_kg, previous?.lean_mass_kg), "кг")}</span><span>{segmentValue(item.lean_percent, "%")}</span><span>{segmentValue(item.fat_mass_kg, "кг")}</span><span className="text-muted">{signedValue(delta(item.fat_mass_kg, previous?.fat_mass_kg), "кг")}</span><span>{segmentValue(item.fat_percent, "%")}</span></div>;
}

export function InBodyAnalysis({ latest, previous }: { latest?: BodyMeasurement | null; previous?: BodyMeasurement | null }) {
  if (!latest) return <Card className="p-5"><div className="flex items-start gap-3"><ScanLine className="mt-0.5 size-5 text-accent" /><div><p className="font-medium text-ink">Добавьте первый InBody</p><p className="mt-1 text-sm text-muted">После сохранения появятся водный баланс, висцеральный жир, BMR и сегментарная асимметрия.</p></div></div></Card>;
  const ecw = latest.ecw_tbw_ratio;
  const ecwKnown = typeof ecw === "number";
  const ecwReference = ecwKnown && ecw >= 0.36 && ecw <= 0.39;
  const visceralValue = latest.visceral_fat_area_cm2 ?? latest.visceral_fat_level;
  const visceralArea = typeof latest.visceral_fat_area_cm2 === "number";
  const visceralReference = typeof visceralValue === "number" && (visceralArea ? visceralValue < 100 : visceralValue < 10);
  const leftArm = segment(latest, "left_arm"); const rightArm = segment(latest, "right_arm");
  const leftLeg = segment(latest, "left_leg"); const rightLeg = segment(latest, "right_leg");
  const armDifference = relativeDifference(leftArm?.lean_mass_kg, rightArm?.lean_mass_kg);
  const legDifference = relativeDifference(leftLeg?.lean_mass_kg, rightLeg?.lean_mass_kg);
  const comparisonMetrics = previous ? [
    { label: "Вес", current: latest.weight_kg, previous: previous.weight_kg, unit: "кг", maximumFractionDigits: 1 },
    { label: "Жир", current: latest.body_fat_percent, previous: previous.body_fat_percent, unit: "%", deltaUnit: "п.п.", maximumFractionDigits: 1 },
    { label: "Жировая масса", current: latest.fat_mass_kg, previous: previous.fat_mass_kg, unit: "кг", maximumFractionDigits: 1 },
    { label: "Безжировая масса", current: latest.lean_mass_kg, previous: previous.lean_mass_kg, unit: "кг", maximumFractionDigits: 1 },
    { label: "Скелетные мышцы", current: latest.skeletal_muscle_mass_kg, previous: previous.skeletal_muscle_mass_kg, unit: "кг", maximumFractionDigits: 1 },
    { label: "Общая вода", current: latest.total_body_water_l, previous: previous.total_body_water_l, unit: "л", maximumFractionDigits: 1 },
    { label: "ECW / TBW", current: latest.ecw_tbw_ratio, previous: previous.ecw_tbw_ratio, maximumFractionDigits: 3, minimumFractionDigits: 3 },
    { label: "Висцеральный жир", current: latest.visceral_fat_area_cm2 ?? latest.visceral_fat_level, previous: previous.visceral_fat_area_cm2 ?? previous.visceral_fat_level, unit: typeof latest.visceral_fat_area_cm2 === "number" ? "см²" : "", maximumFractionDigits: 0 },
    { label: "Базовый обмен", current: latest.basal_metabolic_rate_kcal, previous: previous.basal_metabolic_rate_kcal, unit: "ккал", maximumFractionDigits: 0 },
    { label: "InBody Score", current: latest.inbody_score, previous: previous.inbody_score, maximumFractionDigits: 0 },
  ] : [];

  return <section className="space-y-4" aria-labelledby="inbody-analysis-title">
    <div><p className="text-[11px] font-semibold uppercase tracking-[.18em] text-accent">InBody snapshot</p><h2 id="inbody-analysis-title" className="mt-1 text-lg font-semibold text-ink">Разбор от {formatDate(latest.measured_at)}</h2><p className="mt-1 text-sm text-muted">Сравнение только с предыдущим сохранённым InBody. Референсы производителя показаны как контекст, а не медицинский диагноз.</p></div>
    {previous && <Card className="p-4"><div><p className="text-sm font-semibold text-ink">Сравнение двух последних InBody</p><p className="mt-1 text-xs text-muted">{formatDate(latest.measured_at)} против {formatDate(previous.measured_at)} · было → стало → изменение</p></div><div className="mt-4 grid gap-2 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">{comparisonMetrics.map((item) => <ComparisonItem key={item.label} {...item} />)}</div></Card>}
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <Metric icon={Scale} label="Скелетные мышцы" value={latest.skeletal_muscle_mass_kg} unit="кг" context={previous ? `Δ ${formatNumber(delta(latest.skeletal_muscle_mass_kg, previous.skeletal_muscle_mass_kg), { maximumFractionDigits: 2 })} кг` : "Первая точка"} tone={delta(latest.skeletal_muscle_mass_kg, previous?.skeletal_muscle_mass_kg) != null && delta(latest.skeletal_muscle_mass_kg, previous?.skeletal_muscle_mass_kg)! > 0 ? "good" : "neutral"} />
      <Metric icon={Droplets} label="ECW / TBW" value={latest.ecw_tbw_ratio} maximumFractionDigits={3} context={ecwKnown ? (ecwReference ? "В справочном диапазоне 0,360–0,390" : "Вне справочного диапазона 0,360–0,390") : "Нет данных"} tone={ecwKnown ? (ecwReference ? "good" : "warning") : "neutral"} />
      <Metric icon={ShieldAlert} label={visceralArea ? "Висцеральный жир" : "Уровень висцерального жира"} value={visceralValue} unit={visceralArea ? "см²" : undefined} context={typeof visceralValue === "number" ? (visceralReference ? (visceralArea ? "Ниже справочных 100 см²" : "Ниже справочного уровня 10") : "Выше справочного ориентира") : "Нет данных"} tone={typeof visceralValue === "number" ? (visceralReference ? "good" : "warning") : "neutral"} />
      <Metric icon={Flame} label="Базовый обмен" value={latest.basal_metabolic_rate_kcal} unit="ккал" context="Оценка прибора, не цель питания" />
      <Metric icon={ScanLine} label="InBody Score" value={latest.inbody_score} context="Сравнивайте только в одинаковых условиях" />
      <Metric icon={Activity} label="Фазовый угол" value={latest.phase_angle_degrees} unit="°" context="Тренд информативнее одной точки" />
      <Metric icon={Droplets} label="Общая вода" value={latest.total_body_water_l} unit="л" context={typeof latest.intracellular_water_l === "number" && typeof latest.extracellular_water_l === "number" ? `ICW ${formatNumber(latest.intracellular_water_l)} · ECW ${formatNumber(latest.extracellular_water_l)} л` : "ICW / ECW не заполнены"} />
      <Card className="p-4"><p className="text-xs text-muted">Изменение состава</p><div className="mt-3 flex flex-wrap gap-2"><Delta value={delta(latest.fat_mass_kg, previous?.fat_mass_kg)} unit="кг жира" better="lower" /><Delta value={delta(latest.lean_mass_kg, previous?.lean_mass_kg)} unit="кг lean" better="higher" /></div></Card>
    </div>
    <Card className="overflow-hidden"><div className="flex flex-col gap-2 border-b border-line p-4 sm:flex-row sm:items-center sm:justify-between"><div><p className="text-sm font-semibold text-ink">Сегментарный баланс</p><p className="mt-1 text-xs text-muted">Lean/Fat по пяти зонам с листа InBody; Δ — к предыдущему измерению.</p></div><div className="flex flex-wrap gap-2"><Badge>Руки L/R: {armDifference === null ? "—" : `${formatNumber(armDifference, { maximumFractionDigits: 1 })}%`}</Badge><Badge>Ноги L/R: {legDifference === null ? "—" : `${formatNumber(legDifference, { maximumFractionDigits: 1 })}%`}</Badge></div></div><div className="overflow-x-auto"><div className="min-w-[780px]"><div className="grid grid-cols-[minmax(7rem,1.3fr)_repeat(6,minmax(4rem,1fr))] gap-2 bg-white/[.025] px-3 py-2 text-[11px] font-medium uppercase tracking-wide text-muted"><span>Сегмент</span><span>Lean, кг</span><span>Δ lean</span><span>Lean, %</span><span>Fat, кг</span><span>Δ fat</span><span>Fat, %</span></div>{latest.segments?.length ? latest.segments.map((item) => <SegmentRow key={item.segment} item={item} previous={previous ? segment(previous, item.segment) : undefined} />) : <p className="p-4 text-sm text-muted">Сегментарные данные не заполнены.</p>}</div></div></Card>
  </section>;
}
