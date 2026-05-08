import { revalidateTag } from "next/cache";
import type { NextRequest } from "next/server";
import {
  createOperatorPutHandler,
  createOperatorDeleteHandler,
  isStringParam,
  operatorApiPut,
  operatorApiDelete,
} from "~/lib/operator/route-wrapper";
import { TENANT_RESOLVE_AFFECTING_KEYS } from "~/lib/settings-keys";

const VALID_KEY_PATTERN = /^[a-z0-9_.]{1,255}$/;

// SchoolSettingMutationResponse mirrors the photo-specific response payload in
// backend/api/operator/settings.go. The backend includes school_slug only for
// settings whose tenant-resolve payload must be revalidated immediately, which
// is currently limited to operations.student_photos_enabled.
interface SchoolSettingMutationResponse {
  school_slug?: string;
}

function maybeRevalidateTenant(
  key: string,
  response: SchoolSettingMutationResponse | null | undefined,
): void {
  if (!TENANT_RESOLVE_AFFECTING_KEYS.has(key)) return;
  const slug = response?.school_slug;
  if (!slug) return;
  // `expire: 0` forces immediate invalidation — same pattern as the
  // tenant-side /api/settings/values/[key] proxy.
  revalidateTag(`tenant-${slug}`, { expire: 0 });
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
    const result = await operatorApiPut<SchoolSettingMutationResponse>(
      `/operator/schools/${params.id}/settings/values/${key}`,
      token,
      body,
    );
    maybeRevalidateTenant(key, result);
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
    const result = await operatorApiDelete<SchoolSettingMutationResponse>(
      `/operator/schools/${params.id}/settings/values/${key}`,
      token,
    );
    maybeRevalidateTenant(key, result);
    return result;
  },
);
