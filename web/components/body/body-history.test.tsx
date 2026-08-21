import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { BodyHistory, BodyMeasurementDetails, bodyHistoryPath } from "./body-history";
import type { BodyMeasurement } from "@/lib/types";

const measurement: BodyMeasurement = {
  id: 7,
  measured_at: "2026-08-07T08:16:00+03:00",
  source: "inbody",
  external_id: "inbody-2026-08-07",
  weight_kg: 102.5,
  body_fat_percent: 17.6,
  skeletal_muscle_mass_kg: 48.2,
  total_body_water_l: 62,
  ecw_tbw_ratio: 0.379,
  inbody_score: 82,
  segments: [{ segment: "left_arm", lean_mass_kg: 5.02, fat_mass_kg: 0.8 }],
};

describe("Body history", () => {
  it("builds an all-time request and applies the InBody source only when selected", () => {
    expect(bodyHistoryPath("all", 2)).toBe("/body-measurements?page=2&page_size=25");
    expect(bodyHistoryPath("inbody", 1)).toBe("/body-measurements?page=1&page_size=25&source=inbody");
  });

  it("renders the complete saved measurement including water and segments", () => {
    render(<BodyMeasurementDetails entry={measurement} />);
    expect(screen.getByText("102,5 кг")).toBeInTheDocument();
    expect(screen.getByText("0,379")).toBeInTheDocument();
    const segment = screen.getByText("Левая рука").closest("tr");
    expect(segment).not.toBeNull();
    expect(within(segment!).getByText("5,02 кг")).toBeInTheDocument();
  });

  it("exposes all records, the InBody filter and a full-row viewer", async () => {
    const onScopeChange = vi.fn();
    render(<BodyHistory
      data={{ items: [measurement], total: 1, page: 1, page_size: 25 }}
      scope="all"
      page={1}
      onScopeChange={onScopeChange}
      onPageChange={vi.fn()}
      onEdit={vi.fn()}
      onDelete={vi.fn()}
    />);

    expect(screen.getByText(/за всё время/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Только InBody" }));
    expect(onScopeChange).toHaveBeenCalledWith("inbody");
    fireEvent.click(screen.getByRole("button", { name: "Просмотреть измерение" }));
    const dialog = await screen.findByRole("dialog");
    expect(dialog).toBeInTheDocument();
    expect(within(dialog).getByText("External ID: inbody-2026-08-07")).toBeInTheDocument();
  });
});
