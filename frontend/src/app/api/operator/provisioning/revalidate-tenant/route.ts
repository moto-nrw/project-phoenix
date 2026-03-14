import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { revalidatePath } from "next/cache";
import { getOperatorToken } from "~/lib/operator/cookies";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "RevalidateTenantRoute" });

/**
 * POST /api/operator/provisioning/revalidate-tenant
 *
 * Purges the Next.js ISR cache for tenant route trees.
 * Called after subdomain/slug changes so the old address stops serving
 * stale cached pages immediately instead of waiting for the 5-min TTL.
 */
export async function POST(request: NextRequest) {
  const token = await getOperatorToken();
  if (!token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  try {
    const body = (await request.json()) as { slugs?: string[] };
    const slugs = body.slugs;

    if (!Array.isArray(slugs) || slugs.length === 0) {
      return NextResponse.json(
        { error: "slugs array is required" },
        { status: 400 },
      );
    }

    for (const slug of slugs) {
      if (typeof slug === "string" && slug.length > 0) {
        revalidatePath(`/${slug}`, "layout");
      }
    }

    logger.info("tenant_cache_revalidated", { slugs });

    return NextResponse.json({ status: "ok", revalidated: slugs });
  } catch (error) {
    logger.error("tenant_revalidation_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json({ error: "Revalidation failed" }, { status: 500 });
  }
}
