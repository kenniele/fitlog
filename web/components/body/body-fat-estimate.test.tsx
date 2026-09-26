import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { BodyFatEstimateCard } from "./body-fat-estimate";
import type { BodyFatEstimate } from "@/lib/types";

const estimate: BodyFatEstimate = {
  status: "estimated", measured_at: "2026-09-01T08:00:00+03:00", from: "2026-09-02", through: "2026-09-11",
  timezone: "Europe/Moscow", baseline_percent: 20, tdee_kcal: 2700, total_days: 10, observed_days: 10,
  missing_days: 0, coverage_percent: 100, observed_deficit_kcal: 7700, estimated_deficit_kcal: 7700,
  percent: 19.2, lower_percent: 16.5, upper_percent: 21.8,
};

describe("BodyFatEstimateCard", () => {
  afterEach(cleanup);
  it("leads with the estimate and keeps the sensitivity range in collapsed details", () => {
    render(<BodyFatEstimateCard estimate={estimate} />);
    expect(screen.getByLabelText("Расчётный процент жира")).toHaveTextContent("≈19,2%");
    expect(screen.getByText(/Изменение от InBody/)).toHaveTextContent("-0,8 п.п. по расчёту");
    const scenarios = screen.getByLabelText("Диапазон сценариев");
    expect(scenarios).toHaveTextContent("16,5–21,8%");
    expect(scenarios.closest("details")).not.toHaveAttribute("open");
    expect(scenarios).not.toBeVisible();
    expect(screen.getByText(/не статистический доверительный интервал/)).toBeInTheDocument();
    expect(screen.getByText(/Фактический процент может отличаться/)).toBeVisible();
    expect(screen.getByText(/10 из 10 дней/)).toBeInTheDocument();
  });

  it("labels imputed gaps and surplus without claiming measured fat loss", () => {
    render(<BodyFatEstimateCard estimate={{ ...estimate, missing_days: 2, observed_days: 8, coverage_percent: 80, estimated_deficit_kcal: -500 }} />);
    expect(screen.getByText(/Накопленный профицит/)).toBeInTheDocument();
    expect(screen.getByText(/Пропущено дней: 2/)).toHaveTextContent("среднее питание известных дней");
  });

  it.each(["no_tdee", "insufficient_nutrition", "incomplete_inbody", "outside_model"] as const)("explains %s without an estimate", (status) => {
    render(<BodyFatEstimateCard estimate={{ ...estimate, status, percent: undefined, lower_percent: undefined, upper_percent: undefined }} />);
    expect(screen.getByRole("status")).toBeInTheDocument();
    expect(screen.queryByLabelText("Расчётный процент жира")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Диапазон сценариев")).not.toBeInTheDocument();
  });
});
