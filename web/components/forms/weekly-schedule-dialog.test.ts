import { describe, expect, it } from "vitest";
import { scheduledSessionPayload, weeklyScheduleDefaults } from "@/components/forms/weekly-schedule-dialog";
import type { WorkoutPlan } from "@/lib/types";

const plan: WorkoutPlan = {
  id: 7,
  name: "Full Body",
  active: true,
  templates: [
    { id: 11, name: "A", position: 1, exercises: [{ position: 1, name: "Тяга" }] },
    { id: 12, name: "B", position: 2, exercises: [] },
    { id: 13, name: "C", position: 3, exercises: [] },
  ],
};

describe("weekly schedule", () => {
  it("defaults active templates to Monday, Wednesday and Friday", () => {
    const rows = weeklyScheduleDefaults(plan, "2026-08-20", 1);
    expect(rows.map((row) => [row.templateName, row.date, row.time])).toEqual([
      ["A", "2026-08-24", "18:30"],
      ["B", "2026-08-26", "18:30"],
      ["C", "2026-08-28", "18:30"],
    ]);
  });

  it("rotates the same every-other-day cadence from a Sunday week start", () => {
    const rows = weeklyScheduleDefaults(plan, "2026-08-20", 7);
    expect(rows.map((row) => row.date)).toEqual(["2026-08-23", "2026-08-25", "2026-08-27"]);
  });

  it("builds a real scheduled-session payload for the selected template", () => {
    const row = weeklyScheduleDefaults(plan, "2026-08-20", 1)[1];
    expect(scheduledSessionPayload(plan, row)).toEqual({
      date: "2026-08-26",
      status: "scheduled",
      scheduled_at: "2026-08-26T18:30",
      plan_id: 7,
      template_id: 12,
      source: "manual",
      external_id: "web-schedule:7:12:2026-08-26T18:30",
      notes: "",
    });
  });
});
