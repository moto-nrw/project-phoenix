import { revalidatePath, revalidateTag } from "next/cache";
import { createOperatorPostHandler } from "~/lib/operator/route-wrapper";
import { isValidSlug } from "~/lib/operator/provisioning-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "RevalidateTenantRoute" });

export const POST = createOperatorPostHandler<
  { status: string; revalidated: string[] },
  { slugs?: string[] }
>(async (_request, body, _token, _params) => {
  const slugs = body.slugs;

  if (!Array.isArray(slugs) || slugs.length === 0) {
    throw new Error("API error (400): slugs array is required");
  }

  for (const slug of slugs) {
    if (typeof slug === "string" && slug.length > 0 && isValidSlug(slug)) {
      revalidateTag(`tenant-${slug}`, { expire: 0 });
      revalidatePath(`/${slug}`, "layout");
    }
  }

  logger.info("tenant_cache_revalidated", { slugs });

  return { status: "ok", revalidated: slugs };
});
