import type { Config } from "tailwindcss";

const token = (name: string) =>
  `color-mix(in srgb, var(--${name}) calc(<alpha-value> * 100%), transparent)`;

const config: Config = {
  darkMode: ["class", "[data-theme='dark']"],
  content: [
    "./app/**/*.{ts,tsx}",
    "./components/**/*.{ts,tsx}",
    "./lib/**/*.{ts,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        canvas: token("canvas"),
        surface: token("surface-1"),
        elevated: token("surface-2"),
        ink: token("text-primary"),
        muted: token("text-secondary"),
        accent: token("accent"),
        blue: token("accent-blue"),
        warning: token("warning"),
        critical: token("critical"),
        line: token("border"),
      },
      borderRadius: {
        card: "var(--radius-surface)",
        control: "var(--radius-control)",
      },
      boxShadow: { glow: "0 0 40px rgba(124,255,178,.08)" },
    },
  },
  plugins: [],
};

export default config;
