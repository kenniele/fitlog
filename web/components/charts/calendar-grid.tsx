"use client";

import { useEffect, useId, useRef, useState } from "react";
import type { CSSProperties, ReactNode } from "react";
import { monthNames } from "@/lib/dashboard";
import { formatDate } from "@/lib/format";
import { Dialog } from "@/components/ui/dialog";

export function CalendarGrid({
  dates,
  today,
  label,
  color,
  valueLabel,
  details,
  previewDetails,
  focusDate,
}: {
  dates: string[];
  today: string;
  label: string;
  color: (date: string) => string | undefined;
  valueLabel: (date: string) => string;
  details?: (date: string) => ReactNode;
  previewDetails?: (date: string) => ReactNode;
  focusDate?: string;
}) {
  const tooltipId = useId();
  const scrollRef = useRef<HTMLDivElement>(null);
  const buttons = useRef(new Map<string, HTMLButtonElement>());
  const [active, setActive] = useState<string | null>(null);
  const [preview, setPreview] = useState<{
    date: string;
    left: number;
    top: number;
  } | null>(null);
  const [focused, setFocused] = useState<string | null>(null);
  const padding = dates[0]
    ? (new Date(`${dates[0]}T00:00:00Z`).getUTCDay() + 6) % 7
    : 0;
  const columns = Math.ceil((dates.length + padding) / 7);
  const initial =
    focusDate && dates.includes(focusDate)
      ? focusDate
      : dates.includes(today)
        ? today
        : dates.findLast((date) => date <= today);
  const tabStop = focused && dates.includes(focused) ? focused : initial;
  useEffect(() => {
    const scroller = scrollRef.current;
    const button = initial ? buttons.current.get(initial) : null;
    if (scroller && button && window.matchMedia("(max-width: 639px)").matches) {
      scroller.scrollLeft =
        button.getBoundingClientRect().left -
        scroller.getBoundingClientRect().left +
        scroller.scrollLeft -
        scroller.clientWidth / 2;
    }
  }, [initial]);
  function show(date: string, element: HTMLButtonElement) {
    const rect = element.getBoundingClientRect();
    setPreview({
      date,
      left: Math.max(12, Math.min(rect.left - 100, window.innerWidth - 284)),
      top: Math.max(12, Math.min(rect.bottom + 10, window.innerHeight - 260)),
    });
  }
  return (
    <div className="relative">
      <div
        ref={scrollRef}
        className="scrollbar-thin overflow-x-auto px-1 pb-3 pt-1"
        onScroll={() => setPreview(null)}
      >
        <div
          className="calendar-grid-shell"
          style={
            {
              "--weeks": columns,
              maxWidth: columns < 12 ? columns * 36 + 32 : undefined,
            } as CSSProperties
          }
        >
          <div
            aria-hidden
            className="ml-8 grid h-8 text-xs text-muted"
            style={{
              gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
            }}
          >
            {dates
              .filter((date, i) => i === 0 || date.endsWith("-01"))
              .map((date) => (
                <span
                  key={date}
                  className="whitespace-nowrap"
                  style={{
                    gridColumn:
                      Math.floor((dates.indexOf(date) + padding) / 7) + 1,
                  }}
                >
                  {monthNames[Number(date.slice(5, 7)) - 1].slice(0, 3)}
                </span>
              ))}
          </div>
          <div className="flex gap-3">
            <div
              aria-hidden
              className="grid w-5 shrink-0 grid-rows-7 items-center gap-1 text-[10px] text-muted"
            >
              <span>Пн</span>
              <span />
              <span>Ср</span>
              <span />
              <span>Пт</span>
              <span />
              <span>Вс</span>
            </div>
            <div
              role="group"
              aria-label={label}
              className="grid min-w-0 flex-1 grid-flow-col grid-rows-7 gap-1"
              style={{
                gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
              }}
              onMouseLeave={() => setPreview(null)}
            >
              {Array.from({ length: padding }, (_, index) => (
                <span key={`pad-${index}`} />
              ))}
              {dates.map((date, index) =>
                date > today ? (
                  <span
                    key={date}
                    className="heat-cell"
                    data-future="true"
                    aria-label={`${formatDate(date)}: будущая дата`}
                  />
                ) : (
                  <button
                    key={date}
                    ref={(node) => {
                      if (node) buttons.current.set(date, node);
                      else buttons.current.delete(date);
                    }}
                    type="button"
                    className="heat-cell"
                    style={{ background: color(date) }}
                    data-today={date === today}
                    aria-label={`${formatDate(date)}: ${valueLabel(date)}`}
                    aria-pressed={active === date}
                    aria-describedby={
                      preview?.date === date ? tooltipId : undefined
                    }
                    tabIndex={date === tabStop ? 0 : -1}
                    onPointerEnter={(event) => {
                      if (event.pointerType === "mouse")
                        show(date, event.currentTarget);
                    }}
                    onFocus={(event) => {
                      setFocused(date);
                      show(date, event.currentTarget);
                    }}
                    onBlur={() => setPreview(null)}
                    onClick={() => {
                      setPreview(null);
                      setActive(date);
                    }}
                    onKeyDown={(event) => {
                      if (event.key === "Escape") {
                        setPreview(null);
                        return;
                      }
                      const offset = {
                        ArrowDown: 1,
                        ArrowUp: -1,
                        ArrowRight: 7,
                        ArrowLeft: -7,
                      }[event.key];
                      const next =
                        event.key === "Home"
                          ? 0
                          : event.key === "End"
                            ? dates.findLastIndex((day) => day <= today)
                            : offset !== undefined
                              ? index + offset
                              : null;
                      if (next === null) return;
                      event.preventDefault();
                      const day =
                        dates[Math.max(0, Math.min(next, dates.length - 1))];
                      if (day <= today) buttons.current.get(day)?.focus();
                    }}
                  />
                ),
              )}
            </div>
          </div>
        </div>
      </div>
      <p className="mt-1 text-xs text-muted sm:hidden">
        Листайте календарь · нажмите день для подробностей
      </p>
      {preview && !active && (
        <div
          role="tooltip"
          id={tooltipId}
          className="pointer-events-none fixed z-50 hidden w-[272px] rounded-card border border-line bg-elevated p-4 text-xs shadow-[var(--shadow-popover)] sm:block"
          style={{ left: preview.left, top: preview.top }}
        >
          <p className="font-medium">{formatDate(preview.date)}</p>
          {previewDetails ? (
            previewDetails(preview.date)
          ) : (
            <p className="mt-2 text-muted">{valueLabel(preview.date)}</p>
          )}
          <p className="mt-3 text-muted">Нажмите для подробностей</p>
        </div>
      )}
      <Dialog
        open={active !== null}
        onOpenChange={(open) => {
          if (!open) setActive(null);
        }}
        title={active ? formatDate(active) : "День"}
        description="Записанные показатели за этот день"
        className="sm:max-w-lg"
      >
        {active &&
          (details ? (
            details(active)
          ) : (
            <p className="py-4 text-lg">{valueLabel(active)}</p>
          ))}
      </Dialog>
    </div>
  );
}
