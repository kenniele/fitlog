import type { Config } from "tailwindcss";

const config: Config = {
  darkMode: ["class", "[data-theme='dark']"],
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}", "./lib/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        canvas: "var(--canvas)",
        surface: "var(--surface-1)",
        elevated: "var(--surface-2)",
        ink: "var(--text-primary)",
        muted: "var(--text-secondary)",
        accent: "var(--accent)",
        blue: "var(--accent-blue)",
        warning: "var(--warning)",
        critical: "var(--critical)",
        line: "var(--border)",
      },
      borderRadius: { card: "16px", control: "12px" },
      boxShadow: { glow: "0 0 40px rgba(124,255,178,.08)" },
    },
  },
  plugins: [],
};

export default config;
