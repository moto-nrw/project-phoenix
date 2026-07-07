import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import { proxyPost } from "~/lib/route-proxy.server";
import type {
  BackendSubstitution,
  CreateSubstitutionRequest,
} from "~/lib/substitution-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "SubstitutionsRoute" });

/**
 * Type for paginated response from backend
 */
interface PaginatedSubstitutionResponse {
  status: string;
  data: BackendSubstitution[];
  pagination: {
    current_page: number;
    page_size: number;
    total_pages: number;
    total_records: number;
  };
  message?: string;
}

/**
 * Handler for GET /api/substitutions
 * Returns a list of substitutions, optionally filtered by query parameters
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    // Build URL with any query parameters
    const queryParams = new URLSearchParams();
    request.nextUrl.searchParams.forEach((value, key) => {
      queryParams.append(key, value);
    });

    const endpoint = `/api/substitutions${queryParams.toString() ? "?" + queryParams.toString() : ""}`;

    // Fetch substitutions from the API - backend returns paginated response
    const paginatedResponse = await apiGet<PaginatedSubstitutionResponse>(
      endpoint,
      token,
    );

    logger.debug("substitutions API response received");

    // Return just the data array
    return paginatedResponse.data || [];
  },
);

/**
 * Handler for POST /api/substitutions
 * Creates a new substitution
 */
export const POST = proxyPost<BackendSubstitution, CreateSubstitutionRequest>(
  "/api/substitutions",
  { raw: true },
);
