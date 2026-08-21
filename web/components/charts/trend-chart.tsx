"use client";

import { Area, AreaChart, Bar, BarChart, CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { EmptyState } from "@/components/ui/states";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import type { SeriesPoint } from "@/lib/types";

export type ChartSeries = { key: string; label: string; color?: string; type?: "line" | "area" | "bar"; strokeDasharray?: string };

export function TrendChart({ title, description, data, series, height = 280, variant = "line" }: { title: string; description?: string; data?: SeriesPoint[] | null; series: ChartSeries[]; height?: number; variant?: "line" | "area" | "bar" }) {
  const points = Array.isArray(data) ? data : [];
  return <Card className="min-w-0"><CardHeader><div><CardTitle>{title}</CardTitle>{description && <CardDescription>{description}</CardDescription>}</div><div className="flex flex-wrap justify-end gap-3">{series.map((item, index) => <span key={item.key} className="flex items-center gap-1.5 text-[11px] text-muted"><span className="size-1.5 rounded-full" style={{ background: item.color ?? (index ? "var(--accent-blue)" : "var(--accent)") }} />{item.label}</span>)}</div></CardHeader><CardContent className="pt-2">
    {!points.length ? <EmptyState description="В выбранном периоде нет точек для графика." /> : <div style={{ height }} className="w-full min-w-0" aria-label={title} role="img"><ResponsiveContainer width="100%" height="100%">
      {variant === "bar" ? <BarChart data={points} margin={{ top: 8, right: 4, left: -20, bottom: 0 }}><CartesianGrid vertical={false} stroke="var(--border)" /><XAxis dataKey="date" tick={{ fill: "var(--text-secondary)", fontSize: 11 }} axisLine={false} tickLine={false} minTickGap={28} /><YAxis tick={{ fill: "var(--text-secondary)", fontSize: 11 }} axisLine={false} tickLine={false} /><Tooltip contentStyle={{ background: "var(--surface-2)", border: "1px solid var(--border)", borderRadius: 12, fontSize: 12 }} />{series.map((item, index) => <Bar key={item.key} dataKey={item.key} name={item.label} fill={item.color ?? (index ? "var(--accent-blue)" : "var(--accent)")} radius={[5, 5, 0, 0]} />)}</BarChart>
      : variant === "area" ? <AreaChart data={points} margin={{ top: 8, right: 4, left: -20, bottom: 0 }}><defs><linearGradient id={`gradient-${series[0]?.key}`} x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stopColor="var(--accent)" stopOpacity={0.26} /><stop offset="100%" stopColor="var(--accent)" stopOpacity={0} /></linearGradient></defs><CartesianGrid vertical={false} stroke="var(--border)" /><XAxis dataKey="date" tick={{ fill: "var(--text-secondary)", fontSize: 11 }} axisLine={false} tickLine={false} minTickGap={28} /><YAxis tick={{ fill: "var(--text-secondary)", fontSize: 11 }} axisLine={false} tickLine={false} /><Tooltip contentStyle={{ background: "var(--surface-2)", border: "1px solid var(--border)", borderRadius: 12, fontSize: 12 }} />{series.map((item, index) => <Area key={item.key} type="monotone" dataKey={item.key} name={item.label} stroke={item.color ?? (index ? "var(--accent-blue)" : "var(--accent)")} fill={index ? "transparent" : `url(#gradient-${series[0]?.key})`} strokeWidth={2} connectNulls={false} />)}</AreaChart>
      : <LineChart data={points} margin={{ top: 8, right: 4, left: -20, bottom: 0 }}><CartesianGrid vertical={false} stroke="var(--border)" /><XAxis dataKey="date" tick={{ fill: "var(--text-secondary)", fontSize: 11 }} axisLine={false} tickLine={false} minTickGap={28} /><YAxis tick={{ fill: "var(--text-secondary)", fontSize: 11 }} axisLine={false} tickLine={false} /><Tooltip contentStyle={{ background: "var(--surface-2)", border: "1px solid var(--border)", borderRadius: 12, fontSize: 12 }} />{series.map((item, index) => <Line key={item.key} type="monotone" dataKey={item.key} name={item.label} stroke={item.color ?? (index ? "var(--accent-blue)" : "var(--accent)")} strokeDasharray={item.strokeDasharray} strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls={false} />)}</LineChart>}
    </ResponsiveContainer></div>}
  </CardContent></Card>;
}

export function MiniTrend({ data, dataKey = "value", color = "var(--accent)" }: { data?: SeriesPoint[] | null; dataKey?: string; color?: string }) {
  const points = Array.isArray(data) ? data : [];
  if (!points.length) return <div className="h-8" />;
  return <div className="h-8 w-24" aria-hidden><ResponsiveContainer width="100%" height="100%"><LineChart data={points}><Line type="monotone" dataKey={dataKey} dot={false} stroke={color} strokeWidth={1.8} connectNulls={false} /></LineChart></ResponsiveContainer></div>;
}
