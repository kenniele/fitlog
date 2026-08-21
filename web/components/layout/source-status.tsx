"use client";

import { useQuery } from "@tanstack/react-query";
import { formatDistanceToNow, parseISO } from "date-fns";
import { ru } from "date-fns/locale";
import { CloudOff, RefreshCw } from "lucide-react";
import { apiFetch, listItems, type ListResponse } from "@/lib/api";
import type { SourceStatus as Status } from "@/lib/types";
import { Badge } from "@/components/ui/badge";

function ago(value?: string | null) {
  if (!value) return "не обновлялось";
  try { return formatDistanceToNow(parseISO(value), { addSuffix: true, locale: ru }); } catch { return "время неизвестно"; }
}

export function SourceStatus() {
  const query = useQuery({ queryKey: ["sources"], queryFn: () => apiFetch<ListResponse<Status> | Status[]>("/sources") });
  const sources = listItems(query.data);
  if (query.isPending) return <span className="flex items-center gap-2 text-xs text-muted"><RefreshCw className="size-3 animate-spin" />Источники</span>;
  if (query.isError || !sources.length) return <Badge><CloudOff className="mr-1 size-3" />Нет статуса источников</Badge>;
  return <div className="hidden items-center gap-2 2xl:flex">{sources.slice(0, 3).map((source) => <Badge key={source.source} tone={source.connected === true ? "good" : source.last_error ? "critical" : "neutral"}><span className={source.connected === true ? "mr-1 size-1.5 rounded-full bg-accent" : "mr-1 size-1.5 rounded-full bg-muted"} />{source.label || source.source} · {source.connected === true ? ago(source.last_synced_at) : source.status === "manual_sync" ? `ручной · ${ago(source.last_synced_at)}` : "не подключён"}</Badge>)}</div>;
}
