"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import { Command, LogOut, Menu, Plus, Search } from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { apiFetch } from "@/lib/api";
import { cn } from "@/lib/utils";
import { navigation, quickActions } from "@/lib/navigation";
import { DateRangeControls } from "@/components/layout/date-range-controls";
import { SourceStatus } from "@/components/layout/source-status";
import { CommandPalette } from "@/components/layout/command-palette";
import { AuthBoundary } from "@/components/layout/auth-boundary";
import type { Settings } from "@/lib/types";
import { setDashboardTimezone } from "@/lib/format";
import { ErrorState, InlineError, PageSkeleton } from "@/components/ui/states";
import { Dialog } from "@/components/ui/dialog";

function Brand() { return <Link href="/dashboard" className="flex items-center gap-2.5"><span className="grid size-8 place-items-center rounded-[10px] border border-accent/20 bg-accent/10 text-sm font-black text-accent">F</span><span><span className="block text-sm font-semibold tracking-tight text-ink">FitLog</span><span className="block text-[9px] font-semibold uppercase tracking-[.16em] text-muted">Control Center</span></span></Link>; }

export function DashboardShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [drawer, setDrawer] = useState(false);
  const [palette, setPalette] = useState(false);
  const [appliedTimezone, setAppliedTimezone] = useState<string | null>(null);
  const login = pathname === "/dashboard/login";
  const rangeAware = ["/dashboard", "/dashboard/training", "/dashboard/recovery", "/dashboard/nutrition", "/dashboard/body", "/dashboard/analytics"].includes(pathname);
  const preferences = useQuery({ queryKey: ["settings"], queryFn: () => apiFetch<Settings>("/settings"), enabled: !login });
  const logout = useMutation({ mutationFn: () => apiFetch<unknown>("/auth/session", { method: "DELETE" }), onSuccess: () => { queryClient.clear(); router.replace("/dashboard/login"); } });
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => { if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") { event.preventDefault(); setPalette(true); } };
    window.addEventListener("keydown", onKey); return () => window.removeEventListener("keydown", onKey);
  }, []);
  useEffect(() => {
    if (!preferences.data) return;
    const profileTimezone = preferences.data.timezone || "UTC";
    document.documentElement.dataset.theme = preferences.data?.theme === "light" || preferences.data?.theme === "system" ? preferences.data.theme : "dark";
    setDashboardTimezone(profileTimezone);
    setAppliedTimezone(profileTimezone);
  }, [preferences.data]);
  useEffect(() => setDrawer(false), [pathname]);
  if (login) return <AuthBoundary>{children}</AuthBoundary>;
  if (preferences.isError) return <AuthBoundary><main className="mx-auto max-w-3xl p-6"><ErrorState error={preferences.error} retry={() => { void preferences.refetch(); }} title="Не удалось загрузить настройки" /></main></AuthBoundary>;
  if (preferences.isPending || appliedTimezone !== (preferences.data.timezone || "UTC")) return <AuthBoundary><main className="mx-auto max-w-7xl p-6"><PageSkeleton /></main></AuthBoundary>;
  const navigationContent = <><nav aria-label="Основная навигация" className="flex-1 space-y-1 px-2 py-3">{navigation.map((item) => { const active = item.exact ? pathname === item.href : pathname.startsWith(item.href); const Icon = item.icon; return <Link key={item.href} href={item.href} className={cn("flex h-10 items-center gap-3 rounded-control px-3 text-sm font-medium transition", active ? "bg-white/[.07] text-ink shadow-[inset_2px_0_var(--accent)]" : "text-muted hover:bg-white/[.035] hover:text-ink")}><Icon className={cn("size-[17px]", active && "text-accent")} /><span>{item.label}</span></Link>; })}</nav><div className="space-y-2 border-t border-line p-2"><Button variant="ghost" className="w-full justify-start" onClick={() => logout.mutate()} loading={logout.isPending}><LogOut className="size-4" />Выйти</Button><InlineError error={logout.error} /></div></>;
  const desktopSidebar = <><div className="flex h-16 items-center px-4"><Brand /></div>{navigationContent}</>;
  return <AuthBoundary><div className="min-h-screen"><aside className="fixed inset-y-0 left-0 z-40 hidden w-[218px] flex-col border-r border-line bg-surface/75 backdrop-blur-xl lg:flex">{desktopSidebar}</aside><Dialog open={drawer} onOpenChange={setDrawer} title="FitLog Control Center" description="Навигация" placement="left" className="flex flex-col lg:hidden" contentClassName="flex min-h-0 flex-1 flex-col p-0">{navigationContent}</Dialog><div className="min-w-0 lg:pl-[218px]"><header className="sticky top-0 z-30 border-b border-line bg-canvas/82 backdrop-blur-xl"><div className="flex min-h-16 items-center gap-2 px-3 sm:px-5"><Button variant="ghost" size="icon" className="lg:hidden" onClick={() => setDrawer(true)} aria-label="Открыть меню"><Menu className="size-5" /></Button><div className="lg:hidden"><Brand /></div><div className="ml-auto hidden min-w-0 items-center gap-3 lg:flex"><SourceStatus /><button onClick={() => setPalette(true)} className="flex h-9 min-w-[170px] items-center gap-2 rounded-control border border-line bg-white/[.025] px-3 text-xs text-muted transition hover:border-white/15 hover:text-ink"><Search className="size-3.5" />Поиск<span className="ml-auto flex items-center gap-0.5 rounded border border-line px-1.5 py-0.5 text-[10px]"><Command className="size-2.5" />K</span></button><details className="group relative"><summary className="flex h-10 cursor-pointer list-none items-center gap-2 rounded-control border border-line bg-elevated px-4 text-sm font-medium text-ink transition hover:border-white/15 hover:bg-white/[.06]"><Plus className="size-4" />Добавить</summary><div className="absolute right-0 top-11 z-50 w-56 rounded-control border border-line bg-elevated p-1.5 shadow-2xl">{quickActions.map((action) => <Link key={action.href} href={action.href} className="block rounded-lg px-3 py-2 text-sm text-muted hover:bg-white/[.05] hover:text-ink">{action.label}</Link>)}</div></details></div><Button variant="ghost" size="icon" className="ml-auto lg:hidden" onClick={() => setPalette(true)} aria-label="Поиск"><Search className="size-4" /></Button></div>{rangeAware && <div className="border-t border-line px-3 py-2 sm:px-5"><Suspense fallback={<div className="h-9" />}><DateRangeControls /></Suspense></div>}</header><main className="min-w-0 p-3 sm:p-5 lg:p-6"><div className="mx-auto w-full max-w-[1600px] space-y-6">{children}</div></main></div><CommandPalette open={palette} onOpenChange={setPalette} /></div></AuthBoundary>;
}
