import type { NextRequest } from "next/server";
import { revalidatePath, revalidateTag } from "next/cache";
import {
  createOperatorPutHandler,
  createOperatorDeleteHandler,
  isStringParam,
  operatorApiGet,
  operatorApiPut,
  operatorApiDelete,
} from "~/lib/operator/route-wrapper";
import { TENANT_RESOLVE_AFFECTING_KEYS } from "~/lib/settings-keys";
import { createLogger } from "~/lib/logger";

const logger = createLogger({
  component: "OperatorSchoolSettingsValuesRoute",
});

const VALID_KEY_PATTERN = /^[a-z0-9_.]{1,255}$/;

interface SchoolSummary {
  id: number;
  slug: string;
}

interface SchoolSummariesEnvelope {
  data?: SchoolSummary[];
}

async function resolveSchoolSlug(
  schoolId: string,
  token: string,
): Promise<string | null> {
  try {
    const numericId = Number(schoolId);
    if (!Number.isFinite(numericId)) return null;
    // No dedicated single-school endpoint exists; the summaries list is the
    // cheapest server-side path that returns the slug for an operator. List
    // size is bounded (one entry per school in the platform) so a single
    // server-to-server call per write is acceptable for an admin action.
    const resp = await operatorApiGet<
      SchoolSummariesEnvelope | SchoolSummary[]
    >("/operator/schools/summaries", token);
    const summaries = Array.isArray(resp) ? resp : (resp.data ?? []);
    const match = summaries.find((s) => s.id === numericId);
    return match?.slug ?? null;
  } catch (error) {
    logger.warn("school_slug_lookup_failed", {
      school_id: schoolId,
      error: error instanceof Error ? error.message : String(error),
    });
    return null;
  }
}

async function revalidateTenantIfNeeded(
  schoolId: string,
  key: string,
  token: string,
): Promise<void> {
  if (!TENANT_RESOLVE_AFFECTING_KEYS.has(key)) return;

  const slug = await resolveSchoolSlug(schoolId, token);
  if (!slug) {
    logger.warn("tenant_revalidate_skipped_no_slug", {
      school_id: schoolId,
      key,
    });
    return;
  }

  revalidateTag(`tenant-${slug}`, { expire: 0 });
  revalidatePath(`/${slug}`, "layout");
  logger.info("tenant_cache_revalidated", { slug, key });
}

export const PUT = createOperatorPutHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    const key = params.key as string;
    if (!VALID_KEY_PATTERN.test(key)) {
      throw new Error("API error (400): Invalid settings key format");
    }
    const result = await operatorApiPut(
      `/operator/schools/${params.id}/settings/values/${key}`,
      token,
      body,
    );
    await revalidateTenantIfNeeded(params.id, key, token);
    return result;
  },
);

export const DELETE = createOperatorDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }
    const key = params.key as string;
    if (!VALID_KEY_PATTERN.test(key)) {
      throw new Error("API error (400): Invalid settings key format");
    }
    const result = await operatorApiDelete(
      `/operator/schools/${params.id}/settings/values/${key}`,
      token,
    );
    await revalidateTenantIfNeeded(params.id, key, token);
    return result;
  },
);
