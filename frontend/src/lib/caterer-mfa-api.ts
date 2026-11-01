// Client-side API of the caterer's two-factor sign-in (#4497) on the account
// page. It stays out of caterer-api so the account page does not load the
// order and delivery-list code. Every call goes through a Next.js route under
// /api/caterer/account/mfa, which forwards it with the caterer session.

import { authFetch } from "./api-helpers";

/** The caterer's two-factor sign-in as the account page shows it (#4497). */
export interface CatererMFAStatus {
  enabled: boolean;
  maskedEmail: string;
}

export async function fetchCatererMFAStatus(): Promise<CatererMFAStatus> {
  const data = await authFetch<{ enabled: boolean; masked_email?: string }>(
    "/api/caterer/account/mfa",
  );
  return { enabled: data.enabled, maskedEmail: data.masked_email ?? "" };
}

/** Mails the code that switches two-factor sign-in on. */
export async function startCatererMFA(): Promise<{
  challengeToken: string;
  maskedEmail: string;
}> {
  const data = await authFetch<{
    challenge_token: string;
    masked_email?: string;
  }>("/api/caterer/account/mfa/start", { method: "POST", body: {} });
  return {
    challengeToken: data.challenge_token,
    maskedEmail: data.masked_email ?? "",
  };
}

/**
 * Switches two-factor sign-in on with the mailed code. The backend answers
 * with a new session with the second factor proven, and the BFF installs it,
 * because the older sessions of the account lose access at their next
 * renewal. `sessionRenewed` is false when the code was right but the new
 * session could not be installed.
 */
export async function confirmCatererMFA(
  challengeToken: string,
  code: string,
): Promise<{ sessionRenewed: boolean }> {
  const data = await authFetch<{ session_renewed: boolean }>(
    "/api/caterer/account/mfa/confirm",
    {
      method: "POST",
      body: { challenge_token: challengeToken, code },
    },
  );
  return { sessionRenewed: data.session_renewed };
}

/** Switches two-factor sign-in off after checking the current password. */
export async function disableCatererMFA(
  currentPassword: string,
): Promise<void> {
  await authFetch<unknown>("/api/caterer/account/mfa/disable", {
    method: "POST",
    body: { currentPassword },
  });
}
