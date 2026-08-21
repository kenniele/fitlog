import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { MetricCard } from "@/components/charts/metric-card";

describe("MetricCard", () => {
  it("renders duration deltas as time instead of raw seconds", () => {
    render(<MetricCard label="Сон" metric={{ value: "6 ч 48 мин", delta: 1_492, format: "duration" }} />);

    expect(screen.getByText("+25 мин")).toBeInTheDocument();
    expect(screen.queryByText("+1 492")).not.toBeInTheDocument();
  });

  it("keeps the direction for negative duration deltas", () => {
    render(<MetricCard label="Сон" metric={{ value: "6 ч 48 мин", delta: -3_600, format: "duration" }} />);

    expect(screen.getByText("−1 ч 0 мин")).toBeInTheDocument();
  });
});
