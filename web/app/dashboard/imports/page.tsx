"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { AlertTriangle, Check, ChevronLeft, ChevronRight, Eye, FileUp, RotateCcw } from "lucide-react";
import { apiFetch, listItems, type ListResponse } from "@/lib/api";
import type { ImportPreview, ImportRun } from "@/lib/types";
import { useQuickAction } from "@/lib/hooks";
import { PageHeader, SectionHeader } from "@/components/ui/page";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Field, Input, Select } from "@/components/ui/field";
import { DataTable } from "@/components/ui/data-table";
import { Dialog } from "@/components/ui/dialog";
import { ErrorState, InlineError, PageSkeleton } from "@/components/ui/states";
import { formatDate, formatNumber } from "@/lib/format";
import { PAGE_SIZE, Pagination } from "@/components/ui/pagination";
import { adaptersForDataType, resolveImportAdapter } from "@/lib/import-adapters";

const dataTypes = [["workouts", "Тренировки"], ["sets", "Подходы"], ["recovery", "Восстановление"], ["sleep", "Сон"], ["nutrition", "Питание"], ["body", "Измерения тела"]];
type PreviewVariables = { selectedMapping: Record<string, string>; nextStep: 3 | 4 };

function ImportsContent() {
  const client = useQueryClient();
  const [selectedRun, setSelectedRun] = useState<ImportRun | null>(null);
  const [open, setOpen] = useState(false); const [journalPage, setJournalPage] = useState(1); const [step, setStep] = useState(1); const [dataType, setDataType] = useState("workouts"); const [adapter, setAdapter] = useState("csv"); const [file, setFile] = useState<File | null>(null); const [content, setContent] = useState(""); const [preview, setPreview] = useState<ImportPreview | null>(null); const [mapping, setMapping] = useState<Record<string, string>>({}); const [result, setResult] = useState<ImportRun | null>(null);
  const start = useCallback(() => { setStep(1); setPreview(null); setResult(null); setFile(null); setContent(""); setMapping({}); setOpen(true); }, []);
  useQuickAction(start);
  const journal = useQuery({ queryKey: ["imports", journalPage], queryFn: () => apiFetch<ListResponse<ImportRun>>(`/imports?page=${journalPage}&page_size=${PAGE_SIZE}`), placeholderData: keepPreviousData });
  const runDetails = useQuery({
    queryKey: ["import", selectedRun?.id],
    queryFn: () => apiFetch<ImportRun>(`/imports/${selectedRun?.id}`),
    enabled: selectedRun != null,
  });
  const adapterConfig = resolveImportAdapter(adapter);
  const source = adapterConfig.source;
  const format = adapterConfig.format;
  const requestBody = (fileContent: string, selectedMapping: Record<string, string>) => ({ data_type: dataType, filename: file?.name ?? "import", format, content: fileContent, mapping: selectedMapping, source });
  const previewMutation = useMutation({
    mutationFn: async ({ selectedMapping, nextStep }: PreviewVariables) => {
      if (!file) throw new Error("Выберите CSV или JSON файл");
      const fileContent = content || await file.text();
      if (!fileContent.trim()) throw new Error("Файл пуст");
      setContent(fileContent);
      const data = await apiFetch<ImportPreview>("/imports/preview", { method: "POST", body: requestBody(fileContent, selectedMapping) });
      return { data, selectedMapping, nextStep };
    },
    onSuccess: ({ data, selectedMapping, nextStep }) => {
      setPreview(data);
      setMapping(nextStep === 3 ? (data.suggested_mapping ?? {}) : selectedMapping);
      setStep(nextStep);
    },
  });
  const execute = useMutation({
    mutationFn: async () => {
      if (!file) throw new Error("Файл больше не выбран");
      const fileContent = content || await file.text();
      return apiFetch<ImportRun>("/imports/execute", { method: "POST", body: requestBody(fileContent, mapping) });
    },
    onSuccess: async (data) => { setResult(data); setStep(6); setJournalPage(1); await client.invalidateQueries({ queryKey: ["imports"] }); },
  });
  const columns = useMemo<ColumnDef<ImportRun>[]>(() => [
    { accessorKey: "started_at", header: "Дата", cell: ({ getValue }) => formatDate(getValue<string>()) },
    { accessorKey: "filename", header: "Файл", cell: ({ getValue }) => String(getValue() ?? "—") },
    { accessorKey: "source", header: "Источник", cell: ({ getValue }) => <Badge>{String(getValue() ?? "—")}</Badge> },
    { accessorKey: "data_type", header: "Тип", cell: ({ getValue }) => <Badge>{String(getValue() ?? "—")}</Badge> },
    { accessorKey: "status", header: "Статус", cell: ({ getValue }) => <Badge tone={getValue() === "completed" ? "good" : getValue() === "failed" ? "critical" : "warning"}>{String(getValue() ?? "—")}</Badge> },
    { accessorKey: "imported_rows", header: "Импортировано", cell: ({ getValue }) => formatNumber(getValue<number | null>()) },
    { accessorKey: "skipped_rows", header: "Пропущено", cell: ({ getValue }) => formatNumber(getValue<number | null>()) },
    { accessorKey: "failed_rows", header: "Ошибки", cell: ({ getValue }) => formatNumber(getValue<number | null>()) },
    { id: "actions", header: "", cell: ({ row }) => <Button variant="ghost" size="icon" aria-label={`Детали импорта ${row.original.filename ?? row.original.id}`} onClick={() => setSelectedRun(row.original)}><Eye className="size-4" /></Button> },
  ], []);
  if (journal.isPending) return <PageSkeleton />;
  if (journal.isError) return <ErrorState error={journal.error} retry={() => journal.refetch()} />;
  const detailedRun = runDetails.data ?? selectedRun;
  const required = preview?.required_fields ?? []; const targetFields = preview?.target_fields ?? required; const available = preview?.columns ?? []; const importFailed = result?.status === "failed";
  return <><PageHeader eyebrow="Data Imports" title="Импорт данных с проверкой" description="Файл проходит preview, mapping, повторную валидацию, поиск дублей и только затем запись." actions={<Button variant="primary" onClick={start}><FileUp className="size-4" />Новый импорт</Button>} /><Card><div className="border-b border-line p-4"><SectionHeader title="Журнал импортов" description="Фактический результат каждой операции." /></div><DataTable data={listItems(journal.data)} columns={columns} emptyTitle="Импортов ещё не было" /><Pagination data={journal.data} page={journalPage} onPageChange={setJournalPage} disabled={journal.isFetching} /></Card><Dialog open={open} onOpenChange={setOpen} title="Импорт CSV / JSON" description={`Шаг ${step} из 6`} className="sm:max-w-4xl"><div className="mb-6 grid grid-cols-6 gap-1">{Array.from({ length: 6 }).map((_, index) => <div key={index} className={`h-1.5 rounded-full ${index + 1 <= step ? "bg-accent" : "bg-white/[.06]"}`} />)}</div>
    {step === 1 && <div className="space-y-4"><Field label="Тип данных"><Select value={dataType} onChange={(event) => { const nextType = event.target.value; setDataType(nextType); if (!adaptersForDataType(nextType).some((candidate) => candidate.value === adapter)) setAdapter("csv"); }}>{dataTypes.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</Select></Field><Field label="Формат / адаптер" hint="WHOOP, FatSecret и InBody — файловые источники, не статус постоянного подключения."><Select value={adapter} onChange={(event) => setAdapter(event.target.value)}>{adaptersForDataType(dataType).map((candidate) => <option key={candidate.value} value={candidate.value}>{candidate.label}</option>)}</Select></Field><div className="flex justify-end"><Button variant="primary" onClick={() => setStep(2)}>Продолжить<ChevronRight className="size-4" /></Button></div></div>}
    {step === 2 && <div className="space-y-4"><Field label="CSV или JSON файл"><Input type="file" accept=".csv,.json,text/csv,application/json" onChange={(event) => { setFile(event.target.files?.[0] ?? null); setContent(""); }} /></Field>{file && <p className="rounded-control border border-line bg-white/[.025] p-3 text-sm text-muted">{file.name} · {formatNumber(file.size / 1024)} КБ</p>}<InlineError error={previewMutation.error} /><div className="flex justify-between"><Button variant="ghost" onClick={() => setStep(1)}><ChevronLeft className="size-4" />Назад</Button><Button variant="primary" loading={previewMutation.isPending} disabled={!file} onClick={() => previewMutation.mutate({ selectedMapping: {}, nextStep: 3 })}>Загрузить и проверить</Button></div></div>}
    {step === 3 && preview && <div className="space-y-4"><SectionHeader title="Сопоставление колонок" description="Обязательные поля отмечены звёздочкой; необязательные можно сопоставить или оставить пустыми. После выбора FitLog повторно вызывает preview." /><div className="grid gap-3 sm:grid-cols-2">{targetFields.map((field) => <Field key={field} label={`${field}${required.includes(field) ? " *" : ""}`}><Select value={mapping[field] ?? ""} onChange={(event) => setMapping((current) => ({ ...current, [field]: event.target.value }))}><option value="">Не сопоставлено</option>{available.map((column) => <option key={column} value={column}>{column}</option>)}</Select></Field>)}</div><InlineError error={previewMutation.error} /><div className="flex justify-between"><Button variant="ghost" onClick={() => setStep(2)}><ChevronLeft className="size-4" />Назад</Button><Button variant="primary" loading={previewMutation.isPending} disabled={required.some((field) => !mapping[field])} onClick={() => previewMutation.mutate({ selectedMapping: mapping, nextStep: 4 })}>Проверить строки<ChevronRight className="size-4" /></Button></div></div>}
    {step === 4 && preview && <div className="space-y-4"><div className="grid gap-3 sm:grid-cols-4">{[["Всего", preview.total_rows], ["Валидно", preview.valid_rows], ["Дубли", preview.duplicate_rows], ["Ошибки", preview.invalid_rows]].map(([label, value]) => <Card key={String(label)} className="p-4"><p className="text-xs text-muted">{label}</p><p className="mt-2 text-2xl font-semibold">{formatNumber(value as number | undefined)}</p></Card>)}</div>{preview.rows?.length ? <div><SectionHeader title="Предпросмотр валидных строк" description="Первые 10 строк после mapping и нормализации." className="mb-2" /><div className="max-h-64 overflow-auto rounded-control border border-line"><table className="w-full min-w-max text-left text-xs"><thead className="sticky top-0 bg-elevated"><tr>{targetFields.map((field) => <th key={field} className="border-b border-line px-3 py-2 font-semibold text-muted">{field}</th>)}</tr></thead><tbody>{preview.rows.map((row, rowIndex) => <tr key={rowIndex} className="border-b border-line/70 last:border-0">{targetFields.map((field) => <td key={field} className="max-w-56 truncate px-3 py-2">{String(row[field] ?? "—") || "—"}</td>)}</tr>)}</tbody></table></div></div> : null}{preview.errors?.length ? <div className="max-h-48 overflow-y-auto rounded-control border border-critical/20 bg-critical/[.035] p-3">{preview.errors.slice(0, 30).map((error, index) => <p key={index} className="text-xs leading-6 text-critical">Строка {error.row ?? "—"}{Object.keys(error.fields ?? {}).length ? ` · ${Object.entries(error.fields ?? {}).map(([field, message]) => `${field}: ${message}`).join("; ")}` : error.field ? ` · ${error.field}` : ""}: {error.message}</p>)}</div> : <p className="flex items-center gap-2 rounded-control border border-accent/15 bg-accent/[.04] p-3 text-sm text-accent"><Check className="size-4" />Ошибок валидации нет</p>}<p className="text-xs leading-5 text-muted">Строки со стабильным внешним ID, уже присутствующим в FitLog, будут безопасно пропущены как дубли.</p><div className="flex justify-between"><Button variant="ghost" onClick={() => setStep(3)}><ChevronLeft className="size-4" />Mapping</Button><Button variant="primary" disabled={(preview.valid_rows ?? 0) === 0} onClick={() => setStep(5)}>К подтверждению<ChevronRight className="size-4" /></Button></div></div>}
    {step === 5 && preview && <div className="space-y-4"><div className="flex gap-3 rounded-control border border-warning/20 bg-warning/[.04] p-4"><AlertTriangle className="size-5 shrink-0 text-warning" /><div><h3 className="text-sm font-semibold">Подтвердите запись в базу</h3><p className="mt-1 text-sm leading-6 text-muted">Будет обработано {formatNumber(preview.valid_rows)} валидных строк. Обнаруженные дубли будут пропущены. Операция попадёт в журнал.</p></div></div><InlineError error={execute.error} /><div className="flex justify-between"><Button variant="ghost" onClick={() => setStep(4)}><ChevronLeft className="size-4" />Назад</Button><Button variant="primary" loading={execute.isPending} onClick={() => execute.mutate()}>Выполнить импорт</Button></div></div>}
    {step === 6 && result && <div className="space-y-5 text-center"><span className={`mx-auto grid size-14 place-items-center rounded-full border ${importFailed ? "border-critical/20 bg-critical/10" : "border-accent/20 bg-accent/10"}`}>{importFailed ? <AlertTriangle className="size-6 text-critical" /> : <Check className="size-6 text-accent" />}</span><div><h3 className="text-lg font-semibold">{importFailed ? "Импорт не выполнен" : "Импорт завершён"}</h3><p className="mt-1 text-sm text-muted">Импортировано {formatNumber(result.imported_rows)}, пропущено {formatNumber(result.skipped_rows)}, ошибок {formatNumber(result.failed_rows)}.</p></div>{result.errors?.length ? <div className="max-h-40 overflow-y-auto rounded-control border border-warning/20 bg-warning/[.04] p-3 text-left">{result.errors.slice(0, 30).map((error, index) => <p key={index} className="text-xs leading-6 text-warning">Строка {error.row ?? "—"}: {error.message}{Object.keys(error.fields ?? {}).length ? ` · ${Object.entries(error.fields ?? {}).map(([field, message]) => `${field}: ${message}`).join("; ")}` : ""}</p>)}</div> : result.error_summary ? <p className="rounded-control border border-warning/20 bg-warning/[.04] p-3 text-sm text-warning">{result.error_summary}</p> : null}<div className="flex justify-center gap-2"><Button onClick={start}><RotateCcw className="size-4" />Ещё импорт</Button><Button variant="primary" onClick={() => setOpen(false)}>Готово</Button></div></div>}
  </Dialog><Dialog open={Boolean(selectedRun)} onOpenChange={(value) => { if (!value) setSelectedRun(null); }} title="Детали импорта" description={detailedRun?.filename ?? "Операция импорта"} className="sm:max-w-2xl">
    {runDetails.isError ? <ErrorState error={runDetails.error} retry={() => runDetails.refetch()} /> : detailedRun && <div className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {[["Источник", detailedRun.source ?? "—"], ["Тип", detailedRun.data_type ?? "—"], ["Статус", detailedRun.status ?? "—"], ["Всего строк", formatNumber(detailedRun.total_rows)], ["Начат", formatDate(detailedRun.started_at)], ["Завершён", formatDate(detailedRun.completed_at)]].map(([label, value]) => <Card key={String(label)} className="p-3"><p className="text-xs text-muted">{label}</p><p className="mt-1 text-sm font-semibold">{value}</p></Card>)}
      </div>
      <div className="grid grid-cols-3 gap-3">{[["Импортировано", detailedRun.imported_rows], ["Пропущено", detailedRun.skipped_rows], ["С ошибкой", detailedRun.failed_rows]].map(([label, value]) => <div key={String(label)} className="rounded-control border border-line p-3 text-center"><p className="text-xs text-muted">{label}</p><p className="mt-1 text-xl font-semibold">{formatNumber(value as number | null)}</p></div>)}</div>
      {detailedRun.errors?.length ? <div className="max-h-72 overflow-y-auto rounded-control border border-warning/20 bg-warning/[.04] p-3">{detailedRun.errors.map((error, index) => <p key={index} className="text-xs leading-6 text-warning">Строка {error.row ?? "—"}: {error.message}{Object.keys(error.fields ?? {}).length ? ` · ${Object.entries(error.fields ?? {}).map(([field, message]) => `${field}: ${message}`).join("; ")}` : ""}</p>)}</div> : runDetails.isPending ? <p className="rounded-control border border-line p-3 text-sm text-muted">Загрузка сохранённых ошибок…</p> : <p className="rounded-control border border-accent/15 bg-accent/[.04] p-3 text-sm text-accent">Сохранённых ошибок по строкам нет.</p>}
      {(detailedRun.failed_rows ?? 0) > (detailedRun.errors?.length ?? 0) && <p className="text-xs text-muted">Показаны первые {formatNumber(detailedRun.errors?.length ?? 0)} ошибок из {formatNumber(detailedRun.failed_rows)}; полное число строк сохранено в счётчике.</p>}
      <div className="flex justify-end"><Button variant="primary" onClick={() => setSelectedRun(null)}>Закрыть</Button></div>
    </div>}
  </Dialog></>;
}

export default function ImportsPage() { return <Suspense fallback={<PageSkeleton />}><ImportsContent /></Suspense>; }
