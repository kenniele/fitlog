import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SessionCalendar, calendarSessionQuery } from "@/components/training/session-calendar";
import { buildWeekCalendarDates } from "@/lib/week";

vi.mock("next/link", () => ({ default: ({ href, children, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a href={String(href)} {...props}>{children}</a> }));

describe("session calendar", () => {
  it("pads a partial range to the persisted week boundary", () => {
    expect(buildWeekCalendarDates("2026-08-19", "2026-08-21", 1)).toEqual([
      null, null, "2026-08-19", "2026-08-20", "2026-08-21", null, null,
    ]);
    expect(buildWeekCalendarDates("2026-08-19", "2026-08-21", 7)).toEqual([
      null, null, null, "2026-08-19", "2026-08-20", "2026-08-21", null,
    ]);
  });

  it("preserves current server filters and selects calendar date basis", () => {
    const params = new URLSearchParams(calendarSessionQuery("from=2026-08-17&to=2026-08-23&status=scheduled&plan_id=7"));
    expect(params.get("from")).toBe("2026-08-17");
    expect(params.get("to")).toBe("2026-08-23");
    expect(params.get("status")).toBe("scheduled");
    expect(params.get("plan_id")).toBe("7");
    expect(params.get("date_basis")).toBe("calendar");
  });

  it("places calendar-basis sessions into linked status cards", () => {
    render(<SessionCalendar from="2026-08-17" to="2026-08-23" firstDayOfWeek={1} sessions={[{
      id: 42,
      calendar_date: "2026-08-19",
      scheduled_at: "2026-08-19T15:30:00Z",
      template_name: "Full Body B",
      status: "scheduled",
      working_sets: 12,
      volume_kg: 4200,
    }]} />);
    const link = screen.getByRole("link", { name: /Full Body B/ });
    expect(link).toHaveAttribute("href", "/dashboard/training/sessions/42");
    expect(link).toHaveTextContent("запланирована");
    expect(screen.getByText("1 сессия")).toBeInTheDocument();
  });
});
