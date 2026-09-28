"use client";

import { AlertTriangle, ScanLine, RefreshCw } from "lucide-react";
import { APIError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export function Skeleton({ className }: { className?: string }) {
  return (
    <div
      aria-hidden
      className={cn("animate-pulse rounded-control bg-ink/[.06]", className)}
    />
  );
}

export function PageSkeleton() {
  return (
    <div aria-busy="true" aria-label="Загрузка" className="space-y-8">
      <Skeleton className="h-8 w-52" />
      <div className="grid grid-cols-2 gap-5 lg:grid-cols-4">
        {Array.from({ length: 4 }).map((_, index) => (
          <Skeleton key={index} className="h-20" />
        ))}
      </div>
      <Skeleton className="h-48" />
      <Skeleton className="h-64" />
    </div>
  );
}

export function EmptyState({
  title = "Пока нет данных",
  description = "Добавьте первую запись или измените выбранный период.",
  action,
}: {
  title?: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex min-h-44 flex-col items-center justify-center px-5 py-10 text-center">
      <div className="mb-4 text-accent">
        <ScanLine className="size-5 text-muted" />
      </div>
      <h3 className="text-base font-medium text-ink">{title}</h3>
      <p className="mt-1 max-w-md text-sm leading-6 text-muted">
        {description}
      </p>
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}

export function ErrorState({
  error,
  retry,
  title = "Не удалось загрузить данные",
}: {
  error: unknown;
  retry?: () => void;
  title?: string;
}) {
  const message =
    error instanceof APIError
      ? error.message
      : error instanceof Error
        ? error.message
        : "Неизвестная ошибка";
  return (
    <div
      role="alert"
      className="flex flex-wrap items-center gap-4 rounded-control bg-critical/[.045] px-5 py-5"
    >
      <AlertTriangle className="size-5 shrink-0 text-critical" />
      <div className="min-w-40 flex-1">
        <h3 className="text-sm font-medium">{title}</h3>
        <p className="mt-1 text-sm leading-5 text-muted">{message}</p>
      </div>
      {retry && (
        <Button variant="ghost" onClick={retry}>
          <RefreshCw className="size-4" />
          Повторить
        </Button>
      )}
    </div>
  );
}

export function InlineError({ error }: { error: unknown }) {
  if (!error) return null;
  const message =
    error instanceof Error ? error.message : "Не удалось выполнить действие";
  const fields =
    error instanceof APIError && error.fields
      ? Object.entries(error.fields).flatMap(([field, value]) => {
          const messages = Array.isArray(value) ? value : [value];
          return messages.map((item) => `${field}: ${item}`);
        })
      : [];
  return (
    <div
      role="alert"
      className="rounded-control border border-critical/20 bg-critical/10 px-3 py-2 text-sm text-critical"
    >
      <p>{message}</p>
      {fields.length > 0 && (
        <ul className="mt-1 list-disc space-y-0.5 pl-5 text-xs">
          {fields.map((field) => (
            <li key={field}>{field}</li>
          ))}
        </ul>
      )}
    </div>
  );
}
