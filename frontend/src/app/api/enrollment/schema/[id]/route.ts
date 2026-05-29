import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentSchemaByIdRoute" });

async function bearerHeader() {
  const session = await auth();
  const token = session?.user?.token;
  if (!token) return null;
  return `Bearer ${token}`;
}

async function resolveId(context: {
  params: Promise<Record<string, string | string[] | undefined>>;
}): Promise<string | null> {
  const params = await context.params;
  const raw = params.id;
  if (typeof raw !== "string" || raw === "") return null;
  return encodeURIComponent(raw);
}

export async function GET(
  _request: NextRequest,
  context: {
    params: Promise<Record<string, string | string[] | undefined>>;
  },
) {
  const id = await resolveId(context);
  if (!id) {
    return NextResponse.json({ error: "Invalid id" }, { status: 400 });
  }
  const authHeader = await bearerHeader();
  if (!authHeader) {
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }
  try {
    const response = await fetch(
      `${getServerApiUrl()}/api/enrollment/schema/${id}`,
      { headers: { Authorization: authHeader }, cache: "no-store" },
    );
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("schema_get_by_id_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export async function PUT(
  request: NextRequest,
  context: {
    params: Promise<Record<string, string | string[] | undefined>>;
  },
) {
  const id = await resolveId(context);
  if (!id) {
    return NextResponse.json({ error: "Invalid id" }, { status: 400 });
  }
  const authHeader = await bearerHeader();
  if (!authHeader) {
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }
  try {
    const body = (await request.json()) as {
      fields?: unknown;
      core_requirements?: unknown;
    };
    const response = await fetch(
      `${getServerApiUrl()}/api/enrollment/schema/${id}`,
      {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          Authorization: authHeader,
        },
        // Forward core_requirements only when present so the backend's
        // "absent = preserve existing" semantics still hold; sending it
        // makes the admin's guardian-phone-required toggle stick.
        body: JSON.stringify({
          fields: body.fields ?? [],
          ...(body.core_requirements !== undefined
            ? { core_requirements: body.core_requirements }
            : {}),
        }),
      },
    );
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("schema_put_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export async function DELETE(
  _request: NextRequest,
  context: {
    params: Promise<Record<string, string | string[] | undefined>>;
  },
) {
  const id = await resolveId(context);
  if (!id) {
    return NextResponse.json({ error: "Invalid id" }, { status: 400 });
  }
  const authHeader = await bearerHeader();
  if (!authHeader) {
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }
  try {
    const response = await fetch(
      `${getServerApiUrl()}/api/enrollment/schema/${id}`,
      {
        method: "DELETE",
        headers: { Authorization: authHeader },
      },
    );
    if (response.status === 204) {
      return new NextResponse(null, { status: 204 });
    }
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("schema_delete_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
