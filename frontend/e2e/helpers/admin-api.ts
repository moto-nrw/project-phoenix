import {
  request as apiRequest,
  type APIRequestContext,
} from "@playwright/test";
import { ADMIN, TENANT_SLUG } from "./seed-data";
import { BACKEND_URL } from "./iot";

/**
 * Logs in as the admin (demo1@mail.de) via the public auth endpoint and
 * returns a tenant-scoped access token. Use for tests that need to drive
 * the backend through admin permissions without going through the
 * NextAuth/storageState path.
 */
export async function loginAsAdmin(ctx: APIRequestContext): Promise<string> {
  const res = await ctx.post(`${BACKEND_URL}/auth/login`, {
    data: {
      email: ADMIN.email,
      password: ADMIN.password,
      tenant_slug: TENANT_SLUG,
    },
  });
  if (!res.ok()) {
    throw new Error(
      `admin login failed (${res.status()}): ${await res.text()}`,
    );
  }
  const body = (await res.json()) as {
    data?: { access_token?: string };
    access_token?: string;
  };
  const token = body.data?.access_token ?? body.access_token;
  if (!token) {
    throw new Error(`admin login returned no token: ${JSON.stringify(body)}`);
  }
  return token;
}

/**
 * Convenience wrapper: spins up a request context, logs in as admin,
 * runs the supplied function with `(ctx, headers)`, then disposes the
 * context. The headers include both `Authorization` and `Content-Type`,
 * ready for typical JSON POST/PUT/DELETE requests.
 */
export async function withAdminContext<T>(
  fn: (
    ctx: APIRequestContext,
    headers: { Authorization: string; "Content-Type": string },
  ) => Promise<T>,
): Promise<T> {
  const ctx = await apiRequest.newContext();
  try {
    const token = await loginAsAdmin(ctx);
    return await fn(ctx, {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    });
  } finally {
    await ctx.dispose();
  }
}
