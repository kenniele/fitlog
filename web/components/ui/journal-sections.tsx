"use client";

import { useId, useState, type ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { ErrorState, PageSkeleton } from "@/components/ui/states";

export function JournalSections({ journal, analytics }: { journal: ReactNode; analytics: ReactNode }) {
  const [view, setView] = useState<"journal" | "analytics">("journal");
  const id = useId();
  return <section className="space-y-5">
    <div role="group" aria-label="Записи и динамика" className="flex gap-2 border-b border-line pb-3">
      <Button aria-pressed={view === "journal"} aria-controls={id} variant={view === "journal" ? "secondary" : "ghost"} onClick={() => setView("journal")}>Журнал</Button>
      <Button aria-pressed={view === "analytics"} aria-controls={id} variant={view === "analytics" ? "secondary" : "ghost"} onClick={() => setView("analytics")}>Динамика</Button>
    </div>
    <div id={id} className="space-y-6">{view === "journal" ? journal : analytics}</div>
  </section>;
}

export function QuerySection({ query, children, title }: { query: { isPending: boolean; isError: boolean; error: unknown; refetch: () => unknown }; children: ReactNode; title: string }) {
  if (query.isError) return <ErrorState title={title} error={query.error} retry={() => { void query.refetch(); }} />;
  if (query.isPending) return <PageSkeleton />;
  return <>{children}</>;
}
