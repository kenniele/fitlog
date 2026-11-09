"use client";

import { flexRender, getCoreRowModel, type ColumnDef, useReactTable } from "@tanstack/react-table";
import { EmptyState } from "@/components/ui/states";

export function DataTable<T>({ data, columns, emptyTitle, emptyDescription, rowKey }: { data: T[]; columns: ColumnDef<T>[]; emptyTitle?: string; emptyDescription?: string; rowKey?: (row: T) => string }) {
  // Every Control Center list is server-paginated. Client sorting would only
  // reorder the visible slice and falsely imply a globally sorted dataset.
  const table = useReactTable({ data, columns, enableSorting: false, getCoreRowModel: getCoreRowModel(), getRowId: rowKey });
  if (!data.length) return <EmptyState title={emptyTitle} description={emptyDescription ?? "Здесь появятся ваши сохранённые записи."} />;
  return <><div className="divide-y divide-line sm:hidden">{table.getRowModel().rows.map((row) => {
    const cells = row.getVisibleCells();
    const values = cells.filter((cell) => cell.column.id !== "actions");
    const actions = cells.find((cell) => cell.column.id === "actions");
    const first = values[0];
    return <article key={row.id} className="space-y-3 p-4">
      <div className="flex items-center justify-between gap-2"><div className="min-w-0 break-words text-sm font-medium">{first && flexRender(first.column.columnDef.cell, first.getContext())}</div>{actions && <div className="shrink-0">{flexRender(actions.column.columnDef.cell, actions.getContext())}</div>}</div>
      <dl className="grid grid-cols-2 gap-x-4 gap-y-3">{values.slice(1).map((cell) => <div key={cell.id} className="min-w-0 break-words"><dt className="text-xs text-muted">{typeof cell.column.columnDef.header === "string" ? cell.column.columnDef.header : cell.column.id}</dt><dd className="mt-1 text-sm tabular-nums">{flexRender(cell.column.columnDef.cell, cell.getContext())}</dd></div>)}</dl>
    </article>;
  })}</div><div className="hidden w-full overflow-x-auto scrollbar-thin sm:block"><table className="w-full min-w-[680px] border-collapse text-left text-sm tabular-nums">
    <thead><tr className="border-b border-line">{table.getHeaderGroups()[0]?.headers.map((header) => <th key={header.id} className="px-4 py-3 text-[11px] font-semibold uppercase tracking-[.08em] text-muted">{header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}</th>)}</tr></thead>
    <tbody>{table.getRowModel().rows.map((row) => <tr key={row.id} className="border-b border-line/70 transition last:border-0 hover:bg-ink/[.025]">{row.getVisibleCells().map((cell) => <td key={cell.id} className="px-4 py-4 align-middle text-ink">{flexRender(cell.column.columnDef.cell, cell.getContext())}</td>)}</tr>)}</tbody>
  </table></div></>;
}
