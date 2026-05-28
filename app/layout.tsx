import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "dmark.is | Icelandic DMARC and SPF protection",
  description:
    "Scan Icelandic domains for DMARC and SPF risk, then unlock automated SPF flattening and readable DMARC reporting.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
