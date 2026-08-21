import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ActivityHeatmap, DistributionBars, TrainingStreakCards, activityIntensity } from "@/components/charts/training-insights";

describe("training insight charts", () => {
  it("normalizes activity into stable intensity levels", () => {
    expect(activityIntensity(0, 10)).toBe(0);
    expect(activityIntensity(1, 10)).toBe(1);
    expect(activityIntensity(5, 10)).toBe(2);
    expect(activityIntensity(10, 10)).toBe(4);
  });

  it("renders missing-safe analytics and real streak values", () => {
    const { rerender } = render(<ActivityHeatmap data={[]} />);
    expect(screen.getByText("В выбранном периоде нет календарных точек.")).toBeInTheDocument();
    rerender(<TrainingStreakCards streak={{ current_days: 2, longest_last_30_days: 4, active_days_last_30: 9 }} />);
    expect(screen.getByText("Текущая серия")).toBeInTheDocument();
    expect(screen.getByText("Лучшая за 30 дней")).toBeInTheDocument();
    expect(screen.getByText("Активность за 30 дней")).toBeInTheDocument();
  });

  it("labels distributions for assistive technology", () => {
    render(<DistributionBars title="RIR" description="Распределение" data={[{ label: "RIR 2", value: 7 }]} />);
    expect(screen.getByRole("listitem")).toHaveTextContent("RIR 2");
    expect(screen.getByRole("listitem")).toHaveTextContent("7 подходов");
  });
});
