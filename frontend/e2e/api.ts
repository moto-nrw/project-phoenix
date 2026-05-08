import {
  request as apiRequest,
  type APIRequestContext,
} from "@playwright/test";
import { type TenantSession } from "./auth";
import { getBackendBaseURL, type E2EDevice } from "./state";

async function resolveAccessTokenFromSession(
  session: TenantSession,
): Promise<string> {
  // Browser auth belongs to auth.setup.ts + storageState. API fixtures
  // derive their backend Bearer token from that already-authenticated browser
  // session so the harness has one auth truth instead of separate UI/API login
  // flows that can drift.
  const frontendCtx = await apiRequest.newContext({
    baseURL: session.appRoot,
    storageState: session.storageStatePath,
  });

  try {
    const res = await frontendCtx.post("/api/auth/token");
    if (!res.ok()) {
      throw new Error(
        `session token lookup failed for ${session.email} (${res.status()}): ${await res.text()}`,
      );
    }

    const body = (await res.json()) as {
      access_token?: string;
    };

    if (!body.access_token) {
      throw new Error(
        `session token lookup returned no access token for ${session.email}`,
      );
    }

    return body.access_token;
  } finally {
    await frontendCtx.dispose();
  }
}

export async function createBackendApiContext(): Promise<APIRequestContext> {
  return apiRequest.newContext({
    baseURL: getBackendBaseURL(),
  });
}

export async function createTenantApiContext(
  session: TenantSession,
): Promise<APIRequestContext> {
  const token = await resolveAccessTokenFromSession(session);
  return apiRequest.newContext({
    baseURL: getBackendBaseURL(),
    extraHTTPHeaders: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
  });
}

export async function createDeviceApiContext(
  device: Pick<E2EDevice, "api_key" | "pin">,
): Promise<APIRequestContext> {
  return apiRequest.newContext({
    baseURL: getBackendBaseURL(),
    extraHTTPHeaders: {
      Authorization: `Bearer ${device.api_key}`,
      "X-Staff-PIN": device.pin,
      "Content-Type": "application/json",
    },
  });
}
