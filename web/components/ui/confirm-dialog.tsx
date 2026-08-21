"use client";

import { useState } from "react";
import { Dialog, DialogActions } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/field";
import { InlineError } from "@/components/ui/states";

type ConfirmDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  confirmText?: string;
  exactText?: string;
  onConfirm: () => void;
  busy?: boolean;
  error?: unknown;
};

function Confirmation({ exactText, confirmText, onCancel, onConfirm, busy, error }: Pick<ConfirmDialogProps, "exactText" | "confirmText" | "onConfirm" | "busy" | "error"> & { onCancel: () => void }) {
  const [typed, setTyped] = useState("");
  const allowed = !exactText || typed === exactText;

  return <>
    {exactText && <div className="space-y-2"><p className="text-sm text-muted">Введите <strong className="font-semibold text-ink">{exactText}</strong>, чтобы подтвердить.</p><Input aria-label="Текст подтверждения" value={typed} onChange={(event) => setTyped(event.target.value)} autoComplete="off" /></div>}
    <InlineError error={error} />
    <DialogActions><Button type="button" variant="ghost" onClick={onCancel}>Отмена</Button><Button type="button" variant="danger" loading={busy} disabled={!allowed} onClick={onConfirm}>{confirmText ?? "Удалить"}</Button></DialogActions>
  </>;
}

export function ConfirmDialog({ open, onOpenChange, title, description, confirmText, exactText, onConfirm, busy, error }: ConfirmDialogProps) {
  return <Dialog open={open} onOpenChange={onOpenChange} title={title} description={description} className="sm:max-w-md">
    <Confirmation exactText={exactText} confirmText={confirmText} onCancel={() => onOpenChange(false)} onConfirm={onConfirm} busy={busy} error={error} />
  </Dialog>;
}
