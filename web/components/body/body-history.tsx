"use client";

import { useMemo, useState } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import { Eye, Pencil, Trash2 } from "lucide-react";
import type { ListResponse } from "@/lib/api";
import type { BodyMeasurement, BodySegment } from "@/lib/types";
import { formatDate, formatNumber } from "@/lib/format";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { DataTable } from "@/components/ui/data-table";
import { Dialog } from "@/components/ui/dialog";
import { PAGE_SIZE, Pagination } from "@/components/ui/pagination";

export type BodyHistoryScope = "all" | "inbody";

export function bodyHistoryPath(scope: BodyHistoryScope, page: number) {
  const params = new URLSearchParams({ page: String(page), page_size: String(PAGE_SIZE) });
  if (scope === "inbody") params.set("source", "inbody");
  return `/body-measurements?${params.toString()}`;
}

const segmentLabels: Record<string, string> = {
  left_arm: "Левая рука",
  right_arm: "Правая рука",
  trunk: "Корпус",
  left_leg: "Левая нога",
  right_leg: "Правая нога",
};

function sourceLabel(source?: string | null) {
  switch (source) {
  case "inbody": return "InBody";
  case "manual": return "Вручную";
  case "csv": return "CSV";
  case "json": return "JSON";
  default: return source || "—";
  }
}

function value(number: number | null | undefined, suffix = "", maximumFractionDigits = 1) {
  return formatNumber(number, { maximumFractionDigits }, suffix);
}

type Detail = { label: string; value: string };

function DetailGroup({ title, items }: { title: string; items: Detail[] }) {
  return <section className="rounded-control border border-line bg-canvas/25 p-4">
    <h3 className="text-xs font-semibold uppercase tracking-[.12em] text-muted">{title}</h3>
    <dl className="mt-3 grid gap-x-5 gap-y-3 sm:grid-cols-2">
      {items.map((item) => <div key={item.label}><dt className="text-xs text-muted">{item.label}</dt><dd className="mt-0.5 text-sm font-medium text-ink">{item.value}</dd></div>)}
    </dl>
  </section>;
}

function SegmentTable({ segments }: { segments?: BodySegment[] | null }) {
  if (!segments?.length) return <p className="text-sm text-muted">Сегментарные показатели не заполнены.</p>;
  return <div className="overflow-x-auto rounded-control border border-line"><table className="w-full min-w-[620px] border-collapse text-left text-sm">
    <thead><tr className="border-b border-line bg-white/[.025] text-[11px] uppercase tracking-wide text-muted"><th className="px-3 py-2">Сегмент</th><th className="px-3 py-2">Lean, кг</th><th className="px-3 py-2">Lean, %</th><th className="px-3 py-2">Fat, кг</th><th className="px-3 py-2">Fat, %</th></tr></thead>
    <tbody>{segments.map((segment) => <tr key={segment.segment} className="border-b border-line/70 last:border-0"><td className="px-3 py-2 font-medium text-ink">{segmentLabels[segment.segment] ?? segment.segment}</td><td className="px-3 py-2">{value(segment.lean_mass_kg, " кг", 2)}</td><td className="px-3 py-2">{value(segment.lean_percent, "%")}</td><td className="px-3 py-2">{value(segment.fat_mass_kg, " кг", 2)}</td><td className="px-3 py-2">{value(segment.fat_percent, "%")}</td></tr>)}</tbody>
  </table></div>;
}

export function BodyMeasurementDetails({ entry }: { entry: BodyMeasurement }) {
  const composition: Detail[] = [
    { label: "Вес", value: value(entry.weight_kg, " кг") },
    { label: "Жир", value: value(entry.body_fat_percent, "%") },
    { label: "Жировая масса", value: value(entry.fat_mass_kg, " кг") },
    { label: "Безжировая масса", value: value(entry.lean_mass_kg, " кг") },
    { label: "Скелетные мышцы", value: value(entry.skeletal_muscle_mass_kg, " кг") },
    { label: "BMI", value: value(entry.bmi) },
    { label: "Белок", value: value(entry.protein_mass_kg, " кг") },
    { label: "Минералы", value: value(entry.mineral_mass_kg, " кг") },
  ];
  const water: Detail[] = [
    { label: "Общая вода (TBW)", value: value(entry.total_body_water_l, " л") },
    { label: "Внутриклеточная вода (ICW)", value: value(entry.intracellular_water_l, " л") },
    { label: "Внеклеточная вода (ECW)", value: value(entry.extracellular_water_l, " л") },
    { label: "ECW / TBW", value: value(entry.ecw_tbw_ratio, "", 3) },
  ];
  const inbody: Detail[] = [
    { label: "InBody Score", value: value(entry.inbody_score, "", 0) },
    { label: "Висцеральный жир, уровень", value: value(entry.visceral_fat_level, "", 0) },
    { label: "Висцеральный жир, площадь", value: value(entry.visceral_fat_area_cm2, " см²") },
    { label: "Базовый обмен", value: value(entry.basal_metabolic_rate_kcal, " ккал", 0) },
    { label: "Фазовый угол", value: value(entry.phase_angle_degrees, "°", 2) },
  ];
  const circumferences: Detail[] = [
    { label: "Талия", value: value(entry.waist_cm, " см") },
    { label: "Грудь", value: value(entry.chest_cm, " см") },
    { label: "Бицепс", value: value(entry.biceps_cm, " см") },
    { label: "Бедро", value: value(entry.thigh_cm, " см") },
  ];

  return <div className="space-y-4">
    <div className="flex flex-wrap items-center gap-2"><Badge tone={entry.source === "inbody" ? "good" : "neutral"}>{sourceLabel(entry.source)}</Badge><span className="text-xs text-muted">ID {entry.id}</span>{entry.external_id ? <span className="text-xs text-muted">External ID: {entry.external_id}</span> : null}</div>
    <div className="grid gap-3 lg:grid-cols-2"><DetailGroup title="Состав тела" items={composition} /><DetailGroup title="Водный баланс" items={water} /><DetailGroup title="Расширенный InBody" items={inbody} /><DetailGroup title="Окружности" items={circumferences} /></div>
    <section><h3 className="mb-2 text-xs font-semibold uppercase tracking-[.12em] text-muted">Сегментарный анализ</h3><SegmentTable segments={entry.segments} /></section>
    {entry.notes ? <section className="rounded-control border border-line bg-canvas/25 p-4"><h3 className="text-xs font-semibold uppercase tracking-[.12em] text-muted">Комментарий</h3><p className="mt-2 whitespace-pre-wrap text-sm text-ink">{entry.notes}</p></section> : null}
  </div>;
}

export function BodyHistory({ data, scope, page, fetching, onScopeChange, onPageChange, onEdit, onDelete }: {
  data: ListResponse<BodyMeasurement>;
  scope: BodyHistoryScope;
  page: number;
  fetching?: boolean;
  onScopeChange: (scope: BodyHistoryScope) => void;
  onPageChange: (page: number) => void;
  onEdit: (entry: BodyMeasurement) => void;
  onDelete: (entry: BodyMeasurement) => void;
}) {
  const [viewing, setViewing] = useState<BodyMeasurement | null>(null);
  const rows = data.items ?? [];
  const columns = useMemo<ColumnDef<BodyMeasurement>[]>(() => [
    { accessorKey: "measured_at", header: "Дата и время", cell: ({ getValue }) => formatDate(getValue<string>(), "dd.MM.yyyy HH:mm") },
    { accessorKey: "source", header: "Тип", cell: ({ row }) => <Badge tone={row.original.source === "inbody" ? "good" : "neutral"}>{sourceLabel(row.original.source)}</Badge> },
    { accessorKey: "weight_kg", header: "Вес", cell: ({ getValue }) => value(getValue<number | null>(), " кг") },
    { accessorKey: "body_fat_percent", header: "Жир", cell: ({ getValue }) => value(getValue<number | null>(), "%") },
    { accessorKey: "skeletal_muscle_mass_kg", header: "Мышцы", cell: ({ getValue }) => value(getValue<number | null>(), " кг") },
    { accessorKey: "fat_mass_kg", header: "Жир. масса", cell: ({ getValue }) => value(getValue<number | null>(), " кг") },
    { accessorKey: "lean_mass_kg", header: "Безжир. масса", cell: ({ getValue }) => value(getValue<number | null>(), " кг") },
    { accessorKey: "total_body_water_l", header: "Вода", cell: ({ getValue }) => value(getValue<number | null>(), " л") },
    { accessorKey: "visceral_fat_level", header: "Висц. жир", cell: ({ row }) => value(row.original.visceral_fat_area_cm2 ?? row.original.visceral_fat_level, row.original.visceral_fat_area_cm2 != null ? " см²" : "") },
    { accessorKey: "inbody_score", header: "Score", cell: ({ getValue }) => value(getValue<number | null>(), "", 0) },
    { id: "actions", header: "", cell: ({ row }) => <div className="flex justify-end gap-1"><Button variant="ghost" size="icon" aria-label="Просмотреть измерение" onClick={() => setViewing(row.original)}><Eye className="size-4" /></Button><Button variant="ghost" size="icon" aria-label="Редактировать измерение" onClick={() => onEdit(row.original)}><Pencil className="size-4" /></Button><Button variant="ghost" size="icon" aria-label="Удалить измерение" onClick={() => onDelete(row.original)}><Trash2 className="size-4" /></Button></div> },
  ], [onDelete, onEdit]);

  return <>
    <Card aria-labelledby="body-history-title">
      <CardHeader className="flex-col gap-3 sm:flex-row sm:items-center">
        <div><CardTitle id="body-history-title">История измерений</CardTitle><CardDescription>Все сохранённые Body и InBody за всё время. Выбранный сверху период влияет только на аналитику, но не скрывает записи здесь.</CardDescription></div>
        <div className="flex rounded-control border border-line bg-canvas/45 p-1" role="group" aria-label="Фильтр истории измерений">
          <Button type="button" size="sm" variant={scope === "all" ? "secondary" : "ghost"} aria-pressed={scope === "all"} onClick={() => onScopeChange("all")}>Все записи</Button>
          <Button type="button" size="sm" variant={scope === "inbody" ? "secondary" : "ghost"} aria-pressed={scope === "inbody"} onClick={() => onScopeChange("inbody")}>Только InBody</Button>
        </div>
      </CardHeader>
      <DataTable data={rows} columns={columns} emptyTitle={scope === "inbody" ? "Нет записей InBody" : "Нет измерений"} rowKey={(row) => String(row.id)} />
      <Pagination data={data} page={page} onPageChange={onPageChange} disabled={fetching} />
    </Card>
    <Dialog open={Boolean(viewing)} onOpenChange={(open) => { if (!open) setViewing(null); }} title={viewing ? `Измерение от ${formatDate(viewing.measured_at, "dd.MM.yyyy HH:mm")}` : "Измерение"} description="Полный сохранённый набор показателей без подстановки отсутствующих значений." className="sm:max-w-5xl">
      {viewing ? <BodyMeasurementDetails entry={viewing} /> : null}
    </Dialog>
  </>;
}
