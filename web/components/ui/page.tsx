import { cn } from "@/lib/utils";

export function PageHeader({ eyebrow, title, description, actions }: { eyebrow?: string; title: string; description?: string; actions?: React.ReactNode }) {
  return <header className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between"><div className="min-w-0">{eyebrow && <p className="mb-1 eyebrow">{eyebrow}</p>}<h1 className="text-2xl font-medium tracking-[-.035em] text-ink sm:text-3xl">{title}</h1>{description && <p className="mt-1 max-w-2xl text-sm leading-6 text-muted">{description}</p>}</div>{actions && <div className="flex shrink-0 flex-wrap gap-2">{actions}</div>}</header>;
}

export function SectionHeader({ title, description, actions, className }: { title: string; description?: string; actions?: React.ReactNode; className?: string }) {
  return <div className={cn("flex flex-wrap items-start justify-between gap-3", className)}><div><h2 className="text-base font-medium text-ink">{title}</h2>{description && <p className="mt-1 text-xs leading-5 text-muted">{description}</p>}</div>{actions}</div>;
}
