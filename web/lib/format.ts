let dashboardTimezone = typeof Intl !== "undefined" ? Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC" : "UTC";

export function setDashboardTimezone(value: string | null | undefined) {
  if (!value) return;
  try {
    new Intl.DateTimeFormat("en", { timeZone: value }).format();
    dashboardTimezone = value;
  } catch {
    // The API validates IANA zones. Retain the last known-good client value.
  }
}

export function getDashboardTimezone() { return dashboardTimezone; }

type ZonedParts = { year: string; month: string; day: string; hour: string; minute: string };

function zonedParts(value: Date, timeZone = dashboardTimezone): ZonedParts | null {
  if (Number.isNaN(value.getTime())) return null;
  try {
    const parts = new Intl.DateTimeFormat("en-CA", {
      timeZone, year: "numeric", month: "2-digit", day: "2-digit",
      hour: "2-digit", minute: "2-digit", hourCycle: "h23",
    }).formatToParts(value);
    const map = Object.fromEntries(parts.map((part) => [part.type, part.value]));
    if (!map.year || !map.month || !map.day || !map.hour || !map.minute) return null;
    return { year: map.year, month: map.month, day: map.day, hour: map.hour, minute: map.minute };
  } catch {
    return null;
  }
}

export function dateInTimeZone(value: Date | string = new Date(), timeZone = dashboardTimezone) {
  if (typeof value === "string" && /^\d{4}-\d{2}-\d{2}$/.test(value)) return value;
  const parts = zonedParts(value instanceof Date ? value : new Date(value), timeZone);
  return parts ? `${parts.year}-${parts.month}-${parts.day}` : "";
}

export function dateTimeLocalInTimeZone(value: Date | string = new Date(), timeZone = dashboardTimezone) {
  const parts = zonedParts(value instanceof Date ? value : new Date(value), timeZone);
  return parts ? `${parts.year}-${parts.month}-${parts.day}T${parts.hour}:${parts.minute}` : "";
}

export function shiftISODate(value: string, days: number) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return value;
  const date = new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3])));
  date.setUTCDate(date.getUTCDate() + days);
  return date.toISOString().slice(0, 10);
}

export function daysBetweenISO(left: string, right: string) {
  const parse = (value: string) => {
    const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
    return match ? Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3])) : Number.NaN;
  };
  const delta = parse(right) - parse(left);
  return Number.isFinite(delta) ? Math.round(delta / 86_400_000) : 0;
}

export function formatMissing(value: unknown, suffix = "—"): string {
  if (value === null || value === undefined || value === "" || (typeof value === "number" && Number.isNaN(value))) return "—";
  return `${String(value)}${suffix === "—" ? "" : suffix}`;
}

export function formatNumber(value: number | null | undefined, options: Intl.NumberFormatOptions = {}, suffix = "") {
  if (value === null || value === undefined || !Number.isFinite(value)) return "—";
  return `${new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 1, ...options }).format(value)}${suffix}`;
}

export function formatPercent(value: number | null | undefined) {
  return formatNumber(value, { maximumFractionDigits: 0 }, "%");
}

export function formatDate(value: string | null | undefined, pattern = "dd.MM.yyyy", timeZone = dashboardTimezone) {
  if (!value) return "—";
  if (/^\d{4}-\d{2}-\d{2}$/.test(value)) {
    const [year, month, day] = value.split("-");
    return `${day}.${month}.${year}`;
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "—";
  try {
    return new Intl.DateTimeFormat("ru-RU", {
      timeZone, day: "2-digit", month: "2-digit", year: "numeric",
      ...(pattern.includes("HH") ? { hour: "2-digit", minute: "2-digit", hourCycle: "h23" as const } : {}),
    }).format(parsed).replace(",", "");
  } catch {
    return "—";
  }
}

export function toDateTimeLocal(value: string | null | undefined, timeZone = dashboardTimezone) {
  if (!value) return "";
  return dateTimeLocalInTimeZone(value, timeZone);
}

export function formatDuration(seconds: number | null | undefined) {
  if (seconds === null || seconds === undefined || !Number.isFinite(seconds)) return "—";
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.round((seconds % 3600) / 60);
  return hours > 0 ? `${hours} ч ${minutes} мин` : `${minutes} мин`;
}

export function signedDelta(value: number | null | undefined, suffix = "") {
  if (value === null || value === undefined || !Number.isFinite(value)) return "—";
  return `${value > 0 ? "+" : ""}${formatNumber(value)}${suffix}`;
}
