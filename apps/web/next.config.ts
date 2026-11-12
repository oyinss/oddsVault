import type { NextConfig } from "next";

/**
 * The site is deployed as a static export to Cloudflare Pages. Every page is a
 * client component that reads the Go API through the same-origin `/svc` proxy
 * declared in `public/_worker.js`, so no Node server is required at runtime.
 */
const nextConfig: NextConfig = {
  reactStrictMode: true,
  poweredByHeader: false,
  output: "export",
  // Emit `events/index.html` rather than `events.html`. Cloudflare Pages serves
  // both, but directories keep the asset-server fallback unambiguous.
  trailingSlash: true,
  images: { unoptimized: true },
};

export default nextConfig;
