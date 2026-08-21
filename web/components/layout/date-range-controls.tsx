"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { CalendarDays } from "lucide-react";
import { Input, Checkbox } from "@/components/ui/field";
import { cn } from "@/lib/utils";
import { rangeFromParams, type RangePreset } from "@/lib/range";
import { dateInTimeZone, getDashboardTimezone, shiftISODate } from "@/lib/format";

const options: Array<{ value: RangePreset; label: string }> = [
  { value: "7", label: "7д" }, { value: "30", label: "30д" }, { value: "90", label: "90д" }, { value: "180", label: "180д" }, { value: "custom", label: "Период" },
];

export function DateRangeControls() {
  const pathname = usePathname();
  const router = useRouter();
  const search = useSearchParams();
  const state = rangeFromParams(search);
  const update = (changes: Partial<typeof state>) => {
    const next = { ...state, ...changes };
    const params = new URLSearchParams(search.toString());
    params.set("range", next.range); params.set("from", next.from); params.set("to", next.to);
    if (next.compare) params.set("compare", "1"); else params.delete("compare");
    router.replace(`${pathname}?${params.toString()}`, { scroll: false });
  };
  const preset = (value: RangePreset) => {
    if (value === "custom") return update({ range: value });
    const to = dateInTimeZone(new Date(), getDashboardTimezone());
    update({ range: value, to, from: shiftISODate(to, -(Number(value) - 1)) });
  };
  return <div className="flex min-w-0 flex-wrap items-center gap-2"><div className="flex rounded-control border border-line bg-canvas/50 p-1" aria-label="Диапазон дат">{options.map((item) => <button key={item.value} onClick={() => preset(item.value)} className={cn("h-7 rounded-lg px-2.5 text-xs font-medium transition", state.range === item.value ? "bg-white/[.09] text-ink" : "text-muted hover:text-ink")}>{item.label}</button>)}</div>{state.range === "custom" && <div className="flex items-center gap-1"><CalendarDays className="size-4 text-muted" /><Input aria-label="Начало периода" type="date" value={state.from} max={state.to} onChange={(event) => update({ from: event.target.value })} className="h-9 w-[138px] text-xs" /><span className="text-muted">—</span><Input aria-label="Конец периода" type="date" value={state.to} min={state.from} onChange={(event) => update({ to: event.target.value })} className="h-9 w-[138px] text-xs" /></div>}<Checkbox label="Сравнить" checked={state.compare} onChange={(event) => update({ compare: event.target.checked })} className="ml-1" /></div>;
}
