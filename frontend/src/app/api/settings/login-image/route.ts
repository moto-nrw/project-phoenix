import { revalidateTag } from "next/cache";
import type { NextRequest } from "next/server";
import { createFileUploadHandler } from "~/lib/file-upload-wrapper";
import { createDeleteHandler, createGetHandler } from "~/lib/route-wrapper";
import { createLogger } from "~/lib/logger";
import { uncachedAuth } from "~/server/auth";

const logger = createLogger({ component: "LoginImageRoute" });

interface LoginImageResponse {
  login_image_url: string;
}

/** Extract tenant slug from the request to revalidate ISR cache.
 *  Tries subdomain first (host header), then falls back to path prefix (referer).
 *
 *  Subdomain detection compares against NEXT_PUBLIC_TENANT_DOMAIN so that dotted
 *  base domains (e.g. moto-app.de) are not mistakenly split at the first dot. */
function revalidateTenantCache(request: NextRequest) {
  const host = request.headers.get("host") ?? "";
  const hostname = host.split(":")[0] ?? "";
  const baseDomain = process.env.NEXT_PUBLIC_TENANT_DOMAIN;

  // Try subdomain: tenant.moto-app.de → "tenant"
  // Only match when the hostname ends with ".{baseDomain}" so that
  // bare-domain requests (moto-app.de) fall through to the referer path.
  // NEXT_PUBLIC_TENANT_DOMAIN is deploy-only (not set in local dev) — when
  // missing, subdomain detection is skipped and path-based fallback is used.
  if (baseDomain && hostname.endsWith(`.${baseDomain}`)) {
    const slug = hostname.slice(0, -(baseDomain.length + 1));
    if (slug && slug !== "www") {
      revalidateTag(`tenant-${slug}`, { expire: 0 });
      return;
    }
  }

  // Fallback: extract slug from referer path (e.g. /school-a/settings)
  const referer = request.headers.get("referer") ?? "";
  try {
    const refUrl = new URL(referer);
    const pathSlug = refUrl.pathname.split("/")[1];
    if (pathSlug) {
      revalidateTag(`tenant-${pathSlug}`, { expire: 0 });
      return;
    }
  } catch {
    // Invalid referer URL — fall through to warning
  }

  // In subdomain mode (baseDomain is set but slug extraction failed above),
  // this means the host didn't match — likely misconfiguration. In path mode
  // (baseDomain unset), reaching here means the referer had no usable slug.
  logger.error("tenant_cache_revalidation_failed", {
    host,
    referer: referer || "(empty)",
    base_domain_configured: !!baseDomain,
  });
}

/** Fetch with 401 retry — refreshes the NextAuth session token once on backend 401. */
async function fetchWithRetry(
  token: string,
  doFetch: (authToken: string) => Promise<Response>,
): Promise<Response> {
  let response = await doFetch(token);

  if (response.status === 401) {
    const refreshed = await uncachedAuth();
    if (refreshed?.user?.token && refreshed.user.token !== token) {
      response = await doFetch(refreshed.user.token);
    }
  }

  return response;
}

// POST handler for login image upload
export const POST = createFileUploadHandler<LoginImageResponse>(
  async (request: NextRequest, formData: FormData, token: string) => {
    const file = formData.get("login_image");
    if (!file || !(file instanceof File)) {
      throw new Error("No image file provided");
    }

    const validatedFormData = new FormData();
    validatedFormData.append("login_image", file);

    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const url = `${getServerApiUrl()}/api/settings/login-image`;
    const response = await fetchWithRetry(token, (t) =>
      fetch(url, {
        method: "POST",
        headers: { Authorization: `Bearer ${t}` },
        body: validatedFormData,
      }),
    );

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(
        `API error (${response.status}): ${errorText || response.statusText}`,
      );
    }

    const data = ((await response.json()) as { data: LoginImageResponse }).data;

    revalidateTenantCache(request);

    return data;
  },
  {
    maxSizeInMB: 2,
    allowedMimeTypes: ["image/jpeg", "image/jpg", "image/png", "image/webp"],
    allowedExtensions: [".jpg", ".jpeg", ".png", ".webp"],
  },
);

// DELETE handler for removing login image
export const DELETE = createDeleteHandler(
  async (request: NextRequest, token: string) => {
    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const url = `${getServerApiUrl()}/api/settings/login-image`;
    const response = await fetchWithRetry(token, (t) =>
      fetch(url, {
        method: "DELETE",
        headers: { Authorization: `Bearer ${t}` },
      }),
    );

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(
        `API error (${response.status}): ${errorText || response.statusText}`,
      );
    }

    revalidateTenantCache(request);

    return null;
  },
);

// GET handler for fetching current login image URL
export const GET = createGetHandler(
  async (_request: NextRequest, token: string) => {
    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const url = `${getServerApiUrl()}/api/settings/login-image`;
    const response = await fetchWithRetry(token, (t) =>
      fetch(url, {
        method: "GET",
        headers: {
          Authorization: `Bearer ${t}`,
          "Content-Type": "application/json",
        },
      }),
    );

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(
        `API error (${response.status}): ${errorText || response.statusText}`,
      );
    }

    const responseData = (await response.json()) as {
      data: { login_image_url: string | null; can_edit: boolean };
    };
    return responseData.data;
  },
);
