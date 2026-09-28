import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { BodyMeasurement } from "@/lib/types";
import { InBodyView } from "./inbody-explorer";
import { relativeDifference } from "@/lib/body";

afterEach(cleanup);

Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: vi.fn(() => ({
    matches: false,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  })),
});
const rows: BodyMeasurement[] = [
  {
    id: 2,
    measured_at: "2026-09-20T09:00:00Z",
    weight_kg: 100,
    body_fat_percent: 18,
    ecw_tbw_ratio: 0.379,
    segments: [{ segment: "left_arm", lean_mass_kg: 4.02, fat_percent: 106 }],
  },
  {
    id: 1,
    measured_at: "2026-08-20T09:00:00Z",
    weight_kg: 101,
    body_fat_percent: 19,
    ecw_tbw_ratio: 0.38,
    segments: [{ segment: "left_arm", lean_mass_kg: 4.01 }],
  },
];
describe("InBody exploration", () => {
  it("selects a segment by keyboard without changing its shape", () => {
    render(<InBodyView measurements={rows} />);
    const arm = screen.getByRole("button", { name: "Выбрать: Левая рука" });
    const shape = arm.querySelector("path")?.getAttribute("d");
    fireEvent.keyDown(arm, { key: "Enter" });
    expect(arm).toHaveAttribute("aria-pressed", "true");
    expect(
      screen.getByRole("heading", { name: "Левая рука" }),
    ).toBeInTheDocument();
    expect(screen.getByText("4,02 кг")).toBeInTheDocument();
    expect(arm.querySelector("path")?.getAttribute("d")).toBe(shape);
  });
  it("changes measurement and comparison using actual records", () => {
    render(<InBodyView measurements={rows} />);
    fireEvent.click(screen.getByRole("button", { name: "Сравнить измерения" }));
    expect(screen.getByLabelText("Базовое измерение")).toHaveValue("1");
    fireEvent.change(screen.getByLabelText("Выбранное измерение"), {
      target: { value: "1" },
    });
    expect(screen.getByLabelText("Базовое измерение")).toHaveValue("2");
    fireEvent.click(
      screen.getByRole("button", { name: "Все показатели и сравнение" }),
    );
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText(/Δ \+1 п.п./)).toBeInTheDocument();
    expect(within(dialog).getByText(/Δ \+0,001/)).toBeInTheDocument();
  });
  it("keeps first-measurement and missing-segment states explicit", () => {
    render(<InBodyView measurements={[rows[0]]} />);
    expect(
      screen.queryByRole("button", { name: "Сравнить измерения" }),
    ).not.toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Выбрать: Правая нога" }),
    );
    expect(
      screen.getByText("Первое измерение · без сравнения"),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Без данных").length).toBeGreaterThan(0);
  });
  it("offers the existing create flow when no measurements exist", () => {
    render(<InBodyView measurements={[]} />);
    expect(
      screen.getByRole("link", { name: /Добавить InBody/ }),
    ).toHaveAttribute("href", "/dashboard/body?action=new");
    expect(relativeDifference(3.9, 4.1)).toBeCloseTo(5);
    expect(relativeDifference(null, 4.1)).toBeNull();
  });
});
