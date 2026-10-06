import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ScriptableSetup } from "./scriptable-setup";
import { apiFetch } from "@/lib/api";

vi.mock("@/lib/api", () => ({ apiFetch: vi.fn() }));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

describe("Scriptable setup", () => {
  it("issues a widget-only key only on demand and copies that key", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    vi.mocked(apiFetch).mockResolvedValue({
      access_token: "widget-test-key",
      expires_at: "2027-01-04T10:00:00Z",
      scope: "widget:read",
    });
    render(<ScriptableSetup />);
    expect(apiFetch).not.toHaveBeenCalled();
    fireEvent.click(screen.getByText("Виджет через Scriptable"));
    expect(
      screen.getByRole("link", { name: "Скачать FitLog.js" }),
    ).toHaveAttribute("href", "/scriptable/FitLog.js");
    fireEvent.click(
      screen.getByRole("button", { name: "Создать ключ виджета" }),
    );
    await screen.findByLabelText("Ключ виджета");
    expect(apiFetch).toHaveBeenCalledWith("/auth/widget-token", {
      method: "POST",
    });
    fireEvent.click(screen.getByRole("button", { name: "Скопировать ключ" }));
    await screen.findByText(
      "Ключ скопирован. Вставьте его в окно подключения FitLog в Scriptable.",
    );
    expect(writeText).toHaveBeenCalledWith("widget-test-key");
  });

  it("copies the downloadable script without requesting a credential", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { clipboard: { writeText } });
    const fetchScript = vi.fn().mockResolvedValue({
      ok: true,
      text: async () => "// Scriptable test source",
    });
    vi.stubGlobal("fetch", fetchScript);
    render(<ScriptableSetup />);
    fireEvent.click(screen.getByText("Виджет через Scriptable"));
    fireEvent.click(screen.getByRole("button", { name: "Скопировать скрипт" }));
    await screen.findByText(
      "Скрипт скопирован. Вставьте его в новый скрипт в Scriptable.",
    );
    expect(fetchScript).toHaveBeenCalledWith("/scriptable/FitLog.js", {
      cache: "no-store",
    });
    expect(writeText).toHaveBeenCalledWith("// Scriptable test source");
    expect(apiFetch).not.toHaveBeenCalled();
  });
});
