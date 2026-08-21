import { forwardRef } from "react";
import { cn } from "@/lib/utils";

export const Input = forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(function Input({ className, ...props }, ref) {
  return <input ref={ref} className={cn("h-10 w-full rounded-control border border-line bg-canvas/55 px-3 text-sm text-ink placeholder:text-muted/60 transition hover:border-white/15 focus:border-accent/60 focus:outline-none", className)} {...props} />;
});

export const Textarea = forwardRef<HTMLTextAreaElement, React.TextareaHTMLAttributes<HTMLTextAreaElement>>(function Textarea({ className, ...props }, ref) {
  return <textarea ref={ref} className={cn("min-h-24 w-full resize-y rounded-control border border-line bg-canvas/55 px-3 py-2 text-sm text-ink placeholder:text-muted/60 transition hover:border-white/15 focus:border-accent/60 focus:outline-none", className)} {...props} />;
});

export const Select = forwardRef<HTMLSelectElement, React.SelectHTMLAttributes<HTMLSelectElement>>(function Select({ className, ...props }, ref) {
  return <select ref={ref} className={cn("h-10 w-full rounded-control border border-line bg-canvas/80 px-3 text-sm text-ink transition hover:border-white/15 focus:border-accent/60 focus:outline-none", className)} {...props} />;
});

export function Field({ label, error, hint, children, className }: { label: string; error?: string; hint?: string; children: React.ReactNode; className?: string }) {
  return <label className={cn("grid gap-1.5 text-sm", className)}><span className="font-medium text-ink">{label}</span>{children}{error ? <span role="alert" className="text-xs text-critical">{error}</span> : hint ? <span className="text-xs text-muted">{hint}</span> : null}</label>;
}

export function Checkbox({ label, className, ...props }: React.InputHTMLAttributes<HTMLInputElement> & { label: string }) {
  return <label className={cn("flex cursor-pointer items-center gap-2 text-sm text-muted", className)}><input type="checkbox" className="size-4 rounded border-line bg-canvas accent-[var(--accent)]" {...props} /><span>{label}</span></label>;
}
