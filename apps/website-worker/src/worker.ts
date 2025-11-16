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
    return res;
  },
} satisfies ExportedHandler<Env>;

