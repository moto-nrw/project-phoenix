import { reportSessionTenantMismatch } from "./api-error";
import type { ErrorCode } from "./error-codes.generated";

const SESSION_TENANT_MISMATCH =
  "identity.session_tenant_mismatch" satisfies ErrorCode;

function isOwnApiRequest(input: RequestInfo | URL): boolean {
  const raw = input instanceof Request ? input.url : input.toString();
  try {
    const url = new URL(raw, window.location.origin);
    return (
      url.origin === window.location.origin && url.pathname.startsWith("/api/")
    );
  } catch {
    // An address the observer cannot read is not this tab's BFF; the
    // request itself must not fail because of the observer.
    return false;
  }
}

async function isRefusal(response: Response): Promise<boolean> {
  try {
    const body: unknown = await response.clone().json();
    return (
      typeof body === "object" &&
      body !== null &&
      (body as { code?: unknown }).code === SESSION_TENANT_MISMATCH
    );
  } catch {
    // Not an error envelope, so not the refusal.
    return false;
  }
}

let restoreFetch: (() => void) | null = null;

/**
 * Reports every refusal of this tab's BFF (#5130), whichever client sent the
 * request: many read only `response.ok` and never build an ApiError, so the
 * report in apiErrorFromBody misses them. Axios runs on XHR; its errors reach
 * apiErrorFromBody. The event stream cannot read a status. Installing twice
 * keeps one observer; returns the function that removes it.
 */
export function installSessionTenantMismatchObserver(): () => void {
  if (typeof window === "undefined") return () => {};
  if (restoreFetch) return restoreFetch;

  const originalFetch = window.fetch.bind(window);
  const observedFetch: typeof window.fetch = async (input, init) => {
    const response = await originalFetch(input, init);
    if (
      response.status === 409 &&
      isOwnApiRequest(input) &&
      (await isRefusal(response))
    ) {
      reportSessionTenantMismatch();
    }
    return response;
  };

  window.fetch = observedFetch;
  restoreFetch = () => {
    if (window.fetch === observedFetch) window.fetch = originalFetch;
    restoreFetch = null;
  };
  return restoreFetch;
}
