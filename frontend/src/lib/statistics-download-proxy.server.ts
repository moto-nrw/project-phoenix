import type { NextRequest } from "next/server";
import { handleApiError } from "./api-response-error.server";
import { tenant } from "./route-wrapper.server";

/**
 * Query of a Statistik request: the single-valued keys plus every
 * group_id and school_class, which the backend reads as repeatable filters.
 */
export function statisticsQuery(
  request: NextRequest,
  keys: readonly string[],
): string {
  const params = new URLSearchParams();
  for (const key of keys) {
    const value = request.nextUrl.searchParams.get(key);
    if (value) params.set(key, value);
  }
  for (const key of ["group_id", "school_class"] as const) {
    for (const value of request.nextUrl.searchParams.getAll(key)) {
      if (value) params.append(key, value);
    }
  }
  const query = params.toString();
  return query ? `?${query}` : "";
}

/** Streams a Statistik export on the tenant session. */
export async function proxyStatisticsDownload(
  request: NextRequest,
  backendPath: string,
  keys: readonly string[],
): Promise<Response> {
  try {
    return await tenant.downloadFile(
      request,
      `${backendPath}${statisticsQuery(request, keys)}`,
    );
  } catch (error) {
    return handleApiError(error, request);
  }
}
