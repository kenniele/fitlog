import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { SettingsForm } from "@/components/forms/settings-form";

describe("SettingsForm", () => {
  it("blocks an inverted sleep target range before calling the API", async () => {
    const request = vi.spyOn(globalThis, "fetch");
    const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    const user = userEvent.setup();
    render(<QueryClientProvider client={client}><SettingsForm settings={{ timezone: "Europe/Moscow", first_day_of_week: 1 }} /></QueryClientProvider>);

    await user.type(screen.getByLabelText("Минимум, секунд"), "36000");
    await user.type(screen.getByLabelText("Максимум, секунд"), "28800");
    await user.click(screen.getByRole("button", { name: "Сохранить настройки" }));

    expect(await screen.findByText("Минимум не может превышать максимум")).toBeInTheDocument();
    expect(request).not.toHaveBeenCalled();
    request.mockRestore();
  });
});
