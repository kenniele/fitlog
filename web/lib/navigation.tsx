import { Activity, Apple, BarChart3, CalendarRange, Database, Dumbbell, LayoutDashboard, Ruler, Settings2 } from "lucide-react";

export const navigation = [
  { href: "/dashboard", label: "Обзор", icon: LayoutDashboard, exact: true },
  { href: "/dashboard/training", label: "Тренировки", icon: Dumbbell },
  { href: "/dashboard/recovery", label: "Восстановление", icon: Activity },
  { href: "/dashboard/nutrition", label: "Питание", icon: Apple },
  { href: "/dashboard/body", label: "Состав тела", icon: Ruler },
  { href: "/dashboard/analytics", label: "Аналитика", icon: BarChart3 },
  { href: "/dashboard/plans", label: "Программы", icon: CalendarRange },
  { href: "/dashboard/imports", label: "Импорт данных", icon: Database },
  { href: "/dashboard/settings", label: "Настройки", icon: Settings2 },
];

export const quickActions = [
  { href: "/dashboard/training?action=new", label: "Добавить тренировку" },
  { href: "/dashboard/recovery?action=new", label: "Записать восстановление" },
  { href: "/dashboard/nutrition?action=new", label: "Записать питание" },
  { href: "/dashboard/body?action=new", label: "Добавить измерение" },
  { href: "/dashboard/imports?action=new", label: "Импортировать файл" },
];
