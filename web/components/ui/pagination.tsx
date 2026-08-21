"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import type { ListResponse } from "@/lib/api";
import { Button } from "@/components/ui/button";

export const PAGE_SIZE = 25;

export function Pagination<T>({
  data,
  page,
  onPageChange,
  disabled = false,
}: {
  data: ListResponse<T> | undefined;
  page: number;
  onPageChange: (page: number) => void;
  disabled?: boolean;
}) {
  const currentPage = data?.page ?? page;
  const pageSize = Math.max(1, data?.page_size ?? PAGE_SIZE);
  const total = Math.max(0, data?.total ?? data?.items?.length ?? 0);
  const pageCount = Math.max(1, Math.ceil(total / pageSize));

  return (
    <nav className="flex flex-wrap items-center justify-between gap-3 border-t border-line px-4 py-3" aria-label="Пагинация списка">
      <p className="text-xs text-muted">
        Страница {currentPage} из {pageCount} · {total} записей
      </p>
      <div className="flex items-center gap-2">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          disabled={disabled || currentPage <= 1}
          onClick={() => onPageChange(Math.max(1, currentPage - 1))}
        >
          <ChevronLeft className="size-4" />
          Назад
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          disabled={disabled || currentPage >= pageCount}
          onClick={() => onPageChange(Math.min(pageCount, currentPage + 1))}
        >
          Далее
          <ChevronRight className="size-4" />
        </Button>
      </div>
    </nav>
  );
}
