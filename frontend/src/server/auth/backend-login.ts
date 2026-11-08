import "server-only";
import type { CredentialsSignin } from "next-auth";
import { sendBackend } from "~/lib/backend-client.server";
import { authLogger } from "./jwt-claims";
import { LOGIN_RATE_LIMITED, type LoginRefusalCode } from "~/lib/login-refusal";
import type { PasswordLogin, PortalDescriptor } from "./portals";
import type { TokenPair } from "./token-refresh";

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
  request: Request | undefined,
): Promise<LoginOutcome> {
  try {
    const response = await sendBackend(
      `${portal.backend.authPrefix}/login`,
      null,
      {
        method: "POST",
        json: body,
        request,
        timeout: "login",
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
    // A login that answers 200 with a further step is no session here.
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
async function credentialsRefusal(
  code: LoginRefusalCode,
): Promise<CredentialsSignin> {
  const { CredentialsSignin } = await import("next-auth");
  const error = new CredentialsSignin();
  error.code = code;
  return error;
}

export async function refusalFor(
  login: PasswordLogin,
  outcome: Extract<LoginOutcome, { ok: false }>,
): Promise<CredentialsSignin> {
  if (outcome.status === 429) return credentialsRefusal(LOGIN_RATE_LIMITED);
  const named = login.refusals.find((code) => code === outcome.code);
  return credentialsRefusal(named ?? "identity.invalid_credentials");
}
