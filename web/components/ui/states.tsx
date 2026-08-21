"use client";

import { AlertTriangle, DatabaseZap, RefreshCw } from "lucide-react";
import { APIError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { cn } from "@/lib/utils";

export function Skeleton({ className }: { className?: string }) { return <div aria-hidden className={cn("animate-pulse rounded-lg bg-white/[.06]", className)} />; }

export function PageSkeleton() {
  return <div aria-busy="true" aria-label="Загрузка" className="space-y-5"><div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">{Array.from({ length: 4 }).map((_, index) => <Skeleton key={index} className="h-32" />)}</div><Skeleton className="h-80" /><Skeleton className="h-56" /></div>;
}

export function EmptyState({ title = "Пока нет данных", description = "Добавьте первую запись или измените выбранный период.", action }: { title?: string; description?: string; action?: React.ReactNode }) {
  return <div className="flex min-h-40 flex-col items-center justify-center px-5 py-10 text-center"><div className="mb-3 rounded-full border border-line bg-white/[.035] p-3"><DatabaseZap className="size-5 text-muted" /></div><h3 className="text-sm font-semibold text-ink">{title}</h3><p className="mt-1 max-w-md text-sm leading-6 text-muted">{description}</p>{action && <div className="mt-4">{action}</div>}</div>;
}

export function ErrorState({ error, retry, title = "Не удалось загрузить данные" }: { error: unknown; retry?: () => void; title?: string }) {
  const message = error instanceof APIError ? error.message : error instanceof Error ? error.message : "Неизвестная ошибка";
  return <Card className="border-critical/20 bg-critical/[.045]"><div role="alert" className="flex min-h-40 flex-col items-center justify-center px-5 py-9 text-center"><AlertTriangle className="mb-3 size-5 text-critical" /><h3 className="text-sm font-semibold">{title}</h3><p className="mt-1 max-w-xl text-sm text-muted">{message}</p>{retry && <Button className="mt-4" onClick={retry}><RefreshCw className="size-4" />Повторить</Button>}</div></Card>;
}

export function InlineError({ error }: { error: unknown }) {
  if (!error) return null;
  const message = error instanceof Error ? error.message : "Не удалось выполнить действие";
  const fields = error instanceof APIError && error.fields ? Object.entries(error.fields).flatMap(([field, value]) => {
    const messages = Array.isArray(value) ? value : [value];
    return messages.map((item) => `${field}: ${item}`);
  }) : [];
  return <div role="alert" className="rounded-control border border-critical/20 bg-critical/10 px-3 py-2 text-sm text-critical"><p>{message}</p>{fields.length > 0 && <ul className="mt-1 list-disc space-y-0.5 pl-5 text-xs">{fields.map((field) => <li key={field}>{field}</li>)}</ul>}</div>;
}
