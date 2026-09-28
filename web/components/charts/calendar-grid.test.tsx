import {
  cleanup,
  act,
  fireEvent,
  render,
  screen,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CalendarGrid } from "./calendar-grid";
import { calendarDates } from "@/lib/dashboard";

afterEach(cleanup);

Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: vi.fn(() => ({
    matches: false,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  })),
});
describe("calendar interaction", () => {
  const props = {
    dates: calendarDates("2026-09-01", "2026-09-10"),
    today: "2026-09-09",
    label: "Сон",
    color: () => undefined,
    valueLabel: () => "Без данных",
  };
  it("gives keyboard users one entry point and moves by calendar weeks", () => {
    render(<CalendarGrid {...props} />);
    const ninth = screen.getByRole("button", {
      name: "09.09.2026: Без данных",
    });
    expect(ninth).toHaveAttribute("tabindex", "0");
    act(() => ninth.focus());
    fireEvent.keyDown(ninth, { key: "ArrowLeft" });
    expect(
      screen.getByRole("button", { name: "02.09.2026: Без данных" }),
    ).toHaveFocus();
  });
  it("distinguishes future dates and opens day details by tap", () => {
    render(<CalendarGrid {...props} details={() => <p>Записей нет</p>} />);
    expect(
      screen.queryByRole("button", { name: /10.09.2026/ }),
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText("10.09.2026: будущая дата")).toHaveAttribute(
      "data-future",
      "true",
    );
    fireEvent.click(
      screen.getByRole("button", { name: "01.09.2026: Без данных" }),
    );
    expect(screen.getByRole("dialog")).toHaveTextContent("Записей нет");
  });
});
