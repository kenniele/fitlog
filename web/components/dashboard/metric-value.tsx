import { metricText, numeric } from "@/lib/dashboard";
import { cn } from "@/lib/utils";

export function MetricValue({
  value,
  metric = "",
  unit = "",
  className,
}: {
  value: unknown;
  metric?: string;
  unit?: string;
  className?: string;
}) {
  if (numeric(value) === null)
    return (
      <span className={cn("flex min-h-[1.1em] items-center", className)}>
        <span className="text-base font-normal tracking-normal text-muted">
          Без данных
        </span>
      </span>
    );
  return (
    <span className={cn("metric-number block font-medium", className)}>
      {metricText(value, metric)}
      {unit && (
        <span className="ml-1.5 text-[.4em] font-normal tracking-normal text-muted">
          {unit}
        </span>
      )}
    </span>
  );
}
