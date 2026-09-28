"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import {
  LogOut,
  Menu,
  Plus,
  Search,
  PanelLeftClose,
  PanelLeftOpen,
} from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { apiFetch } from "@/lib/api";
import { navigation } from "@/lib/navigation";
import { DateRangeControls } from "@/components/layout/date-range-controls";
import { SourceStatus } from "@/components/layout/source-status";
import { CommandPalette } from "@/components/layout/command-palette";
import { AuthBoundary } from "@/components/layout/auth-boundary";
import type { Settings } from "@/lib/types";
import { setDashboardTimezone } from "@/lib/format";
import { ErrorState, InlineError, PageSkeleton } from "@/components/ui/states";
import { Dialog } from "@/components/ui/dialog";

export function DashboardShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [expanded, setExpanded] = useState(false);
  const [drawer, setDrawer] = useState(false);
  const [palette, setPalette] = useState(false);
  const [appliedTimezone, setAppliedTimezone] = useState<string | null>(null);
  const login = pathname === "/dashboard/login";
  const rangeAware = [
    "/dashboard/training",
    "/dashboard/recovery",
    "/dashboard/nutrition",
    "/dashboard/body",
    "/dashboard/analytics",
  ].includes(pathname);
  const preferences = useQuery({
    queryKey: ["settings"],
    queryFn: () => apiFetch<Settings>("/settings"),
    enabled: !login,
  });
  const logout = useMutation({
    mutationFn: () => apiFetch<unknown>("/auth/session", { method: "DELETE" }),
    onSuccess: () => {
      queryClient.clear();
      router.replace("/dashboard/login");
    },
  });
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setPalette(true);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);
  useEffect(() => {
    if (!preferences.data) return;
    const profileTimezone = preferences.data.timezone || "UTC";
    document.documentElement.dataset.theme =
      preferences.data?.theme === "light" ||
      preferences.data?.theme === "system"
        ? preferences.data.theme
        : "dark";
    setDashboardTimezone(profileTimezone);
    setAppliedTimezone(profileTimezone);
  }, [preferences.data]);
  useEffect(() => setDrawer(false), [pathname]);
  if (login) return <AuthBoundary>{children}</AuthBoundary>;
  if (preferences.isError)
    return (
      <AuthBoundary>
        <main className="mx-auto max-w-3xl p-6">
          <ErrorState
            error={preferences.error}
            retry={() => {
              void preferences.refetch();
            }}
            title="Не удалось загрузить настройки"
          />
        </main>
      </AuthBoundary>
    );
  if (
    preferences.isPending ||
    appliedTimezone !== (preferences.data.timezone || "UTC")
  )
    return (
      <AuthBoundary>
        <main className="mx-auto max-w-7xl p-6">
          <PageSkeleton />
        </main>
      </AuthBoundary>
    );
  const current = navigation.find((item) =>
    item.exact ? pathname === item.href : pathname.startsWith(item.href),
  );
  const navigationContent = (
    <>
      <nav
        aria-label="Основная навигация"
        className="flex-1 space-y-1 px-3 py-5"
      >
        {navigation.map((item) => {
          const active = item.exact
            ? pathname === item.href
            : pathname.startsWith(item.href);
          const Icon = item.icon;
          return (
            <Link
              key={item.href}
              href={item.href}
              aria-label={item.label}
              aria-current={active ? "page" : undefined}
              className="nav-link"
            >
              <Icon aria-hidden className="size-[18px] shrink-0" />
              <span className="nav-label whitespace-nowrap">{item.label}</span>
            </Link>
          );
        })}
      </nav>
      <div className="border-t border-line p-3">
        <Button
          variant="ghost"
          className="w-full justify-start px-3"
          onClick={() => logout.mutate()}
          loading={logout.isPending}
          aria-label="Выйти"
        >
          <LogOut className="size-[18px] shrink-0" />
          <span className="nav-label">Выйти</span>
        </Button>
        <InlineError error={logout.error} />
      </div>
    </>
  );
  return (
    <AuthBoundary>
      <div className="min-h-screen">
        <a
          href="#main-content"
          className="sr-only z-[100] rounded-control bg-accent p-3 text-[var(--accent-ink)] focus:not-sr-only focus:fixed focus:left-4 focus:top-4"
        >
          К содержимому
        </a>
        <aside
          data-expanded={expanded}
          className="navigation-rail fixed inset-y-0 left-0 z-40 hidden flex-col border-r border-line bg-canvas lg:flex"
        >
          <div className="flex h-[72px] items-center gap-4 px-6">
            <Link
              href="/dashboard"
              aria-label="FitLog — обзор"
              className="text-xl font-semibold text-accent"
            >
              F<span className="nav-label text-ink">itLog</span>
            </Link>
          </div>
          {navigationContent}
          <button
            className="nav-link mx-3 mb-4"
            onClick={() => setExpanded(!expanded)}
            aria-expanded={expanded}
            aria-label={expanded ? "Свернуть навигацию" : "Закрепить навигацию"}
          >
            {expanded ? (
              <PanelLeftClose className="size-[18px] shrink-0" />
            ) : (
              <PanelLeftOpen className="size-[18px] shrink-0" />
            )}
            <span className="nav-label whitespace-nowrap">
              {expanded ? "Свернуть" : "Закрепить"}
            </span>
          </button>
        </aside>
        <Dialog
          open={drawer}
          onOpenChange={setDrawer}
          title="FitLog"
          description="Разделы и данные"
          placement="left"
          className="flex flex-col lg:hidden"
          contentClassName="flex min-h-0 flex-1 flex-col p-0"
        >
          {navigationContent}
        </Dialog>
        <div className="min-w-0 lg:pl-[72px]">
          <header className="sticky top-0 z-30 border-b border-line bg-canvas">
            <div className="mx-auto flex h-[72px] max-w-[1728px] items-center gap-3 px-4 sm:px-8 lg:px-10">
              <Button
                variant="ghost"
                size="icon"
                className="-ml-2 lg:hidden"
                onClick={() => setDrawer(true)}
                aria-label="Открыть меню"
              >
                <Menu className="size-5" />
              </Button>
              <span className="text-sm font-medium">
                FitLog<span className="mx-3 text-muted/50">/</span>
                <span className="font-normal text-muted">
                  {current?.label ?? "Тренировка"}
                </span>
              </span>
              <div className="ml-auto flex items-center gap-2 sm:gap-4">
                <SourceStatus />
                <button
                  onClick={() => setPalette(true)}
                  className="quiet-control"
                  aria-label="Поиск"
                >
                  <Search className="size-4" />
                  <span className="hidden sm:inline">Поиск</span>
                  <kbd className="hidden text-xs text-muted lg:inline">⌘ K</kbd>
                </button>
                <Button
                  onClick={() => setPalette(true)}
                  className="hidden sm:inline-flex"
                >
                  <Plus className="size-4" />
                  Добавить
                </Button>
              </div>
            </div>
            {rangeAware && (
              <div className="border-t border-line px-4 py-2 sm:px-8 lg:px-10">
                <Suspense fallback={<div className="h-11" />}>
                  <DateRangeControls />
                </Suspense>
              </div>
            )}
          </header>
          <main
            id="main-content"
            tabIndex={-1}
            className="min-w-0 px-4 py-7 outline-none sm:px-8 sm:py-10 lg:px-10"
          >
            <div
              key={pathname}
              className="reveal mx-auto w-full max-w-[1648px] space-y-8"
            >
              {children}
            </div>
          </main>
        </div>
        <CommandPalette open={palette} onOpenChange={setPalette} />
      </div>
    </AuthBoundary>
  );
}
