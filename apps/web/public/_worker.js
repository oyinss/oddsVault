/**
 * Cloudflare Pages advanced-mode worker.
 *
 * The Next.js app is a static export, so this worker owns every request. It
 * serves the exported assets, keeps the personal bet journal behind a
 * passphrase session, and proxies `/svc/*` to the Go API on the same origin.
 *
 * The journal is protected in two places:
 *   - here, with a signed session cookie, so visitors never get the page;
 *   - in the Go API (`JOURNAL_TOKEN`), because the tunnel URL is public and
 *     would otherwise expose `/api/v1/my-bets` directly.
 *
 * Deployed as `out/_worker.js` (copied from `public/` by `next build`).
 */
const API_PREFIX = "/svc";
const SESSION_COOKIE = "ov_journal";
const SESSION_TTL_MS = 7 * 24 * 60 * 60 * 1000;

export default {
  async fetch(request, env) {
    const url = new URL(request.url);

    if (url.pathname === "/__journal/login") {
      return journalLogin(request, env);
    }
    if (url.pathname === "/__journal/logout") {
      return journalLogout();
    }

    if (isJournalPath(url.pathname)) {
      if (!env.JOURNAL_PASSPHRASE) {
        return html(
          loginPage("This deployment has no journal passphrase configured."),
          503,
        );
      }
      if (!(await hasSession(request, env))) {
        if (url.pathname.startsWith(`${API_PREFIX}/`)) {
          return json({ error: "journal locked" }, 401);
        }
        return html(loginPage(""), 401);
      }
    }

    if (
      url.pathname === API_PREFIX ||
      url.pathname.startsWith(`${API_PREFIX}/`)
    ) {
      return proxyToApi(request, env, url);
    }

    const response = await env.ASSETS.fetch(request);
    if (response.status !== 404) return response;

    const notFound = await env.ASSETS.fetch(
      new URL("/404.html", url).toString(),
    );
    if (!notFound.ok) return response;

    return new Response(notFound.body, {
      status: 404,
      headers: notFound.headers,
    });
  },
};

/* --------------------------------------------------------------- journal */

function isJournalPath(pathname) {
  return (
    pathname === "/my-bets" ||
    pathname === "/my-bets/" ||
    pathname.startsWith("/my-bets/") ||
    pathname === "/svc/api/v1/my-bets"
  );
}

async function journalLogin(request, env) {
  if (request.method !== "POST") {
    return html(loginPage(""), 405);
  }
  if (!env.JOURNAL_PASSPHRASE) {
    return html(
      loginPage("This deployment has no journal passphrase configured."),
      503,
    );
  }

  let supplied = "";
  try {
    const form = await request.formData();
    supplied = String(form.get("password") ?? "");
  } catch {
    supplied = "";
  }

  if (!(await secretEquals(supplied, env.JOURNAL_PASSPHRASE))) {
    return html(loginPage("Wrong passphrase. Try again."), 401);
  }

  const token = await issueSession(env);
  return new Response(null, {
    status: 303,
    headers: {
      Location: "/my-bets/",
      "Set-Cookie": `${SESSION_COOKIE}=${token}; Path=/; HttpOnly; Secure; SameSite=Lax; Max-Age=${SESSION_TTL_MS / 1000}`,
      "Cache-Control": "no-store",
    },
  });
}

function journalLogout() {
  return new Response(null, {
    status: 303,
    headers: {
      Location: "/",
      "Set-Cookie": `${SESSION_COOKIE}=; Path=/; HttpOnly; Secure; SameSite=Lax; Max-Age=0`,
      "Cache-Control": "no-store",
    },
  });
}

async function hasSession(request, env) {
  const raw = cookieValue(request.headers.get("cookie"), SESSION_COOKIE);
  if (!raw) return false;

  const dot = raw.indexOf(".");
  if (dot <= 0) return false;

  const expires = Number(raw.slice(0, dot));
  const signature = raw.slice(dot + 1);
  if (!Number.isFinite(expires) || expires < Date.now()) return false;

  return secretEquals(signature, await sign(env, expires));
}

async function issueSession(env) {
  const expires = Date.now() + SESSION_TTL_MS;
  return `${expires}.${await sign(env, expires)}`;
}

async function sign(env, expires) {
  const secret = env.JOURNAL_SESSION_SECRET || env.JOURNAL_PASSPHRASE || "";
  const key = await crypto.subtle.importKey(
    "raw",
    new TextEncoder().encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const signature = await crypto.subtle.sign(
    "HMAC",
    key,
    new TextEncoder().encode(`journal:${expires}`),
  );
  return base64url(new Uint8Array(signature));
}

function cookieValue(header, name) {
  if (!header) return null;
  for (const part of header.split(";")) {
    const [key, ...rest] = part.trim().split("=");
    if (key === name) return rest.join("=");
  }
  return null;
}

async function secretEquals(a, b) {
  const [left, right] = await Promise.all([digest(a), digest(b)]);
  let difference = 0;
  for (let index = 0; index < left.length; index += 1) {
    difference |= left[index] ^ right[index];
  }
  return difference === 0;
}

async function digest(value) {
  return new Uint8Array(
    await crypto.subtle.digest("SHA-256", new TextEncoder().encode(value)),
  );
}

function base64url(bytes) {
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary)
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
}

/* ----------------------------------------------------------------- proxy */

/**
 * Forward a read-only request to the Go API and stream the response back.
 * Only GET and HEAD are allowed: every API endpoint the site uses is a read.
 */
async function proxyToApi(request, env, url) {
  const origin = (env.API_ORIGIN ?? "").trim().replace(/\/+$/, "");
  if (!origin) {
    return json(
      { error: "API_ORIGIN is not configured for this deployment" },
      503,
    );
  }
  if (request.method !== "GET" && request.method !== "HEAD") {
    return json({ error: "method not allowed" }, 405);
  }

  const target = new URL(
    `${url.pathname.slice(API_PREFIX.length)}${url.search}`,
    origin,
  );

  const headers = new Headers(request.headers);
  for (const name of [
    "authorization",
    "host",
    "cf-connecting-ip",
    "cf-connecting-ipv6",
    "cf-ipcountry",
    "cf-ray",
    "cf-visitor",
    "x-forwarded-proto",
    "x-real-ip",
  ]) {
    headers.delete(name);
  }
  headers.set("x-forwarded-host", url.host);

  // The tunnel is public, so the API itself refuses journal reads without the
  // shared token. Only this proxy adds it.
  if (
    url.pathname === "/svc/api/v1/my-bets" &&
    env.JOURNAL_TOKEN
  ) {
    headers.set("authorization", `Bearer ${env.JOURNAL_TOKEN}`);
  }

  let upstream;
  try {
    upstream = await fetch(target, {
      method: request.method,
      headers,
      redirect: "manual",
    });
  } catch (error) {
    return json(
      {
        error: "upstream unreachable",
        detail: error instanceof Error ? error.message : String(error),
      },
      502,
    );
  }

  const responseHeaders = new Headers(upstream.headers);
  // The runtime already decoded the body, so the upstream encoding headers
  // no longer describe it. The API sets no-store on JSON; media keeps its own
  // cache headers so crests are not re-fetched on every render.
  responseHeaders.delete("content-encoding");
  responseHeaders.delete("content-length");
  if (!responseHeaders.has("cache-control")) {
    responseHeaders.set("cache-control", "no-store");
  }

  return new Response(upstream.body, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers: responseHeaders,
  });
}

function json(body, status) {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      "content-type": "application/json; charset=utf-8",
      "cache-control": "no-store",
    },
  });
}

function html(body, status) {
  return new Response(body, {
    status,
    headers: {
      "content-type": "text/html; charset=utf-8",
      "cache-control": "no-store",
      "referrer-policy": "no-referrer",
    },
  });
}

/* ------------------------------------------------------------ login page */

function loginPage(error) {
  return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex">
<title>Journal locked · OddsVault</title>
<style>
:root{color-scheme:dark;--bg:#05070d;--panel:#090d16;--text:#e9eff9;--muted:#93a1b5;--line:rgba(255,255,255,.1);--accent:#10b981;--accent-ink:#03231a}
html.light{color-scheme:light;--bg:#f6f8fc;--panel:#fff;--text:#0f172a;--muted:#475569;--line:rgba(15,23,42,.14);--accent:#10b981;--accent-ink:#03231a}
*{box-sizing:border-box}
body{margin:0;min-height:100dvh;display:grid;place-items:center;background:var(--bg);color:var(--text);font:14px/1.55 ui-sans-serif,system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;padding:24px}
.card{width:100%;max-width:380px;background:var(--panel);border:1px solid var(--line);border-radius:16px;padding:26px}
.brand{display:inline-flex;align-items:center;gap:9px;color:inherit;text-decoration:none;font-weight:600;letter-spacing:-.01em}
.brand img{width:30px;height:30px;display:block}
h1{font-size:18px;margin:18px 0 6px}
p{margin:0 0 18px;color:var(--muted);font-size:12.5px}
label{display:block;font:11px ui-monospace,monospace;letter-spacing:.14em;text-transform:uppercase;color:var(--muted);margin-bottom:6px}
input{width:100%;padding:10px 12px;border-radius:10px;border:1px solid var(--line);background:transparent;color:var(--text);font:inherit;outline:none}
input:focus{border-color:var(--accent)}
button{margin-top:14px;width:100%;padding:10px 12px;border:0;border-radius:10px;background:var(--accent);color:var(--accent-ink);font-weight:600;font-size:13px;cursor:pointer}
.error{margin:0 0 12px;padding:8px 10px;border:1px solid rgba(239,68,68,.4);border-radius:9px;color:#f87171;font-size:12px}
.back{display:block;margin-top:16px;text-align:center;font:10px ui-monospace,monospace;letter-spacing:.14em;text-transform:uppercase;color:var(--muted);text-decoration:none}
.back:hover{color:var(--text)}
</style>
</head>
<body>
<main class="card">
<a class="brand" href="/">
<img src="/brand/logo-64.png" width="30" height="30" alt="">
<span>Odds<span style="color:var(--accent)">Vault</span></span>
</a>
<h1>Journal locked</h1>
<p>The personal bet journal is private on this deployment. Enter the passphrase to read it.</p>
${error ? `<p class="error">${error}</p>` : ""}
<form method="post" action="/__journal/login">
<label for="password">Passphrase</label>
<input id="password" name="password" type="password" autocomplete="current-password" required autofocus>
<button type="submit">Unlock journal</button>
</form>
<a class="back" href="/">← back to the site</a>
</main>
<script>(function(){try{var s=localStorage.getItem("oddsvault-theme");var m=(s==="light"||s==="dark")?s:"system";var d=m==="dark"||(m==="system"&&matchMedia("(prefers-color-scheme: dark)").matches);document.documentElement.classList.toggle("light",!d);document.documentElement.style.colorScheme=d?"dark":"light";}catch(e){}})();</script>
</body>
</html>`;
}
