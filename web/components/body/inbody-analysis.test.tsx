import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { InBodyAnalysis, relativeDifference } from "./inbody-analysis";

describe("InBodyAnalysis", () => {
  it("calculates symmetric relative difference", () => {
    expect(relativeDifference(3.9, 4.1)).toBeCloseTo(5);
    expect(relativeDifference(null, 4.1)).toBeNull();
  });

  it("renders water, visceral and segment context without diagnosing", () => {
    render(<InBodyAnalysis latest={{
      id: 1, measured_at: "2026-08-20T10:00:00+03:00", skeletal_muscle_mass_kg: 34.2,
      ecw_tbw_ratio: 0.381, visceral_fat_area_cm2: 72, basal_metabolic_rate_kcal: 1780,
      segments: [{ segment: "left_arm", lean_mass_kg: 3.9 }, { segment: "right_arm", lean_mass_kg: 4.1 }],
    }} />);
    expect(screen.getByText("ECW / TBW")).toBeInTheDocument();
    expect(screen.getByText("Ниже справочных 100 см²")).toBeInTheDocument();
    expect(screen.getByText(/не медицинский диагноз/i)).toBeInTheDocument();
    expect(screen.getByText("Левая рука")).toBeInTheDocument();
  });
});
