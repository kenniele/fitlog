import { cn } from "@/lib/utils";

export function Badge({ tone = "neutral", children, className }: { tone?: "neutral" | "good" | "warning" | "critical" | "blue"; children: React.ReactNode; className?: string }) {
  return <span className={cn(
    "inline-flex items-center rounded-md border px-2 py-0.5 text-[11px] font-medium",
    tone === "neutral" && "border-transparent bg-elevated text-muted",
    tone === "good" && "border-accent/0 bg-accent/10 text-accent",
    tone === "warning" && "border-warning/0 bg-warning/10 text-warning",
    tone === "critical" && "border-critical/0 bg-critical/10 text-critical",
    tone === "blue" && "border-blue/0 bg-blue/10 text-blue",
    className,
  )}>{children}</span>;
}
