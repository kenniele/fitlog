import { describe, expect, it } from "vitest";
import { buildWeekCalendarDates, nextWeekStartISO, startOfWeekISO, weekDayLabels } from "@/lib/week";

describe("configured week boundaries", () => {
  it("separates Sunday and Monday with a Monday-first week", () => {
    expect(startOfWeekISO("2026-08-16", 1)).toBe("2026-08-10");
    expect(startOfWeekISO("2026-08-17", 1)).toBe("2026-08-17");
    expect(nextWeekStartISO("2026-08-20", 1)).toBe("2026-08-24");
    expect(weekDayLabels(1)).toEqual(["Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"]);
  });

  it("groups Sunday and Monday into the same Sunday-first week", () => {
    expect(startOfWeekISO("2026-08-16", 7)).toBe("2026-08-16");
    expect(startOfWeekISO("2026-08-17", 7)).toBe("2026-08-16");
    expect(nextWeekStartISO("2026-08-20", 7)).toBe("2026-08-23");
    expect(weekDayLabels(7)).toEqual(["Вс", "Пн", "Вт", "Ср", "Чт", "Пт", "Сб"]);
  });

  it("pads calendar cells against the selected week start", () => {
    expect(buildWeekCalendarDates("2026-08-19", "2026-08-21", 1)).toEqual([
      null, null, "2026-08-19", "2026-08-20", "2026-08-21", null, null,
    ]);
    expect(buildWeekCalendarDates("2026-08-19", "2026-08-21", 7)).toEqual([
      null, null, null, "2026-08-19", "2026-08-20", "2026-08-21", null,
    ]);
  });
});
