import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentSchemaRoute" });

async function bearerHeader() {
  const session = await auth();
  const token = session?.user?.token;
  if (!token) return null;
  return `Bearer ${token}`;
}

async function GETHandler(_request: NextRequest) {
  const authHeader = await bearerHeader();
  if (!authHeader) {
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }
  try {
    const response = await fetch(`${getServerApiUrl()}/api/enrollment/schema`, {
      headers: { Authorization: authHeader },
    });
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("enrollment_schema_get_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const GET = withTenantAuth(GETHandler);

async function POSTHandler(request: NextRequest) {
  const authHeader = await bearerHeader();
  if (!authHeader) {
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }
  try {
    const body = (await request.json()) as {
      name?: unknown;
      fields?: unknown;
      core_requirements?: unknown;
      legal_blocks?: unknown;
    };
    const response = await fetch(`${getServerApiUrl()}/api/enrollment/schema`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: authHeader,
      },
      body: JSON.stringify({
        name: typeof body.name === "string" ? body.name : "",
        fields: body.fields ?? [],
        // Forward core_requirements so per-template required-state for
        // built-in fields (e.g. guardian_phone) is persisted. Omitting it
        // here silently dropped the admin toggle.
        ...(body.core_requirements === undefined
          ? {}
          : { core_requirements: body.core_requirements }),
        // Same for legal_blocks: absent = backend keeps its defaults,
        // present = the template's consent blocks are persisted.
        ...(body.legal_blocks === undefined
          ? {}
          : { legal_blocks: body.legal_blocks }),
      }),
    });
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("enrollment_schema_post_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const POST = withTenantAuth(POSTHandler);
