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
    return new Response(res.body, { status: res.status, statusText: res.statusText, headers: newHeaders });
  },
} satisfies ExportedHandler<Env>;
