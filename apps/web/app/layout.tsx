import type { Metadata, Viewport } from "next";
import "@fortawesome/fontawesome-svg-core/styles.css";

import "./globals.css";

export const metadata: Metadata = {
  metadataBase: new URL("https://oddsvault-web.pages.dev"),
  title: "OddsVault — Sports Odds Intelligence",
  description:
    "Timestamped bookmaker odds, verified results and versioned settlement, kept as immutable history.",
  manifest: "/manifest.webmanifest",
  openGraph: {
    type: "website",
    url: "/",
    siteName: "OddsVault",
    title: "OddsVault — Sports Odds Intelligence",
    description:
      "Timestamped bookmaker odds, verified results and versioned settlement, kept as immutable history.",
    images: [
      {
        url: "/og.png",
        width: 1200,
        height: 630,
        alt: "OddsVault",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "OddsVault — Sports Odds Intelligence",
    description:
      "Timestamped bookmaker odds, verified results and versioned settlement, kept as immutable history.",
    images: ["/og.png"],
  },
};

export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#f6f8fc" },
    { media: "(prefers-color-scheme: dark)", color: "#05070d" },
  ],
};

/**
 * Runs before paint so a light-mode visitor never sees a dark flash. The
 * `?theme=` override exists for screenshots and shared links; it does not
 * persist. The toggle stores the real choice in localStorage.
 */
const THEME_SCRIPT = `(function () {
  try {
    var stored = window.localStorage.getItem("oddsvault-theme");
    var override = new URLSearchParams(window.location.search).get("theme");
    var mode = override === "light" || override === "dark" ? override : stored;
    if (mode !== "light" && mode !== "dark") mode = "system";
    var dark = mode === "dark" || (mode === "system" && window.matchMedia("(prefers-color-scheme: dark)").matches);
    var root = document.documentElement;
    root.classList.toggle("light", !dark);
    root.style.colorScheme = dark ? "dark" : "light";
  } catch (error) {}
})();`;

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body className="min-h-dvh antialiased">
        <script dangerouslySetInnerHTML={{ __html: THEME_SCRIPT }} />
        {children}
      </body>
    </html>
  );
}
