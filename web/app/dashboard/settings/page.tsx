"use client";

import Link from "next/link";
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Download, FileUp, Trash2 } from "lucide-react";
import { apiFetch, downloadFromAPI, listItems, type ListResponse } from "@/lib/api";
import type { Settings, SourceStatus } from "@/lib/types";
import { PageHeader, SectionHeader } from "@/components/ui/page";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { ErrorState, InlineError, PageSkeleton } from "@/components/ui/states";
import { SettingsForm } from "@/components/forms/settings-form";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { formatDate } from "@/lib/format";

export default function SettingsPage() {
  const client = useQueryClient(); const [deleteAll, setDeleteAll] = useState(false);
  const settings = useQuery({ queryKey: ["settings"], queryFn: () => apiFetch<Settings>("/settings") });
  const sources = useQuery({ queryKey: ["sources"], queryFn: () => apiFetch<ListResponse<SourceStatus> | SourceStatus[]>("/sources") });
  const deleteMutation = useMutation({
    mutationFn: async () => {
      await apiFetch("/settings/data", { method: "DELETE", body: { confirmation: "DELETE MY DATA" } });
    },
    onSuccess: () => { client.clear(); location.assign("/dashboard/login"); },
  });
  const exportMutation = useMutation({
    mutationFn: () => downloadFromAPI("/api/v1/export?type=all", `fitlog-export-${new Date().toISOString().slice(0, 10)}.json`),
  });
  if (settings.isError || sources.isError) return <ErrorState error={settings.error ?? sources.error} retry={() => { void Promise.all([settings.refetch(), sources.refetch()]); }} />;
  if (settings.isPending || sources.isPending) return <PageSkeleton />;
  return <><PageHeader eyebrow="Settings" title="Настройки Control Center" description="Timezone, цели, метрические единицы и управление данными." /><div className="grid gap-4 xl:grid-cols-[1.4fr_1fr]"><Card className="p-5"><SectionHeader title="Персональные настройки" description="Изменения сохраняются через API и сразу влияют на аналитику." className="mb-5" /><SettingsForm settings={settings.data} /></Card><div className="space-y-4"><Card className="p-5"><SectionHeader title="Источники данных" description="Connected показывается только при connected: true. Никаких фиктивных синхронизаций." className="mb-4" /><div className="space-y-3">{listItems(sources.data).map((source) => <div key={source.source} className="rounded-control border border-line bg-canvas/35 p-3"><div className="flex items-center justify-between gap-2"><div><p className="text-sm font-medium">{source.label ?? source.source}</p><p className="mt-1 text-xs text-muted">Последнее обновление: {formatDate(source.last_synced_at, "dd.MM.yyyy HH:mm")}</p></div><Badge tone={source.connected === true ? "good" : source.last_error ? "critical" : "neutral"}>{source.connected === true ? "Connected" : source.status === "provider_sync" || source.status === "manual_sync" ? "API-синк" : source.status === "file_import_only" ? "Только file import" : "Не подключён"}</Badge></div>{source.last_error && <p className="mt-2 text-xs text-critical">{source.last_error}</p>}</div>)}</div><Link href="/dashboard/imports?action=new" className="mt-4 inline-flex h-9 items-center gap-2 rounded-control border border-line bg-elevated px-3 text-sm font-medium text-ink hover:border-white/15"><FileUp className="size-4" />Импортировать файл</Link></Card><Card className="p-5"><SectionHeader title="Ваши данные" description="Экспорт выполняется сервером; удаление требует точной фразы." className="mb-4" /><div className="flex flex-wrap gap-2"><Button onClick={() => exportMutation.mutate()} loading={exportMutation.isPending}><Download className="size-4" />Скачать экспорт</Button><Button variant="danger" onClick={() => setDeleteAll(true)}><Trash2 className="size-4" />Удалить все данные</Button></div><InlineError error={exportMutation.error} /><InlineError error={deleteMutation.error} /></Card></div></div><ConfirmDialog open={deleteAll} onOpenChange={setDeleteAll} title="Удалить все данные FitLog?" description="Это необратимое действие удалит тренировки, питание, восстановление, сон и измерения." confirmText="Удалить навсегда" exactText="DELETE MY DATA" onConfirm={() => deleteMutation.mutate()} busy={deleteMutation.isPending} error={deleteMutation.error} /></>;
}
