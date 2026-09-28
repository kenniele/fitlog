"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { ArrowRight, ArrowLeftRight } from "lucide-react";
import { fetchAllList } from "@/lib/api";
import type { BodyMeasurement } from "@/lib/types";
import { formatDate } from "@/lib/format";
import { metricText, numeric } from "@/lib/dashboard";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/field";
import { Dialog } from "@/components/ui/dialog";
import { EmptyState, ErrorState, Skeleton } from "@/components/ui/states";
import { BodyMeasurementDetails } from "./body-history";
import { relativeDifference } from "@/lib/body";
import { SegmentedBody, bodySegments, type BodyRegion } from "./segmented-body";
import { MetricValue } from "@/components/dashboard/metric-value";

const overviewMetrics = [
  { key: "weight_kg", label: "Вес", unit: "кг" },
  { key: "body_fat_percent", label: "Жир", unit: "%", deltaUnit: "п.п." },
  { key: "skeletal_muscle_mass_kg", label: "Скелетные мышцы", unit: "кг" },
  { key: "fat_mass_kg", label: "Жировая масса", unit: "кг" },
  { key: "lean_mass_kg", label: "Безжировая масса", unit: "кг" },
  { key: "total_body_water_l", label: "Общая вода", unit: "л" },
  { key: "ecw_tbw_ratio", label: "ECW / TBW", unit: "" },
  { key: "basal_metabolic_rate_kcal", label: "Базовый обмен", unit: "ккал" },
  { key: "inbody_score", label: "InBody Score", unit: "" },
  { key: "phase_angle_degrees", label: "Фазовый угол", unit: "°" },
  { key: "intracellular_water_l", label: "Внутриклеточная вода", unit: "л" },
  { key: "extracellular_water_l", label: "Внеклеточная вода", unit: "л" },
  { key: "protein_mass_kg", label: "Белковая масса", unit: "кг" },
  { key: "mineral_mass_kg", label: "Минералы", unit: "кг" },
  { key: "bmi", label: "ИМТ", unit: "" },
  {
    key: "visceral_fat_area_cm2",
    label: "Висцеральный жир, площадь",
    unit: "см²",
  },
  { key: "visceral_fat_level", label: "Висцеральный жир, уровень", unit: "" },
  { key: "waist_cm", label: "Талия", unit: "см" },
  { key: "chest_cm", label: "Грудь", unit: "см" },
  { key: "biceps_cm", label: "Бицепс", unit: "см" },
  { key: "thigh_cm", label: "Бедро", unit: "см" },
] satisfies Array<{
  key: keyof BodyMeasurement;
  label: string;
  unit: string;
  deltaUnit?: string;
}>;
const segmentMetrics = [
  { key: "lean_mass_kg", label: "Безжировая масса", unit: "кг" },
  {
    key: "lean_percent",
    label: "Безжировая масса к стандарту",
    unit: "%",
    deltaUnit: "п.п.",
  },
  { key: "fat_mass_kg", label: "Жировая масса", unit: "кг" },
  {
    key: "fat_percent",
    label: "Жировая масса к стандарту",
    unit: "%",
    deltaUnit: "п.п.",
  },
] as const;
function MeasuredRow({
  label,
  metric,
  value,
  before,
  unit,
  deltaUnit = unit,
}: {
  label: string;
  metric: string;
  value: unknown;
  before: unknown;
  unit: string;
  deltaUnit?: string;
}) {
  const a = numeric(value);
  const b = numeric(before);
  return (
    <div className="body-detail-row">
      <dt className="text-sm text-muted">{label}</dt>
      <dd className="text-right text-sm tabular-nums">
        <span className="font-medium">{metricText(a, metric, unit)}</span>
        {a !== null && b !== null && (
          <span className="mt-1 block text-xs text-muted">
            Было {metricText(b, metric, unit)} · Δ {a - b > 0 ? "+" : ""}
            {metricText(a - b, metric, deltaUnit)}
          </span>
        )}
      </dd>
    </div>
  );
}
export function InBodyView({
  measurements,
}: {
  measurements: BodyMeasurement[];
}) {
  const rows = [...measurements].sort(
    (a, b) =>
      new Date(b.measured_at).getTime() - new Date(a.measured_at).getTime(),
  );
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [baselineId, setBaselineId] = useState<string | null>(null);
  const [comparing, setComparing] = useState(false);
  const [region, setRegion] = useState<BodyRegion>("all");
  const [mobileDetail, setMobileDetail] = useState(false);
  const [fullDetail, setFullDetail] = useState(false);
  const selected = rows.find((row) => String(row.id) === selectedId) ?? rows[0];
  const selectedIndex = rows.findIndex((row) => row.id === selected?.id);
  const automatic = rows[selectedIndex + 1];
  const before = comparing
    ? (rows.find(
        (row) => String(row.id) === baselineId && row.id !== selected?.id,
      ) ??
      automatic ??
      rows.find((row) => row.id !== selected?.id))
    : automatic;
  if (!selected)
    return (
      <EmptyState
        title="История тела начинается с первого измерения"
        description="Сохраните InBody — здесь появятся состав тела, сегменты и сравнение измерений."
        action={
          <Link href="/dashboard/body?action=new" className="text-link">
            Добавить InBody <ArrowRight className="size-4" />
          </Link>
        }
      />
    );
  const select = (value: BodyRegion) => {
    setRegion(value);
    if (window.matchMedia("(max-width: 767px)").matches) setMobileDetail(true);
  };
  const currentSegment = selected.segments?.find(
    (segment) => segment.segment === region,
  );
  const previousSegment = before?.segments?.find(
    (segment) => segment.segment === region,
  );
  const regionLabel =
    bodySegments.find((segment) => segment.key === region)?.label ?? "Всё тело";
  const arms = relativeDifference(
    selected.segments?.find((s) => s.segment === "left_arm")?.lean_mass_kg,
    selected.segments?.find((s) => s.segment === "right_arm")?.lean_mass_kg,
  );
  const legs = relativeDifference(
    selected.segments?.find((s) => s.segment === "left_leg")?.lean_mass_kg,
    selected.segments?.find((s) => s.segment === "right_leg")?.lean_mass_kg,
  );
  const details = (
    <div key={`${selected.id}-${before?.id}-${region}`} className="reveal">
      <h3 className="text-xl font-medium">{regionLabel}</h3>
      <p className="mt-2 text-xs leading-5 text-muted">
        {before
          ? `Сравнение с ${formatDate(before.measured_at)}`
          : "Первое измерение · без сравнения"}
      </p>
      <dl className="mt-4">
        {region === "all"
          ? overviewMetrics
              .slice(2, 8)
              .map((item) => (
                <MeasuredRow
                  key={item.key}
                  label={item.label}
                  metric={item.key}
                  value={selected[item.key]}
                  before={before?.[item.key]}
                  unit={item.unit}
                />
              ))
          : segmentMetrics.map((item) => (
              <MeasuredRow
                key={item.key}
                label={item.label}
                metric={item.key}
                value={currentSegment?.[item.key]}
                before={previousSegment?.[item.key]}
                unit={item.unit}
                deltaUnit={item.key.endsWith("percent") ? "п.п." : item.unit}
              />
            ))}
      </dl>
      {region !== "all" && (
        <p className="mt-4 text-xs leading-5 text-muted">
          Проценты сегментов — значения относительно стандарта прибора, а не
          доля жира в сегменте.
        </p>
      )}
      <Button
        variant="ghost"
        className="mt-4 -ml-3 text-accent"
        onClick={() => {
          setMobileDetail(false);
          setFullDetail(true);
        }}
      >
        Все показатели и сравнение <ArrowRight className="size-4" />
      </Button>
    </div>
  );
  return (
    <div>
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-muted">
          Измерение от{" "}
          <span className="text-ink">
            {formatDate(selected.measured_at, "dd.MM.yyyy HH:mm")}
          </span>
        </p>
        {rows.length > 1 ? (
          <Button
            size="sm"
            variant={comparing ? "secondary" : "ghost"}
            aria-pressed={comparing}
            onClick={() => setComparing(!comparing)}
          >
            <ArrowLeftRight className="size-4" />
            Сравнить измерения
          </Button>
        ) : (
          <p className="text-xs text-muted">
            Сравнение появится со вторым измерением
          </p>
        )}
      </div>
      {comparing && (
        <div className="mb-7 grid gap-3 rounded-card bg-surface p-4 sm:grid-cols-2">
          <label className="grid gap-2 text-xs text-muted">
            Базовое измерение
            <Select
              aria-label="Базовое измерение"
              value={String(before?.id ?? "")}
              onChange={(event) => setBaselineId(event.target.value)}
            >
              {rows
                .filter((row) => row.id !== selected.id)
                .map((row) => (
                  <option key={row.id} value={row.id}>
                    {formatDate(row.measured_at, "dd.MM.yyyy HH:mm")}
                  </option>
                ))}
            </Select>
          </label>
          <label className="grid gap-2 text-xs text-muted">
            Выбранное измерение
            <Select
              aria-label="Выбранное измерение"
              value={String(selected.id)}
              onChange={(event) => setSelectedId(event.target.value)}
            >
              {rows.map((row) => (
                <option key={row.id} value={row.id}>
                  {formatDate(row.measured_at, "dd.MM.yyyy HH:mm")}
                </option>
              ))}
            </Select>
          </label>
        </div>
      )}
      <div className="grid items-center gap-7 md:grid-cols-[.8fr_1fr_1.2fr] lg:gap-12">
        <div className="grid grid-cols-3 gap-3 md:grid-cols-1 md:gap-9">
          {overviewMetrics.slice(0, 3).map((item) => {
            const value = numeric(selected[item.key]);
            const baseline = numeric(before?.[item.key]);
            return (
              <div key={item.key}>
                <p className="mb-3 text-xs text-muted">{item.label}</p>
                <MetricValue
                  value={value}
                  unit={item.unit}
                  className={
                    item.key === "weight_kg"
                      ? "text-4xl sm:text-6xl"
                      : "text-3xl sm:text-4xl"
                  }
                />
                {value !== null && baseline !== null && (
                  <p className="mt-2 text-xs tabular-nums text-muted">
                    {value - baseline > 0 ? "+" : ""}
                    {metricText(
                      value - baseline,
                      item.key,
                      "deltaUnit" in item ? item.deltaUnit : item.unit,
                    )}{" "}
                    к {formatDate(before?.measured_at)}
                  </p>
                )}
              </div>
            );
          })}
        </div>
        <div className="text-center">
          <SegmentedBody selected={region} onSelect={select} />
          <button
            className="quiet-control mt-2"
            aria-pressed={region === "all"}
            onClick={() => select("all")}
          >
            Всё тело
          </button>
          <Button
            variant="ghost"
            className="mx-auto mt-2 block text-accent md:hidden"
            onClick={() => setFullDetail(true)}
          >
            Все показатели
          </Button>
          <p className="mx-auto mt-2 max-w-60 text-xs leading-5 text-muted">
            Выберите область на схеме.
            <br />
            Схема не отражает вашу фигуру.
          </p>
        </div>
        <div className="hidden min-w-0 md:block">{details}</div>
      </div>
      <div className="mt-7 flex flex-wrap gap-x-8 gap-y-2 text-xs text-muted">
        <span>Асимметрия рук: {metricText(arms, "", "%")}</span>
        <span>Асимметрия ног: {metricText(legs, "", "%")}</span>
        <span>Слева на схеме — правая сторона тела</span>
      </div>
      <div className="mt-8 border-t border-line pt-4">
        <p className="mb-2 text-xs text-muted">История измерений</p>
        <div
          role="group"
          aria-label="История InBody"
          className="scrollbar-thin flex gap-2 overflow-x-auto pb-2"
        >
          {[...rows].reverse().map((row) => (
            <button
              key={row.id}
              aria-pressed={selected.id === row.id}
              onClick={() => setSelectedId(String(row.id))}
              className={`min-h-12 shrink-0 rounded-control px-4 py-2 text-left transition-colors ${selected.id === row.id ? "bg-elevated text-ink" : "text-muted hover:bg-surface"}`}
            >
              <span className="block text-xs">
                {formatDate(row.measured_at)}
              </span>
              <span className="mt-1 block text-sm tabular-nums">
                {metricText(row.weight_kg, "", "кг")}
              </span>
            </button>
          ))}
        </div>
      </div>
      <Dialog
        open={mobileDetail}
        onOpenChange={setMobileDetail}
        title={regionLabel}
        description={formatDate(selected.measured_at)}
      >
        <div className="[&_h3]:hidden">{details}</div>
      </Dialog>
      <Dialog
        open={fullDetail}
        onOpenChange={setFullDetail}
        title="Показатели InBody"
        description={
          before
            ? `${formatDate(before.measured_at)} → ${formatDate(selected.measured_at)}`
            : formatDate(selected.measured_at)
        }
        className="sm:max-w-4xl"
      >
        <dl className="grid gap-x-8 sm:grid-cols-2">
          {overviewMetrics.map((item) => (
            <MeasuredRow
              key={item.key}
              label={item.label}
              metric={item.key}
              value={selected[item.key]}
              before={before?.[item.key]}
              unit={item.unit}
              deltaUnit={"deltaUnit" in item ? item.deltaUnit : item.unit}
            />
          ))}
        </dl>
        <p className="mt-5 text-xs leading-5 text-muted">
          Справочные ориентиры прибора: ECW / TBW 0,360–0,390; висцеральный жир
          — ниже 100 см² или уровня 10. Площадь и уровень не сравниваются между
          собой.
        </p>
        <p className="mt-3 text-xs text-muted">
          Оценки прибора и референсы — контекст измерения, не медицинский
          диагноз.
        </p>
        <details className="mt-5">
          <summary className="cursor-pointer py-3 text-sm text-muted">
            Исходная запись, сегменты и заметки
          </summary>
          <BodyMeasurementDetails entry={selected} />
        </details>
      </Dialog>
    </div>
  );
}
export function InBodyExplorer() {
  const query = useQuery({
    queryKey: ["inbody-history"],
    queryFn: () =>
      fetchAllList<BodyMeasurement>("/body-measurements?source=inbody"),
  });
  if (query.isPending)
    return (
      <div aria-busy="true" aria-label="Загрузка InBody">
        <Skeleton className="h-[460px]" />
      </div>
    );
  if (query.isError)
    return (
      <ErrorState
        title="Не удалось загрузить InBody"
        error={query.error}
        retry={() => query.refetch()}
      />
    );
  return <InBodyView measurements={query.data} />;
}
