import type { Metadata, Viewport } from "next";
import "./globals.css";
import { Providers } from "./providers";

export const metadata: Metadata = {
  title: { default: "FitLog Control Center", template: "%s · FitLog" },
  description: "Персональный центр тренировок, восстановления, питания и состава тела.",
};

export const viewport: Viewport = { colorScheme: "dark", themeColor: "#07090C", width: "device-width", initialScale: 1 };

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="ru" data-theme="dark"><body><Providers>{children}</Providers></body></html>;
}
