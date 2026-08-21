import { Activity, Apple, BarChart3, CalendarRange, Database, Dumbbell, LayoutDashboard, Ruler, Settings2 } from "lucide-react";

export const navigation = [
  { href: "/dashboard", label: "Overview", icon: LayoutDashboard, exact: true },
  { href: "/dashboard/training", label: "Training", icon: Dumbbell },
  { href: "/dashboard/recovery", label: "Recovery", icon: Activity },
  { href: "/dashboard/nutrition", label: "Nutrition", icon: Apple },
  { href: "/dashboard/body", label: "Body", icon: Ruler },
  { href: "/dashboard/analytics", label: "Analytics", icon: BarChart3 },
  { href: "/dashboard/plans", label: "Plans", icon: CalendarRange },
  { href: "/dashboard/imports", label: "Data Imports", icon: Database },
  { href: "/dashboard/settings", label: "Settings", icon: Settings2 },
];

export const quickActions = [
  { href: "/dashboard/training?action=new", label: "Добавить тренировку" },
  { href: "/dashboard/recovery?action=new", label: "Записать восстановление" },
  { href: "/dashboard/nutrition?action=new", label: "Записать питание" },
  { href: "/dashboard/body?action=new", label: "Добавить измерение" },
  { href: "/dashboard/imports?action=new", label: "Импортировать файл" },
];
