import { handleAuthFailure } from "./auth-api";

interface FetchOptions extends RequestInit {
  retry?: boolean;
}

/**
 * Custom fetch wrapper that handles token refresh automatically
 * This is used for client-side fetch requests to Next.js API routes
 */
export async function fetchWithAuth(
  url: string,
  options: FetchOptions = {},
): Promise<Response> {
  const { retry = true, ...fetchOptions } = options;

  // Make the initial request
  const response = await fetch(url, fetchOptions);

  // If we get a 401 and haven't retried yet, attempt token refresh
  if (response.status === 401 && retry) {
    // Only attempt token refresh on the client side
    if (typeof window !== "undefined") {
      try {
        // Try to refresh the token and update the session
        const refreshSuccessful = await handleAuthFailure();

        if (refreshSuccessful) {
          // Retry the request with retry=false to prevent infinite loops
          return fetchWithAuth(url, { ...fetchOptions, retry: false });
        }
      } catch (error) {
        console.error("Token refresh failed:", error);
      }
    }
  }

  return response;
}
