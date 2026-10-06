import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CompactSummary, SummaryNumbers } from "./compact-summary";
import { apiFetch, APIError } from "@/lib/api";
import type { CompactSnapshot } from "@/lib/compact";

vi.mock("@/lib/api", async () => ({
  ...(await vi.importActual("@/lib/api")),
  apiFetch: vi.fn(),
}));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

const snapshot: CompactSnapshot = {
  version: 1,
  generated_at: "2026-10-06T10:00:00Z",
  date: "2026-10-06",
  timezone: "Europe/Moscow",
  recovery: null,
  sleep: null,
  nutrition: null,
  body: {
    measured_at: "2026-08-01T23:00:00Z",
    weight_kg: 80,
    body_fat_percent: null,
    skeletal_muscle_mass_kg: null,
  },
  training: {
    sessions: 0,
    working_sets: 0,
    repetitions: 0,
    volume_kg: 0,
    average_rir: null,
  },
  next_workout: null,
  sources: [],
};

describe("compact summary", () => {
  it("shows actual zero, missing values and each section's observation date without charts", () => {
    const { container } = render(<SummaryNumbers data={snapshot} />);
    expect(screen.getAllByText("0")).toHaveLength(3);
    expect(screen.getByText("0 кг")).toBeInTheDocument();
    expect(screen.getAllByText("—").length).toBeGreaterThan(10);
    expect(screen.getByText("02.08.2026")).toBeInTheDocument();
    expect(screen.getByText("30.09.2026 — 06.10.2026")).toBeInTheDocument();
    expect(screen.getByText("Пока не запланирована")).toBeInTheDocument();
    expect(container.querySelector("svg.recharts-surface")).toBeNull();
  });

  it("hides previous health data when the session expires during refresh", async () => {
    vi.mocked(apiFetch)
      .mockResolvedValueOnce(snapshot)
      .mockRejectedValue(new APIError("expired", 401));
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={client}>
        <CompactSummary />
      </QueryClientProvider>,
    );
    await screen.findByText("80 кг");
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Обновить сводку" }),
      ).toBeEnabled(),
    );
    fireEvent.click(screen.getByRole("button", { name: "Обновить сводку" }));
    await screen.findByText("Войдите, чтобы посмотреть сводку.");
    expect(screen.queryByText("80 кг")).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Войти в FitLog" }),
    ).toHaveAttribute("href", "/dashboard/login?next=%2Fdashboard%2Fsummary");
    expect(apiFetch).toHaveBeenCalledWith("/dashboard/summary");
  });
});
