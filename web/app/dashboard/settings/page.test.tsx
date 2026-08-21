import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import SettingsPage from "@/app/dashboard/settings/page";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("SettingsPage query errors", () => {
  it("shows a secondary sources failure and retries both page queries", async () => {
    const requested: string[] = [];
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      requested.push(url);
      if (url.endsWith("/sources")) {
        return new Response(JSON.stringify({ error: { code: "sources_unavailable", message: "Источники временно недоступны" } }), { status: 503, headers: { "content-type": "application/json" } });
      }
      return new Response(JSON.stringify({ data: { timezone: "Europe/Moscow", first_day_of_week: 1, units: "metric" } }), { status: 200, headers: { "content-type": "application/json" } });
    });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const user = userEvent.setup();

    render(<QueryClientProvider client={client}><SettingsPage /></QueryClientProvider>);

    expect(await screen.findByRole("alert")).toHaveTextContent("Источники временно недоступны");
    await user.click(screen.getByRole("button", { name: "Повторить" }));
    await waitFor(() => {
      expect(requested.filter((url) => url.endsWith("/settings"))).toHaveLength(2);
      expect(requested.filter((url) => url.endsWith("/sources"))).toHaveLength(2);
    });
  });
});
