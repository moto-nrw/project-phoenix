import { type NextRequest, NextResponse } from "next/server";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";
import { DEMO_TOKEN_COOKIE, demoBackendUrl } from "../forward";

const logger = createLogger({ component: "DemoHandoffRoute" });

const OGS_ROLES = new Set(["caregiver", "lead", "all"]);

// Switches the demo between the OGS app and the parents app (#3468). Both
// apps redeem the same token on their own host, so the switch is an
// ordinary navigation with the token: this route reads it from the cookie
// the entry left and redirects to the other app's entry page, the token in
// the fragment. No script of this page ever sees the token, and no server
// log records it. Without a usable token the visitor lands on this host's
// entry page, which offers a new link.
export async function GET(request: NextRequest) {
  const role = request.nextUrl.searchParams.get("role") ?? "";
  const token = request.cookies.get(DEMO_TOKEN_COOKIE)?.value;
  if (!token || (role !== "parent" && !OGS_ROLES.has(role))) {
    return redirect("/demo");
  }
  const fragment = new URLSearchParams({ token, role, switched: "1" });
  if (role === "parent") {
    const parentsHost = process.env.NEXT_PUBLIC_PARENTS_HOSTNAME;
    if (!parentsHost) {
      throw new Error("NEXT_PUBLIC_PARENTS_HOSTNAME is not set.");
    }
    return redirect(
      `${requestProtocol(request)}://${parentsHost}/demo#${fragment.toString()}`,
    );
  }
  const schoolUrl = await readySchoolUrl(request, token);
  return redirect(
    schoolUrl ? `${schoolUrl}/demo#${fragment.toString()}` : "/demo",
  );
}

/** The origin of the token's demo school, or null when it cannot be entered. */
async function readySchoolUrl(
  request: NextRequest,
  token: string,
): Promise<string | null> {
  try {
    const response = await fetch(demoBackendUrl("status"), {
      headers: {
        ...getClientForwardHeaders(request),
        Authorization: `Bearer ${token}`,
      },
    });
    if (!response.ok) return null;
    const body = (await response.json()) as {
      status?: string;
      school_url?: string;
    };
    return body.status === "ready" && body.school_url?.startsWith("http")
      ? body.school_url.replace(/\/+$/, "")
      : null;
  } catch (error) {
    logger.error("demo_handoff_status_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return null;
  }
}

function requestProtocol(request: NextRequest): string {
  const forwarded = request.headers
    .get("x-forwarded-proto")
    ?.split(",")[0]
    ?.trim();
  if (forwarded === "http" || forwarded === "https") return forwarded;
  return request.nextUrl.protocol.replace(/:$/, "");
}

function redirect(location: string): NextResponse {
  return new NextResponse(null, {
    status: 303,
    headers: {
      Location: location,
      "Cache-Control": "no-store",
      "Referrer-Policy": "no-referrer",
    },
  });
}
