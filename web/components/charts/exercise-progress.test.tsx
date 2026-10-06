import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ExerciseProgress, progressSummary } from "./exercise-progress";
import { apiFetch } from "@/lib/api";

vi.mock("@/lib/api", () => ({ apiFetch: vi.fn() }));

describe("exercise progress", () => {
  it("uses ordered observed values and keeps missing values and zero out of the strength trend", () => {
    const summary = progressSummary([
      { date: "2026-10-03", estimated_1rm: 110 },
      { date: "2026-10-01", estimated_1rm: null },
      { date: "2026-10-02", estimated_1rm: 100 },
      { date: "2026-10-04", estimated_1rm: 0 },
    ]);
    expect(summary.measured).toHaveLength(2);
    expect(summary.first).toBe(100);
    expect(summary.last).toBe(110);
    expect(summary.change).toBeCloseTo(10);
    expect(
      progressSummary([{ date: "2026-10-02", estimated_1rm: 100 }]).change,
    ).toBeNull();
    expect(progressSummary([]).last).toBeUndefined();
  });

  it("requests only the selected exercise and finished workouts when the period changes", async () => {
    vi.mocked(apiFetch).mockResolvedValue({ daily: [] });
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={client}>
        <ExerciseProgress exercises={[{ id: 7, name: "Жим лёжа" }]} />
      </QueryClientProvider>,
    );
    expect(apiFetch).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText("Упражнение для прогресса"), {
      target: { value: "7" },
    });
    await screen.findByText("Пока нет точек прогресса");
    expect(apiFetch).toHaveBeenCalledWith(
      expect.stringContaining("exercise_id=7&status=finished"),
    );
    fireEvent.click(screen.getByRole("button", { name: "4 нед." }));
    await waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(2));
    expect(screen.getByRole("button", { name: "4 нед." })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });
});
