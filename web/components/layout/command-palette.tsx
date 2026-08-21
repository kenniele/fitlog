"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { ArrowRight, Search } from "lucide-react";
import { Dialog } from "@/components/ui/dialog";
import { Input } from "@/components/ui/field";
import { apiFetch, listItems, type ListResponse } from "@/lib/api";
import type { Exercise } from "@/lib/types";
import { navigation, quickActions } from "@/lib/navigation";
import { useDebouncedValue } from "@/lib/hooks";

export function CommandPalette({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 300);
  const router = useRouter();
  const queryReady = open && search === debouncedSearch && debouncedSearch.trim().length >= 2;
  const query = useQuery({ queryKey: ["command-search", debouncedSearch], queryFn: () => apiFetch<ListResponse<Exercise>>(`/exercises?search=${encodeURIComponent(debouncedSearch)}&page=1&page_size=6`), enabled: queryReady });
  useEffect(() => { if (!open) setSearch(""); }, [open]);
  const actions = useMemo(() => [...navigation.map((item) => ({ href: item.href, label: item.label, group: "Навигация" })), ...quickActions.map((item) => ({ ...item, group: "Быстрые действия" }))].filter((item) => item.label.toLowerCase().includes(search.toLowerCase())), [search]);
  const exerciseResults = queryReady ? listItems(query.data) : [];
  const go = (href: string) => { onOpenChange(false); router.push(href); };
  return <Dialog open={open} onOpenChange={onOpenChange} title="Поиск и команды" description="Перейдите в раздел или выполните быстрое действие." className="sm:max-w-xl"><div className="relative"><Search className="pointer-events-none absolute left-3 top-3 size-4 text-muted" /><Input data-dialog-autofocus value={search} onChange={(event) => setSearch(event.target.value)} className="pl-9" placeholder="Например: питание, тренировка…" aria-label="Поиск команд" /></div><div className="mt-4 max-h-[55vh] space-y-1 overflow-y-auto scrollbar-thin">{actions.map((item) => <button key={`${item.group}-${item.href}`} onClick={() => go(item.href)} className="flex w-full items-center justify-between rounded-control px-3 py-2.5 text-left transition hover:bg-white/[.05]"><span><span className="block text-sm text-ink">{item.label}</span><span className="text-[11px] text-muted">{item.group}</span></span><ArrowRight className="size-4 text-muted" /></button>)}{exerciseResults.map((exercise) => <button key={`exercise-${exercise.id}`} onClick={() => go(`/dashboard/training?exercise_id=${exercise.id}`)} className="flex w-full items-center justify-between rounded-control px-3 py-2.5 text-left transition hover:bg-white/[.05]"><span><span className="block text-sm text-ink">{exercise.name}</span><span className="text-[11px] text-muted">Упражнение</span></span><ArrowRight className="size-4 text-muted" /></button>)}{!actions.length && !query.isFetching && !exerciseResults.length && <p className="py-8 text-center text-sm text-muted">Ничего не найдено</p>}</div></Dialog>;
}
