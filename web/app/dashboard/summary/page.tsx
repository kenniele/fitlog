import type { Metadata } from "next";
import { CompactSummary } from "@/components/summary/compact-summary";

export const metadata: Metadata = { title: "Сводка" };

export default function SummaryPage() {
  return <CompactSummary />;
}
