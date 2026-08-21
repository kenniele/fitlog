import { daysBetweenISO, shiftISODate } from "@/lib/format";

export type FirstDayOfWeek = 1 | 2 | 3 | 4 | 5 | 6 | 7;

const shortDayLabels: Record<FirstDayOfWeek, string> = {
  1: "Пн",
  2: "Вт",
  3: "Ср",
  4: "Чт",
  5: "Пт",
  6: "Сб",
  7: "Вс",
};

export function parseFirstDayOfWeek(value: unknown): FirstDayOfWeek | null {
  return typeof value === "number" && Number.isInteger(value) && value >= 1 && value <= 7
    ? value as FirstDayOfWeek
    : null;
}

function isoWeekday(value: string): FirstDayOfWeek | null {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return null;
  const date = new Date(`${value}T00:00:00Z`);
  if (Number.isNaN(date.getTime())) return null;
  return (date.getUTCDay() || 7) as FirstDayOfWeek;
}

export function startOfWeekISO(value: string, firstDayOfWeek: FirstDayOfWeek) {
  const weekday = isoWeekday(value);
  if (weekday === null) return value;
  return shiftISODate(value, -((weekday - firstDayOfWeek + 7) % 7));
}

export function nextWeekStartISO(value: string, firstDayOfWeek: FirstDayOfWeek) {
  const currentWeekStart = startOfWeekISO(value, firstDayOfWeek);
  if (currentWeekStart === value) return value;
  return shiftISODate(currentWeekStart, 7);
}

export function weekDayLabels(firstDayOfWeek: FirstDayOfWeek) {
  return Array.from({ length: 7 }, (_, index) => {
    const day = ((firstDayOfWeek - 1 + index) % 7 + 1) as FirstDayOfWeek;
    return shortDayLabels[day];
  });
}

export function buildWeekCalendarDates(from: string, to: string, firstDayOfWeek: FirstDayOfWeek): Array<string | null> {
  const firstWeekday = isoWeekday(from);
  if (firstWeekday === null || isoWeekday(to) === null) return [];
  const dayCount = daysBetweenISO(from, to) + 1;
  if (dayCount < 1 || dayCount > 366) return [];
  const before = (firstWeekday - firstDayOfWeek + 7) % 7;
  const after = (7 - ((before + dayCount) % 7)) % 7;
  return [
    ...Array.from({ length: before }, () => null),
    ...Array.from({ length: dayCount }, (_, index) => shiftISODate(from, index)),
    ...Array.from({ length: after }, () => null),
  ];
}
