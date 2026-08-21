import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { APIError } from "@/lib/api";
import { EmptyState, ErrorState } from "@/components/ui/states";

describe("API states", () => {
  it("renders an actionable empty state", () => {
    render(<EmptyState title="Записей нет" description="Добавьте первую запись" action={<button>Добавить</button>} />);
    expect(screen.getByText("Записей нет")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Добавить" })).toBeInTheDocument();
  });

  it("shows the API message and retries", () => {
    const retry = vi.fn();
    render(<ErrorState error={new APIError("Сервис временно недоступен", 503, "unavailable")} retry={retry} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Сервис временно недоступен");
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect(retry).toHaveBeenCalledOnce();
  });
});
