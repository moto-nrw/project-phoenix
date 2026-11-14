// Client-side API of a second factor a person switches on in their own
// account: the caterer's (#4497) and the parents' (#7051). Every call goes
// through a Next.js route under the portal's base path, which forwards it
// with the portal's session.

import { authFetch } from "./api-helpers";

/** The second factor as the account settings show it. */
export interface EmailCodeFactorStatus {
  enabled: boolean;
  /** False when the factor cannot be switched on for this account. */
  available: boolean;
  maskedEmail: string;
}

export interface EmailCodeFactorApi {
  status(): Promise<EmailCodeFactorStatus>;
  /** Mails the code that switches the factor on. */
  start(): Promise<{ challengeToken: string; maskedEmail: string }>;
  /**
   * Switches the factor on with the mailed code. The backend answers with a
   * new session with the factor proven, which the BFF installs, because the
   * older sessions of the account lose access at their next renewal.
   * `sessionRenewed` is false when the code was right but the new session
   * could not be installed.
   */
  confirm(
    challengeToken: string,
    code: string,
  ): Promise<{ sessionRenewed: boolean }>;
  /** Switches the factor off after checking the current password. */
  disable(currentPassword: string): Promise<void>;
}

/** The factor's routes under `base` (`/api/caterer/account/mfa`, …). */
export function emailCodeFactorApi(base: string): EmailCodeFactorApi {
  return {
    async status() {
      const data = await authFetch<{
        enabled: boolean;
        available?: boolean;
        masked_email?: string;
      }>(base);
      return {
        enabled: data.enabled,
        available: data.available ?? true,
        maskedEmail: data.masked_email ?? "",
      };
    },
    async start() {
      const data = await authFetch<{
        challenge_token: string;
        masked_email?: string;
      }>(`${base}/start`, { method: "POST", body: {} });
      return {
        challengeToken: data.challenge_token,
        maskedEmail: data.masked_email ?? "",
      };
    },
    async confirm(challengeToken, code) {
      const data = await authFetch<{ session_renewed: boolean }>(
        `${base}/confirm`,
        { method: "POST", body: { challenge_token: challengeToken, code } },
      );
      return { sessionRenewed: data.session_renewed };
    },
    async disable(currentPassword) {
      await authFetch<unknown>(`${base}/disable`, {
        method: "POST",
        body: { currentPassword },
      });
    },
  };
}
