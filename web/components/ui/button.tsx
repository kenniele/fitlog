import { forwardRef } from "react";
import { LoaderCircle } from "lucide-react";
import { cn } from "@/lib/utils";

type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "ghost" | "danger";
  size?: "sm" | "md" | "icon";
  loading?: boolean;
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { className, variant = "secondary", size = "md", loading, disabled, children, ...props }, ref,
) {
  return <button ref={ref} disabled={disabled || loading} className={cn(
    "inline-flex shrink-0 items-center justify-center gap-2 rounded-control border text-sm font-medium transition-colors duration-150 disabled:pointer-events-none disabled:opacity-45",
    variant === "primary" && "border-accent/30 bg-accent text-[var(--accent-ink)] hover:brightness-110",
    variant === "secondary" && "border-line bg-elevated text-ink hover:border-ink/15 hover:bg-ink/[.06]",
    variant === "ghost" && "border-transparent bg-transparent text-muted hover:bg-ink/[.05] hover:text-ink",
    variant === "danger" && "border-critical/25 bg-critical/10 text-critical hover:bg-critical/15",
    size === "sm" && "min-h-11 px-3 sm:min-h-9",
    size === "md" && "min-h-11 px-4",
    size === "icon" && "size-11 p-0",
    className,
  )} {...props}>{loading && <LoaderCircle aria-hidden className="size-4 animate-spin" />}{children}</button>;
});
