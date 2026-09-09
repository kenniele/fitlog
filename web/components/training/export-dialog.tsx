"use client";

import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { Download } from "lucide-react";
import { downloadFromAPI } from "@/lib/api";
import { formatDate } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Dialog, DialogActions } from "@/components/ui/dialog";
import { Field, Select } from "@/components/ui/field";
import { InlineError } from "@/components/ui/states";

export function TrainingExport({ filters }: { filters: string }) {
  const [open, setOpen] = useState(false);
  const [format, setFormat] = useState("csv");
  const [scope, setScope] = useState("range");
  const params = new URLSearchParams(filters);
  const from = params.get("from");
  const to = params.get("to");
  const download = useMutation({
    mutationFn: async () => {
      const query = new URLSearchParams(filters);
      query.delete("page");
      query.delete("page_size");
      query.delete("compare");
      query.set("scope", scope);
      if (scope === "all") {
        query.delete("from");
        query.delete("to");
      }
      const period = scope === "all" ? "all" : `${from}_${to}`;
      await downloadFromAPI(`/workout-sessions/export.${format}?${query}`, `fitlog-training-${period}.${format}`);
    },
    onSuccess: () => setOpen(false),
  });

  return <>
    <Button onClick={() => { download.reset(); setOpen(true); }}><Download className="size-4" />Экспорт</Button>
    <Dialog open={open} onOpenChange={setOpen} title="Экспорт тренировок" description="Скачать тренировки с упражнениями и подходами. Текущие фильтры применяются к обоим периодам.">
      <div className="grid gap-4">
        <Field label="Период">
          <Select value={scope} onChange={(event) => setScope(event.target.value)} disabled={download.isPending}>
            <option value="range">Выбранный период: {formatDate(from)} — {formatDate(to)}</option>
            <option value="all">За всё время</option>
          </Select>
        </Field>
        <Field label="Формат" hint={format === "csv" ? "Таблица подходов: даты, упражнения, вес, повторы, RIR, отдых и заметки." : "Вложенные данные тренировок, упражнений и подходов, включая плановые и фактические значения."}>
          <Select value={format} onChange={(event) => setFormat(event.target.value)} disabled={download.isPending}>
            <option value="csv">CSV</option>
            <option value="json">JSON</option>
          </Select>
        </Field>
        <p className="text-xs text-muted">В файл попадут все подходящие тренировки со всех страниц списка.</p>
        <InlineError error={download.error} />
      </div>
      <DialogActions>
        <Button onClick={() => setOpen(false)} disabled={download.isPending}>Отмена</Button>
        <Button variant="primary" onClick={() => download.mutate()} loading={download.isPending}><Download className="size-4" />Скачать {format.toUpperCase()}</Button>
      </DialogActions>
    </Dialog>
  </>;
}
