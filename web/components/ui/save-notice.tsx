"use client";
import { useEffect, useState } from "react";
export function notifySaved(message = "Запись сохранена") {
  window.dispatchEvent(new CustomEvent("fitlog:saved", { detail: message }));
}
export function SaveNotice() {
  const [notice, setNotice] = useState<{ message: string; id: number } | null>(null);
  useEffect(() => {
    const show = (event: Event) => setNotice({ message: (event as CustomEvent<string>).detail, id: Date.now() });
    window.addEventListener("fitlog:saved", show);
    return () => window.removeEventListener("fitlog:saved", show);
  }, []);
  useEffect(() => {
    if (!notice) return;
    const timer = window.setTimeout(() => setNotice(null), 5000);
    return () => window.clearTimeout(timer);
  }, [notice]);
  return <div role="status" aria-live="polite" aria-atomic="true" className="pointer-events-none fixed inset-x-4 bottom-6 z-[90] flex justify-center">{notice && <p key={notice.id} className="rounded-control border border-line bg-elevated px-5 py-3 text-sm text-ink">{notice.message}</p>}</div>;
}
