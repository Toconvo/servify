export interface Env {
  ASSETS: Fetcher;
}

/**
 * Minimal Worker that serves static assets from apps/website using Wrangler assets binding.
 * - Clean URLs supported by default
 * - For SPA-style routes (no extension), we can fallback to index.html
 */
export default {
  async fetch(request, env, ctx): Promise<Response> {
    // Try to serve static asset
    const url = new URL(request.url);

    // Health checks
    if (url.pathname === "/.well-known/healthz" || url.pathname === "/healthz") {
      return new Response("ok", {
        status: 200,
        headers: {
          "Content-Type": "text/plain; charset=utf-8",
          "Cache-Control": "no-cache",
        },
      });
    }

    // Dynamic sitemap using the current host
    if (url.pathname === "/sitemap.xml") {
      const base = `${url.origin}`;
      const sitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>${base}/</loc><changefreq>weekly</changefreq><priority>1.0</priority></url>
  <url><loc>${base}/sdk-demo.html</loc><changefreq>weekly</changefreq><priority>0.8</priority></url>
  <url><loc>${base}/ws-demo.html</loc><changefreq>weekly</changefreq><priority>0.8</priority></url>
</urlset>`;
      return new Response(sitemap, {
        status: 200,
        headers: {
          "Content-Type": "application/xml; charset=utf-8",
          "Cache-Control": "no-cache",
        },
      });
    }

    let res = await env.ASSETS.fetch(request);

    // If not found and looks like a SPA route (no dot/extension), fallback to index.html
    if (res.status === 404 && !url.pathname.split('/').pop()?.includes('.')) {
      const indexUrl = new URL(request.url);
      indexUrl.pathname = "/index.html";
      res = await env.ASSETS.fetch(new Request(indexUrl.toString(), request));
    }
    // Add simple caching strategy:
    // - assets/* : long cache (1 year), immutable
    // - html     : no-cache (allow instant updates)
    const newHeaders = new Headers(res.headers);
    newHeaders.set("X-Served-By", "servify-website-worker");
    const isAsset = url.pathname.startsWith("/assets/");
    const isHTML = url.pathname.endsWith(".html") || url.pathname === "/" || url.pathname === "/index.html";
    if (isAsset) {
      newHeaders.set("Cache-Control", "public, max-age=31536000, immutable");
    } else if (isHTML) {
      newHeaders.set("Cache-Control", "no-cache, no-store, must-revalidate");
    }
    // Security headers (reasonable defaults; HSTS effective over HTTPS)
    newHeaders.set("X-Content-Type-Options", "nosniff");
    newHeaders.set("Referrer-Policy", "no-referrer-when-downgrade");
    newHeaders.set("Permissions-Policy", "camera=(), microphone=(), geolocation=()");
    // CSP: allow self assets; allow inline styles for this minimal site; images from self and data:
    const csp = [
      "default-src 'self'",
      "script-src 'self'",
      "style-src 'self' 'unsafe-inline'",
      "img-src 'self' data:",
      "connect-src 'self'",
      "frame-ancestors 'none'",
      "base-uri 'self'",
      "form-action 'self'"
    ].join("; ");
    newHeaders.set("Content-Security-Policy", csp);
    // Optional HSTS (uncomment if domain is HTTPS-only)
    // newHeaders.set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload");
    return new Response(res.body, { status: res.status, statusText: res.statusText, headers: newHeaders });
  },
} satisfies ExportedHandler<Env>;
