"use client";

import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const focusableSelector = [
  "a[href]",
  "button:not([disabled])",
  "input:not([disabled])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  "[tabindex]:not([tabindex='-1'])",
].join(",");

function focusableElements(container: HTMLElement) {
  return Array.from(container.querySelectorAll<HTMLElement>(focusableSelector))
    .filter((element) => !element.hidden && element.getAttribute("aria-hidden") !== "true");
}

export function Dialog({ open, onOpenChange, title, description, children, className, contentClassName, placement = "center" }: { open: boolean; onOpenChange: (open: boolean) => void; title: string; description?: string; children: React.ReactNode; className?: string; contentClassName?: string; placement?: "center" | "left" }) {
  const [mounted, setMounted] = useState(false);
  const overlayRef = useRef<HTMLDivElement>(null);
  const dialogRef = useRef<HTMLDivElement>(null);
  const titleId = useId();
  const descriptionId = useId();

  useEffect(() => setMounted(true), []);

  useEffect(() => {
    if (!open || !mounted) return;

    const previouslyFocused = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const previousOverflow = document.body.style.overflow;
    const overlay = overlayRef.current;
    const background = Array.from(document.body.children)
      .filter((element): element is HTMLElement => element instanceof HTMLElement && element !== overlay && element.dataset.fitlogDialogOverlay !== "true")
      .map((element) => ({ element, inert: element.inert, ariaHidden: element.getAttribute("aria-hidden") }));

    document.body.style.overflow = "hidden";
    for (const { element } of background) {
      element.inert = true;
      element.setAttribute("aria-hidden", "true");
    }

    const focusTimer = window.setTimeout(() => {
      const dialog = dialogRef.current;
      if (!dialog) return;
      const preferred = dialog.querySelector<HTMLElement>("[data-dialog-autofocus]");
      (preferred ?? focusableElements(dialog)[0] ?? dialog).focus();
    }, 0);

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onOpenChange(false);
        return;
      }
      if (event.key !== "Tab") return;

      const dialog = dialogRef.current;
      if (!dialog) return;
      const focusable = focusableElements(dialog);
      if (!focusable.length) {
        event.preventDefault();
        dialog.focus();
        return;
      }

      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const active = document.activeElement;
      if (event.shiftKey && (active === first || !dialog.contains(active))) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && (active === last || !dialog.contains(active))) {
        event.preventDefault();
        first.focus();
      }
    };

    document.addEventListener("keydown", onKeyDown);
    return () => {
      window.clearTimeout(focusTimer);
      document.removeEventListener("keydown", onKeyDown);
      document.body.style.overflow = previousOverflow;
      for (const { element, inert, ariaHidden } of background) {
        element.inert = inert;
        if (ariaHidden === null) element.removeAttribute("aria-hidden");
        else element.setAttribute("aria-hidden", ariaHidden);
      }
      if (previouslyFocused?.isConnected) previouslyFocused.focus();
    };
  }, [mounted, onOpenChange, open]);

  if (!open || !mounted) return null;

  return createPortal(
    <div
      ref={overlayRef}
      data-fitlog-dialog-overlay="true"
      className={cn("fixed inset-0 z-[70] flex bg-black/70 backdrop-blur-sm", placement === "left" ? "items-stretch justify-start p-0" : "items-end justify-center p-0 sm:items-center sm:p-5")}
      onMouseDown={(event) => { if (event.currentTarget === event.target) onOpenChange(false); }}
    >
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={description ? descriptionId : undefined}
        tabIndex={-1}
        className={cn("overflow-y-auto border border-line bg-surface shadow-2xl", placement === "left" ? "h-[100dvh] max-h-none w-[min(86vw,280px)] rounded-none border-y-0 border-l-0" : "max-h-[94vh] w-full rounded-t-[20px] sm:max-w-2xl sm:rounded-card", className)}
      >
        <div className="sticky top-0 z-10 flex items-start justify-between gap-4 border-b border-line bg-surface/95 px-5 py-4 backdrop-blur">
          <div><h2 id={titleId} className="text-base font-semibold text-ink">{title}</h2>{description && <p id={descriptionId} className="mt-1 text-sm text-muted">{description}</p>}</div>
          <Button type="button" variant="ghost" size="icon" aria-label="Закрыть" onClick={() => onOpenChange(false)}><X className="size-4" /></Button>
        </div>
        <div className={cn("p-5", contentClassName)}>{children}</div>
      </div>
    </div>,
    document.body,
  );
}

export function DialogActions({ children }: { children: React.ReactNode }) {
  return <div className="mt-6 flex flex-col-reverse gap-2 border-t border-line pt-4 sm:flex-row sm:justify-end">{children}</div>;
}
