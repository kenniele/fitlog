"use client";

import { useCallback, useEffect } from "react";

// Only user-initiated dismissal uses this guard. A successful save closes directly.
export function useEditorClose(open: boolean, dirty: boolean, busy: boolean, onOpenChange: (open: boolean) => void) {
  useEffect(() => {
    if (!open || (!dirty && !busy)) return;
    const preventLoss = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ""; };
    window.addEventListener("beforeunload", preventLoss);
    return () => window.removeEventListener("beforeunload", preventLoss);
  }, [open, dirty, busy]);
  return useCallback((next: boolean) => {
    if (!next && busy) return;
    if (!next && dirty && !window.confirm("Есть несохранённые изменения. Закрыть и удалить их?")) return;
    onOpenChange(next);
  }, [busy, dirty, onOpenChange]);
}
