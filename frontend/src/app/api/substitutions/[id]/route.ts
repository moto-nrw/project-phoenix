import type { NextRequest } from "next/server";
import { apiDelete } from "~/lib/api-helpers.server";
import { createDeleteHandler } from "~/lib/route-wrapper.server";
import { proxyGet, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import type { BackendSubstitution } from "~/lib/substitution-helpers";

/**
 * Handler for GET /api/substitutions/[id]
 * Returns a single substitution by ID
 */
export const GET = proxyGet<BackendSubstitution>(
  (params) => `/api/substitutions/${requirePathSegmentParam(params)}`,
  { raw: true },
);

/**
 * Handler for PUT /api/substitutions/[id]
 * Updates an existing substitution
 */
export const PUT = proxyPut<BackendSubstitution, Partial<BackendSubstitution>>(
  (params) => `/api/substitutions/${requirePathSegmentParam(params)}`,
  { raw: true },
);

/**
 * Handler for DELETE /api/substitutions/[id]
 * Deletes a substitution
 */
export const DELETE = createDeleteHandler(
  async (
    request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    if (!id) {
      throw new Error("Substitution ID is required");
    }

    const endpoint = `/api/substitutions/${id}`;

    // Delete substitution via the API
    await apiDelete(endpoint, token);

    // Return success response
    return { success: true };
  },
);
