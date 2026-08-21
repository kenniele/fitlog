"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense } from "react";
import { useForm } from "react-hook-form";
import { ShieldCheck } from "lucide-react";
import { z } from "zod";
import { apiFetch } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Field, Input } from "@/components/ui/field";
import { InlineError } from "@/components/ui/states";
import type { AuthSession } from "@/components/layout/auth-boundary";

const schema = z.object({ token: z.string().trim().min(8, "Вставьте полный токен доступа") });
type Values = z.infer<typeof schema>;

function LoginContent() {
  const router = useRouter();
  const search = useSearchParams();
  const client = useQueryClient();
  const form = useForm<Values>({ resolver: zodResolver(schema), defaultValues: { token: "" } });
  const login = useMutation({ mutationFn: (body: Values) => apiFetch<AuthSession>("/auth/session", { method: "POST", body }), onSuccess: (session) => { client.setQueryData(["auth-session"], session); const next = search.get("next"); router.replace(next?.startsWith("/dashboard") && next !== "/dashboard/login" ? next : "/dashboard"); } });
  return <main className="relative grid min-h-screen place-items-center overflow-hidden p-4"><div className="subtle-grid pointer-events-none absolute inset-0" /><Card className="relative w-full max-w-md p-6 shadow-glow sm:p-8"><div className="mb-7 flex items-center gap-3"><span className="grid size-11 place-items-center rounded-[14px] border border-accent/20 bg-accent/10"><ShieldCheck className="size-5 text-accent" /></span><div><p className="text-lg font-semibold tracking-tight">FitLog Control Center</p><p className="text-sm text-muted">Защищённое персональное пространство</p></div></div><form onSubmit={form.handleSubmit((values) => login.mutate(values))} className="space-y-4"><Field label="Токен доступа" error={form.formState.errors.token?.message} hint="Оператор задаёт статический секрет в FITLOG_DASHBOARD_TOKEN."><Input autoFocus autoComplete="current-password" type="password" {...form.register("token")} /></Field><InlineError error={login.error} /><Button type="submit" variant="primary" className="w-full" loading={login.isPending}>Войти</Button></form><p className="mt-5 text-xs leading-5 text-muted">Токен отправляется только на same-origin API. После проверки API устанавливает подписанную HttpOnly cookie.</p></Card></main>;
}

export default function LoginPage() { return <Suspense fallback={<main className="grid min-h-screen place-items-center text-sm text-muted">Загрузка…</main>}><LoginContent /></Suspense>; }
