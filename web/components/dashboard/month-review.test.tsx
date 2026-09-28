import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { apiFetch } from "@/lib/api";
import { MonthReview } from "./month-review";

vi.mock("@/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api")>()),
  apiFetch: vi.fn(),
}));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});
describe("monthly review", () => {
  it("uses adjacent calendar months rather than equal-length rolling ranges", async () => {
    vi.mocked(apiFetch).mockResolvedValue({ daily: [], summary: {} });
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={client}>
        <MonthReview today="2026-03-03" />
      </QueryClientProvider>,
    );
    await waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(8));
    const paths = vi.mocked(apiFetch).mock.calls.map(([path]) => path);
    expect(
      paths.filter((path) => path.includes("from=2026-02-01&to=2026-02-28")),
    ).toHaveLength(4);
    expect(
      paths.filter((path) => path.includes("from=2026-03-01&to=2026-03-03")),
    ).toHaveLength(4);
  });
  it("keeps nutrition and workouts usable when recovery fails", async () => {
    vi.mocked(apiFetch).mockImplementation(async (path) => {
      if (path.includes("/recovery?")) throw new Error("WHOOP недоступен");
      return {
        daily: [{ date: "2026-03-01", calories_kcal: 0, protein_g: null }],
        summary: { sessions: 2, working_sets: 30 },
      };
    });
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={client}>
        <MonthReview today="2026-03-03" />
      </QueryClientProvider>,
    );
    expect(await screen.findByText("WHOOP недоступен")).toBeInTheDocument();
    expect(await screen.findByText("0 ккал")).toBeInTheDocument();
    expect(screen.getByText("30")).toBeInTheDocument();
    expect(screen.getByText(/в среднем · 1 из 3 дней/)).toBeInTheDocument();
  });
});
