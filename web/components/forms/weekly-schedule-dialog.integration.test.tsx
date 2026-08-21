import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { WeeklyScheduleDialog } from "@/components/forms/weekly-schedule-dialog";
import type { WorkoutPlan } from "@/lib/types";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("WeeklyScheduleDialog requests", () => {
  it("posts one real scheduled session per active template", async () => {
    const bodies: Array<Record<string, unknown>> = [];
    vi.spyOn(globalThis, "fetch").mockImplementation(async (_input, init) => {
      if (!init?.method || init.method === "GET") {
        return new Response(JSON.stringify({ data: { timezone: "Europe/Moscow", first_day_of_week: 1 } }), { status: 200, headers: { "content-type": "application/json" } });
      }
      bodies.push(JSON.parse(String(init?.body)) as Record<string, unknown>);
      return new Response(JSON.stringify({ data: { id: bodies.length } }), { status: 200, headers: { "content-type": "application/json" } });
    });
    const plan: WorkoutPlan = {
      id: 7,
      name: "Full Body",
      active: true,
      templates: [
        { id: 11, name: "A", position: 1, exercises: [{ name: "Тяга", position: 1 }] },
        { id: 12, name: "B", position: 2, exercises: [{ name: "Жим", position: 1 }] },
        { id: 13, name: "C", position: 3, exercises: [{ name: "Ноги", position: 1 }] },
      ],
    };
    const client = new QueryClient({ defaultOptions: { mutations: { retry: false }, queries: { retry: false, staleTime: Infinity } } });
    client.setQueryData(["settings"], { timezone: "Europe/Moscow", first_day_of_week: 1 });
    const user = userEvent.setup();
    render(<QueryClientProvider client={client}><WeeklyScheduleDialog plan={plan} open onOpenChange={vi.fn()} /></QueryClientProvider>);

    await user.click(await screen.findByRole("button", { name: "Добавить 3 тренировки" }));

    await waitFor(() => expect(bodies).toHaveLength(3));
    expect(bodies.map((body) => body.template_id)).toEqual([11, 12, 13]);
    expect(bodies.every((body) => body.status === "scheduled" && body.plan_id === 7 && typeof body.scheduled_at === "string" && String(body.external_id).startsWith("web-schedule:7:"))).toBe(true);
    expect(await screen.findByText("Неделя добавлена в расписание")).toBeInTheDocument();
  });

  it("retries only rows left after a partial server failure", async () => {
    const templateIDs: unknown[] = [];
    let request = 0;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (_input, init) => {
      if (!init?.method || init.method === "GET") {
        return new Response(JSON.stringify({ data: { timezone: "Europe/Moscow", first_day_of_week: 1 } }), { status: 200, headers: { "content-type": "application/json" } });
      }
      request++;
      const body = JSON.parse(String(init?.body)) as Record<string, unknown>;
      templateIDs.push(body.template_id);
      if (request === 2) return new Response(JSON.stringify({ error: { message: "Шаблон временно недоступен" } }), { status: 503, headers: { "content-type": "application/json" } });
      return new Response(JSON.stringify({ data: { id: request } }), { status: 200, headers: { "content-type": "application/json" } });
    });
    const plan: WorkoutPlan = {
      id: 7,
      name: "Full Body",
      active: true,
      templates: [
        { id: 11, name: "A", position: 1, exercises: [{ name: "Тяга", position: 1 }] },
        { id: 12, name: "B", position: 2, exercises: [{ name: "Жим", position: 1 }] },
        { id: 13, name: "C", position: 3, exercises: [{ name: "Ноги", position: 1 }] },
      ],
    };
    const client = new QueryClient({ defaultOptions: { mutations: { retry: false }, queries: { retry: false, staleTime: Infinity } } });
    client.setQueryData(["settings"], { timezone: "Europe/Moscow", first_day_of_week: 1 });
    const user = userEvent.setup();
    render(<QueryClientProvider client={client}><WeeklyScheduleDialog plan={plan} open onOpenChange={vi.fn()} /></QueryClientProvider>);

    await user.click(await screen.findByRole("button", { name: "Добавить 3 тренировки" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Создано 1 из 3");
    await user.click(screen.getByRole("button", { name: "Добавить 2 тренировки" }));

    expect(await screen.findByText("Неделя добавлена в расписание")).toBeInTheDocument();
    expect(templateIDs).toEqual([11, 12, 12, 13]);
  });
});
