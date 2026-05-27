/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./internal/web/views/**/*.templ", "./internal/web/**/*.go"],
  theme: {
    extend: {
      colors: {
        background: "#FFFFFF",
        subtle: "#FAFAFA",
        card: "#FFFFFF",
        foreground: "#0F172A",
        muted: "#64748B",
        accent: "#10B981",
        "accent-hover": "#059669",
        yellow: "#F59E0B",
        warn: "#F59E0B",
        fail: "#EF4444",
        border: "#E2E8F0",
        ink: "#0F172A",
      },
      fontFamily: {
        sans: ["Inter", "ui-sans-serif", "system-ui", "sans-serif"],
        mono: ["JetBrains Mono", "ui-monospace", "SFMono-Regular", "Menlo", "monospace"],
      },
    },
  },
  plugins: [],
};
