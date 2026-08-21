import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DateRangeControls } from "@/components/layout/date-range-controls";

const navigation = vi.hoisted(() => ({ replace: vi.fn() }));

vi.mock("next/navigation", () => ({
  usePathname: () => "/dashboard",
  useRouter: () => ({ replace: navigation.replace }),
  useSearchParams: () => new URLSearchParams("range=30&from=2026-07-23&to=2026-08-21&compare=1"),
}));

describe("DateRangeControls", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-08-21T12:00:00+03:00"));
    navigation.replace.mockReset();
  });

  afterEach(() => vi.useRealTimers());

  it("writes a selected preset and dates back to the URL", () => {
    render(<DateRangeControls />);
    fireEvent.click(screen.getByRole("button", { name: "7д" }));
    expect(navigation.replace).toHaveBeenCalledWith(
      "/dashboard?range=7&from=2026-08-15&to=2026-08-21&compare=1",
      { scroll: false },
    );
  });
});
