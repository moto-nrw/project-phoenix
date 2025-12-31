import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers";
import { createGetHandler, createPostHandler } from "~/lib/route-wrapper";
import type {
  BackendSubstitution,
  CreateSubstitutionRequest,
} from "~/lib/substitution-helpers";

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

    // Log for debugging
    console.log("Substitutions API Response:", paginatedResponse);

    // Return just the data array
    return paginatedResponse.data || [];
  },
);

/**
 * Handler for POST /api/substitutions
 * Creates a new substitution
 */
export const POST = createPostHandler(
  async (req: NextRequest, body: CreateSubstitutionRequest, token: string) => {
    const endpoint = `/api/substitutions`;

    // Log the request payload for debugging
    console.log(
      "Creating substitution with payload:",
      JSON.stringify(body, null, 2),
    );

    // Create substitution via the API
    return await apiPost<BackendSubstitution>(endpoint, token, body);
  },
);
