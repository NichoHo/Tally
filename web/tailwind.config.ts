import type { Config } from "tailwindcss";

// Design tokens from CLAUDE.md section 12: calm, restrained, one accent.
const config: Config = {
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        page: "#FAFAF8", // warm near-white background
        card: "#FFFFFF",
        line: "#E7E7E2", // hairline borders
        ink: "#1A1A18", // primary text
        muted: "#6B6B66", // secondary text
        accent: { DEFAULT: "#1D7A8C", dark: "#155E6C" },
        ok: { DEFAULT: "#1D9E75", dark: "#136B4F", tint: "#E7F5EF" },
        warn: { DEFAULT: "#BA7517", dark: "#8A5611", tint: "#F8F0E3" },
        danger: { DEFAULT: "#DC2626", dark: "#A31B1B", tint: "#FBEAEA" },
      },
      borderRadius: {
        card: "12px",
        control: "8px",
      },
      fontFamily: {
        sans: ["var(--font-inter)", "system-ui", "sans-serif"],
      },
    },
  },
  plugins: [],
};

export default config;
