/**
 * The account a tab holds data for, sent with every request to its own BFF
 * (#5183). All tabs of a portal share one session cookie: an account that
 * signs in in tab B replaces the session of tab A at once, and tab A hears
 * of it only later. The BFF compares this header with the account of the
 * cookie and refuses a request that names another one
 * (`identity.session_account_mismatch`), so tab A neither shows data of the
 * new account nor acts as it.
 *
 * Set centrally: the fetch observer (session-tenant-mismatch-observer.ts)
 * and the Axios client (api-transport.ts) add it to requests for this
 * origin's `/api/`. A request without it passes: navigation, server
 * actions and image or download links cannot send one.
 */
export const SESSION_ACCOUNT_HEADER = "X-Moto-Session-Account";

let boundAccount: string | null = null;

/**
 * Binds this tab's requests to the account its SessionProvider shows; null
 * while it shows none. SessionAccountBinding calls it in every portal.
 */
export function bindSessionAccount(accountId: string | null): void {
  boundAccount = accountId;
}

/** The account this tab's requests name, or null when they name none. */
export function boundSessionAccount(): string | null {
  return boundAccount;
}

/** Whether the address is this tab's own BFF (same origin, under /api/). */
export function isOwnApiUrl(raw: string): boolean {
  if (typeof window === "undefined") return false;
  try {
    const url = new URL(raw, window.location.origin);
    return (
      url.origin === window.location.origin && url.pathname.startsWith("/api/")
    );
  } catch {
    // An address that cannot be read is not this tab's BFF.
    return false;
  }
}
