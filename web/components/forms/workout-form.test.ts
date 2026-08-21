import { describe, expect, it } from "vitest";
import { buildWorkoutSessionPayload, workoutSchema, workoutValuesFromSession } from "@/components/forms/workout-form";
import type { WorkoutSession } from "@/lib/types";

describe("workout session form mapping", () => {
  it("provides explicit blank defaults so reopening create does not retain the previous workout", () => {
    const values = workoutValuesFromSession(null, "Europe/Moscow");
    expect(values).toMatchObject({
      plan_id: "",
      template_id: "",
      program_name: "",
      notes: "",
      external_id: "",
      exercises: [{ exercise_id: "", name: "", note: "", external_id: "", sets: [{ weight_kg: undefined, rir: undefined, rest_seconds: undefined, comment: "", external_id: "" }] }],
    });
    expect(values.strain).toBeUndefined();
  });

  it("round-trips rest after exercise through form values and the API payload", () => {
    const session: WorkoutSession = {
      id: 42,
      date: "2026-08-21",
      status: "finished",
      started_at: "2026-08-21T15:00:00Z",
      finished_at: "2026-08-21T16:00:00Z",
      exercises: [{
        id: 7,
        exercise_id: 3,
        name: "Присед",
        completed: true,
        rest_after_exercise_seconds: 180,
        sets: [{ id: 9, type: "working", actual_reps: 8, completed: true }],
      }],
    };

    const values = workoutValuesFromSession(session, "Europe/Moscow");
    expect(values.exercises?.[0].rest_after_exercise_seconds).toBe(180);
    expect(workoutSchema.safeParse(values).success).toBe(true);

    const payload = buildWorkoutSessionPayload(values, true);
    expect(payload).toMatchObject({
      exercises: [{ exercise_id: 3, position: 1, rest_after_exercise_seconds: 180 }],
    });
  });

  it("keeps an empty scheduled session metadata-only instead of inventing a blank exercise", () => {
    const values = workoutValuesFromSession({
      id: 43,
      date: "2026-08-22",
      status: "scheduled",
      scheduled_at: "2026-08-22T06:00:00Z",
      exercises: [],
    }, "Europe/Moscow");

    expect(values.exercises).toBeUndefined();
    expect(workoutSchema.safeParse(values).success).toBe(true);
    expect(buildWorkoutSessionPayload(values, true)).not.toHaveProperty("exercises");
  });

  it("keeps planned prescription separate and patches actual values by stable IDs", () => {
    const values = workoutValuesFromSession({
      id: 45,
      date: "2026-08-22",
      status: "scheduled",
      scheduled_at: "2026-08-22T06:00:00Z",
      plan_id: 11,
      template_id: 12,
      has_progression_snapshot: true,
      exercises: [{
        id: 21,
        exercise_id: 3,
        name: "Присед",
        completed: false,
        sets: [{
          id: 31,
          type: "working",
          planned_weight_kg: 80,
          planned_min_reps: 6,
          planned_max_reps: 10,
          planned_rir: 2,
          completed: false,
        }],
      }],
    }, "Europe/Moscow");

    expect(values.exercises?.[0].sets[0]).toMatchObject({
      id: "31",
      planned_weight_kg: 80,
      planned_min_reps: 6,
      planned_max_reps: 10,
      planned_rir: 2,
      completed: false,
    });
    expect(values.exercises?.[0].sets[0].weight_kg).toBeUndefined();
    expect(values.exercises?.[0].sets[0].reps).toBeUndefined();
    expect(workoutSchema.safeParse(values).success).toBe(true);

    const completed = structuredClone(values);
    completed.status = "finished";
    completed.started_at = "2026-08-22T18:00";
    completed.finished_at = "2026-08-22T19:00";
    Object.assign(completed.exercises![0].sets[0], { weight_kg: 82.5, reps: 8, rir: 1, completed: true });
    const payload = buildWorkoutSessionPayload(completed, true) as { exercises: Array<{ id: number; sets: Array<Record<string, unknown>> }> };
    expect(payload.exercises[0].id).toBe(21);
    expect(payload.exercises[0].sets[0]).toMatchObject({ id: 31, weight_kg: 82.5, reps: 8, rir: 1, completed: true });
    expect(payload.exercises[0].sets[0]).not.toHaveProperty("planned_weight_kg");
  });

  it("rejects negative rest after exercise", () => {
    const values = workoutValuesFromSession({
      id: 44,
      date: "2026-08-21",
      status: "finished",
      started_at: "2026-08-21T15:00:00Z",
      finished_at: "2026-08-21T16:00:00Z",
      exercises: [{
        id: 8,
        name: "Тяга",
        rest_after_exercise_seconds: -1,
        sets: [{ id: 10, type: "working", actual_reps: 5, completed: true }],
      }],
    }, "Europe/Moscow");

    expect(workoutSchema.safeParse(values).success).toBe(false);
  });
});
