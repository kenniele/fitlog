"use client";

import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";
import { rangeFromParams, rangeQuery } from "@/lib/range";

export function useRangeSearch() {
  const search = useSearchParams();
  const range = rangeFromParams(search);
  return { range, query: rangeQuery(range).toString(), search };
}

export function useQuickAction(open: () => void, action = "new") {
  const search = useSearchParams();
  const pathname = usePathname();
  const router = useRouter();
  useEffect(() => {
    if (search.get("action") !== action) return;
    open();
    const next = new URLSearchParams(search.toString());
    next.delete("action");
    router.replace(next.size ? `${pathname}?${next.toString()}` : pathname, { scroll: false });
  }, [action, open, pathname, router, search]);
}

export function useDebouncedValue<T>(value: T, delay = 300) {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const timeout = window.setTimeout(() => setDebounced(value), delay);
    return () => window.clearTimeout(timeout);
  }, [delay, value]);
  return debounced;
}

export function numberValue(value: string) { return value === "" ? undefined : Number(value); }
