"use client";

import { createContext, forwardRef, useContext, useId } from "react";
import { cn } from "@/lib/utils";

const FieldContext = createContext<{ description?: string; invalid?: boolean }>({});

export const Input = forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(function Input({ className, ...props }, ref) {
  const field = useContext(FieldContext);
  return <input aria-describedby={field.description} aria-invalid={field.invalid || undefined} ref={ref} className={cn("min-h-11 w-full rounded-control border border-line bg-canvas/55 px-3 text-base sm:text-sm text-ink placeholder:text-muted/60 transition hover:border-ink/15 focus:border-accent/60 focus:outline-none", className)} {...props} />;
});

export const Textarea = forwardRef<HTMLTextAreaElement, React.TextareaHTMLAttributes<HTMLTextAreaElement>>(function Textarea({ className, ...props }, ref) {
  const field = useContext(FieldContext);
  return <textarea aria-describedby={field.description} aria-invalid={field.invalid || undefined} ref={ref} className={cn("min-h-24 w-full resize-y rounded-control border border-line bg-canvas/55 px-3 py-2 text-base sm:text-sm text-ink placeholder:text-muted/60 transition hover:border-ink/15 focus:border-accent/60 focus:outline-none", className)} {...props} />;
});

export const Select = forwardRef<HTMLSelectElement, React.SelectHTMLAttributes<HTMLSelectElement>>(function Select({ className, ...props }, ref) {
  const field = useContext(FieldContext);
  return <select aria-describedby={field.description} aria-invalid={field.invalid || undefined} ref={ref} className={cn("min-h-11 w-full rounded-control border border-line bg-canvas/80 px-3 text-base sm:text-sm text-ink transition hover:border-ink/15 focus:border-accent/60 focus:outline-none", className)} {...props} />;
});

export function Field({ label, error, hint, children, className }: { label: string; error?: string; hint?: string; children: React.ReactNode; className?: string }) {
  const descriptionId = useId();
  return <FieldContext.Provider value={{ description: error || hint ? descriptionId : undefined, invalid: Boolean(error) }}><label className={cn("grid gap-1.5 text-sm", className)}><span className="font-medium text-ink">{label}</span>{children}{error ? <span id={descriptionId} role="alert" className="text-xs text-critical">{error}</span> : hint ? <span id={descriptionId} className="text-xs text-muted">{hint}</span> : null}</label></FieldContext.Provider>;
}

export function Checkbox({ label, className, ...props }: React.InputHTMLAttributes<HTMLInputElement> & { label: string }) {
  return <label className={cn("flex cursor-pointer items-center gap-2 text-sm text-muted", className)}><input type="checkbox" className="size-4 rounded border-line bg-canvas accent-[var(--accent)]" {...props} /><span>{label}</span></label>;
}
