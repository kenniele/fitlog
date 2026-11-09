"use client";

import { useId } from "react";
import { Input } from "@/components/ui/field";

// The API keeps seconds; the editor presents the same value as hours and minutes.
export function DurationInput({ label, value, onChange, error }: { label: string; value?: number; onChange: (seconds: number | undefined) => void; error?: string }) {
  const errorId = useId();
  const hours = value == null ? "" : Math.floor(value / 3600);
  const minutes = value == null ? "" : Math.floor((value % 3600) / 60);
  return <fieldset className="min-w-0 space-y-1.5"><legend className="text-sm font-medium">{label}</legend><div className="flex items-center gap-2">
    <Input aria-label={`${label}: часы`} aria-describedby={error ? errorId : undefined} aria-invalid={Boolean(error)} type="number" min="0" step="1" value={hours} onChange={(event) => { const raw = event.target.value; onChange(raw === "" && !minutes ? undefined : Number(raw) * 3600 + Number(minutes) * 60); }} /><span className="text-sm text-muted">ч</span>
    <Input aria-label={`${label}: минуты`} aria-describedby={error ? errorId : undefined} aria-invalid={Boolean(error)} type="number" min="0" max="59" step="1" value={minutes} onChange={(event) => { const raw = event.target.value; onChange(raw === "" && !hours ? undefined : Number(hours) * 3600 + Number(raw) * 60); }} /><span className="text-sm text-muted">мин</span>
  </div>{error && <p id={errorId} role="alert" className="text-xs text-critical">{error}</p>}</fieldset>;
}
