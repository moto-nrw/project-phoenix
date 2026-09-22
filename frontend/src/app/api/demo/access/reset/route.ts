import { type NextRequest, NextResponse } from "next/server";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";
import { DEMO_TOKEN_COOKIE, demoBackendUrl } from "../forward";

const logger = createLogger({ component: "DemoResetRoute" });

const DEMO_ROLES = new Set(["caregiver", "lead", "parent", "all"]);

// Starts the visitor's demo over (#3470). The banner sends only the role it
// is in; the token comes from the cookie the entry left. The backend orders
// a fresh demo school for the same access and names the waiting room, where
// the setup screen of the first entry shows again. The role and the restart
// ride along in the fragment, so the entry lands in the same role, skips the
// role cards and reports a restart. Without a usable token the visitor is
// sent back to the mailed link, as a role switch does.
export async function POST(request: NextRequest) {
  const token = request.cookies.get(DEMO_TOKEN_COOKIE)?.value;
  if (!token) {
    return NextResponse.json(
      { error: "demo access is unknown" },
      { status: 404 },
    );
  }
  let body: { role?: unknown } = {};
  try {
    body = (await request.json()) as typeof body;
  } catch {
    // An unreadable body is a restart without a role.
  }
  const role =
    typeof body.role === "string" && DEMO_ROLES.has(body.role)
      ? body.role
      : undefined;
  try {
    const backend = await fetch(demoBackendUrl("reset"), {
      method: "POST",
      headers: {
        ...getClientForwardHeaders(request),
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ token }),
    });
    const payload = (await backend.json().catch(() => ({}))) as {
      entry_url?: unknown;
    };
    if (!backend.ok || typeof payload.entry_url !== "string") {
      return NextResponse.json(payload, { status: backend.status });
    }
    const fragment = new URLSearchParams({ restarted: "1" });
    if (role) fragment.set("role", role);
    return NextResponse.json(
      { entry_url: `${payload.entry_url}&${fragment.toString()}` },
      { status: backend.status, headers: { "Cache-Control": "no-store" } },
    );
  } catch (error) {
    logger.error("demo_reset_forward_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
