import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { WorkoutForm } from "@/components/forms/workout-form";

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it("requires a program before a workout and clears the workout when switching programs", async () => {
  const plans = [
    { id: 4, name: "Фуллбади", templates: [{ id: 11, name: "Фуллбади А" }] },
    { id: 5, name: "Фуллбади приоритет грудь", templates: [{ id: 12, name: "ГП Фулбади А" }] },
  ];
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
    const url = String(input);
    const data = url.includes("workout-plans") ? { items: plans, total: 2 }
      : url.includes("settings") ? { timezone: "Europe/Moscow", first_day_of_week: 1 }
      : { items: [], total: 0 };
    return new Response(JSON.stringify({ data }), { headers: { "content-type": "application/json" } });
  });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const user = userEvent.setup();
  render(<QueryClientProvider client={client}><WorkoutForm open onOpenChange={vi.fn()} /></QueryClientProvider>);
  const program = await screen.findByRole("combobox", { name: "Программа тренировок" });
  const workout = screen.getByRole("combobox", { name: "Тренировка программы" });
  expect(workout).toBeDisabled();
  await screen.findByRole("option", { name: "Фуллбади" });
  await user.selectOptions(program, "4");
  expect(workout).toBeEnabled();
  expect(within(workout).getByRole("option", { name: "Фуллбади А" })).toBeInTheDocument();
  expect(within(workout).queryByRole("option", { name: "ГП Фулбади А" })).not.toBeInTheDocument();
  await user.selectOptions(workout, "11");
  await user.selectOptions(program, "5");
  expect(workout).toHaveValue("");
  expect(within(workout).queryByRole("option", { name: "Фуллбади А" })).not.toBeInTheDocument();
  expect(within(workout).getByRole("option", { name: "ГП Фулбади А" })).toBeInTheDocument();
});
