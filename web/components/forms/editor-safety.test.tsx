import { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { NutritionForm } from "@/components/forms/record-forms";
import { SettingsForm } from "@/components/forms/settings-form";

afterEach(() => { cleanup(); vi.restoreAllMocks(); });
const client = () => new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });

function Editor() {
  const [open, setOpen] = useState(true);
  return <QueryClientProvider client={client()}><NutritionForm open={open} onOpenChange={setOpen} /></QueryClientProvider>;
}

it("keeps entered values and focus when dismissal is rejected, then permits explicit discard", async () => {
  const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
  const user = userEvent.setup();
  render(<Editor />);
  const calories = await screen.findByLabelText("Калории, ккал");
  await user.type(calories, "2100");
  expect(calories).toHaveFocus();
  await user.keyboard("{Escape}");
  expect(calories).toHaveValue(2100);
  await user.click(screen.getByRole("button", { name: "Отмена" }));
  expect(calories).toHaveValue(2100);
  const dialog = screen.getByRole("dialog");
  fireEvent.mouseDown(dialog.parentElement!);
  expect(calories).toHaveValue(2100);
  expect(confirm).toHaveBeenCalledTimes(3);
  confirm.mockReturnValue(true);
  await user.click(screen.getByRole("button", { name: "Закрыть" }));
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
});

it("keeps the editor open during save and closes without discard confirmation after success", async () => {
  let resolve!: (response: Response) => void;
  vi.spyOn(globalThis, "fetch").mockImplementation(() => new Promise((done) => { resolve = done; }));
  const confirm = vi.spyOn(window, "confirm");
  const user = userEvent.setup();
  render(<Editor />);
  await user.type(await screen.findByLabelText("Калории, ккал"), "2100");
  await user.click(screen.getByRole("button", { name: "Сохранить" }));
  await user.keyboard("{Escape}");
  expect(screen.getByRole("dialog")).toBeInTheDocument();
  expect(confirm).not.toHaveBeenCalled();
  resolve(new Response(JSON.stringify({ data: { id: 1 } }), { status: 200, headers: { "content-type": "application/json" } }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
});

it("converts hours and minutes to API seconds and preserves other recovery settings", async () => {
  let payload: Record<string, unknown> | undefined;
  vi.spyOn(globalThis, "fetch").mockImplementation(async (_url, init) => {
    payload = JSON.parse(String(init?.body));
    return new Response(JSON.stringify({ data: payload }), { status: 200, headers: { "content-type": "application/json" } });
  });
  const user = userEvent.setup();
  render(<QueryClientProvider client={client()}><SettingsForm settings={{ timezone: "Europe/Moscow", first_day_of_week: 1, recovery_ranges: { low: 34, high: 67, custom: "preserved" } }} /></QueryClientProvider>);
  await user.type(screen.getByLabelText("Минимум сна: часы"), "7");
  await user.clear(screen.getByLabelText("Минимум сна: минуты"));
  await user.type(screen.getByLabelText("Минимум сна: минуты"), "30");
  await user.type(screen.getByLabelText("Максимум сна: часы"), "9");
  await user.click(screen.getByRole("button", { name: "Сохранить настройки" }));
  await waitFor(() => expect(payload?.sleep_target_min_seconds).toBe(27000));
  expect(payload?.sleep_target_max_seconds).toBe(32400);
  expect(payload?.recovery_ranges).toEqual({ low: 34, high: 67, custom: "preserved" });
  expect(payload).not.toHaveProperty("recovery_low");
});
