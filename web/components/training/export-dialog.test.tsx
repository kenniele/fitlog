import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { TrainingExport } from "./export-dialog";
import { downloadFromAPI } from "@/lib/api";

vi.mock("@/lib/api", async (importOriginal) => ({
  ...await importOriginal<typeof import("@/lib/api")>(),
  downloadFromAPI: vi.fn(),
}));

const filters = "from=2026-08-01&to=2026-08-31&status=finished&exercise_id=42&plan_id=9&search=bench&date_basis=calendar&page=3&page_size=25&compare=true";

function setup() {
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  render(<QueryClientProvider client={client}><TrainingExport filters={filters} /></QueryClientProvider>);
  fireEvent.click(screen.getByRole("button", { name: "Экспорт" }));
}

describe("Training export", () => {
  beforeEach(() => vi.mocked(downloadFromAPI).mockReset());
  afterEach(cleanup);

  it("downloads CSV for the displayed period and filters without pagination", async () => {
    setup();
    fireEvent.click(await screen.findByRole("button", { name: "Скачать CSV" }));
    await waitFor(() => expect(downloadFromAPI).toHaveBeenCalledOnce());
    const [path, filename] = vi.mocked(downloadFromAPI).mock.calls[0];
    const query = new URLSearchParams(path.split("?")[1]);
    expect(path).toMatch(/^\/workout-sessions\/export.csv\?/);
    expect(filename).toBe("fitlog-training-2026-08-01_2026-08-31.csv");
    expect(Object.fromEntries(query)).toEqual({
      from: "2026-08-01", to: "2026-08-31", status: "finished", exercise_id: "42", plan_id: "9",
      search: "bench", date_basis: "calendar", scope: "range",
    });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("downloads all-time JSON with the same training filters", async () => {
    setup();
    fireEvent.change(await screen.findByLabelText("Период"), { target: { value: "all" } });
    fireEvent.change(screen.getByRole("combobox", { name: /Формат/ }), { target: { value: "json" } });
    fireEvent.click(screen.getByRole("button", { name: "Скачать JSON" }));
    await waitFor(() => expect(downloadFromAPI).toHaveBeenCalledOnce());
    const [path, filename] = vi.mocked(downloadFromAPI).mock.calls[0];
    expect(path).toContain("/export.json?");
    expect(Object.fromEntries(new URLSearchParams(path.split("?")[1]))).toEqual({
      status: "finished", exercise_id: "42", plan_id: "9", search: "bench", date_basis: "calendar", scope: "all",
    });
    expect(filename).toBe("fitlog-training-all.json");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("keeps the dialog open after failure and allows retry", async () => {
    vi.mocked(downloadFromAPI).mockRejectedValueOnce(new Error("Не удалось скачать файл"));
    setup();
    fireEvent.click(await screen.findByRole("button", { name: "Скачать CSV" }));
    expect(await screen.findByText("Не удалось скачать файл")).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Скачать CSV" }));
    await waitFor(() => expect(downloadFromAPI).toHaveBeenCalledTimes(2));
  });
});
