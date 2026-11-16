#!/usr/bin/env node
//
// Local web server for the static export.
//
// The deployed site is served by a Cloudflare Pages worker that forwards /svc/*
// to the API origin. `python3 -m http.server` cannot do that, so a local export
// built with the default base answered every API call with 404 the moment a
// production build overwrote `out/`.
//
// This server makes the same production export work locally: it serves files
// from apps/web/out and reverse-proxies /svc/* and /api/* to the Go API.
//
//   node scripts/local-web.mjs [port]
//
import { createServer, request as httpRequest } from "node:http";
import { createReadStream, promises as fs } from "node:fs";
import { extname, join, normalize } from "node:path";

const ROOT = new URL("..", import.meta.url).pathname.replace(/\/$/, "");
const WEB_OUT = join(ROOT, "apps/web/out");
const PORT = Number(process.argv[2] ?? process.env.WEB_PORT ?? 3000);
const API = {
  host: "127.0.0.1",
  port: Number(process.env.API_PORT ?? 8080),
};

const MIME = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".mjs": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".txt": "text/plain; charset=utf-8",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".webp": "image/webp",
  ".ico": "image/x-icon",
  ".woff": "font/woff",
  ".woff2": "font/woff2",
  ".map": "application/json; charset=utf-8",
};

// /svc is the deployed prefix. /api is accepted too, so a build that talks to
// the API directly still works once out/ has been replaced by a prod build.
const API_PREFIXES = ["/svc/", "/api/"];

function proxy(req, res) {
  const stripSvc = req.url.startsWith("/svc/");
  const path = stripSvc ? req.url.slice(4) : req.url;

  const upstream = httpRequest(
    {
      host: API.host,
      port: API.port,
      method: req.method,
      path,
      headers: { ...req.headers, host: `${API.host}:${API.port}` },
    },
    (upstreamRes) => {
      res.writeHead(upstreamRes.statusCode ?? 502, upstreamRes.headers);
      upstreamRes.pipe(res);
    },
  );

  upstream.on("error", () => {
    if (!res.headersSent) {
      res.writeHead(502, { "content-type": "text/plain; charset=utf-8" });
    }
    res.end("API unreachable on 127.0.0.1:" + API.port);
  });

  req.pipe(upstream);
}

async function serveFile(req, res, pathname) {
  const clean = normalize(decodeURIComponent(pathname)).replace(/^(\.\.[/\\])+/, "");
  let file = join(WEB_OUT, clean);
  if (!file.startsWith(WEB_OUT)) {
    res.writeHead(403).end("forbidden");
    return;
  }

  let stat = await fs.stat(file).catch(() => null);
  if (stat?.isDirectory()) {
    file = join(file, "index.html");
    stat = await fs.stat(file).catch(() => null);
  }
  if (!stat) {
    // A directory path without the trailing slash, e.g. /corners
    const withSlash = join(WEB_OUT, clean, "index.html");
    const fallback = await fs.stat(withSlash).catch(() => null);
    if (fallback) {
      res.writeHead(301, { location: pathname + "/" }).end();
      return;
    }
    res.writeHead(404, { "content-type": "text/plain; charset=utf-8" });
    res.end("not found: " + pathname);
    return;
  }

  res.writeHead(200, {
    "content-type": MIME[extname(file).toLowerCase()] ?? "application/octet-stream",
    "content-length": stat.size,
    "cache-control": "no-cache",
  });
  if (req.method === "HEAD") {
    res.end();
    return;
  }
  createReadStream(file).pipe(res);
}

createServer((req, res) => {
  const url = req.url ?? "/";
  if (API_PREFIXES.some((prefix) => url.startsWith(prefix))) {
    proxy(req, res);
    return;
  }
  serveFile(req, res, url.split("?")[0]).catch(() => {
    if (!res.headersSent) res.writeHead(500);
    res.end("error");
  });
}).listen(PORT, "127.0.0.1", () => {
  console.log(`web on http://localhost:${PORT} (api proxy -> 127.0.0.1:${API.port})`);
});
