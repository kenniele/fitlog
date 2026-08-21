import { render, screen, within } from "@testing-library/react";
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

  it("shows an explicit latest-versus-previous comparison including segment deltas", () => {
    render(<InBodyAnalysis
      latest={{
        id: 2, measured_at: "2026-08-07T08:16:00+03:00", weight_kg: 102.5, body_fat_percent: 17.6,
        fat_mass_kg: 18, lean_mass_kg: 84.5, skeletal_muscle_mass_kg: 48.2, total_body_water_l: 62,
        ecw_tbw_ratio: 0.379, visceral_fat_level: 8, basal_metabolic_rate_kcal: 2194, inbody_score: 82,
        segments: [{ segment: "left_arm", lean_mass_kg: 5.02, lean_percent: 110.1, fat_mass_kg: 0.8, fat_percent: 99.1 }],
      }}
      previous={{
        id: 1, measured_at: "2026-06-24T09:15:00+03:00", weight_kg: 106, body_fat_percent: 19.6,
        fat_mass_kg: 20.8, lean_mass_kg: 85.2, skeletal_muscle_mass_kg: 48.5, total_body_water_l: 62.4,
        ecw_tbw_ratio: 0.38, visceral_fat_level: 9, basal_metabolic_rate_kcal: 2210, inbody_score: 80,
        segments: [{ segment: "left_arm", lean_mass_kg: 4.97, lean_percent: 108, fat_mass_kg: 1.1, fat_percent: 129.4 }],
      }}
    />);

    expect(screen.getByText("07.08.2026 против 24.06.2026 · было → стало → изменение")).toBeInTheDocument();
    const weight = screen.getByLabelText("Сравнение Вес");
    expect(within(weight).getByText("106 кг")).toBeInTheDocument();
    expect(within(weight).getByText("102,5 кг")).toBeInTheDocument();
    expect(within(weight).getByText("Δ -3,5 кг")).toBeInTheDocument();
    const ratio = screen.getByLabelText("Сравнение ECW / TBW");
    expect(within(ratio).getByText("0,380")).toBeInTheDocument();
    expect(within(ratio).getByText("0,379")).toBeInTheDocument();
    const segmentRows = screen.getAllByLabelText("Сегмент Левая рука");
    expect(within(segmentRows.at(-1)!).getByText("+0,05 кг")).toBeInTheDocument();
  });
});
