"use client";

import { useId, useRef } from "react";

export const bodySegments = [
  {
    key: "right_arm",
    label: "Правая рука",
    path: "M94 94 C80 94 69 106 65 125 L56 167 C52 179 48 192 46 207 L38 240 C35 249 38 257 43 258 L47 250 L49 257 Q53 259 55 253 L59 239 L67 216 L78 183 C83 172 84 160 86 147 L99 116 Z",
  },
  {
    key: "left_arm",
    label: "Левая рука",
    path: "M166 94 C180 94 191 106 195 125 L204 167 C208 179 212 192 214 207 L222 240 C225 249 222 257 217 258 L213 250 L211 257 Q207 259 205 253 L201 239 L193 216 L182 183 C177 172 176 160 174 147 L161 116 Z",
  },
  {
    key: "trunk",
    label: "Корпус",
    path: "M116 75 L116 86 Q109 92 94 94 L88 110 Q87 129 97 152 L103 189 Q103 211 96 230 L91 262 Q109 276 130 272 Q151 276 169 262 L164 230 Q157 211 157 189 L163 152 Q173 129 172 110 L166 94 Q151 92 144 86 L144 75 Z",
  },
  {
    key: "right_leg",
    label: "Правая нога",
    path: "M91 263 Q109 273 130 273 L126 324 L122 358 Q124 376 119 401 L111 452 L112 465 Q110 472 96 474 L79 475 Q74 472 80 465 L91 451 L90 416 Q86 393 90 372 L91 355 Q82 324 88 289 Z",
  },
  {
    key: "left_leg",
    label: "Левая нога",
    path: "M169 263 Q151 273 130 273 L134 324 L138 358 Q136 376 141 401 L149 452 L148 465 Q150 472 164 474 L181 475 Q186 472 180 465 L169 451 L170 416 Q174 393 170 372 L169 355 Q178 324 172 289 Z",
  },
] as const;
export type BodyRegion = (typeof bodySegments)[number]["key"] | "all";
export function SegmentedBody({
  selected,
  onSelect,
}: {
  selected: BodyRegion;
  onSelect: (region: BodyRegion) => void;
}) {
  const id = useId();
  const regions = useRef(new Map<string, SVGGElement>());
  return (
    <svg
      viewBox="0 0 260 500"
      className="mx-auto h-[360px] w-full max-w-[260px] sm:h-[420px]"
      role="group"
      aria-label="Сегменты тела, вид спереди"
    >
      <defs>
        <linearGradient id={`${id}-body`} x1="0" y1="0" x2="1" y2=".2">
          <stop stopColor="var(--body-dark)" />
          <stop offset=".38" stopColor="var(--body-light)" />
          <stop offset=".7" stopColor="var(--body-mid)" />
          <stop offset="1" stopColor="var(--body-dark)" />
        </linearGradient>
        <linearGradient id={`${id}-head`} x1="0" y1="0" x2="1" y2=".25">
          <stop stopColor="var(--body-dark)" />
          <stop offset=".5" stopColor="var(--body-light)" />
          <stop offset="1" stopColor="var(--body-mid)" />
        </linearGradient>
      </defs>
      <ellipse
        cx="130"
        cy="483"
        rx="57"
        ry="3"
        fill="var(--border)"
        aria-hidden="true"
      />
      <path
        d="M107 40 Q106 14 130 14 Q154 14 153 40 L155 48 Q157 58 150 60 Q145 79 130 83 Q115 79 110 60 Q103 58 105 48 Z"
        fill={`url(#${id}-head)`}
        aria-hidden="true"
      />
      {bodySegments.map((segment, index) => (
        <g
          key={segment.key}
          ref={(node) => {
            if (node) regions.current.set(segment.key, node);
          }}
          role="button"
          aria-label={`Выбрать: ${segment.label}`}
          aria-pressed={selected === segment.key}
          tabIndex={0}
          className="body-region"
          fill={`url(#${id}-body)`}
          onClick={() => onSelect(segment.key)}
          onKeyDown={(event) => {
            if (event.key === "Enter" || event.key === " ") {
              event.preventDefault();
              onSelect(segment.key);
            }
            if (
              ["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown"].includes(
                event.key,
              )
            ) {
              event.preventDefault();
              const offset = ["ArrowLeft", "ArrowUp"].includes(event.key)
                ? -1
                : 1;
              regions.current
                .get(bodySegments[(index + offset + 5) % 5].key)
                ?.focus();
            }
          }}
        >
          <path d={segment.path} />
        </g>
      ))}
      <g
        fill="none"
        stroke="var(--body-dark)"
        strokeWidth="1"
        opacity=".35"
        pointerEvents="none"
        aria-hidden="true"
      >
        <path d="M99 106 Q116 105 128 117 M161 106 Q144 105 132 117 M130 125 L130 191 M102 149 Q115 155 126 148 M158 149 Q145 155 134 148 M110 230 Q130 238 150 230 M98 287 Q101 323 107 340 M162 287 Q159 323 153 340 M96 363 Q104 367 118 362 M164 363 Q156 367 142 362" />
      </g>
    </svg>
  );
}
