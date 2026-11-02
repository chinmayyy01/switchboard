import type { Metadata, Viewport } from "next";
import type { ReactNode } from "react";
import { Geist_Mono, Manrope } from "next/font/google";
import "./globals.css";

const manrope = Manrope({
  subsets: ["latin"],
  variable: "--font-manrope",
  display: "swap",
});

const geistMono = Geist_Mono({
  subsets: ["latin"],
  variable: "--font-geist-mono",
  display: "swap",
});

export const metadata: Metadata = {
  metadataBase: new URL(
    process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000",
  ),
  title: "Switchboard: AI voice receptionist for home services",
  description:
    "An AI voice receptionist for home services teams. It answers calls, books real jobs with staff availability, answers policy questions, and hands off to your team with the full transcript.",
  openGraph: {
    title: "Switchboard: AI voice receptionist for home services",
    description:
      "Every call answered. Every job booked. Try the live voice agent in your browser.",
    type: "website",
    siteName: "Switchboard",
  },
  twitter: {
    card: "summary_large_image",
    title: "Switchboard: AI voice receptionist for home services",
    description:
      "Every call answered. Every job booked. Try the live voice agent in your browser.",
  },
};

export const viewport: Viewport = {
  themeColor: "#111111",
  colorScheme: "dark",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en" className={`${manrope.variable} ${geistMono.variable}`}>
      <body className="min-h-screen bg-[#111111] font-sans text-[#EAE5DC] antialiased">
        {children}
      </body>
    </html>
  );
}
