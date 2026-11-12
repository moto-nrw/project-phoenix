// Elternkonto aus dem Link der Ferienanmeldung (#6673, ADR 0216). Der Link
// ist die Berechtigung; er gilt 30 Tage und nur für diese Seite.

import {
  apiErrorFromBody,
  transportFetch,
  type ApiError,
} from "~/lib/api-error";

export interface HolidayParentSetup {
  /** The address the link went to; the account's login. */
  readonly email: string;
  readonly traegerName: string;
  /** The address already has an account: no password is set here. */
  readonly accountExists: boolean;
}

interface BackendSetup {
  email: string;
  traeger_name: string;
  account_exists: boolean;
}

function setupPath(token: string): string {
  return `/api/holiday-parent-setups/${encodeURIComponent(token)}`;
}

async function readError(
  response: Response,
  fallback: string,
): Promise<ApiError> {
  const body: unknown = await response.json().catch(() => null);
  return apiErrorFromBody(fallback, response.status, body);
}

function unwrap<T>(body: unknown): T {
  return (
    body && typeof body === "object" && "data" in body
      ? (body as { data: T }).data
      : body
  ) as T;
}

export async function fetchHolidayParentSetup(
  token: string,
): Promise<HolidayParentSetup> {
  const response = await transportFetch(setupPath(token), {
    cache: "no-store",
  });
  if (!response.ok) {
    throw await readError(response, "Elternkonto konnte nicht geladen werden");
  }
  const data = unwrap<BackendSetup>(await response.json());
  return {
    email: data.email,
    traegerName: data.traeger_name,
    accountExists: data.account_exists,
  };
}

/**
 * Sets the password of a new account, or, without one, only adds the
 * access at the Träger to the account that exists. Answers whether the
 * account existed.
 */
export async function completeHolidayParentSetup(
  token: string,
  passwords?: { password: string; confirmPassword: string },
): Promise<{ accountExists: boolean }> {
  const response = await transportFetch(setupPath(token), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(
      passwords
        ? {
            password: passwords.password,
            confirm_password: passwords.confirmPassword,
          }
        : {},
    ),
  });
  if (!response.ok) {
    throw await readError(
      response,
      "Elternkonto konnte nicht eingerichtet werden",
    );
  }
  const data = unwrap<{ account_exists?: boolean } | null>(
    await response.json().catch(() => null),
  );
  return { accountExists: data?.account_exists === true };
}
