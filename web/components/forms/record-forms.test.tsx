import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { BodyForm } from "@/components/forms/record-forms";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("BodyForm", () => {
  it("submits core and segmental InBody values while dropping empty segments", async () => {
    let payload: Record<string, unknown> | undefined;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (_input, init) => {
      payload = JSON.parse(String(init?.body)) as Record<string, unknown>;
      return new Response(JSON.stringify({ data: { id: 1, ...payload } }), { status: 200, headers: { "content-type": "application/json" } });
    });
    const client = new QueryClient({ defaultOptions: { mutations: { retry: false }, queries: { retry: false } } });
    const user = userEvent.setup();
    render(<QueryClientProvider client={client}><BodyForm open onOpenChange={vi.fn()} /></QueryClientProvider>);

    await user.type(screen.getByLabelText("Базовый обмен, ккал"), "1780");
    await user.type(screen.getAllByLabelText("Lean, кг")[0], "3.9");
    await user.click(screen.getByRole("button", { name: "Сохранить" }));

    await waitFor(() => expect(payload).toBeDefined());
    expect(payload?.basal_metabolic_rate_kcal).toBe(1780);
    expect(payload?.source).toBe("inbody");
    expect(payload?.segments).toEqual([{ segment: "left_arm", lean_mass_kg: 3.9 }]);
  });
});
