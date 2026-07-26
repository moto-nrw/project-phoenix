import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";
import {
  getOriginalRequestHost,
  hostnameFromAuthority,
} from "~/lib/client-headers.server";

const logger = createLogger({ component: "EnrollmentFormBootstrapRoute" });

function configuredParentsHostname(): string {
  const authority = process.env.NEXT_PUBLIC_PARENTS_HOSTNAME;
  if (!authority) {
    throw new Error("NEXT_PUBLIC_PARENTS_HOSTNAME is not set.");
  }
  const hostname = hostnameFromAuthority(authority);
  if (!hostname) {
    throw new Error(
      "NEXT_PUBLIC_PARENTS_HOSTNAME is not a valid host authority.",
    );
  }
  return hostname;
}

interface RouteContext {
  params: Promise<{ tenantSlug: string; phaseId: string }>;
}

async function GETHandler(request: NextRequest, context: RouteContext) {
  const { tenantSlug, phaseId } = await context.params;
  // Runtime env validation normally rejects this during application startup.
  // Keep the route independently fail-closed when build tooling explicitly
  // skips env validation while collecting route metadata.
  const parentsHostname = configuredParentsHostname();
  try {
    const requestHostname = hostnameFromAuthority(
      getOriginalRequestHost(request),
    );
    if (!requestHostname) {
      return NextResponse.json(
        { error: "Invalid request host" },
        { status: 400 },
      );
    }
    const isParentPortal = requestHostname === parentsHostname;
    const lateInvite = request.nextUrl.searchParams.get("late_invite")?.trim();
    const backendUrl = new URL(
      `${getServerApiUrl()}/api/enrollment/form-bootstrap/public/${encodeURIComponent(
        tenantSlug,
      )}/${encodeURIComponent(phaseId)}`,
    );
    if (lateInvite) backendUrl.searchParams.set("late_invite", lateInvite);
    const response = await fetch(backendUrl, { cache: "no-store" });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      return NextResponse.json(payload, { status: response.status });
    }

    // The parent portal uses the same public metadata, but tenant cookies are
    // domain-scoped across tenant subdomains in production. The proxy-owned
    // original host is the security boundary: client-controlled query flags
    // and fetch credential options must never decide whether auth() runs.
    if (isParentPortal) {
      return NextResponse.json(payload, { status: response.status });
    }

    const session = await auth().catch((error: unknown) => {
      logger.warn("profile_auth_prefetch_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      return null;
    });
    const token = session?.user?.token;
    if (!token) {
      return NextResponse.json(payload, { status: response.status });
    }

    const profileResponse = await fetch(
      `${getServerApiUrl()}/api/enrollment/me/profile`,
      {
        headers: { Authorization: `Bearer ${token}` },
        cache: "no-store",
      },
    ).catch((error: unknown) => {
      logger.warn("profile_prefetch_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      return null;
    });
    if (!profileResponse?.ok) {
      return NextResponse.json(payload, { status: response.status });
    }

    const profilePayload = await profileResponse.json().catch(() => null);
    const data =
      payload && typeof payload === "object" && "data" in payload
        ? (payload.data as Record<string, unknown>)
        : (payload as Record<string, unknown>);
    return NextResponse.json(
      {
        ...payload,
        data: {
          ...data,
          profile:
            profilePayload &&
            typeof profilePayload === "object" &&
            "data" in profilePayload
              ? profilePayload.data
              : profilePayload,
        },
      },
      { status: response.status },
    );
  } catch (error) {
    logger.error("form_bootstrap_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

const authenticatedTenantGET = withTenantAuth(GETHandler);

/**
 * The parents host deliberately bypasses tenant Auth.js. Tenant cookies are
 * domain-scoped and may be sent to that host, but portal isolation forbids
 * reading or refreshing them there.
 */
export function GET(request: NextRequest, context: RouteContext) {
  const requestHostname = hostnameFromAuthority(
    getOriginalRequestHost(request),
  );
  if (!requestHostname || requestHostname === configuredParentsHostname()) {
    return GETHandler(request, context);
  }
  return authenticatedTenantGET(request, context);
}
