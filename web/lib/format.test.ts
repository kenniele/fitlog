import { afterEach, describe, expect, it } from "vitest";
import { dateInTimeZone, formatMissing, setDashboardTimezone, toDateTimeLocal } from "@/lib/format";

afterEach(() => setDashboardTimezone("UTC"));

describe("formatMissing", () => {
  it.each([null, undefined, "", Number.NaN])("renders missing input %s as an em dash", (value) => {
    expect(formatMissing(value)).toBe("—");
  });

  it("keeps zero instead of treating it as missing", () => {
    expect(formatMissing(0)).toBe("0");
  });
});

describe("dashboard timezone", () => {
  it("converts persisted timestamptz values into the profile timezone for datetime-local fields", () => {
    setDashboardTimezone("Europe/Moscow");
    expect(toDateTimeLocal("2026-08-21T00:00:00Z")).toBe("2026-08-21T03:00");

    setDashboardTimezone("America/New_York");
    expect(toDateTimeLocal("2026-08-21T00:00:00Z")).toBe("2026-08-20T20:00");
  });

  it("uses the profile timezone when deriving the current local date", () => {
    setDashboardTimezone("Pacific/Kiritimati");
    expect(dateInTimeZone("2026-08-21T12:00:00Z")).toBe("2026-08-22");
  });
});
