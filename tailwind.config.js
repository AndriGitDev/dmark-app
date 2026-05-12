/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./internal/web/views/**/*.templ", "./internal/web/**/*.go"],
  theme: {
    extend: {
      colors: {
        background: "#0a0a0a",
        card: "#141414",
        foreground: "#e8e8e8",
        muted: "#9a9a9a",
        accent: "#4ad1c2",
        warn: "#e8a44a",
        fail: "#e85a4a",
      },
      fontFamily: {
        sans: ["Inter", "ui-sans-serif", "system-ui", "sans-serif"],
        mono: ["JetBrains Mono", "ui-monospace", "SFMono-Regular", "Menlo", "monospace"],
      },
    },
  },
  plugins: [],
};
