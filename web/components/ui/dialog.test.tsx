import { useState } from "react";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Dialog } from "@/components/ui/dialog";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";

afterEach(cleanup);

function DialogHarness() {
  const [open, setOpen] = useState(false);
  return <>
    <button type="button" onClick={() => setOpen(true)}>Открыть диалог</button>
    <Dialog open={open} onOpenChange={setOpen} title="Настройки" description="Описание диалога">
      <button type="button">Первое действие</button>
      <button type="button">Последнее действие</button>
    </Dialog>
  </>;
}

function ConfirmHarness() {
  const [open, setOpen] = useState(false);
  return <>
    <button type="button" onClick={() => setOpen(true)}>Удалить данные</button>
    <ConfirmDialog open={open} onOpenChange={setOpen} title="Подтвердите удаление" description="Действие необратимо" exactText="DELETE" onConfirm={vi.fn()} />
  </>;
}

function PreferredFocusHarness() {
  return <Dialog open onOpenChange={vi.fn()} title="Команды">
    <input data-dialog-autofocus aria-label="Поиск команд" />
    <button type="button">Действие</button>
  </Dialog>;
}

describe("Dialog", () => {
  it("isolates the modal, traps focus and restores the opener on Escape", async () => {
    const user = userEvent.setup();
    const { container } = render(<DialogHarness />);
    const opener = screen.getByRole("button", { name: "Открыть диалог" });

    await user.click(opener);
    const dialog = await screen.findByRole("dialog", { name: "Настройки" });
    const labelledBy = dialog.getAttribute("aria-labelledby");
    const describedBy = dialog.getAttribute("aria-describedby");

    expect(labelledBy).toBeTruthy();
    expect(labelledBy).not.toBe("dialog-title");
    expect(document.getElementById(labelledBy!)).toHaveTextContent("Настройки");
    expect(document.getElementById(describedBy!)).toHaveTextContent("Описание диалога");
    expect(container).toHaveAttribute("aria-hidden", "true");
    expect(container.inert).toBe(true);
    expect(document.body.style.overflow).toBe("hidden");

    const close = screen.getByRole("button", { name: "Закрыть" });
    const last = screen.getByRole("button", { name: "Последнее действие" });
    await waitFor(() => expect(close).toHaveFocus());
    last.focus();
    await user.tab();
    expect(close).toHaveFocus();
    await user.tab({ shift: true });
    expect(last).toHaveFocus();

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
    expect(container).not.toHaveAttribute("aria-hidden");
    expect(document.body.style.overflow).toBe("");
  });

  it("uses a different accessible title id for each dialog instance", async () => {
    const close = vi.fn();
    const view = render(<><Dialog open onOpenChange={close} title="Первый"><span /></Dialog><Dialog open={false} onOpenChange={close} title="Второй"><span /></Dialog></>);
    const firstId = (await screen.findByRole("dialog", { name: "Первый" })).getAttribute("aria-labelledby");
    view.rerender(<><Dialog open={false} onOpenChange={close} title="Первый"><span /></Dialog><Dialog open onOpenChange={close} title="Второй"><span /></Dialog></>);
    const secondId = (await screen.findByRole("dialog", { name: "Второй" })).getAttribute("aria-labelledby");
    expect(firstId).not.toBe(secondId);
  });

  it("focuses the explicitly preferred control", async () => {
    render(<PreferredFocusHarness />);
    await waitFor(() => expect(screen.getByLabelText("Поиск команд")).toHaveFocus());
  });
});

describe("ConfirmDialog", () => {
  it("clears typed confirmation after Cancel and after the close button", async () => {
    const user = userEvent.setup();
    render(<ConfirmHarness />);

    await user.click(screen.getByRole("button", { name: "Удалить данные" }));
    await user.type(await screen.findByLabelText("Текст подтверждения"), "DELETE");
    expect(screen.getByRole("button", { name: "Удалить" })).toBeEnabled();
    await user.click(screen.getByRole("button", { name: "Отмена" }));

    await user.click(screen.getByRole("button", { name: "Удалить данные" }));
    expect(await screen.findByLabelText("Текст подтверждения")).toHaveValue("");
    expect(screen.getByRole("button", { name: "Удалить" })).toBeDisabled();
    await user.type(screen.getByLabelText("Текст подтверждения"), "DELETE");
    await user.click(screen.getByRole("button", { name: "Закрыть" }));

    await user.click(screen.getByRole("button", { name: "Удалить данные" }));
    expect(await screen.findByLabelText("Текст подтверждения")).toHaveValue("");
    expect(screen.getByRole("button", { name: "Удалить" })).toBeDisabled();
  });
});
