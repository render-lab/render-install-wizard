// -----------------------------------------------------------------------------
// render.com App Router route handler for GET /agents.sh
//
// WHERE THIS GOES:
//   Copy this file into the render.com Next.js (App Router) frontend repo at:
//       app/agents.sh/route.ts
//   and copy the co-located `agents.sh` next to it (e.g. app/agents.sh/agents.sh),
//   adjusting the `readFileSync` path below to match wherever you vendor it.
//
// WHY THIS EXISTS:
//   render.com already serves /agents (HTML), /agents.md and /llms.txt (markdown)
//   via Accept-header content negotiation. The one missing piece is /agents.sh,
//   which currently 404s. This handler closes that gap so the documented
//   one-liner works:
//       curl -fsSL render.com/agents.sh | sh
//
// THE SCRIPT IS VENDORED, NOT AUTHORED HERE:
//   `agents.sh` is a byte-identical copy of scripts/agents.sh in
//   github.com/render-oss/render-install-wizard. Do NOT hand-edit it here.
//   The `sync-agents-sh.yml` CI job in that repo fails on any drift between the
//   source script and the vendored copy, so this endpoint always serves the
//   audited, checksummed bootstrap.
// -----------------------------------------------------------------------------

import { readFileSync } from "node:fs";
import { join } from "node:path";

// Read the vendored script once at module load (cold start), not per request.
// `process.cwd()` is the app root in a Next.js server; adjust the trailing path
// segments to wherever the vendored script lives in the frontend repo.
const SCRIPT = readFileSync(
  join(process.cwd(), "app", "agents.sh", "agents.sh"),
  "utf8",
);

// Cache at the edge/CDN (Cloudflare) for 5 minutes, and allow serving a slightly
// stale copy while revalidating so a deploy never causes a cache-miss stampede.
const CACHE_CONTROL = "public, max-age=300, s-maxage=300, stale-while-revalidate=86400";

// Force static-ish behavior: the body only changes on deploy, so let Next cache it.
export const dynamic = "force-static";
export const revalidate = 300;

export async function GET(): Promise<Response> {
  return new Response(SCRIPT, {
    status: 200,
    headers: {
      // text/x-shellscript so `curl | sh` and browsers treat it as a script.
      "Content-Type": "text/x-shellscript; charset=utf-8",
      "Cache-Control": CACHE_CONTROL,
      // Cloudflare honors CDN-Cache-Control independently of the browser header.
      "CDN-Cache-Control": CACHE_CONTROL,
      // The body is deterministic for a given deploy; advertise its length.
      "Content-Length": String(Buffer.byteLength(SCRIPT, "utf8")),
      // Defense-in-depth: never let a browser sniff this into HTML.
      "X-Content-Type-Options": "nosniff",
    },
  });
}
