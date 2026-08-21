"use client";

import { flexRender, getCoreRowModel, type ColumnDef, useReactTable } from "@tanstack/react-table";
import { EmptyState } from "@/components/ui/states";

export function DataTable<T>({ data, columns, emptyTitle, rowKey }: { data: T[]; columns: ColumnDef<T>[]; emptyTitle?: string; rowKey?: (row: T) => string }) {
  // Every Control Center list is server-paginated. Client sorting would only
  // reorder the visible slice and falsely imply a globally sorted dataset.
  const table = useReactTable({ data, columns, enableSorting: false, getCoreRowModel: getCoreRowModel(), getRowId: rowKey });
  if (!data.length) return <EmptyState title={emptyTitle} />;
  return <div className="w-full overflow-x-auto scrollbar-thin"><table className="w-full min-w-[680px] border-collapse text-left text-sm">
    <thead><tr className="border-b border-line">{table.getHeaderGroups()[0]?.headers.map((header) => <th key={header.id} className="px-4 py-3 text-[11px] font-semibold uppercase tracking-[.08em] text-muted">{header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}</th>)}</tr></thead>
    <tbody>{table.getRowModel().rows.map((row) => <tr key={row.id} className="border-b border-line/70 transition last:border-0 hover:bg-white/[.025]">{row.getVisibleCells().map((cell) => <td key={cell.id} className="px-4 py-3 align-middle text-ink">{flexRender(cell.column.columnDef.cell, cell.getContext())}</td>)}</tr>)}</tbody>
  </table></div>;
}
