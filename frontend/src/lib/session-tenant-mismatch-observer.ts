import { isSessionRefusalCode, reportSessionTenantMismatch } from "./api-error";
import type { ErrorCode } from "./error-codes.generated";
import {
  boundSessionAccount,
  isOwnApiUrl,
  SESSION_ACCOUNT_HEADER,
} from "./session-account";

function requestUrl(input: RequestInfo | URL): string {
  return input instanceof Request ? input.url : input.toString();
}

/** The refusal's code, or null for any other answer. */
async function refusalCode(response: Response): Promise<ErrorCode | null> {
  try {
    const body: unknown = await response.clone().json();
    const code =
      typeof body === "object" && body !== null
        ? (body as { code?: unknown }).code
        : undefined;
    return isSessionRefusalCode(code) ? code : null;
  } catch {
    // Not an error envelope, so not the refusal.
    return null;
  }
}

/**
 * The init that names this tab's account (#5183). The request's own headers
 * stay; a header the caller set itself wins.
 */
function withSessionAccount(
  input: RequestInfo | URL,
  init: RequestInit | undefined,
  accountId: string,
): RequestInit {
  const headers = new Headers(
    init?.headers ?? (input instanceof Request ? input.headers : undefined),
  );
  if (!headers.has(SESSION_ACCOUNT_HEADER)) {
    headers.set(SESSION_ACCOUNT_HEADER, accountId);
  }
  return { ...init, headers };
}

let restoreFetch: (() => void) | null = null;

/**
 * Names this tab's account on every fetch to its own BFF (#5183) and reports
 * every refusal of it, whichever client sent the request: the session of
 * another school than this subdomain (#5130) or of another account than this
 * tab. Many clients read only `response.ok` and never build an ApiError, so
 * the report in apiErrorFromBody misses them. Axios runs on XHR; its
 * interceptor adds the header and its errors reach apiErrorFromBody.
 * SSE also uses fetch, so this observes the actual stream refusal (#5374).
 * Installing twice keeps one observer; returns the function that removes it.
 */
export function installSessionTenantMismatchObserver(): () => void {
  if (typeof window === "undefined") return () => {};
  if (restoreFetch) return restoreFetch;

  const originalFetch = window.fetch.bind(window);
  const observedFetch: typeof window.fetch = async (input, init) => {
    const ownApi = isOwnApiUrl(requestUrl(input));
    const accountId = ownApi ? boundSessionAccount() : null;
    const response = await originalFetch(
      input,
      accountId === null ? init : withSessionAccount(input, init, accountId),
    );
    const refusal =
      response.status === 409 && ownApi ? await refusalCode(response) : null;
    if (refusal) reportSessionTenantMismatch(refusal);
    return response;
  };

  window.fetch = observedFetch;
  restoreFetch = () => {
    if (window.fetch === observedFetch) window.fetch = originalFetch;
    restoreFetch = null;
  };
  return restoreFetch;
}
