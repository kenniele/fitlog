"use client";

import { useQuery } from "@tanstack/react-query";
import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";
import { apiFetch, APIError } from "@/lib/api";
import { ErrorState, PageSkeleton } from "@/components/ui/states";

export type AuthSession = { authenticated?: boolean; owner_id?: number | string; expires_at?: string | null };

export function AuthBoundary({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const login = pathname === "/dashboard/login";
  const query = useQuery({ queryKey: ["auth-session"], queryFn: () => apiFetch<AuthSession>("/auth/session"), retry: false, enabled: !login });
  useEffect(() => {
    if (!login && query.error instanceof APIError && query.error.status === 401) {
      const searchString = typeof window === "undefined" ? "" : window.location.search.slice(1);
      const next = searchString ? `${pathname}?${searchString}` : pathname;
      router.replace(`/dashboard/login?next=${encodeURIComponent(next)}`);
    }
  }, [login, pathname, query.error, router]);
  if (login) return children;
  if (query.isPending || (query.error instanceof APIError && query.error.status === 401)) return <div className="mx-auto max-w-7xl p-6"><PageSkeleton /></div>;
  if (query.isError) return <main className="mx-auto max-w-3xl p-6"><ErrorState error={query.error} retry={() => query.refetch()} /></main>;
  return children;
}
