import "server-only";
import type { CredentialsSignin } from "next-auth";
import { getServerApiUrl } from "~/lib/server-api-url";
import { authLogger } from "./jwt-claims";
import {
  INVALID_CREDENTIALS,
  type PasswordLogin,
  type PortalDescriptor,
} from "./portals";
import type { TokenPair } from "./token-refresh";

const LOGIN_TIMEOUT_MS = 10_000;

export type LoginOutcome =
  | { ok: true; tokens: TokenPair }
  | { ok: false; status?: number; code?: string };

async function errorCode(response: Response): Promise<string | undefined> {
  try {
    const body = JSON.parse(await response.text()) as { code?: unknown };
    return typeof body.code === "string" ? body.code : undefined;
  } catch {
    return undefined; // a gateway error page carries no code
  }
}

/**
 * The password login of a portal's Credentials provider. Only status and
 * code leave this function; the backend body may name the account and is
 * never logged.
 */
export async function loginAtBackend(
  portal: PortalDescriptor,
  body: Record<string, string>,
  headers: Record<string, string>,
): Promise<LoginOutcome> {
  try {
    const response = await fetch(
      `${getServerApiUrl()}${portal.backend.authPrefix}/login`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json", ...headers },
        body: JSON.stringify(body),
        cache: "no-store",
        signal: AbortSignal.timeout(LOGIN_TIMEOUT_MS),
      },
    );
    if (!response.ok) {
      const code = await errorCode(response);
      authLogger.warn("portal_login_refused", {
        portal: portal.key,
        status: response.status,
        code,
      });
      return { ok: false, status: response.status, code };
    }
    const payload = (await response.json()) as Record<string, unknown>;
    const data = (
      portal.backend.enveloped.includes("login") ? payload.data : payload
    ) as Partial<TokenPair> & { status?: unknown };
    // An MFA-aware login answers 200 with the step it needs; the login page
    // drives that through lib/mfa-api, never this provider.
    if (
      (typeof data?.status === "string" && data.status !== "authenticated") ||
      typeof data?.access_token !== "string" ||
      !data.access_token
    ) {
      const code = typeof data?.status === "string" ? data.status : undefined;
      authLogger.warn("portal_login_refused", {
        portal: portal.key,
        status: response.status,
        code,
      });
      return { ok: false, status: response.status, code };
    }
    return {
      ok: true,
      tokens: {
        access_token: data.access_token,
        refresh_token: data.refresh_token ?? "",
      },
    };
  } catch (error) {
    authLogger.error("portal_login_unavailable", {
      portal: portal.key,
      error: error instanceof Error ? error.message : String(error),
    });
    return { ok: false };
  }
}

/**
 * The CredentialsSignin the login pages read by its `code`. Imported on use:
 * next-auth's entry pulls in next/server, which only resolves inside Next.
 */
export async function credentialsRefusal(
  code: string,
): Promise<CredentialsSignin> {
  const { CredentialsSignin } = await import("next-auth");
  const error = new CredentialsSignin();
  error.code = code;
  return error;
}

/** Null: the portal reports no reason, Auth.js answers its generic refusal. */
export async function refusalFor(
  login: PasswordLogin,
  outcome: Extract<LoginOutcome, { ok: false }>,
): Promise<CredentialsSignin | null> {
  if (login.refusals === "none") return null;
  if (outcome.status === 429) return credentialsRefusal("rate_limited");
  const byStatus =
    outcome.status === undefined
      ? undefined
      : login.refusalsByStatus?.[outcome.status];
  const byCode =
    outcome.code === undefined ? undefined : login.refusals[outcome.code];
  return credentialsRefusal(byStatus ?? byCode ?? INVALID_CREDENTIALS);
}
