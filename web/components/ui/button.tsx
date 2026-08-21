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
    "inline-flex shrink-0 items-center justify-center gap-2 rounded-control border text-sm font-medium transition duration-150 disabled:pointer-events-none disabled:opacity-45",
    variant === "primary" && "border-accent/30 bg-accent text-[#07120c] hover:bg-[#98ffc3]",
    variant === "secondary" && "border-line bg-elevated text-ink hover:border-white/15 hover:bg-white/[.06]",
    variant === "ghost" && "border-transparent bg-transparent text-muted hover:bg-white/[.05] hover:text-ink",
    variant === "danger" && "border-critical/25 bg-critical/10 text-critical hover:bg-critical/15",
    size === "sm" && "h-9 px-3",
    size === "md" && "h-10 px-4",
    size === "icon" && "size-10 p-0",
    className,
  )} {...props}>{loading && <LoaderCircle aria-hidden className="size-4 animate-spin" />}{children}</button>;
});
